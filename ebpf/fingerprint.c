// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// fingerprint.c - Workload behavioral fingerprinting.
//
// Builds per-process behavioral feature vectors by tracking syscall
// frequency and network connection patterns.  Compares runtime behavior
// against known-good baselines and emits alerts when significant
// deviations are detected, indicating potential compromise or
// misconfiguration.

#include "headers/common.h"

/* ---- constants -------------------------------------------------------- */

#define NUM_SYSCALL_BUCKETS 32  /* group syscalls into 32 categories */
#define MAX_TRACKED_PROCS   8192

/* Drift alert thresholds */
#define DRIFT_SYSCALL_THRESHOLD  100  /* >100 calls in a new category */
#define DRIFT_PORT_THRESHOLD     5    /* >5 new destination ports */
#define DRIFT_IP_THRESHOLD       10   /* >10 new destination IPs */

/* Alert event types */
#define ALERT_NEW_SYSCALL_CAT   1
#define ALERT_NEW_DST_PORT      2
#define ALERT_NEW_DST_IP        3
#define ALERT_CONN_SPIKE        4

/* ---- structs ---------------------------------------------------------- */

struct behavior_profile {
    __u64 syscall_counts[NUM_SYSCALL_BUCKETS];
    __u64 unique_dst_ips;
    __u64 unique_dst_ports;
    __u64 total_bytes_out;
    __u64 total_connections;
    __u64 last_updated;
};

struct fingerprint_event {
    __u64 timestamp;
    __u32 pid;
    __u32 alert_type;
    __u64 current_value;
    __u64 baseline_value;
    char  comm[TASK_COMM_LEN];
};

/* Tracepoint args for sys_enter */
struct sys_enter_args {
    unsigned long long unused;
    long               id;
    unsigned long      args[6];
};

/* ---- BPF maps --------------------------------------------------------- */

/* Per-process behavior profiles */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_TRACKED_PROCS);
    __type(key, __u32);    /* pid */
    __type(value, struct behavior_profile);
} process_profiles SEC(".maps");

/* Ring buffer for drift alert events */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 64 * 1024);
} fingerprint_events SEC(".maps");

/* Known-good baseline profiles (populated from userspace after learning) */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);    /* pid or process group id */
    __type(value, struct behavior_profile);
} baseline_profiles SEC(".maps");

/* Destination IP tracking per process: hash of (pid, dst_ip) -> count */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);    /* pid << 32 | dst_ip_hash */
    __type(value, __u64);  /* count */
} dst_ip_tracker SEC(".maps");

/* Destination port tracking per process: hash of (pid, dst_port) -> count */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);    /* pid << 32 | dst_port */
    __type(value, __u64);  /* count */
} dst_port_tracker SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

/* Map syscall number to one of 32 buckets/categories.
 * Categories roughly follow Linux syscall groupings:
 *  0: process mgmt (fork, execve, exit, wait)
 *  1: file I/O (read, write, open, close)
 *  2: file stat/metadata
 *  3: memory management (mmap, mprotect, brk)
 *  4: signal handling
 *  5: IPC (shmget, semget, msgget)
 *  6: network socket ops (socket, bind, listen, accept)
 *  7: network connect/send/recv
 *  8: filesystem ops (mount, stat, chmod)
 *  9: timer/clock ops
 * 10: epoll/poll/select
 * 11-31: other categories
 */
static __always_inline __u32 syscall_to_bucket(long syscall_nr)
{
    if (syscall_nr < 0)
        return 31;

    /* Common syscall categorization for x86_64 */
    switch (syscall_nr) {
    /* File I/O */
    case 0: case 1: case 2: case 3: case 17: case 18: case 19: case 20:
        return 1;  /* read, write, open, close, pread, pwrite, readv, writev */

    /* File stat/metadata */
    case 4: case 5: case 6: case 7: case 8:
        return 2;  /* stat, fstat, lstat, poll, lseek */

    /* Memory management */
    case 9: case 10: case 11: case 12: case 25: case 26: case 27: case 28:
        return 3;  /* mmap, mprotect, munmap, brk, mremap, msync, mincore, madvise */

    /* Process management */
    case 56: case 57: case 58: case 59: case 60: case 61: case 62:
        return 0;  /* clone, fork, vfork, execve, exit, wait4, kill */

    /* Signal handling */
    case 13: case 14: case 15: case 34: case 35:
        return 4;  /* rt_sigaction, rt_sigprocmask, rt_sigreturn, pause, nanosleep */

    /* Network socket operations */
    case 41: case 49: case 50: case 43: case 51: case 52: case 53:
        return 6;  /* socket, bind, listen, accept, getsockopt, setsockopt, socketpair */

    /* Network data transfer */
    case 42: case 44: case 45: case 46: case 47: case 48:
        return 7;  /* connect, sendto, recvfrom, sendmsg, recvmsg, shutdown */

    /* Filesystem operations */
    case 82: case 83: case 84: case 85: case 86: case 87: case 88: case 89: case 90:
        return 8;  /* rename, mkdir, rmdir, creat, link, unlink, symlink, readlink, chmod */

    /* Timer/clock */
    case 96: case 228: case 229: case 230:
        return 9;  /* gettimeofday, clock_gettime, clock_getres, clock_nanosleep */

    /* epoll/poll/select */
    case 7: case 23: case 232: case 233:
        return 10; /* poll, select, epoll_wait, epoll_ctl */

    /* IPC */
    case 29: case 30: case 31: case 64: case 65: case 66: case 67: case 68: case 69: case 70:
        return 5;  /* shmget, shmat, shmctl, semget, semop, semctl, msgget, msgsnd, msgrcv, msgctl */

    default:
        /* Hash remaining syscalls across remaining buckets 11-31 */
        return 11 + ((__u32)syscall_nr % 21);
    }
}

static __always_inline void emit_drift_alert(__u32 pid, __u32 alert_type,
                                              __u64 current, __u64 baseline)
{
    struct fingerprint_event *ev;
    ev = bpf_ringbuf_reserve(&fingerprint_events, sizeof(*ev), 0);
    if (!ev)
        return;

    ev->timestamp      = bpf_ktime_get_ns();
    ev->pid            = pid;
    ev->alert_type     = alert_type;
    ev->current_value  = current;
    ev->baseline_value = baseline;
    bpf_get_current_comm(&ev->comm, sizeof(ev->comm));

    bpf_ringbuf_submit(ev, 0);
}

/* ---- raw tracepoint: syscall profiling -------------------------------- */

SEC("raw_tracepoint/sys_enter")
int fingerprint_syscall(struct bpf_raw_tracepoint_args *raw_ctx)
{
    struct sys_enter_args *args = (struct sys_enter_args *)raw_ctx->args[0];
    long syscall_nr = 0;
    bpf_probe_read_kernel(&syscall_nr, sizeof(syscall_nr), &args->id);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    __u32 bucket = syscall_to_bucket(syscall_nr);
    if (bucket >= NUM_SYSCALL_BUCKETS)
        bucket = NUM_SYSCALL_BUCKETS - 1;

    /* Update process profile */
    struct behavior_profile *prof = bpf_map_lookup_elem(&process_profiles, &pid);
    if (prof) {
        __sync_fetch_and_add(&prof->syscall_counts[bucket], 1);
        prof->last_updated = bpf_ktime_get_ns();

        /* Check against baseline for drift */
        struct behavior_profile *baseline = bpf_map_lookup_elem(&baseline_profiles, &pid);
        if (baseline) {
            /* Alert if a previously unused syscall category now has significant activity */
            if (baseline->syscall_counts[bucket] == 0 &&
                prof->syscall_counts[bucket] >= DRIFT_SYSCALL_THRESHOLD) {
                emit_drift_alert(pid, ALERT_NEW_SYSCALL_CAT,
                                 prof->syscall_counts[bucket], 0);
            }
        }
    } else {
        struct behavior_profile new_prof = {};
        new_prof.syscall_counts[bucket] = 1;
        new_prof.last_updated = bpf_ktime_get_ns();
        bpf_map_update_elem(&process_profiles, &pid, &new_prof, BPF_NOEXIST);
    }

    return 0;
}

/* ---- kprobe: connection pattern profiling ----------------------------- */

/* tcp_v4_connect(struct sock *sk, struct sockaddr *uaddr, int addr_len) */
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(fingerprint_connect, struct sock *sk)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    __u32 dst_ip = 0;
    __u16 dport_be = 0;
    bpf_probe_read_kernel(&dst_ip, sizeof(dst_ip),
                          &sk->__sk_common.skc_daddr);
    bpf_probe_read_kernel(&dport_be, sizeof(dport_be),
                          &sk->__sk_common.skc_dport);
    __u16 dst_port = bpf_ntohs(dport_be);

    /* Update process profile */
    struct behavior_profile *prof = bpf_map_lookup_elem(&process_profiles, &pid);
    if (!prof) {
        struct behavior_profile new_prof = {};
        new_prof.total_connections = 1;
        new_prof.last_updated = bpf_ktime_get_ns();
        bpf_map_update_elem(&process_profiles, &pid, &new_prof, BPF_NOEXIST);
        prof = bpf_map_lookup_elem(&process_profiles, &pid);
        if (!prof)
            return 0;
    }

    __sync_fetch_and_add(&prof->total_connections, 1);
    prof->last_updated = bpf_ktime_get_ns();

    /* Track unique destination IPs */
    __u64 ip_key = ((__u64)pid << 32) | (__u64)dst_ip;
    __u64 *ip_cnt = bpf_map_lookup_elem(&dst_ip_tracker, &ip_key);
    if (!ip_cnt) {
        __u64 one = 1;
        bpf_map_update_elem(&dst_ip_tracker, &ip_key, &one, BPF_NOEXIST);
        __sync_fetch_and_add(&prof->unique_dst_ips, 1);

        /* Check against baseline */
        struct behavior_profile *baseline = bpf_map_lookup_elem(&baseline_profiles, &pid);
        if (baseline && prof->unique_dst_ips > baseline->unique_dst_ips + DRIFT_IP_THRESHOLD) {
            emit_drift_alert(pid, ALERT_NEW_DST_IP,
                             prof->unique_dst_ips, baseline->unique_dst_ips);
        }
    } else {
        __sync_fetch_and_add(ip_cnt, 1);
    }

    /* Track unique destination ports */
    __u64 port_key = ((__u64)pid << 32) | (__u64)dst_port;
    __u64 *port_cnt = bpf_map_lookup_elem(&dst_port_tracker, &port_key);
    if (!port_cnt) {
        __u64 one = 1;
        bpf_map_update_elem(&dst_port_tracker, &port_key, &one, BPF_NOEXIST);
        __sync_fetch_and_add(&prof->unique_dst_ports, 1);

        /* Check against baseline */
        struct behavior_profile *baseline = bpf_map_lookup_elem(&baseline_profiles, &pid);
        if (baseline && prof->unique_dst_ports > baseline->unique_dst_ports + DRIFT_PORT_THRESHOLD) {
            emit_drift_alert(pid, ALERT_NEW_DST_PORT,
                             prof->unique_dst_ports, baseline->unique_dst_ports);
        }
    } else {
        __sync_fetch_and_add(port_cnt, 1);
    }

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
