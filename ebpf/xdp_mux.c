// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// xdp_mux.c - one XDP owner for the RDMA-facing interface.
//
// packet_filter, dns_tracker, roce_cnp and pfc_pause cannot share an
// interface: Linux allows one XDP program per device.  This program is the
// only one the collector should attach.  Feature programs are optional tail
// calls loaded into xdp_features by userspace (index = feature slot).  A
// missing slot is a no-op and this program returns XDP_PASS.  Tail-call depth
// is 1, so feature programs must not tail-call back into the mux.
//
// LIMIT: a successful bpf_tail_call does not return.  The first populated
// slot (lowest index) runs and its verdict is the packet's verdict; later
// slots do not run.  So this is a priority selector over one feature program,
// NOT a way to run roce_cnp, pfc_pause, dns_tracker and packet_filter side by
// side.  Running several at once needs each feature to chain to the next
// (they do not today) or an XDP dispatcher such as libxdp.  The feature
// programs here are observers that return XDP_PASS, so the mux passes every
// packet through today; it would not if a slot held a program that returns
// XDP_DROP.
//
// Slot map (collector must agree):
//   0  roce_cnp
//   1  pfc_pause
//   2  dns_tracker
//   3  packet_filter   (observe-only build; a drop rule still belongs in Cilium)
//   4  roce_ecn

#include "headers/gryvia_core.h"

#define XDP_SLOT_ROCE_CNP      0
#define XDP_SLOT_PFC_PAUSE     1
#define XDP_SLOT_DNS           2
#define XDP_SLOT_PACKET_FILTER 3
#define XDP_SLOT_ROCE_ECN      4
#define XDP_SLOT_MAX           5

struct {
	__uint(type, BPF_MAP_TYPE_PROG_ARRAY);
	__uint(max_entries, XDP_SLOT_MAX);
	__type(key, __u32);
	__type(value, __u32);
} xdp_features SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 2); /* 0 = seen, 1 = tail-call miss (slot empty) */
	__type(key, __u32);
	__type(value, __u64);
} xdp_mux_stats SEC(".maps");

static __always_inline void bump(__u32 slot)
{
	__u64 *v = bpf_map_lookup_elem(&xdp_mux_stats, &slot);

	if (v)
		*v += 1;
}

SEC("xdp")
int gryvia_xdp_mux(struct xdp_md *ctx)
{
	bump(0);
	/* Tried in slot order.  An empty slot makes the call fall through to the
	 * next; the first populated slot takes over and never returns here. */
	bpf_tail_call(ctx, &xdp_features, XDP_SLOT_ROCE_CNP);
	bpf_tail_call(ctx, &xdp_features, XDP_SLOT_PFC_PAUSE);
	bpf_tail_call(ctx, &xdp_features, XDP_SLOT_DNS);
	bpf_tail_call(ctx, &xdp_features, XDP_SLOT_PACKET_FILTER);
	bpf_tail_call(ctx, &xdp_features, XDP_SLOT_ROCE_ECN);
	bump(1);
	return XDP_PASS;
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
