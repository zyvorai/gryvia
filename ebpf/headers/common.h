/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
#ifndef __COMMON_H__
#define __COMMON_H__

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define MAX_ENTRIES 65536
#define TASK_COMM_LEN 16

/* Flow event structure shared with userspace */
struct flow_event {
    __u64 timestamp;
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8  protocol;
    __u8  verdict;  /* 0=forward, 1=drop, 2=reject */
    __u32 bytes;
    __u64 latency_ns;
    __u32 pid;
    char  comm[TASK_COMM_LEN];
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
