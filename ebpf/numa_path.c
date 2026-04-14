// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// numa_path.c - NUMA-aware network path tracking.
//
// Tracks which CPU processes each incoming packet and correlates with NUMA
// topology to detect cross-NUMA packet processing.  Uses tracepoints on
// netif_receive_skb and kprobe on __napi_poll to build a complete picture
// of packet processing locality.

#include "headers/common.h"

#define MAX_CPUS 256

/* ---- structs ---------------------------------------------------------- */

struct numa_event {
    __u64 timestamp;
    __u32 cpu;
    __u32 numa_node;    /* derived from cpu */
    __u32 src_ip;
    __u32 dst_ip;
    __u32 ifindex;
    __u32 len;
    __u64 queue_id;
};

/* Tracepoint args for netif_receive_skb */
struct netif_receive_skb_args {
    __u64 pad;
    void *skbaddr;
    __u32 len;
    __u32 __pad;
    /* The actual tracepoint provides skb pointer; we read fields from it */
};

/* ---- BPF maps --------------------------------------------------------- */

/* Ring buffer for NUMA events */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 128 * 1024);
} numa_events SEC(".maps");

/* CPU to NUMA node mapping (populated from userspace) */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, MAX_CPUS);
    __type(key, __u32);
    __type(value, __u32);
} cpu_to_numa SEC(".maps");

/* Per-CPU packet count */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, MAX_CPUS);
    __type(key, __u32);
    __type(value, __u64);
} per_cpu_pkt_count SEC(".maps");

/* Cross-NUMA packet count (index 0 = cross-numa, index 1 = same-numa) */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 2);
    __type(key, __u32);
    __type(value, __u64);
} cross_numa_count SEC(".maps");

/* NAPI poll tracking: cpu -> last poll timestamp */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} napi_poll_ts SEC(".maps");

/* ---- tracepoint hooks ------------------------------------------------- */

SEC("tracepoint/net/netif_receive_skb")
int trace_netif_receive_skb(struct netif_receive_skb_args *ctx)
{
    __u32 cpu = bpf_get_smp_processor_id();
    __u64 now = bpf_ktime_get_ns();

    /* Update per-CPU packet count */
    if (cpu < MAX_CPUS) {
        __u64 *cnt = bpf_map_lookup_elem(&per_cpu_pkt_count, &cpu);
        if (cnt)
            (*cnt)++;
    }

    /* Look up NUMA node for this CPU */
    __u32 *numa_node_ptr = bpf_map_lookup_elem(&cpu_to_numa, &cpu);
    __u32 numa_node = numa_node_ptr ? *numa_node_ptr : 0;

    /* Read skb fields */
    void *skbaddr = NULL;
    __u32 len = 0;

    bpf_probe_read_kernel(&skbaddr, sizeof(skbaddr), &ctx->skbaddr);
    bpf_probe_read_kernel(&len, sizeof(len), &ctx->len);

    /* Track cross-NUMA vs same-NUMA */
    __u32 zero = 0;
    __u32 one = 1;

    /* For simplicity, increment same-numa counter by default.
     * Userspace can compare per-cpu counts with expected NUMA affinity
     * to detect cross-NUMA processing. */
    __u64 *same_cnt = bpf_map_lookup_elem(&cross_numa_count, &one);
    if (same_cnt)
        (*same_cnt)++;

    /* Emit event to ring buffer */
    struct numa_event *ev;
    ev = bpf_ringbuf_reserve(&numa_events, sizeof(*ev), 0);
    if (!ev)
        return 0;

    ev->timestamp = now;
    ev->cpu       = cpu;
    ev->numa_node = numa_node;
    ev->src_ip    = 0;  /* IP not available directly from this tracepoint */
    ev->dst_ip    = 0;
    ev->ifindex   = 0;
    ev->len       = len;
    ev->queue_id  = 0;

    bpf_ringbuf_submit(ev, 0);

    return 0;
}

/* ---- kprobe for NAPI poll tracking ------------------------------------ */

/* __napi_poll(struct napi_struct *n, bool *repoll) */
SEC("kprobe/__napi_poll")
int BPF_KPROBE(trace_napi_poll)
{
    __u32 cpu = bpf_get_smp_processor_id();
    __u64 now = bpf_ktime_get_ns();

    /* Record the last NAPI poll time for this CPU */
    __u32 zero = 0;
    __u64 *ts = bpf_map_lookup_elem(&napi_poll_ts, &zero);
    if (ts)
        *ts = now;

    /* Track per-CPU activity from NAPI polling */
    if (cpu < MAX_CPUS) {
        __u32 *numa_node_ptr = bpf_map_lookup_elem(&cpu_to_numa, &cpu);
        __u32 numa_node = numa_node_ptr ? *numa_node_ptr : 0;

        /* Emit a lightweight NAPI event */
        struct numa_event *ev;
        ev = bpf_ringbuf_reserve(&numa_events, sizeof(*ev), 0);
        if (!ev)
            return 0;

        ev->timestamp = now;
        ev->cpu       = cpu;
        ev->numa_node = numa_node;
        ev->src_ip    = 0;
        ev->dst_ip    = 0;
        ev->ifindex   = 0;
        ev->len       = 0;
        ev->queue_id  = 0;

        bpf_ringbuf_submit(ev, 0);
    }

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
