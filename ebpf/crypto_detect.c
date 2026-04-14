// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// crypto_detect.c - Detect cryptocurrency mining activity in containers.
//
// Uses kprobe/tcp_v4_connect to detect outbound connections to known mining
// pool ports and tracepoint/sched/sched_process_exec to detect known miner
// binary execution.  Emits HIGH severity events when a PID connects to 2+
// mining ports within 60 seconds or when a known miner binary is executed.

#include "headers/common.h"
#include "headers/security_common.h"

/* Detection stat counter indices */
#define MINING_CTR_PORT_HIT      0
#define MINING_CTR_BINARY_HIT    1
#define MINING_CTR_ALERT_EMITTED 2
#define MINING_CTR_MAX           3

/* Time window for multi-port detection (60 seconds in nanoseconds) */
#define MINING_WINDOW_NS  (60ULL * 1000000000ULL)

/* Suspect PID tracking record */
struct suspect_info {
    __u64 first_seen_ns;     /* timestamp of first suspicious connection */
    __u32 port_hit_count;    /* number of distinct mining port hits */
    __u16 last_port;         /* last suspicious port connected to */
    __u16 _pad;
};

/* ---- BPF maps --------------------------------------------------------- */

// Ring buffer for mining detection events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 64 * 1024);  /* 64 KB */
} mining_events SEC(".maps");

// Known mining pool ports (populated from userspace).
// Key: port number, Value: 1 (present).
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 64);
    __type(key, __u16);
    __type(value, __u8);
} mining_ports SEC(".maps");

// Per-PID suspicious connection tracking (LRU to auto-evict stale entries).
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 8192);
    __type(key, __u32);  /* PID */
    __type(value, struct suspect_info);
} suspect_pids SEC(".maps");

// Detection counters.
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, MINING_CTR_MAX);
    __type(key, __u32);
    __type(value, __u64);
} mining_stats SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

static __always_inline void bump_mining_counter(__u32 idx)
{
    __u64 *cnt = bpf_map_lookup_elem(&mining_stats, &idx);
    if (cnt)
        __sync_fetch_and_add(cnt, 1);
}

static __always_inline void emit_mining_event(__u32 pid, __u32 uid, __u32 gid,
                                              __u8 severity, __u32 dst_ip,
                                              __u16 dst_port)
{
    struct security_event *evt;

    evt = bpf_ringbuf_reserve(&mining_events, sizeof(*evt), 0);
    if (!evt)
        return;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_CRYPTO_MINING;
    evt->severity   = severity;
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    evt->dst_ip     = dst_ip;
    evt->dst_port   = dst_port;
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    bpf_ringbuf_submit(evt, 0);
}

/* ---- kprobe/tcp_v4_connect -------------------------------------------- */

/*
 * Intercept outbound TCP connections.  Check if the destination port
 * matches a known mining pool port.  Track per-PID suspicious hit count
 * and alert when threshold is reached.
 *
 * tcp_v4_connect(struct sock *sk, struct sockaddr *uaddr, int addr_len)
 */
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(detect_mining_connect, void *sk, void *uaddr)
{
    /* Read destination port from sockaddr_in.
     * struct sockaddr_in { sa_family_t sin_family;  // offset 0, 2 bytes
     *                      __be16      sin_port;    // offset 2, 2 bytes
     *                      struct in_addr sin_addr; // offset 4, 4 bytes }
     */
    __u16 dst_port_be = 0;
    bpf_probe_read_user(&dst_port_be, sizeof(dst_port_be),
                        (void *)uaddr + 2);
    __u16 dst_port = bpf_ntohs(dst_port_be);

    /* Check if the port is in our mining ports map. */
    __u8 *is_mining = bpf_map_lookup_elem(&mining_ports, &dst_port);
    if (!is_mining)
        return 0;

    bump_mining_counter(MINING_CTR_PORT_HIT);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 uid = (__u32)uid_gid;
    __u32 gid = (__u32)(uid_gid >> 32);
    __u64 now = bpf_ktime_get_ns();

    /* Read destination IP from sockaddr_in. */
    __u32 dst_ip = 0;
    bpf_probe_read_user(&dst_ip, sizeof(dst_ip), (void *)uaddr + 4);

    /* Update suspect tracking. */
    struct suspect_info *info = bpf_map_lookup_elem(&suspect_pids, &pid);
    if (info) {
        /* Check if still within the detection window. */
        if ((now - info->first_seen_ns) > MINING_WINDOW_NS) {
            /* Window expired -- reset. */
            info->first_seen_ns  = now;
            info->port_hit_count = 1;
            info->last_port      = dst_port;
        } else {
            /* Within window.  Only count distinct ports. */
            if (info->last_port != dst_port)
                info->port_hit_count++;
            info->last_port = dst_port;

            /* Alert when 2+ distinct mining ports hit within window. */
            if (info->port_hit_count >= 2) {
                bump_mining_counter(MINING_CTR_ALERT_EMITTED);
                emit_mining_event(pid, uid, gid, SEC_SEV_HIGH,
                                  dst_ip, dst_port);
                /* Reset to avoid spamming. */
                info->port_hit_count = 0;
                info->first_seen_ns  = now;
            }
        }
    } else {
        /* First suspicious connection from this PID. */
        struct suspect_info new_info = {};
        new_info.first_seen_ns  = now;
        new_info.port_hit_count = 1;
        new_info.last_port      = dst_port;
        bpf_map_update_elem(&suspect_pids, &pid, &new_info, BPF_NOEXIST);
    }

    return 0;
}

/* ---- tracepoint/sched/sched_process_exec ------------------------------ */

/*
 * Detect execution of known miner binaries by checking the process
 * comm name after exec.
 *
 * Known miner process names checked via prefix matching:
 *   xmrig, ethminer, t-rex, nbminer, gminer, phoenixm, lolminer
 */
SEC("tracepoint/sched/sched_process_exec")
int detect_miner_exec(void *ctx)
{
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));

    int matched = 0;

    /* xmrig */
    if (comm[0] == 'x' && comm[1] == 'm' && comm[2] == 'r' &&
        comm[3] == 'i' && comm[4] == 'g')
        matched = 1;
    /* ethminer */
    else if (comm[0] == 'e' && comm[1] == 't' && comm[2] == 'h' &&
             comm[3] == 'm' && comm[4] == 'i' && comm[5] == 'n')
        matched = 1;
    /* t-rex */
    else if (comm[0] == 't' && comm[1] == '-' && comm[2] == 'r' &&
             comm[3] == 'e' && comm[4] == 'x')
        matched = 1;
    /* nbminer */
    else if (comm[0] == 'n' && comm[1] == 'b' && comm[2] == 'm' &&
             comm[3] == 'i' && comm[4] == 'n')
        matched = 1;
    /* gminer */
    else if (comm[0] == 'g' && comm[1] == 'm' && comm[2] == 'i' &&
             comm[3] == 'n' && comm[4] == 'e' && comm[5] == 'r')
        matched = 1;
    /* phoenixm(iner) - comm is truncated to 16 chars */
    else if (comm[0] == 'p' && comm[1] == 'h' && comm[2] == 'o' &&
             comm[3] == 'e' && comm[4] == 'n' && comm[5] == 'i' &&
             comm[6] == 'x' && comm[7] == 'm')
        matched = 1;
    /* lolminer */
    else if (comm[0] == 'l' && comm[1] == 'o' && comm[2] == 'l' &&
             comm[3] == 'm' && comm[4] == 'i' && comm[5] == 'n')
        matched = 1;

    if (!matched)
        return 0;

    bump_mining_counter(MINING_CTR_BINARY_HIT);
    bump_mining_counter(MINING_CTR_ALERT_EMITTED);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 uid = (__u32)uid_gid;
    __u32 gid = (__u32)(uid_gid >> 32);

    /* For binary detection, emit HIGH severity immediately. */
    struct security_event *evt;
    evt = bpf_ringbuf_reserve(&mining_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_CRYPTO_MINING;
    evt->severity   = SEC_SEV_HIGH;
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    __builtin_memcpy(evt->comm, comm, sizeof(evt->comm));

    bpf_ringbuf_submit(evt, 0);

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
