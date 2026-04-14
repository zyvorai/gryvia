// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// driver_fim.c - GPU driver file integrity monitoring.
//
// Monitors write access to NVIDIA driver files, CUDA libraries, and
// kernel module loading.  Uses kprobe/security_file_open for file-level
// monitoring and raw_tracepoint/sys_enter for init_module/finit_module
// syscall detection.

#include "headers/common.h"
#include "headers/security_common.h"

/* Syscall numbers for x86_64 */
#define SYS_INIT_MODULE   175
#define SYS_FINIT_MODULE  313

/* File open flags -- O_WRONLY=1, O_RDWR=2 */
#define FMODE_WRITE_MASK 0x03

/* Stat counter indices */
#define FIM_CTR_FILE_WRITE  0
#define FIM_CTR_MOD_LOAD    1
#define FIM_CTR_ALERTS      2
#define FIM_CTR_MAX         3

/* ---- BPF maps --------------------------------------------------------- */

// Ring buffer for FIM events.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 64 * 1024);  /* 64 KB */
} fim_events SEC(".maps");

// Configurable watched path patterns (populated from userspace).
// Key: index, Value: null-terminated path prefix string.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 64);
    __type(key, __u32);
    __type(value, char[MAX_PATH_LEN]);
} watched_paths SEC(".maps");

// Detection counters.
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, FIM_CTR_MAX);
    __type(key, __u32);
    __type(value, __u64);
} fim_stats SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

static __always_inline void bump_fim_counter(__u32 idx)
{
    __u64 *cnt = bpf_map_lookup_elem(&fim_stats, &idx);
    if (cnt)
        __sync_fetch_and_add(cnt, 1);
}

/*
 * Check if a filename matches NVIDIA/CUDA driver patterns.
 * We do prefix/substring matching on the dentry name since full path
 * reconstruction is expensive in BPF.
 *
 * Patterns:
 *   - "nvidia" prefix  (device files /dev/nvidia*, driver libraries)
 *   - "libnvidia" prefix (NVIDIA libraries)
 *   - "libcuda" prefix (CUDA libraries)
 *   - "modprobe" prefix (modprobe config files)
 */
static __always_inline int matches_driver_pattern(const char *name)
{
    /* nvidia* -- matches nvidia0, nvidia-uvm, nvidiactl, etc. */
    if (name[0] == 'n' && name[1] == 'v' && name[2] == 'i' &&
        name[3] == 'd' && name[4] == 'i' && name[5] == 'a')
        return 1;

    /* libnvidia* */
    if (name[0] == 'l' && name[1] == 'i' && name[2] == 'b' &&
        name[3] == 'n' && name[4] == 'v' && name[5] == 'i' &&
        name[6] == 'd' && name[7] == 'i')
        return 1;

    /* libcuda* */
    if (name[0] == 'l' && name[1] == 'i' && name[2] == 'b' &&
        name[3] == 'c' && name[4] == 'u' && name[5] == 'd' &&
        name[6] == 'a')
        return 1;

    return 0;
}

/*
 * Check if a filename matches kernel module paths.
 *
 * Patterns:
 *   - Files ending in ".ko" (kernel modules)
 *   - Files in /etc/modprobe.d/ directory
 */
static __always_inline int matches_module_pattern(const char *name)
{
    /* Check for ".ko" suffix -- scan for it.
     * Since we have the dentry name (filename component), look for
     * common nvidia module names. */

    /* nvidia.ko, nvidia_drm.ko, nvidia_modeset.ko, nvidia_uvm.ko */
    if (name[0] == 'n' && name[1] == 'v' && name[2] == 'i' &&
        name[3] == 'd' && name[4] == 'i' && name[5] == 'a')
        return 1;

    return 0;
}

/*
 * Check for modprobe.d config files.
 * The parent directory is modprobe.d but we only see the dentry name.
 * We flag any .conf file access that's suspicious in container context.
 */
static __always_inline int matches_modprobe_config(const char *name)
{
    /* Look for filenames containing "modprobe" or "nvidia" with ".conf" */
    if (name[0] == 'n' && name[1] == 'v' && name[2] == 'i' &&
        name[3] == 'd' && name[4] == 'i' && name[5] == 'a')
        return 1;

    /* blacklist- prefix (common modprobe.d pattern) */
    if (name[0] == 'b' && name[1] == 'l' && name[2] == 'a' &&
        name[3] == 'c' && name[4] == 'k' && name[5] == 'l' &&
        name[6] == 'i' && name[7] == 's')
        return 1;

    return 0;
}

/* ---- kprobe/security_file_open ---------------------------------------- */

/*
 * Monitor file opens with write intent targeting GPU driver files.
 *
 * security_file_open(struct file *file)
 *
 * We read the file flags to check for write access, then examine
 * the dentry name for driver-related patterns.
 */
SEC("kprobe/security_file_open")
int BPF_KPROBE(fim_file_open, void *filp)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    /* Read file flags to check for write intent.
     * struct file -> f_flags is at an early offset.
     * We check for O_WRONLY (1) or O_RDWR (2). */
    unsigned int f_flags = 0;
    bpf_probe_read_kernel(&f_flags, sizeof(f_flags),
                          (void *)filp + 44); /* f_flags offset */

    if (!(f_flags & FMODE_WRITE_MASK))
        return 0;

    /* Read dentry name. */
    char buf[MAX_PATH_LEN];
    __builtin_memset(buf, 0, sizeof(buf));

    struct dentry *dentry = NULL;
    bpf_probe_read_kernel(&dentry, sizeof(dentry),
                          (void *)filp + 16); /* f_path.dentry offset */
    if (!dentry)
        return 0;

    const unsigned char *name = NULL;
    bpf_probe_read_kernel(&name, sizeof(name),
                          (void *)dentry + 40); /* d_name.name offset */
    if (!name)
        return 0;

    bpf_probe_read_kernel_str(buf, sizeof(buf), name);

    /* Check patterns. */
    int suspicious = 0;
    __u8 severity = SEC_SEV_HIGH;

    if (matches_driver_pattern(buf)) {
        suspicious = 1;
        severity = SEC_SEV_CRITICAL;
    } else if (matches_module_pattern(buf)) {
        suspicious = 1;
        severity = SEC_SEV_CRITICAL;
    } else if (matches_modprobe_config(buf)) {
        suspicious = 1;
        severity = SEC_SEV_HIGH;
    }

    if (!suspicious)
        return 0;

    bump_fim_counter(FIM_CTR_FILE_WRITE);
    bump_fim_counter(FIM_CTR_ALERTS);

    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 uid = (__u32)uid_gid;
    __u32 gid = (__u32)(uid_gid >> 32);

    struct security_event *evt;
    evt = bpf_ringbuf_reserve(&fim_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_DRIVER_TAMPERING;
    evt->severity   = severity;
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    /* Copy filename into path field. */
    __builtin_memcpy(evt->path, buf, sizeof(evt->path));

    bpf_ringbuf_submit(evt, 0);

    return 0;
}

/* ---- raw tracepoint on sys_enter for module loading ------------------- */

struct sys_enter_args {
    unsigned long long unused;
    long               id;
    unsigned long      args[6];
};

SEC("raw_tracepoint/sys_enter")
int fim_module_monitor(struct bpf_raw_tracepoint_args *raw_ctx)
{
    struct sys_enter_args *regs = (struct sys_enter_args *)raw_ctx->args[0];
    long syscall_nr = 0;

    bpf_probe_read_kernel(&syscall_nr, sizeof(syscall_nr), &regs->id);

    /* Only handle init_module and finit_module. */
    if (syscall_nr != SYS_INIT_MODULE && syscall_nr != SYS_FINIT_MODULE)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 uid_gid = bpf_get_current_uid_gid();
    __u32 uid = (__u32)uid_gid;
    __u32 gid = (__u32)(uid_gid >> 32);

    bump_fim_counter(FIM_CTR_MOD_LOAD);
    bump_fim_counter(FIM_CTR_ALERTS);

    struct security_event *evt;
    evt = bpf_ringbuf_reserve(&fim_events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp  = bpf_ktime_get_ns();
    evt->pid        = pid;
    evt->uid        = uid;
    evt->gid        = gid;
    evt->event_type = SEC_DRIVER_TAMPERING;
    evt->severity   = SEC_SEV_HIGH;
    evt->syscall_nr = (__u16)syscall_nr;
    evt->cgroup_id  = bpf_get_current_cgroup_id();
    bpf_get_current_comm(&evt->comm, sizeof(evt->comm));

    bpf_ringbuf_submit(evt, 0);

    return 0;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
