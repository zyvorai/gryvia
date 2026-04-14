// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// latency_probe.c - Per-connection round-trip latency measurement.
//
// Hooks tcp_sendmsg (kprobe) and tcp_recvmsg (kretprobe) to measure
// the elapsed time between send and the next receive on the same
// socket, providing an application-level RTT estimate.
//
// Results are stored in a histogram map (6 buckets) and individual
// high-latency events are emitted over the perf ring buffer.

#include "headers/common.h"

#define LATENCY_THRESHOLD_NS 50000000ULL  // 50 ms

/* Histogram bucket indices */
#define BUCKET_LT_1MS    0   // < 1 ms
#define BUCKET_1_5MS     1   // 1-5 ms
#define BUCKET_5_10MS    2   // 5-10 ms
#define BUCKET_10_50MS   3   // 10-50 ms
#define BUCKET_50_100MS  4   // 50-100 ms
#define BUCKET_GT_100MS  5   // > 100 ms
#define NUM_BUCKETS      6

/* ---- BPF maps --------------------------------------------------------- */

// Timestamp of the last tcp_sendmsg per socket (keyed by sock pointer).
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);   // sock pointer cast to u64
    __type(value, __u64); // timestamp in ns
} send_ts SEC(".maps");

// Latency histogram: bucket_index -> count.
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, NUM_BUCKETS);
    __type(key, __u32);
    __type(value, __u64);
} latency_hist SEC(".maps");

// Perf ring buffer for threshold-exceeding events.
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} latency_events SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

static __always_inline __u32 bucket_for(__u64 ns)
{
    if (ns < 1000000ULL)
        return BUCKET_LT_1MS;
    if (ns < 5000000ULL)
        return BUCKET_1_5MS;
    if (ns < 10000000ULL)
        return BUCKET_5_10MS;
    if (ns < 50000000ULL)
        return BUCKET_10_50MS;
    if (ns < 100000000ULL)
        return BUCKET_50_100MS;
    return BUCKET_GT_100MS;
}

/* ---- kprobes ---------------------------------------------------------- */

// tcp_sendmsg(struct sock *sk, struct msghdr *msg, size_t size)
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(probe_tcp_sendmsg, struct sock *sk)
{
    __u64 key = (__u64)sk;
    __u64 ts  = bpf_ktime_get_ns();
    bpf_map_update_elem(&send_ts, &key, &ts, BPF_ANY);
    return 0;
}

// tcp_recvmsg(struct sock *sk, struct msghdr *msg, ...)
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(probe_tcp_recvmsg, struct sock *sk)
{
    __u64 key = (__u64)sk;
    __u64 *ts = bpf_map_lookup_elem(&send_ts, &key);
    if (!ts)
        return 0;

    __u64 now     = bpf_ktime_get_ns();
    __u64 latency = now - *ts;

    // Remove the entry so we measure one RTT per send.
    bpf_map_delete_elem(&send_ts, &key);

    // Update histogram.
    __u32 bucket = bucket_for(latency);
    __u64 *cnt = bpf_map_lookup_elem(&latency_hist, &bucket);
    if (cnt)
        __sync_fetch_and_add(cnt, 1);

    // Emit a perf event when the latency exceeds the threshold.
    if (latency >= LATENCY_THRESHOLD_NS) {
        struct flow_event ev = {};
        ev.timestamp  = now;
        ev.latency_ns = latency;
        ev.protocol   = IPPROTO_TCP;

        __u64 pid_tgid = bpf_get_current_pid_tgid();
        ev.pid = pid_tgid >> 32;
        bpf_get_current_comm(&ev.comm, sizeof(ev.comm));

        // Read addresses from the sock.
        bpf_probe_read_kernel(&ev.src_ip, sizeof(ev.src_ip),
                              &sk->__sk_common.skc_rcv_saddr);
        bpf_probe_read_kernel(&ev.dst_ip, sizeof(ev.dst_ip),
                              &sk->__sk_common.skc_daddr);
        __u16 dport = 0;
        bpf_probe_read_kernel(&dport, sizeof(dport),
                              &sk->__sk_common.skc_dport);
        ev.dst_port = bpf_ntohs(dport);
        bpf_probe_read_kernel(&ev.src_port, sizeof(ev.src_port),
                              &sk->__sk_common.skc_num);

        bpf_perf_event_output(ctx, &latency_events, BPF_F_CURRENT_CPU,
                              &ev, sizeof(ev));
    }

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
