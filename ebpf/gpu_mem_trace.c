// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// gpu_mem_trace.c - GPU memory transfer monitoring via uprobes.
//
// Hooks CUDA runtime functions (cudaMemcpy, cudaMemcpyAsync, cudaMalloc,
// cudaFree, cudaLaunchKernel, cudaDeviceSynchronize) to trace GPU memory
// operations.  Each transfer emits a gpu_event to a ring buffer with
// direction, size, and latency information.

#include "headers/gpu_common.h"

/* ---- constants --------------------------------------------------------- */

#define MAX_INFLIGHT   65536
#define RINGBUF_SIZE   (256 * 1024)
#define MAX_ALLOCS     65536
#define MEM_DIR_SLOTS  4      /* H2D, D2H, D2D, PEER */
#define TASK_COMM_LEN  16

/* cudaMemcpyKind enum values from CUDA runtime */
#define CUDA_MEMCPY_H2H  0
#define CUDA_MEMCPY_H2D  1
#define CUDA_MEMCPY_D2H  2
#define CUDA_MEMCPY_D2D  3

/* ---- per-direction byte counters --------------------------------------- */

struct mem_dir_stat {
    __u64 count;
    __u64 total_bytes;
};

/* ---- allocation tracking ----------------------------------------------- */

struct alloc_info {
    __u64 size;
    __u64 alloc_ns;
};

/* ---- BPF maps ---------------------------------------------------------- */

// In-flight CUDA memory operations keyed by pid_tgid.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_INFLIGHT);
    __type(key, __u64);
    __type(value, struct cuda_mem_inflight);
} cuda_inflight_map SEC(".maps");

// Ring buffer for gpu_event emission to userspace.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, RINGBUF_SIZE);
} cuda_events SEC(".maps");

// Per-CPU memory direction byte counters (one per mem_direction).
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, MEM_DIR_SLOTS);
    __type(key, __u32);
    __type(value, struct mem_dir_stat);
} cuda_mem_stats SEC(".maps");

// Live GPU allocations: pointer address -> size.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ALLOCS);
    __type(key, __u64);
    __type(value, struct alloc_info);
} cuda_alloc_map SEC(".maps");

/* ---- helpers ----------------------------------------------------------- */

static __always_inline __u8 cuda_kind_to_direction(__u32 kind)
{
    switch (kind) {
    case CUDA_MEMCPY_H2D:
        return MEM_H2D;
    case CUDA_MEMCPY_D2H:
        return MEM_D2H;
    case CUDA_MEMCPY_D2D:
        return MEM_D2D;
    default:
        return MEM_H2D;
    }
}

static __always_inline void emit_cuda_event(__u8 event_type, __u8 direction,
                                            __u64 bytes, __u64 latency_ns)
{
    struct gpu_event *ev = bpf_ringbuf_reserve(&cuda_events,
                                               sizeof(struct gpu_event), 0);
    if (!ev)
        return;

    __u64 pid_tgid = bpf_get_current_pid_tgid();

    ev->timestamp     = bpf_ktime_get_ns();
    ev->pid           = pid_tgid >> 32;
    ev->gpu_id        = 0;
    ev->event_type    = event_type;
    ev->direction     = direction;
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

static __always_inline void update_mem_stats(__u8 direction, __u64 bytes)
{
    __u32 key = direction;
    if (key >= MEM_DIR_SLOTS)
        return;

    struct mem_dir_stat *stat = bpf_map_lookup_elem(&cuda_mem_stats, &key);
    if (stat) {
        __sync_fetch_and_add(&stat->count, 1);
        __sync_fetch_and_add(&stat->total_bytes, bytes);
    }
}

/* ---- uprobes: cudaMemcpy ---------------------------------------------- */

// cudaMemcpy(void *dst, const void *src, size_t count,
//            enum cudaMemcpyKind kind)
SEC("uprobe/cudaMemcpy")
int BPF_KPROBE(cuda_memcpy_entry, void *dst, void *src, __u64 count,
               __u32 kind)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight info = {};
    info.start_ns  = bpf_ktime_get_ns();
    info.size      = count;
    info.direction = cuda_kind_to_direction(kind);

    bpf_map_update_elem(&cuda_inflight_map, &pid_tgid, &info, BPF_ANY);
    return 0;
}

SEC("uretprobe/cudaMemcpy")
int BPF_KRETPROBE(cuda_memcpy_exit)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight *info = bpf_map_lookup_elem(&cuda_inflight_map,
                                                         &pid_tgid);
    if (!info)
        return 0;

    __u64 latency = bpf_ktime_get_ns() - info->start_ns;

    emit_cuda_event(GPU_EVT_MEM_TRANSFER, info->direction, info->size,
                    latency);
    update_mem_stats(info->direction, info->size);

    bpf_map_delete_elem(&cuda_inflight_map, &pid_tgid);
    return 0;
}

/* ---- uprobes: cudaMemcpyAsync ----------------------------------------- */

// cudaMemcpyAsync(void *dst, const void *src, size_t count,
//                 enum cudaMemcpyKind kind, cudaStream_t stream)
SEC("uprobe/cudaMemcpyAsync")
int BPF_KPROBE(cuda_memcpy_async_entry, void *dst, void *src, __u64 count,
               __u32 kind)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight info = {};
    info.start_ns  = bpf_ktime_get_ns();
    info.size      = count;
    info.direction = cuda_kind_to_direction(kind);

    bpf_map_update_elem(&cuda_inflight_map, &pid_tgid, &info, BPF_ANY);
    return 0;
}

SEC("uretprobe/cudaMemcpyAsync")
int BPF_KRETPROBE(cuda_memcpy_async_exit)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight *info = bpf_map_lookup_elem(&cuda_inflight_map,
                                                         &pid_tgid);
    if (!info)
        return 0;

    __u64 latency = bpf_ktime_get_ns() - info->start_ns;

    emit_cuda_event(GPU_EVT_MEM_TRANSFER, info->direction, info->size,
                    latency);
    update_mem_stats(info->direction, info->size);

    bpf_map_delete_elem(&cuda_inflight_map, &pid_tgid);
    return 0;
}

/* ---- uprobes: cudaMalloc ---------------------------------------------- */

// cudaMalloc(void **devPtr, size_t size)
// We stash the requested size at entry, then at exit we read the allocated
// pointer from the first argument's target and record it.
SEC("uprobe/cudaMalloc")
int BPF_KPROBE(cuda_malloc_entry, void **devPtr, __u64 size)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight info = {};
    info.start_ns  = bpf_ktime_get_ns();
    info.size      = size;
    info.direction = 0;

    bpf_map_update_elem(&cuda_inflight_map, &pid_tgid, &info, BPF_ANY);
    return 0;
}

SEC("uretprobe/cudaMalloc")
int BPF_KRETPROBE(cuda_malloc_exit, long ret)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight *info = bpf_map_lookup_elem(&cuda_inflight_map,
                                                         &pid_tgid);
    if (!info)
        return 0;

    __u64 latency = bpf_ktime_get_ns() - info->start_ns;
    __u64 size = info->size;

    /* Track allocation: use pid_tgid combined with timestamp as a proxy
     * key since we cannot easily read the output pointer.  The userspace
     * collector correlates by pid + size. */
    __u64 alloc_key = pid_tgid ^ info->start_ns;
    struct alloc_info ai = {};
    ai.size     = size;
    ai.alloc_ns = info->start_ns;
    bpf_map_update_elem(&cuda_alloc_map, &alloc_key, &ai, BPF_ANY);

    emit_cuda_event(GPU_EVT_MEM_TRANSFER, MEM_H2D, size, latency);

    bpf_map_delete_elem(&cuda_inflight_map, &pid_tgid);
    return 0;
}

/* ---- uprobes: cudaFree ------------------------------------------------ */

// cudaFree(void *devPtr)
SEC("uprobe/cudaFree")
int BPF_KPROBE(cuda_free_entry, void *devPtr)
{
    __u64 ptr = (__u64)devPtr;

    /* Remove from allocation tracking if present.  We try the pointer
     * value itself as key; the userspace side reconciles via size. */
    bpf_map_delete_elem(&cuda_alloc_map, &ptr);

    emit_cuda_event(GPU_EVT_MEM_TRANSFER, MEM_D2H, 0, 0);
    return 0;
}

/* ---- uprobes: cudaLaunchKernel ---------------------------------------- */

// cudaLaunchKernel(const void *func, dim3 gridDim, dim3 blockDim,
//                  void **args, size_t sharedMem, cudaStream_t stream)
SEC("uprobe/cudaLaunchKernel")
int BPF_KPROBE(cuda_launch_kernel_entry)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight info = {};
    info.start_ns  = bpf_ktime_get_ns();
    info.size      = 0;
    info.direction = 0;

    bpf_map_update_elem(&cuda_inflight_map, &pid_tgid, &info, BPF_ANY);

    emit_cuda_event(GPU_EVT_CUDA_LAUNCH, 0, 0, 0);
    return 0;
}

/* ---- uprobes: cudaDeviceSynchronize ----------------------------------- */

// cudaDeviceSynchronize(void)
// We trace the return to capture the synchronization latency.
SEC("uretprobe/cudaDeviceSynchronize")
int BPF_KRETPROBE(cuda_device_sync_exit)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();

    struct cuda_mem_inflight *info = bpf_map_lookup_elem(&cuda_inflight_map,
                                                         &pid_tgid);
    __u64 latency = 0;
    if (info) {
        latency = bpf_ktime_get_ns() - info->start_ns;
        bpf_map_delete_elem(&cuda_inflight_map, &pid_tgid);
    }

    emit_cuda_event(GPU_EVT_CUDA_SYNC, 0, 0, latency);
    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
