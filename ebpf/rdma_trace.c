// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// rdma_trace.c - RDMA/InfiniBand traffic analysis via kprobes and tracepoints.
//
// Hooks RDMA verbs layer functions (ib_post_send, ib_post_recv, ib_poll_cq)
// and RDMA tracepoints (rdma_create_qp, rdma_destroy_qp) to monitor
// InfiniBand traffic patterns used by GPU-direct RDMA and NCCL network
// transports.  Events are emitted to a ring buffer for the userspace
// collector.

#include "headers/gpu_common.h"

/* ---- constants --------------------------------------------------------- */

#define MAX_INFLIGHT   65536
#define MAX_QPS        8192
#define RINGBUF_SIZE   (256 * 1024)
#define RDMA_STAT_SLOTS 4    /* send_bytes, recv_bytes, send_ops, recv_ops */
#define TASK_COMM_LEN  16

/* ---- aggregate stats --------------------------------------------------- */

struct rdma_agg_stat {
    __u64 value;
};

/* ---- send inflight tracking -------------------------------------------- */

struct rdma_send_info {
    __u64 start_ns;
    __u64 bytes;
};

/* ---- BPF maps ---------------------------------------------------------- */

// Per-QP statistics keyed by QP number.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_QPS);
    __type(key, __u32);
    __type(value, struct rdma_qp_info);
} rdma_qp_map SEC(".maps");

// Ring buffer for RDMA events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, RINGBUF_SIZE);
} rdma_events SEC(".maps");

// In-flight send operations keyed by pid_tgid for latency tracking.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_INFLIGHT);
    __type(key, __u64);
    __type(value, struct rdma_send_info);
} rdma_send_inflight SEC(".maps");

// Per-CPU aggregate send/recv byte counters.
// Slots: 0=send_bytes, 1=recv_bytes, 2=send_ops, 3=recv_ops
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, RDMA_STAT_SLOTS);
    __type(key, __u32);
    __type(value, struct rdma_agg_stat);
} rdma_stats SEC(".maps");

/* ---- helpers ----------------------------------------------------------- */

static __always_inline void emit_rdma_event(__u8 event_type, __u64 bytes,
                                            __u64 latency_ns, __u32 qp_num)
{
    struct gpu_event *ev = bpf_ringbuf_reserve(&rdma_events,
                                               sizeof(struct gpu_event), 0);
    if (!ev)
        return;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    ev->timestamp     = bpf_ktime_get_ns();
    ev->pid           = pid_tgid >> 32;
    ev->gpu_id        = qp_num;  /* repurpose gpu_id for QP number */
    ev->event_type    = event_type;
    ev->direction     = 0;
    ev->nccl_op       = 0;
    ev->_pad          = 0;
    ev->bytes         = bytes;
    ev->latency_ns    = latency_ns;
    ev->src_rank      = 0;
    ev->dst_rank      = 0;
    ev->collective_id = 0;
    ev->world_size    = 0;
    bpf_get_current_comm(&ev->comm, sizeof(ev->comm));

    bpf_ringbuf_submit(ev, 0);
}

static __always_inline void update_rdma_stats(__u32 slot, __u64 value)
{
    if (slot >= RDMA_STAT_SLOTS)
        return;

    struct rdma_agg_stat *stat = bpf_map_lookup_elem(&rdma_stats, &slot);
    if (stat)
        __sync_fetch_and_add(&stat->value, value);
}

/* ---- kprobes: ib_post_send --------------------------------------------- */

// ib_post_send(struct ib_qp *qp, const struct ib_send_wr *send_wr,
//              const struct ib_send_wr **bad_send_wr)
//
// We capture the QP and record the send start time.  The work request
// size is not trivially accessible from the ib_send_wr without deep
// struct reading, so we record a placeholder and rely on completion
// polling for accurate byte counts.
SEC("kprobe/ib_post_send")
int BPF_KPROBE(rdma_post_send, void *qp, void *send_wr)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct rdma_send_info si = {};
    si.start_ns = bpf_ktime_get_ns();
    si.bytes    = 0;

    bpf_map_update_elem(&rdma_send_inflight, &pid_tgid, &si, BPF_ANY);

    /* Update send operation counter */
    update_rdma_stats(2, 1);  /* slot 2 = send_ops */

    emit_rdma_event(GPU_EVT_RDMA_SEND, 0, 0, 0);
    return 0;
}

/* ---- kprobes: ib_post_recv --------------------------------------------- */

// ib_post_recv(struct ib_qp *qp, const struct ib_recv_wr *recv_wr,
//              const struct ib_recv_wr **bad_recv_wr)
SEC("kprobe/ib_post_recv")
int BPF_KPROBE(rdma_post_recv, void *qp, void *recv_wr)
{
    /* Update recv operation counter */
    update_rdma_stats(3, 1);  /* slot 3 = recv_ops */

    emit_rdma_event(GPU_EVT_RDMA_RECV, 0, 0, 0);
    return 0;
}

/* ---- kprobes: ib_poll_cq ----------------------------------------------- */

// ib_poll_cq(struct ib_cq *cq, int num_entries, struct ib_wc *wc)
//
// On entry, record the poll start time.
SEC("kprobe/ib_poll_cq")
int BPF_KPROBE(rdma_poll_cq_entry, void *cq, int num_entries)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct rdma_send_info si = {};
    si.start_ns = bpf_ktime_get_ns();
    si.bytes    = 0;

    bpf_map_update_elem(&rdma_send_inflight, &pid_tgid, &si, BPF_ANY);
    return 0;
}

// On exit, capture the number of completions returned (return value).
SEC("kretprobe/ib_poll_cq")
int BPF_KRETPROBE(rdma_poll_cq_exit, int ret)
{
    if (ret <= 0)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct rdma_send_info *si = bpf_map_lookup_elem(&rdma_send_inflight,
                                                     &pid_tgid);
    __u64 latency = 0;
    if (si) {
        latency = bpf_ktime_get_ns() - si->start_ns;
        bpf_map_delete_elem(&rdma_send_inflight, &pid_tgid);
    }

    /* Emit an event with the completion count in bytes field */
    emit_rdma_event(GPU_EVT_RDMA_SEND, (__u64)ret, latency, 0);
    return 0;
}

/* ---- tracepoints: QP lifecycle ----------------------------------------- */

// tracepoint/rdma/rdma_create_qp - track QP creation
//
// Tracepoint arguments vary by kernel version; we read the QP number
// from the tracepoint context.
SEC("tracepoint/rdma/rdma_create_qp")
int rdma_create_qp(void *ctx)
{
    /* Initialize a new QP tracking entry.  The QP number is typically
     * the second field in the tracepoint args; we use pid_tgid as a
     * proxy key when the exact QP number is not available. */
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 qp_key = (__u32)pid_tgid;

    struct rdma_qp_info qpi = {};
    qpi.bytes_sent   = 0;
    qpi.bytes_recv   = 0;
    qpi.last_send_ns = bpf_ktime_get_ns();
    qpi.retransmits  = 0;
    qpi.completions  = 0;

    bpf_map_update_elem(&rdma_qp_map, &qp_key, &qpi, BPF_ANY);

    emit_rdma_event(GPU_EVT_RDMA_SEND, 0, 0, qp_key);
    return 0;
}

// tracepoint/rdma/rdma_destroy_qp - track QP destruction
SEC("tracepoint/rdma/rdma_destroy_qp")
int rdma_destroy_qp(void *ctx)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 qp_key = (__u32)pid_tgid;

    /* Look up final stats before removing */
    struct rdma_qp_info *qpi = bpf_map_lookup_elem(&rdma_qp_map, &qp_key);
    if (qpi) {
        emit_rdma_event(GPU_EVT_RDMA_RECV, qpi->bytes_sent + qpi->bytes_recv,
                        0, qp_key);
    }

    bpf_map_delete_elem(&rdma_qp_map, &qp_key);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
