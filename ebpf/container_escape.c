// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// container_escape.c - Detect container escape attempts via syscall and
//                      file-access monitoring.
//
// Hooks raw_tracepoint/sys_enter for suspicious syscalls (unshare, setns,
// mount, pivot_root, ptrace, execve) and kprobe/security_file_open for
// access to sensitive host paths from container context.

#include "headers/common.h"
#include "headers/security_common.h"

/* Syscall numbers for x86_64 */
#define SYS_MOUNT        165
#define SYS_UNSHARE      272
#define SYS_SETNS        308
#define SYS_PIVOT_ROOT   155
#define SYS_PTRACE       101
#define SYS_EXECVE       59

/* Clone flags used for namespace manipulation */
#define CLONE_NEWNS   0x00020000
#define CLONE_NEWUSER 0x10000000

/* ptrace request codes */
#define PTRACE_ATTACH 16

/* Escape counter indices */
#define ESCAPE_CTR_UNSHARE     0
#define ESCAPE_CTR_SETNS       1
#define ESCAPE_CTR_MOUNT       2
#define ESCAPE_CTR_PIVOT       3
#define ESCAPE_CTR_PTRACE      4
#define ESCAPE_CTR_EXECVE      5
#define ESCAPE_CTR_FILE_OPEN   6
#define ESCAPE_CTR_MAX         7

/* ---- BPF maps --------------------------------------------------------- */

// Ring buffer for emitting security events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 128 * 1024);  /* 128 KB */
} escape_events SEC(".maps");

// Hash of known container PIDs (populated from userspace via cgroup scanning).
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);   /* PID */
    __type(value, __u64); /* cgroup_id */
} container_pids SEC(".maps");

// Configurable syscall watch list (index -> syscall number).
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 32);
    __type(key, __u32);
    __type(value, __u32);
} sensitive_syscalls SEC(".maps");

// Per-type escape attempt counters.
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, ESCAPE_CTR_MAX);
    __type(key, __u32);
    __type(value, __u64);
} escape_counter SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

static __always_inline int is_container_pid(__u32 pid)
{
    return bpf_map_lookup_elem(&container_pids, &pid) != NULL;
}

static __always_inline void bump_counter(__u32 idx)
{
    __u64 *cnt = bpf_map_lookup_elem(&escape_counter, &idx);
    if (cnt)
        __sync_fetch_and_add(cnt, 1);
}

static __always_inline void emit_escape_event(__u32 pid, __u32 uid, __u32 gid,
                                              __u16 syscall_nr, __u8 severity,
                                              __u64 cgroup_id)
{
    struct security_event *evt;

    evt = bpf_ringbuf_reserve(&escape_events, sizeof(*evt), 0);
    if (!evt)
        return;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_CONTAINER_ESCAPE;
    evt->severity   = severity;
    evt->syscall_nr = syscall_nr;
    evt->cgroup_id  = cgroup_id;
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    bpf_ringbuf_submit(evt, 0);
}

/* ---- raw tracepoint on sys_enter -------------------------------------- */

// struct used by raw_tracepoint/sys_enter
struct sys_enter_args {
    unsigned long long unused;
    long               id;
    unsigned long      args[6];
};

SEC("raw_tracepoint/sys_enter")
int escape_syscall_monitor(struct bpf_raw_tracepoint_args *raw_ctx)
{
    struct sys_enter_args *regs = (struct sys_enter_args *)raw_ctx->args[0];
    long syscall_nr = 0;

    bpf_probe_read_kernel(&syscall_nr, sizeof(syscall_nr), &regs->id);

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    /* Only care about container processes. */
    if (!is_container_pid(pid))
        return 0;

    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 uid = (__u32)uid_gid;
    __u32 gid = (__u32)(uid_gid >> 32);
    __u64 cgroup_id = bpf_get_current_cgroup_id();

    unsigned long arg0 = 0;

    switch (syscall_nr) {
    case SYS_UNSHARE:
        /* Detect unshare(CLONE_NEWNS | CLONE_NEWUSER) - namespace manip */
        bpf_probe_read_kernel(&arg0, sizeof(arg0), &regs->args[0]);
        if (arg0 & (CLONE_NEWNS | CLONE_NEWUSER)) {
            bump_counter(ESCAPE_CTR_UNSHARE);
            emit_escape_event(pid, uid, gid, (__u16)syscall_nr,
                              SEC_SEV_CRITICAL, cgroup_id);
        }
        break;

    case SYS_SETNS:
        /* Detect setns() - entering host namespaces.
         * Any setns from a container is suspicious; path check is done
         * via the file-open kprobe below. */
        bump_counter(ESCAPE_CTR_SETNS);
        emit_escape_event(pid, uid, gid, (__u16)syscall_nr,
                          SEC_SEV_CRITICAL, cgroup_id);
        break;

    case SYS_MOUNT:
        /* mount() from within a container is highly suspicious. */
        bump_counter(ESCAPE_CTR_MOUNT);
        emit_escape_event(pid, uid, gid, (__u16)syscall_nr,
                          SEC_SEV_HIGH, cgroup_id);
        break;

    case SYS_PIVOT_ROOT:
        /* pivot_root() from container -- changing root filesystem. */
        bump_counter(ESCAPE_CTR_PIVOT);
        emit_escape_event(pid, uid, gid, (__u16)syscall_nr,
                          SEC_SEV_CRITICAL, cgroup_id);
        break;

    case SYS_PTRACE:
        /* ptrace(PTRACE_ATTACH, ...) from container. */
        bpf_probe_read_kernel(&arg0, sizeof(arg0), &regs->args[0]);
        if (arg0 == PTRACE_ATTACH) {
            unsigned long target_pid = 0;
            bpf_probe_read_kernel(&target_pid, sizeof(target_pid),
                                  &regs->args[1]);
            /* Targeting a PID outside the container is an escape vector. */
            __u32 tpid = (__u32)target_pid;
            if (!is_container_pid(tpid)) {
                bump_counter(ESCAPE_CTR_PTRACE);
                emit_escape_event(pid, uid, gid, (__u16)syscall_nr,
                                  SEC_SEV_CRITICAL, cgroup_id);
            }
        }
        break;

    case SYS_EXECVE:
        /* Detect execution of known escape tools from container context.
         * Comm will be updated after exec completes; we log the event
         * for the caller comm that initiated it. */
        bump_counter(ESCAPE_CTR_EXECVE);
        emit_escape_event(pid, uid, gid, (__u16)syscall_nr,
                          SEC_SEV_MEDIUM, cgroup_id);
        break;

    default:
        break;
    }

    return 0;
}

/* ---- kprobe on security_file_open ------------------------------------- */

/*
 * security_file_open is called on every file open.  We look for access to
 * paths that indicate a container escape attempt:
 *   - /proc/1/ns/mnt, /proc/1/ns/pid, /proc/1/ns/net
 *   - /proc/sys/kernel/core_pattern  (classic escape vector)
 *   - /dev/nvidia* or other host device files from container context
 */

struct file;
struct dentry;
struct qstr {
    unsigned int hash;
    unsigned int len;
    const unsigned char *name;
};

SEC("kprobe/security_file_open")
int BPF_KPROBE(escape_file_open, struct file *filp)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    if (!is_container_pid(pid))
        return 0;

    /* Read the file path from the dentry name.
     * This uses the short dentry name -- sufficient for detecting the
     * sensitive file names we care about. */
    char buf[MAX_PATH_LEN];
    __builtin_memset(buf, 0, sizeof(buf));

    /* Read d_name from the dentry embedded in filp.
     * struct file -> f_path.dentry -> d_name.name */
    struct dentry *dentry = NULL;
    bpf_probe_read_kernel(&dentry, sizeof(dentry),
                          (void *)filp + 16); /* f_path.dentry offset */
    if (!dentry)
        return 0;

    const unsigned char *name = NULL;
    bpf_probe_read_kernel(&name, sizeof(name),
                          (void *)dentry + 40); /* d_name.name typical offset */
    if (!name)
        return 0;

    bpf_probe_read_kernel_str(buf, sizeof(buf), name);

    /* Check for sensitive path substrings.
     * We use simple byte-level prefix checks that the BPF verifier accepts. */
    int suspicious = 0;
    __u8 severity = SEC_SEV_HIGH;

    /* /proc/1/ns/ namespace files: check for "mnt", "pid", "net" */
    if (buf[0] == 'm' && buf[1] == 'n' && buf[2] == 't' && buf[3] == '\0') {
        suspicious = 1;
        severity = SEC_SEV_CRITICAL;
    } else if (buf[0] == 'p' && buf[1] == 'i' && buf[2] == 'd' &&
               buf[3] == '\0') {
        suspicious = 1;
        severity = SEC_SEV_CRITICAL;
    } else if (buf[0] == 'n' && buf[1] == 'e' && buf[2] == 't' &&
               buf[3] == '\0') {
        suspicious = 1;
        severity = SEC_SEV_CRITICAL;
    }
    /* core_pattern -- classic container escape */
    else if (buf[0] == 'c' && buf[1] == 'o' && buf[2] == 'r' &&
             buf[3] == 'e' && buf[4] == '_' && buf[5] == 'p' &&
             buf[6] == 'a' && buf[7] == 't') {
        suspicious = 1;
        severity = SEC_SEV_CRITICAL;
    }
    /* nvidia device files */
    else if (buf[0] == 'n' && buf[1] == 'v' && buf[2] == 'i' &&
             buf[3] == 'd' && buf[4] == 'i' && buf[5] == 'a') {
        suspicious = 1;
        severity = SEC_SEV_HIGH;
    }

    if (!suspicious)
        return 0;

    bump_counter(ESCAPE_CTR_FILE_OPEN);

    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 uid = (__u32)uid_gid;
    __u32 gid = (__u32)(uid_gid >> 32);
    __u64 cgroup_id = bpf_get_current_cgroup_id();

    struct security_event *evt;
    evt = bpf_ringbuf_reserve(&escape_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_CONTAINER_ESCAPE;
    evt->severity   = severity;
    evt->cgroup_id  = cgroup_id;
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    /* Copy the detected filename into the path field. */
    __builtin_memcpy(evt->path, buf, sizeof(evt->path));

    bpf_ringbuf_submit(evt, 0);

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
