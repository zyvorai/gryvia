// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// straggler.c - per-rank NCCL collective skew.
//
// Hooks the same ncclAllReduce entry/exit as nccl_trace.c but does not emit
// gpu_event.  It keeps the fastest recent span per payload size and emits a
// fabric_signal when a span exceeds it by STRAGGLER_RATIO (2x) and an
// absolute floor (5 ms).  Observe-only: nothing is dropped or modified.
//
// Scope: only ranks running on THIS node are compared with each other (the
// comparison happens in the kernel of one host), and the span is the host-side
// duration of the ncclAllReduce call, which for asynchronous launches is the
// enqueue time, not the on-GPU time.
//
// Rank is read from the NCCL comm object when userspace populates
// nccl_comm_rank_off with the byte offset of the rank field (it is NCCL
// version specific and nothing populates it by default).  While the offset is
// 0 the rank stays 0 and userspace attributes by pid -> pod -> job rank.

#include "headers/gpu_common.h"
#include "headers/fabric_signal.h"

#define STRAGGLER_RATIO_X10      20                    /* 2.0x */
#define STRAGGLER_FLOOR_NS       5000000ULL            /* 5 ms */
#define STRAGGLER_BASELINE_TTL   30000000000ULL        /* 30 s: older "fastest" spans are stale */
#define MAX_INFLIGHT             65536
#define MAX_COLLECTIVE_SIZES     4096

// In-flight spans keyed by pid_tgid (a thread makes one NCCL call at a time).
// LRU so entries orphaned by a missed uretprobe are recycled.
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, MAX_INFLIGHT);
	__type(key, __u64); /* pid_tgid */
	__type(value, struct rank_span);
} straggler_inflight SEC(".maps");

// Fastest recent completed span per collective payload size in bytes.
// ncclComm_t values are per-process pointers and cannot identify a collective
// across ranks, so the payload size is the shared key.
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, MAX_COLLECTIVE_SIZES);
	__type(key, __u64); /* payload bytes */
	__type(value, struct rank_span);
} straggler_fastest SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, FABRIC_RINGBUF_SIZE);
} fabric_events SEC(".maps");

// Userspace writes the byte offset of the rank inside ncclComm; 0 = unknown.
struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, __u32);
} nccl_comm_rank_off SEC(".maps");

static __always_inline __u32 read_rank(void *comm)
{
	__u32 key = 0;
	__u32 *off = bpf_map_lookup_elem(&nccl_comm_rank_off, &key);
	__u32 rank = 0;

	if (!off || *off == 0 || !comm)
		return 0;
	if (bpf_probe_read_user(&rank, sizeof(rank), (char *)comm + *off))
		return 0;
	if (rank >= FABRIC_MAX_RANKS)
		return 0;
	return rank;
}

static __always_inline void emit_straggler(const struct rank_span *mine,
					   const struct rank_span *fast)
{
	struct fabric_signal *ev;

	ev = bpf_ringbuf_reserve(&fabric_events, sizeof(*ev), 0);
	if (!ev)
		return;

	__builtin_memset(ev, 0, sizeof(*ev));
	ev->timestamp_ns = mine->end_ns;
	ev->pid = bpf_get_current_pid_tgid() >> 32;
	ev->cgroup_id_lo = (__u32)bpf_get_current_cgroup_id();
	ev->signal_type = FABRIC_SIG_STRAGGLER;
	ev->nccl_op = NCCL_ALLREDUCE;
	ev->rank = mine->rank;
	ev->peer_rank = fast->rank;
	ev->latency_ns = mine->end_ns - mine->start_ns;
	ev->peer_latency_ns = fast->end_ns - fast->start_ns;
	ev->bytes = mine->bytes;
	bpf_get_current_comm(&ev->comm, sizeof(ev->comm));
	bpf_ringbuf_submit(ev, 0);
}

// ncclAllReduce(const void* sendbuff, void* recvbuff, size_t count,
//               ncclDataType_t datatype, ncclRedOp_t op, ncclComm_t comm,
//               cudaStream_t stream)
SEC("uprobe/ncclAllReduce")
int BPF_UPROBE(straggler_allreduce_entry, void *sendbuff, void *recvbuff,
	       __u64 count, __u64 datatype, __u64 op, void *comm)
{
	__u64 id = bpf_get_current_pid_tgid();
	struct rank_span span = {};

	span.start_ns = bpf_ktime_get_ns();
	span.rank = read_rank(comm);
	span.bytes = count * nccl_dtype_size(datatype);
	bpf_map_update_elem(&straggler_inflight, &id, &span, BPF_ANY);
	return 0;
}

SEC("uretprobe/ncclAllReduce")
int BPF_URETPROBE(straggler_allreduce_exit, int ret)
{
	__u64 id = bpf_get_current_pid_tgid();
	struct rank_span *span;
	struct rank_span done;
	struct rank_span *fast;
	__u64 mine_ns, fast_ns;

	span = bpf_map_lookup_elem(&straggler_inflight, &id);
	if (!span)
		return 0;
	done = *span;
	bpf_map_delete_elem(&straggler_inflight, &id);

	/* ncclResult_t != ncclSuccess: not a completed collective. */
	if (ret != 0)
		return 0;
	done.end_ns = bpf_ktime_get_ns();
	done.seen = 1;
	if (done.end_ns <= done.start_ns)
		return 0;
	mine_ns = done.end_ns - done.start_ns;

	fast = bpf_map_lookup_elem(&straggler_fastest, &done.bytes);
	if (!fast) {
		bpf_map_update_elem(&straggler_fastest, &done.bytes, &done, BPF_ANY);
		return 0;
	}
	fast_ns = fast->end_ns - fast->start_ns;
	/* New minimum, or the baseline is too old to trust: adopt this span. */
	if (fast_ns > mine_ns || done.end_ns - fast->end_ns > STRAGGLER_BASELINE_TTL) {
		bpf_map_update_elem(&straggler_fastest, &done.bytes, &done, BPF_ANY);
		return 0;
	}
	if (fast_ns > 0 && mine_ns > STRAGGLER_FLOOR_NS &&
	    mine_ns * 10 > fast_ns * STRAGGLER_RATIO_X10)
		emit_straggler(&done, fast);
	return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
