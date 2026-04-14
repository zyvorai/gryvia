// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// training_pattern.c - AI training communication pattern analysis.
//
// Detects distributed training patterns by correlating NCCL collective
// operations with TCP traffic for data loading.  Tracks the rank
// communication matrix (which ranks talk to which) and the compute-
// communication cycle timing to identify pipeline bubbles and
// synchronization inefficiencies.

#include "headers/gpu_common.h"
#include "headers/common.h"

/* ---- constants --------------------------------------------------------- */

#define MAX_INFLIGHT     65536
#define RINGBUF_SIZE     (256 * 1024)
#define MAX_RANK_PAIRS   (MAX_RANKS * MAX_RANKS)
#define CYCLE_SLOTS      4
#define TASK_COMM_LEN    16

/* ---- rank communication matrix key ------------------------------------ */

struct rank_pair {
    __u32 src_rank;
    __u32 dst_rank;
};

/* ---- rank communication stats ----------------------------------------- */

struct rank_comm_stat {
    __u64 bytes;
    __u64 count;
    __u64 last_ns;
};

/* ---- compute/communication cycle tracking ------------------------------ */

struct cycle_timing {
    __u64 last_compute_end_ns;
    __u64 last_comm_start_ns;
    __u64 total_compute_ns;
    __u64 total_comm_ns;
};

/* ---- BPF maps ---------------------------------------------------------- */

// Rank communication matrix: bytes exchanged between each (src, dst) pair.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_INFLIGHT);
    __type(key, struct rank_pair);
    __type(value, struct rank_comm_stat);
} rank_comm_matrix SEC(".maps");

// Per-CPU compute/communication cycle timing.
// Slots: 0=compute_time, 1=comm_time, 2=idle_time, 3=overlap_time
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, CYCLE_SLOTS);
    __type(key, __u32);
    __type(value, struct cycle_timing);
} compute_comm_cycle SEC(".maps");

// Ring buffer for training pattern events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, RINGBUF_SIZE);
} pattern_events SEC(".maps");

// In-flight NCCL operations for this program (entry/exit correlation).
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_INFLIGHT);
    __type(key, __u64);
    __type(value, struct nccl_inflight);
} pattern_nccl_inflight SEC(".maps");

/* ---- helpers ----------------------------------------------------------- */

static __always_inline void emit_pattern_event(__u8 event_type, __u8 nccl_op,
                                               __u64 bytes, __u64 latency_ns,
                                               __u32 src_rank, __u32 dst_rank)
{
    struct gpu_event *ev = bpf_ringbuf_reserve(&pattern_events,
                                               sizeof(struct gpu_event), 0);
    if (!ev)
        return;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    ev->timestamp     = bpf_ktime_get_ns();
    ev->pid           = pid_tgid >> 32;
    ev->gpu_id        = 0;
    ev->event_type    = event_type;
    ev->direction     = 0;
    ev->nccl_op       = nccl_op;
    ev->_pad          = 0;
    ev->bytes         = bytes;
    ev->latency_ns    = latency_ns;
    ev->src_rank      = src_rank;
    ev->dst_rank      = dst_rank;
    ev->collective_id = 0;
    ev->world_size    = 0;
    bpf_get_current_comm(&ev->comm, sizeof(ev->comm));

    bpf_ringbuf_submit(ev, 0);
}

static __always_inline void update_comm_start(void)
{
    __u32 key = 1;  /* comm_time slot */
    struct cycle_timing *ct = bpf_map_lookup_elem(&compute_comm_cycle, &key);
    if (ct)
        ct->last_comm_start_ns = bpf_ktime_get_ns();
}

/* ---- uprobes: ncclAllReduce (pattern detection focus) ------------------ */

// ncclAllReduce - shared hook with nccl_trace.c but this program focuses
// on detecting training patterns (compute/comm overlap, rank communication).
SEC("uprobe/ncclAllReduce")
int BPF_KPROBE(pattern_allreduce_entry, void *sendbuff, void *recvbuff,
               __u64 count)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct nccl_inflight info = {};
    info.start_ns = bpf_ktime_get_ns();
    info.op_type  = NCCL_ALLREDUCE;
    info.count    = (__u32)count;
    info.rank     = 0;

    bpf_map_update_elem(&pattern_nccl_inflight, &pid_tgid, &info, BPF_ANY);

    /* Mark communication phase start for compute/comm cycle tracking */
    update_comm_start();

    /* Update rank communication matrix for AllReduce (all-to-all pattern).
     * We record (rank 0 -> rank 0) as a sentinel; the userspace side
     * expands this based on world_size. */
    struct rank_pair rp = {};
    rp.src_rank = 0;
    rp.dst_rank = 0;

    struct rank_comm_stat *rcs = bpf_map_lookup_elem(&rank_comm_matrix, &rp);
    if (rcs) {
        __sync_fetch_and_add(&rcs->bytes, count);
        __sync_fetch_and_add(&rcs->count, 1);
        rcs->last_ns = bpf_ktime_get_ns();
    } else {
        struct rank_comm_stat new_rcs = {};
        new_rcs.bytes   = count;
        new_rcs.count   = 1;
        new_rcs.last_ns = bpf_ktime_get_ns();
        bpf_map_update_elem(&rank_comm_matrix, &rp, &new_rcs, BPF_ANY);
    }

    return 0;
}

/* ---- kprobes: tcp_sendmsg (data loading patterns) ---------------------- */

// tcp_sendmsg(struct sock *sk, struct msghdr *msg, size_t size)
//
// Track non-NCCL TCP communication to detect data loader activity
// (reading training data from network storage, parameter servers, etc.).
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(pattern_tcp_send, void *sk, void *msg, __u64 size)
{
    emit_pattern_event(GPU_EVT_MEM_TRANSFER, 0, size, 0, 0, 0);
    return 0;
}

/* ---- kprobes: tcp_recvmsg (data ingestion) ----------------------------- */

// tcp_recvmsg(struct sock *sk, struct msghdr *msg, size_t len, ...)
//
// Track data ingestion over TCP.  High volumes of tcp_recvmsg between
// NCCL collectives suggest data pipeline bottlenecks or prefetching.
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(pattern_tcp_recv, void *sk, void *msg, __u64 len)
{
    /* Update compute/comm cycle: TCP recv during training often
     * indicates data loading (the "compute" phase from the training
     * loop perspective). */
    __u32 key = 0;  /* compute_time slot */
    struct cycle_timing *ct = bpf_map_lookup_elem(&compute_comm_cycle, &key);
    if (ct)
        ct->last_compute_end_ns = bpf_ktime_get_ns();

    emit_pattern_event(GPU_EVT_MEM_TRANSFER, 0, len, 0, 0, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
