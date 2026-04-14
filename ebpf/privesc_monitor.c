// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// privesc_monitor.c - Detect privilege escalation attempts.
//
// Monitors UID/GID transitions to root, credential changes, and
// capability acquisitions.  Uses raw_tracepoint/sys_enter for
// set*uid/set*gid/capset syscalls and kprobe/commit_creds for
// catching all credential changes at the kernel level.

#include "headers/common.h"
#include "headers/security_common.h"

/* Syscall numbers for x86_64 */
#define SYS_SETUID    105
#define SYS_SETGID    106
#define SYS_CAPSET     90
#define SYS_SETREUID  113
#define SYS_SETREGID  114
#define SYS_SETRESUID 117
#define SYS_SETRESGID 119

/* Dangerous capabilities */
#define CAP_DAC_OVERRIDE  1
#define CAP_NET_ADMIN    12
#define CAP_SYS_PTRACE   19
#define CAP_SYS_ADMIN    21

/* Stat counter indices */
#define PRIVESC_CTR_SETUID     0
#define PRIVESC_CTR_SETGID     1
#define PRIVESC_CTR_CAPSET     2
#define PRIVESC_CTR_CRED_CHG   3
#define PRIVESC_CTR_ALERTS     4
#define PRIVESC_CTR_MAX        5

/* Pre-change credential snapshot */
struct cred_snapshot {
    __u32 uid;
    __u32 gid;
    __u64 timestamp;
};

/* ---- BPF maps --------------------------------------------------------- */

// Ring buffer for privilege escalation events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 64 * 1024);  /* 64 KB */
} privesc_events SEC(".maps");

// Per-PID credential tracking (stores pre-change UID/GID).
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);  /* pid_tgid */
    __type(value, struct cred_snapshot);
} cred_tracking SEC(".maps");

// Detection counters.
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, PRIVESC_CTR_MAX);
    __type(key, __u32);
    __type(value, __u64);
} privesc_stats SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

static __always_inline void bump_privesc_counter(__u32 idx)
{
    __u64 *cnt = bpf_map_lookup_elem(&privesc_stats, &idx);
    if (cnt)
        __sync_fetch_and_add(cnt, 1);
}

static __always_inline void emit_privesc_event(__u32 pid, __u32 uid, __u32 gid,
                                               __u16 syscall_nr, __u8 severity,
                                               __u32 old_uid, __u32 new_uid)
{
    struct security_event *evt;

    evt = bpf_ringbuf_reserve(&privesc_events, sizeof(*evt), 0);
    if (!evt)
        return;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_PRIVILEGE_ESCALATION;
    evt->severity   = severity;
    evt->syscall_nr = syscall_nr;
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    evt->old_uid    = old_uid;
    evt->new_uid    = new_uid;
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    bpf_ringbuf_submit(evt, 0);
}

/* ---- raw tracepoint on sys_enter -------------------------------------- */

struct sys_enter_args {
    unsigned long long unused;
    long               id;
    unsigned long      args[6];
};

SEC("raw_tracepoint/sys_enter")
int privesc_syscall_monitor(struct bpf_raw_tracepoint_args *raw_ctx)
{
    struct sys_enter_args *regs = (struct sys_enter_args *)raw_ctx->args[0];
    long syscall_nr = 0;

    bpf_probe_read_kernel(&syscall_nr, sizeof(syscall_nr), &regs->id);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 current_uid = (__u32)uid_gid;
    __u32 current_gid = (__u32)(uid_gid >> 32);

    unsigned long arg0 = 0;
    unsigned long arg1 = 0;

    switch (syscall_nr) {
    case SYS_SETUID:
        /* setuid(uid_t uid) -- arg0 is target UID */
        bpf_probe_read_kernel(&arg0, sizeof(arg0), &regs->args[0]);

        if (arg0 == 0 && current_uid != 0) {
            /* Non-root process trying to become root. */
            bump_privesc_counter(PRIVESC_CTR_SETUID);
            bump_privesc_counter(PRIVESC_CTR_ALERTS);
            emit_privesc_event(pid, current_uid, current_gid,
                               (__u16)syscall_nr, SEC_SEV_CRITICAL,
                               current_uid, (__u32)arg0);
        }

        /* Store pre-change snapshot for correlation. */
        {
            struct cred_snapshot snap = {};
            snap.uid = current_uid;
            snap.gid = current_gid;
            snap.timestamp = bpf_ktime_get_ns();
            bpf_map_update_elem(&cred_tracking, &pid_tgid, &snap, BPF_ANY);
        }
        break;

    case SYS_SETGID:
        /* setgid(gid_t gid) -- arg0 is target GID */
        bpf_probe_read_kernel(&arg0, sizeof(arg0), &regs->args[0]);

        if (arg0 == 0 && current_gid != 0) {
            bump_privesc_counter(PRIVESC_CTR_SETGID);
            bump_privesc_counter(PRIVESC_CTR_ALERTS);
            emit_privesc_event(pid, current_uid, current_gid,
                               (__u16)syscall_nr, SEC_SEV_HIGH,
                               current_uid, (__u32)arg0);
        }
        break;

    case SYS_SETREUID:
        /* setreuid(uid_t ruid, uid_t euid) -- check euid (arg1) */
        bpf_probe_read_kernel(&arg1, sizeof(arg1), &regs->args[1]);

        if (arg1 == 0 && current_uid != 0) {
            bump_privesc_counter(PRIVESC_CTR_SETUID);
            bump_privesc_counter(PRIVESC_CTR_ALERTS);
            emit_privesc_event(pid, current_uid, current_gid,
                               (__u16)syscall_nr, SEC_SEV_CRITICAL,
                               current_uid, (__u32)arg1);
        }
        break;

    case SYS_SETREGID:
        /* setregid(gid_t rgid, gid_t egid) -- check egid (arg1) */
        bpf_probe_read_kernel(&arg1, sizeof(arg1), &regs->args[1]);

        if (arg1 == 0 && current_gid != 0) {
            bump_privesc_counter(PRIVESC_CTR_SETGID);
            bump_privesc_counter(PRIVESC_CTR_ALERTS);
            emit_privesc_event(pid, current_uid, current_gid,
                               (__u16)syscall_nr, SEC_SEV_HIGH,
                               current_uid, (__u32)arg1);
        }
        break;

    case SYS_SETRESUID:
        /* setresuid(uid_t ruid, uid_t euid, uid_t suid)
         * Check euid (arg1) for escalation to root. */
        bpf_probe_read_kernel(&arg1, sizeof(arg1), &regs->args[1]);

        if (arg1 == 0 && current_uid != 0) {
            bump_privesc_counter(PRIVESC_CTR_SETUID);
            bump_privesc_counter(PRIVESC_CTR_ALERTS);
            emit_privesc_event(pid, current_uid, current_gid,
                               (__u16)syscall_nr, SEC_SEV_CRITICAL,
                               current_uid, (__u32)arg1);
        }
        break;

    case SYS_SETRESGID:
        /* setresgid(gid_t rgid, gid_t egid, gid_t sgid)
         * Check egid (arg1). */
        bpf_probe_read_kernel(&arg1, sizeof(arg1), &regs->args[1]);

        if (arg1 == 0 && current_gid != 0) {
            bump_privesc_counter(PRIVESC_CTR_SETGID);
            bump_privesc_counter(PRIVESC_CTR_ALERTS);
            emit_privesc_event(pid, current_uid, current_gid,
                               (__u16)syscall_nr, SEC_SEV_HIGH,
                               current_uid, (__u32)arg1);
        }
        break;

    case SYS_CAPSET:
        /* capset() from a container is always suspicious.
         * We cannot easily read the capability header/data from BPF,
         * so we flag any capset call and let userspace correlate. */
        bump_privesc_counter(PRIVESC_CTR_CAPSET);
        bump_privesc_counter(PRIVESC_CTR_ALERTS);
        emit_privesc_event(pid, current_uid, current_gid,
                           (__u16)syscall_nr, SEC_SEV_HIGH,
                           current_uid, current_uid);
        break;

    default:
        return 0;
    }

    return 0;
}

/* ---- kprobe/commit_creds ---------------------------------------------- */

/*
 * commit_creds() is the kernel function that applies new credentials to
 * the current task.  By hooking it we catch ALL credential changes,
 * including those from setuid binaries and capability inheritance.
 *
 * commit_creds(struct cred *new)
 *
 * We read the new UID from the cred struct and compare against the
 * current (pre-commit) UID.
 */
SEC("kprobe/commit_creds")
int BPF_KPROBE(privesc_commit_creds, void *new_cred)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 current_uid = (__u32)uid_gid;
    __u32 current_gid = (__u32)(uid_gid >> 32);

    /* Read the new UID from struct cred.
     * struct cred layout (approximate):
     *   offset  4: uid_t uid
     *   offset  8: uid_t gid
     *   offset 12: uid_t suid
     *   offset 16: uid_t sgid
     *   offset 20: uid_t euid
     *   offset 24: uid_t egid
     *
     * We check euid (offset 20) for privilege escalation. */
    __u32 new_euid = 0;
    bpf_probe_read_kernel(&new_euid, sizeof(new_euid),
                          (void *)new_cred + 20);

    /* Only flag transitions TO root FROM non-root. */
    if (new_euid == 0 && current_uid != 0) {
        bump_privesc_counter(PRIVESC_CTR_CRED_CHG);
        bump_privesc_counter(PRIVESC_CTR_ALERTS);
        emit_privesc_event(pid, current_uid, current_gid,
                           0 /* no specific syscall */, SEC_SEV_CRITICAL,
                           current_uid, new_euid);
    }

    /* Also check for dangerous capability additions.
     * struct cred has cap_effective at a further offset.  The exact
     * offset is kernel-version dependent; we read from the known
     * position and check the relevant bits.
     *
     * cap_effective is a kernel_cap_t, which on modern kernels is
     * a __u64.  Offset ~40 on most x86_64 kernels. */
    __u64 cap_eff = 0;
    bpf_probe_read_kernel(&cap_eff, sizeof(cap_eff),
                          (void *)new_cred + 40);

    /* Check for dangerous capabilities being gained. */
    __u64 dangerous_caps = (1ULL << CAP_SYS_ADMIN) |
                           (1ULL << CAP_NET_ADMIN) |
                           (1ULL << CAP_SYS_PTRACE) |
                           (1ULL << CAP_DAC_OVERRIDE);

    if (cap_eff & dangerous_caps) {
        /* Only alert if the process is in a container context.
         * We use cgroup_id as a proxy -- non-root cgroups indicate
         * container context.  Cgroup ID of 1 is typically the root. */
        __u64 cgroup_id = bpf_get_current_cgroup_id();
        if (cgroup_id > 1) {
            bump_privesc_counter(PRIVESC_CTR_CRED_CHG);

            struct security_event *evt;
            evt = bpf_ringbuf_reserve(&privesc_events, sizeof(*evt), 0);
            if (!evt)
                return 0;

            __builtin_memset(evt, 0, sizeof(*evt));
            evt->timestamp  = bpf_ktime_get_ns();
            evt->pid        = pid;
            evt->uid        = current_uid;
            evt->gid        = current_gid;
            evt->event_type = SEC_PRIVILEGE_ESCALATION;
            evt->severity   = SEC_SEV_HIGH;
            evt->cgroup_id  = cgroup_id;
            evt->old_uid    = current_uid;
            evt->new_uid    = new_euid;
            bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

            bpf_ringbuf_submit(evt, 0);
        }
    }

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
