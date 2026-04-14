// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// datapipe_bottleneck.c - Data pipeline bottleneck detection for AI training.
//
// Correlates storage I/O completions (block layer), network data ingestion
// (TCP), and GPU compute phases (CUDA kernel launch/sync) to detect
// bottlenecks in the training data pipeline.  When the GPU is idle
// waiting for data, the program emits bottleneck alert events.

#include "headers/gpu_common.h"

/* ---- constants --------------------------------------------------------- */

#define MAX_INFLIGHT   65536
#define RINGBUF_SIZE   (256 * 1024)
#define IO_STAT_SLOTS  4
#define NET_STAT_SLOTS 2
#define TASK_COMM_LEN  16

/* ---- I/O statistics ---------------------------------------------------- */

struct io_stat {
    __u64 total_bytes;
    __u64 total_ops;
    __u64 total_latency_ns;
};

/* ---- network ingestion statistics -------------------------------------- */

struct net_stat {
    __u64 total_bytes;
    __u64 total_ops;
};

/* ---- GPU busy/idle tracking -------------------------------------------- */

struct gpu_busy_info {
    __u64 last_launch_ns;
    __u64 last_sync_ns;
    __u64 total_busy_ns;
    __u64 total_idle_ns;
    __u8  is_busy;
};

/* ---- BPF maps ---------------------------------------------------------- */

// Per-CPU storage I/O throughput counters.
// Slots: 0=read_bytes, 1=write_bytes, 2=read_ops, 3=write_ops
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, IO_STAT_SLOTS);
    __type(key, __u32);
    __type(value, struct io_stat);
} io_stats SEC(".maps");

// Per-CPU network ingestion rate counters.
// Slots: 0=bytes, 1=ops
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, NET_STAT_SLOTS);
    __type(key, __u32);
    __type(value, struct net_stat);
} net_ingest_stats SEC(".maps");

// Per-PID GPU busy/idle state tracking.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_INFLIGHT);
    __type(key, __u32);
    __type(value, struct gpu_busy_info);
} gpu_busy_map SEC(".maps");

// Ring buffer for pipeline bottleneck alert events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, RINGBUF_SIZE);
} pipeline_events SEC(".maps");

/* ---- helpers ----------------------------------------------------------- */

static __always_inline void emit_pipeline_event(__u8 event_type,
                                                __u64 bytes,
                                                __u64 latency_ns)
{
    struct gpu_event *ev = bpf_ringbuf_reserve(&pipeline_events,
                                               sizeof(struct gpu_event), 0);
    if (!ev)
        return;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    ev->timestamp     = bpf_ktime_get_ns();
    ev->pid           = pid_tgid >> 32;
    ev->gpu_id        = 0;
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

/* ---- tracepoint: block I/O completion ---------------------------------- */

// tracepoint/block/block_rq_complete fires when a block I/O request
// completes.  We extract the number of sectors (each 512 bytes) and
// the latency from the tracepoint arguments.
//
// Args layout for block_rq_complete (simplified):
//   dev, sector, nr_sector, errors, rwbs
SEC("tracepoint/block/block_rq_complete")
int block_rq_complete(void *ctx)
{
    /* We cannot portably destructure the tracepoint args struct across
     * kernel versions without vmlinux.h, so we use a conservative
     * approach: emit a fixed-size event and let userspace correlate. */
    __u32 key = 0;  /* read_bytes slot */
    struct io_stat *stat = bpf_map_lookup_elem(&io_stats, &key);
    if (stat) {
        __sync_fetch_and_add(&stat->total_ops, 1);
        /* Approximate: sector count not easily accessible without
         * vmlinux.h; userspace will read /proc/diskstats for accuracy */
    }

    emit_pipeline_event(GPU_EVT_MEM_TRANSFER, 0, 0);
    return 0;
}

/* ---- kprobe: tcp_recvmsg (network data ingestion rate) ----------------- */

// tcp_recvmsg(struct sock *sk, struct msghdr *msg, size_t len, ...)
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(datapipe_tcp_recv, void *sk, void *msg, __u64 len)
{
    /* Update network ingestion counters */
    __u32 bytes_key = 0;
    struct net_stat *ns = bpf_map_lookup_elem(&net_ingest_stats, &bytes_key);
    if (ns) {
        __sync_fetch_and_add(&ns->total_bytes, len);
        __sync_fetch_and_add(&ns->total_ops, 1);
    }

    return 0;
}

/* ---- uprobe: cudaLaunchKernel (GPU compute start) ---------------------- */

// cudaLaunchKernel(const void *func, dim3 gridDim, dim3 blockDim,
//                  void **args, size_t sharedMem, cudaStream_t stream)
SEC("uprobe/cudaLaunchKernel")
int BPF_KPROBE(datapipe_cuda_launch)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    struct gpu_busy_info *gbi = bpf_map_lookup_elem(&gpu_busy_map, &pid);
    if (gbi) {
        /* Transitioning from idle to busy: record idle duration */
        if (!gbi->is_busy && gbi->last_sync_ns > 0) {
            __u64 idle_ns = now - gbi->last_sync_ns;
            __sync_fetch_and_add(&gbi->total_idle_ns, idle_ns);

            /* If GPU was idle for more than 1ms, emit a bottleneck alert */
            if (idle_ns > 1000000) {
                emit_pipeline_event(GPU_EVT_CUDA_LAUNCH, 0, idle_ns);
            }
        }
        gbi->last_launch_ns = now;
        gbi->is_busy = 1;
    } else {
        struct gpu_busy_info new_gbi = {};
        new_gbi.last_launch_ns = now;
        new_gbi.is_busy        = 1;
        bpf_map_update_elem(&gpu_busy_map, &pid, &new_gbi, BPF_ANY);
    }

    return 0;
}

/* ---- uretprobe: cudaDeviceSynchronize (GPU compute end) ---------------- */

// cudaDeviceSynchronize(void)
SEC("uretprobe/cudaDeviceSynchronize")
int BPF_KRETPROBE(datapipe_cuda_sync)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    struct gpu_busy_info *gbi = bpf_map_lookup_elem(&gpu_busy_map, &pid);
    if (gbi) {
        /* Transitioning from busy to idle */
        if (gbi->is_busy && gbi->last_launch_ns > 0) {
            __u64 busy_ns = now - gbi->last_launch_ns;
            __sync_fetch_and_add(&gbi->total_busy_ns, busy_ns);
        }
        gbi->last_sync_ns = now;
        gbi->is_busy = 0;

        emit_pipeline_event(GPU_EVT_CUDA_SYNC, 0, gbi->total_busy_ns);
    }

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
