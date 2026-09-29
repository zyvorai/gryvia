/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
#ifndef __COMMON_H__
#define __COMMON_H__

#include "gryvia_core.h"

#define MAX_ENTRIES 65536

/*
 * Flow event structure shared with userspace.  All padding is explicit so the
 * layout is identical on every arch and matches rawFlowEvent in
 * collector/pkg/decoder/decoder.go (64 bytes total).
 */
struct flow_event {
    __u64 timestamp;
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8  protocol;
    __u8  verdict;  /* 0=forward, 1=drop, 2=reject */
    __u8  _pad1[2];
    __u32 bytes;
    __u32 _pad2;
    __u64 latency_ns;
    __u32 pid;
    char  comm[TASK_COMM_LEN];
    __u32 _pad3;
};

/* Connection tracking entry */
struct conn_info {
    __u64 start_ns;
    __u64 bytes_sent;
    __u64 bytes_recv;
    __u32 retransmits;
    __u32 drops;
};

#endif /* __COMMON_H__ */
