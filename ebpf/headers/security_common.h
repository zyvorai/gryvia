/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
#ifndef __SECURITY_COMMON_H__
#define __SECURITY_COMMON_H__

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#define MAX_PATH_LEN 256
#define CGROUP_PATH_LEN 128

/* Security event severity */
enum sec_severity {
    SEC_SEV_INFO = 0,
    SEC_SEV_LOW = 1,
    SEC_SEV_MEDIUM = 2,
    SEC_SEV_HIGH = 3,
    SEC_SEV_CRITICAL = 4,
};

/* Security event types */
enum sec_event_type {
    SEC_CONTAINER_ESCAPE = 1,
    SEC_PRIVILEGE_ESCALATION = 2,
    SEC_CRYPTO_MINING = 3,
    SEC_DATA_EXFILTRATION = 4,
    SEC_DRIVER_TAMPERING = 5,
    SEC_SUSPICIOUS_EXEC = 6,
    SEC_NAMESPACE_BREACH = 7,
};

/* Security event structure */
struct security_event {
    __u64 timestamp;
    __u32 pid;
    __u32 uid;
    __u32 gid;
    __u8  event_type;    /* sec_event_type */
    __u8  severity;      /* sec_severity */
    __u16 syscall_nr;
    __u64 cgroup_id;
    char  comm[16];
    char  path[MAX_PATH_LEN];
    __u32 src_ip;
    __u32 dst_ip;
    __u16 dst_port;
    __u16 _pad;
    __u64 bytes;         /* for exfiltration tracking */
    __u32 old_uid;       /* for privesc: UID before change */
    __u32 new_uid;       /* for privesc: UID after change */
};

#endif /* __SECURITY_COMMON_H__ */
