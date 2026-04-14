// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// latency_breakdown.c - Multi-phase request latency breakdown.
//
// Tracks per-request lifecycle across multiple phases: DNS resolution,
// TCP handshake, TLS handshake, and application request/response.
// Provides a complete latency breakdown showing where time is spent
// in each phase of a network request.

#include "headers/common.h"

/* ---- phase tracking --------------------------------------------------- */

#define PHASE_DNS_START       0
#define PHASE_DNS_END         1
#define PHASE_TCP_START       2
#define PHASE_TCP_END         3
#define PHASE_TLS_START       4
#define PHASE_TLS_END         5
#define PHASE_APP_SEND        6
#define PHASE_APP_RECV        7

/* Phase histogram indices */
#define HIST_DNS              0
#define HIST_TCP              1
#define HIST_TLS              2
#define HIST_APP              3
#define HIST_TOTAL            4
#define NUM_PHASE_HISTOGRAMS  5

/* Histogram buckets per phase (8 buckets each) */
#define BUCKETS_PER_PHASE     8
#define TOTAL_HIST_ENTRIES    (NUM_PHASE_HISTOGRAMS * BUCKETS_PER_PHASE)

/* ---- structs ---------------------------------------------------------- */

struct latency_breakdown_event {
    __u64 timestamp;
    __u32 pid;
    __u32 dst_ip;
    __u16 dst_port;
    __u16 _pad;
    __u64 dns_ns;           /* DNS resolution time */
    __u64 tcp_connect_ns;   /* TCP handshake time */
    __u64 tls_handshake_ns; /* TLS handshake time */
    __u64 app_rtt_ns;       /* Application request/response time */
    __u64 total_ns;         /* Total end-to-end */
    char  comm[TASK_COMM_LEN];
};

/* Per-request tracking state, keyed by pid_tgid */
struct request_state {
    __u64 dns_start;
    __u64 dns_end;
    __u64 tcp_start;
    __u64 tcp_end;
    __u64 tls_start;
    __u64 tls_end;
    __u64 app_send;
    __u64 app_recv;
    __u32 dst_ip;
    __u16 dst_port;
    __u16 _pad;
};

/* ---- BPF maps --------------------------------------------------------- */

/* Per-request phase timestamps keyed by pid_tgid */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);   /* pid_tgid */
    __type(value, struct request_state);
} request_tracking SEC(".maps");

/* Ring buffer for breakdown events */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 128 * 1024);
} breakdown_events SEC(".maps");

/* Per-phase latency histogram (percpu for lock-free updates) */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, TOTAL_HIST_ENTRIES);
    __type(key, __u32);
    __type(value, __u64);
} phase_histogram SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

/* Map latency to histogram bucket (8 buckets):
 * <100us, 100us-1ms, 1-5ms, 5-10ms, 10-50ms, 50-100ms, 100-500ms, >500ms */
static __always_inline __u32 latency_bucket(__u64 ns)
{
    if (ns < 100000ULL)           return 0;  /* < 100us */
    if (ns < 1000000ULL)          return 1;  /* 100us-1ms */
    if (ns < 5000000ULL)          return 2;  /* 1-5ms */
    if (ns < 10000000ULL)         return 3;  /* 5-10ms */
    if (ns < 50000000ULL)         return 4;  /* 10-50ms */
    if (ns < 100000000ULL)        return 5;  /* 50-100ms */
    if (ns < 500000000ULL)        return 6;  /* 100-500ms */
    return 7;                                 /* > 500ms */
}

static __always_inline void update_phase_histogram(__u32 phase, __u64 latency_ns)
{
    __u32 bucket = phase * BUCKETS_PER_PHASE + latency_bucket(latency_ns);
    if (bucket >= TOTAL_HIST_ENTRIES)
        return;
    __u64 *cnt = bpf_map_lookup_elem(&phase_histogram, &bucket);
    if (cnt)
        (*cnt)++;
}

static __always_inline void emit_breakdown(struct request_state *rs)
{
    __u64 dns_ns = 0, tcp_ns = 0, tls_ns = 0, app_ns = 0, total_ns = 0;
    __u64 first_ts = 0, last_ts = 0;

    if (rs->dns_start && rs->dns_end) {
        dns_ns = rs->dns_end - rs->dns_start;
        update_phase_histogram(HIST_DNS, dns_ns);
    }

    if (rs->tcp_start && rs->tcp_end) {
        tcp_ns = rs->tcp_end - rs->tcp_start;
        update_phase_histogram(HIST_TCP, tcp_ns);
    }

    if (rs->tls_start && rs->tls_end) {
        tls_ns = rs->tls_end - rs->tls_start;
        update_phase_histogram(HIST_TLS, tls_ns);
    }

    if (rs->app_send && rs->app_recv) {
        app_ns = rs->app_recv - rs->app_send;
        update_phase_histogram(HIST_APP, app_ns);
    }

    /* Compute total from first non-zero timestamp to last */
    first_ts = rs->dns_start;
    if (!first_ts || (rs->tcp_start && rs->tcp_start < first_ts))
        first_ts = rs->tcp_start;

    last_ts = rs->app_recv;
    if (!last_ts)
        last_ts = rs->tcp_end;

    if (first_ts && last_ts && last_ts > first_ts) {
        total_ns = last_ts - first_ts;
        update_phase_histogram(HIST_TOTAL, total_ns);
    }

    struct latency_breakdown_event *ev;
    ev = bpf_ringbuf_reserve(&breakdown_events, sizeof(*ev), 0);
    if (!ev)
        return;

    ev->timestamp       = bpf_ktime_get_ns();
    ev->pid             = bpf_get_current_pid_tgid() >> 32;
    ev->dst_ip          = rs->dst_ip;
    ev->dst_port        = rs->dst_port;
    ev->_pad            = 0;
    ev->dns_ns          = dns_ns;
    ev->tcp_connect_ns  = tcp_ns;
    ev->tls_handshake_ns = tls_ns;
    ev->app_rtt_ns      = app_ns;
    ev->total_ns        = total_ns;
    bpf_get_current_comm(&ev->comm, sizeof(ev->comm));

    bpf_ringbuf_submit(ev, 0);
}

/* ---- kprobes: DNS phase ----------------------------------------------- */

/* udp_sendmsg(struct sock *sk, struct msghdr *msg, size_t len) */
SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(breakdown_dns_start, struct sock *sk)
{
    /* Check if this is a DNS request (dst port 53) */
    __u16 dport_be = 0;
    bpf_probe_read_kernel(&dport_be, sizeof(dport_be),
                          &sk->__sk_common.skc_dport);
    __u16 dport = bpf_ntohs(dport_be);
    if (dport != 53)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 now = bpf_ktime_get_ns();

    struct request_state *rs = bpf_map_lookup_elem(&request_tracking, &pid_tgid);
    if (rs) {
        rs->dns_start = now;
    } else {
        struct request_state new_rs = {};
        new_rs.dns_start = now;
        bpf_map_update_elem(&request_tracking, &pid_tgid, &new_rs, BPF_NOEXIST);
    }

    return 0;
}

/* ---- kprobes: TCP handshake phase ------------------------------------- */

/* tcp_v4_connect(struct sock *sk, struct sockaddr *uaddr, int addr_len) */
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(breakdown_tcp_connect, struct sock *sk)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 now = bpf_ktime_get_ns();

    __u32 dst_ip = 0;
    __u16 dport_be = 0;
    bpf_probe_read_kernel(&dst_ip, sizeof(dst_ip),
                          &sk->__sk_common.skc_daddr);
    bpf_probe_read_kernel(&dport_be, sizeof(dport_be),
                          &sk->__sk_common.skc_dport);

    struct request_state *rs = bpf_map_lookup_elem(&request_tracking, &pid_tgid);
    if (rs) {
        rs->tcp_start = now;
        rs->dst_ip    = dst_ip;
        rs->dst_port  = bpf_ntohs(dport_be);
        /* Mark dns_end if dns was in progress */
        if (rs->dns_start && !rs->dns_end)
            rs->dns_end = now;
    } else {
        struct request_state new_rs = {};
        new_rs.tcp_start = now;
        new_rs.dst_ip    = dst_ip;
        new_rs.dst_port  = bpf_ntohs(dport_be);
        bpf_map_update_elem(&request_tracking, &pid_tgid, &new_rs, BPF_NOEXIST);
    }

    return 0;
}

/* kretprobe for tcp_v4_connect - TCP handshake end */
SEC("kretprobe/tcp_v4_connect")
int BPF_KRETPROBE(breakdown_tcp_connect_ret, int ret)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 now = bpf_ktime_get_ns();

    struct request_state *rs = bpf_map_lookup_elem(&request_tracking, &pid_tgid);
    if (!rs)
        return 0;

    rs->tcp_end = now;

    return 0;
}

/* ---- kprobes: TLS handshake phase ------------------------------------- */

/* tls_sw_sendmsg - indicates TLS handshake activity */
SEC("kprobe/tls_sw_sendmsg")
int BPF_KPROBE(breakdown_tls_handshake)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 now = bpf_ktime_get_ns();

    struct request_state *rs = bpf_map_lookup_elem(&request_tracking, &pid_tgid);
    if (!rs)
        return 0;

    /* Record first TLS send as handshake start, subsequent as end */
    if (!rs->tls_start)
        rs->tls_start = now;
    else
        rs->tls_end = now;

    return 0;
}

/* ---- kprobes: Application phase --------------------------------------- */

/* tcp_sendmsg(struct sock *sk, struct msghdr *msg, size_t size) */
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(breakdown_app_send, struct sock *sk)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 now = bpf_ktime_get_ns();

    struct request_state *rs = bpf_map_lookup_elem(&request_tracking, &pid_tgid);
    if (!rs)
        return 0;

    /* Only set app_send if TCP handshake is complete */
    if (rs->tcp_end) {
        rs->app_send = now;
        /* If TLS was started but not ended, mark it as ended now */
        if (rs->tls_start && !rs->tls_end)
            rs->tls_end = now;
    }

    return 0;
}

/* tcp_recvmsg(struct sock *sk, struct msghdr *msg, ...) */
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(breakdown_app_recv, struct sock *sk)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 now = bpf_ktime_get_ns();

    struct request_state *rs = bpf_map_lookup_elem(&request_tracking, &pid_tgid);
    if (!rs)
        return 0;

    /* Only complete if we have a send timestamp */
    if (!rs->app_send)
        return 0;

    rs->app_recv = now;

    /* Emit the complete breakdown event */
    emit_breakdown(rs);

    /* Clean up tracking state for this request */
    bpf_map_delete_elem(&request_tracking, &pid_tgid);

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
