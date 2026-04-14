/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
#ifndef __GPU_COMMON_H__
#define __GPU_COMMON_H__

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#define MAX_RANKS 512
#define NCCL_OP_NAMES_LEN 8

/* GPU event types */
enum gpu_event_type {
    GPU_EVT_NCCL_OP = 1,
    GPU_EVT_MEM_TRANSFER = 2,
    GPU_EVT_RDMA_SEND = 3,
    GPU_EVT_RDMA_RECV = 4,
    GPU_EVT_CUDA_LAUNCH = 5,
    GPU_EVT_CUDA_SYNC = 6,
};

/* Memory transfer directions */
enum mem_direction {
    MEM_H2D = 0,    /* Host to Device */
    MEM_D2H = 1,    /* Device to Host */
    MEM_D2D = 2,    /* Device to Device */
    MEM_PEER = 3,   /* Peer (cross-GPU) */
};

/* NCCL collective operation types */
enum nccl_op_type {
    NCCL_ALLREDUCE = 0,
    NCCL_ALLGATHER = 1,
    NCCL_BROADCAST = 2,
    NCCL_REDUCE = 3,
    NCCL_REDUCESCATTER = 4,
    NCCL_SEND = 5,
    NCCL_RECV = 6,
    NCCL_ALLTOALL = 7,
};

/* GPU event structure shared with userspace */
struct gpu_event {
    __u64 timestamp;
    __u32 pid;
    __u32 gpu_id;
    __u8  event_type;     /* gpu_event_type */
    __u8  direction;      /* mem_direction */
    __u8  nccl_op;        /* nccl_op_type */
    __u8  _pad;
    __u64 bytes;
    __u64 latency_ns;
    __u32 src_rank;
    __u32 dst_rank;
    __u32 collective_id;
    __u32 world_size;
    char  comm[16];
};

/* NCCL in-flight operation tracking */
struct nccl_inflight {
    __u64 start_ns;
    __u8  op_type;
    __u32 count;
    __u32 rank;
};

/* CUDA memory operation tracking */
struct cuda_mem_inflight {
    __u64 start_ns;
    __u64 size;
    __u8  direction;
};

/* RDMA queue pair tracking */
struct rdma_qp_info {
    __u64 bytes_sent;
    __u64 bytes_recv;
    __u64 last_send_ns;
    __u32 retransmits;
    __u32 completions;
};

#endif /* __GPU_COMMON_H__ */
