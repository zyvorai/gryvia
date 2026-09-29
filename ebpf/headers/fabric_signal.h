/* SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause */
/*
 * fabric_signal.h - side-channel ABI for scheduler-facing fabric signals
 * (straggler.c, rdma_health.c, gds_trace.c).
 *
 * struct gpu_event is frozen at 72 bytes (see gpu_common.h) and must NOT be
 * extended; these programs emit struct fabric_signal on their own
 * `fabric_events` ring buffer instead.  Userspace mirror:
 * collector/pkg/fabric/signal.go (the layout is checked against the
 * _Static_asserts below by its unit tests).
 */
#ifndef __FABRIC_SIGNAL_H__
#define __FABRIC_SIGNAL_H__

#include "gryvia_core.h"

#define FABRIC_RINGBUF_SIZE   (256 * 1024)
#define FABRIC_MAX_RANKS      512
#define FABRIC_COMM_LEN       16

enum fabric_signal_type {
	FABRIC_SIG_STRAGGLER   = 1, /* rank skew on a collective */
	FABRIC_SIG_RDMA_RETRY  = 2, /* QP retry / RNR above threshold */
	FABRIC_SIG_GDS         = 3, /* GPU-direct vs bounce-buffer read */
	FABRIC_SIG_OVERLAP     = 4, /* reserved: GPU idle while NCCL in flight */
};

/*
 * Per-type field use:
 *   STRAGGLER   rank/peer_rank = this / fastest rank (0 when the comm rank
 *               offset is unknown), latency_ns/peer_latency_ns = this /
 *               fastest span, bytes = payload bytes, nccl_op = nccl_op_type.
 *   RDMA_RETRY  rank = QP number, world_size = post calls and
 *               retry_count/rnr_count = retry-exceeded / RNR-exceeded error
 *               completions since the previous signal for this QP,
 *               latency_ns = time since the last post.
 *   GDS         latency_ns = call latency, bytes = bytes transferred,
 *               retry_count = 1 when the nvidia-fs kernel path was seen
 *               (confirmed direct), 0 otherwise (bounce or unknown).
 */
struct fabric_signal {
	__u64 timestamp_ns;
	__u32 pid;
	__u32 cgroup_id_lo;     /* lower 32 of cgroup id; userspace joins to pod */
	__u8  signal_type;
	__u8  nccl_op;
	__u8  _pad[2];
	__u32 rank;
	__u32 world_size;
	__u32 peer_rank;        /* fastest peer, or 0 */
	__u64 latency_ns;       /* this rank */
	__u64 peer_latency_ns;  /* reference (fastest span) */
	__u64 bytes;
	__u32 retry_count;
	__u32 rnr_count;
	char  comm[FABRIC_COMM_LEN];
};

_Static_assert(sizeof(struct fabric_signal) == 80, "fabric_signal ABI: keep in sync with collector/pkg/fabric/signal.go");
_Static_assert(__builtin_offsetof(struct fabric_signal, pid) == 8, "fabric_signal.pid offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, cgroup_id_lo) == 12, "fabric_signal.cgroup_id_lo offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, signal_type) == 16, "fabric_signal.signal_type offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, nccl_op) == 17, "fabric_signal.nccl_op offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, rank) == 20, "fabric_signal.rank offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, world_size) == 24, "fabric_signal.world_size offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, peer_rank) == 28, "fabric_signal.peer_rank offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, latency_ns) == 32, "fabric_signal.latency_ns offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, peer_latency_ns) == 40, "fabric_signal.peer_latency_ns offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, bytes) == 48, "fabric_signal.bytes offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, retry_count) == 56, "fabric_signal.retry_count offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, rnr_count) == 60, "fabric_signal.rnr_count offset");
_Static_assert(__builtin_offsetof(struct fabric_signal, comm) == 64, "fabric_signal.comm offset");

/* straggler.c: one NCCL collective span on one rank. */
struct rank_span {
	__u64 start_ns;
	__u64 end_ns;
	__u64 bytes;
	__u32 rank;
	__u32 seen;             /* 1 once exit recorded */
};
_Static_assert(sizeof(struct rank_span) == 32, "rank_span size");

/* rdma_health.c: per-QP state keyed by QP number. */
struct rdma_health_val {
	__u64 last_send_ns;
	__u32 owner_pid;        /* first process seen posting on this QP */
	__u32 cgroup_lo;
	__u32 posted;           /* post calls since the last signal */
	__u32 retries;          /* IB_WC_RETRY_EXC_ERR completions since the last signal */
	__u32 rnr;              /* IB_WC_RNR_RETRY_EXC_ERR completions since the last signal */
	__u32 _pad;
	char  comm[FABRIC_COMM_LEN];
};
_Static_assert(sizeof(struct rdma_health_val) == 48, "rdma_health_val size");

/* gds_trace.c */
struct gds_inflight {
	__u64 start_ns;
	__u64 size;
	__u32 flags;            /* GDS_FLAG_* */
	__u32 _pad;
};
_Static_assert(sizeof(struct gds_inflight) == 24, "gds_inflight size");

#define GDS_FLAG_NVFS_SEEN 1    /* nvidia-fs kernel path observed */

/*
 * Minimal CO-RE view of struct ib_wc for the completion-error hooks.  status is
 * declared with the kernel's enum type (an int would not match the enum for
 * CO-RE field relocation).  Only the values we compare against are listed;
 * they mirror include/rdma/ib_verbs.h, which follows the verbs ABI.
 */
enum ib_wc_status {
	IB_WC_SUCCESS           = 0,
	IB_WC_RETRY_EXC_ERR     = 12,
	IB_WC_RNR_RETRY_EXC_ERR = 13,
};

struct ib_wc {
	enum ib_wc_status status;
	struct ib_qp *qp;
} __attribute__((preserve_access_index));

#endif /* __FABRIC_SIGNAL_H__ */
