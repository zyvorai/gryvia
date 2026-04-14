// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// cost_tracker.c - Network cost tracking via tc classifiers.
//
// Hooks tc egress and ingress to track per-pod byte counters and
// classify traffic by zone (same-zone, cross-zone, external) for
// cloud cost analysis and optimization.

#include "headers/common.h"

/* ---- structs ---------------------------------------------------------- */

struct cost_key {
    __u32 src_ip;
    __u32 dst_ip;
};

struct cost_value {
    __u64 bytes_sent;
    __u64 bytes_recv;
    __u64 packets_sent;
    __u64 packets_recv;
};

/* Zone classification from IP prefix */
struct zone_key {
    __u32 ip_prefix;    /* IP & mask */
    __u32 prefix_len;   /* CIDR prefix length */
};

/* Aggregate cost stats indices */
#define STAT_SAME_ZONE_BYTES    0
#define STAT_CROSS_ZONE_BYTES   1
#define STAT_EXTERNAL_BYTES     2
#define STAT_TOTAL_PACKETS      3
#define NUM_COST_STATS          4

/* ---- BPF maps --------------------------------------------------------- */

/* Per-pair traffic counters */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, struct cost_key);
    __type(value, struct cost_value);
} traffic_costs SEC(".maps");

/* IP prefix to zone ID mapping (populated from userspace) */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u32);       /* IP prefix (network byte order) */
    __type(value, __u32);     /* zone ID */
} zone_map SEC(".maps");

/* Aggregate cost stats (percpu for lock-free updates) */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, NUM_COST_STATS);
    __type(key, __u32);
    __type(value, __u64);
} cost_stats SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

/* Look up zone ID for an IP address by checking common prefix lengths */
static __always_inline __u32 lookup_zone(__u32 ip)
{
    /* Try /24 prefix first (most specific for pod subnets) */
    __u32 prefix24 = ip & bpf_htonl(0xFFFFFF00);
    __u32 *zone = bpf_map_lookup_elem(&zone_map, &prefix24);
    if (zone)
        return *zone;

    /* Try /16 prefix */
    __u32 prefix16 = ip & bpf_htonl(0xFFFF0000);
    zone = bpf_map_lookup_elem(&zone_map, &prefix16);
    if (zone)
        return *zone;

    /* Try /8 prefix */
    __u32 prefix8 = ip & bpf_htonl(0xFF000000);
    zone = bpf_map_lookup_elem(&zone_map, &prefix8);
    if (zone)
        return *zone;

    /* Unknown zone - treat as external */
    return 0;
}

static __always_inline void classify_and_count(__u32 src_ip, __u32 dst_ip,
                                                __u64 bytes)
{
    __u32 src_zone = lookup_zone(src_ip);
    __u32 dst_zone = lookup_zone(dst_ip);

    __u32 stat_idx;
    if (src_zone == 0 || dst_zone == 0)
        stat_idx = STAT_EXTERNAL_BYTES;
    else if (src_zone == dst_zone)
        stat_idx = STAT_SAME_ZONE_BYTES;
    else
        stat_idx = STAT_CROSS_ZONE_BYTES;

    __u64 *cnt = bpf_map_lookup_elem(&cost_stats, &stat_idx);
    if (cnt)
        *cnt += bytes;

    /* Update total packet counter */
    __u32 pkt_idx = STAT_TOTAL_PACKETS;
    __u64 *pkt_cnt = bpf_map_lookup_elem(&cost_stats, &pkt_idx);
    if (pkt_cnt)
        (*pkt_cnt)++;
}

static __always_inline int parse_ip_header(struct __sk_buff *skb,
                                            __u32 *src_ip, __u32 *dst_ip,
                                            __u64 *pkt_bytes)
{
    void *data     = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    /* Ethernet header */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return -1;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return -1;

    /* IPv4 header */
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return -1;

    *src_ip    = iph->saddr;
    *dst_ip    = iph->daddr;
    *pkt_bytes = (__u64)bpf_ntohs(iph->tot_len);

    return 0;
}

/* ---- tc classifiers --------------------------------------------------- */

/* Egress: track outbound traffic */
SEC("classifier/egress")
int cost_tracker_egress(struct __sk_buff *skb)
{
    __u32 src_ip = 0, dst_ip = 0;
    __u64 pkt_bytes = 0;

    if (parse_ip_header(skb, &src_ip, &dst_ip, &pkt_bytes) < 0)
        return 0;  /* TC_ACT_OK */

    /* Update per-pair counters */
    struct cost_key key = {};
    key.src_ip = src_ip;
    key.dst_ip = dst_ip;

    struct cost_value *val = bpf_map_lookup_elem(&traffic_costs, &key);
    if (val) {
        __sync_fetch_and_add(&val->bytes_sent, pkt_bytes);
        __sync_fetch_and_add(&val->packets_sent, 1);
    } else {
        struct cost_value new_val = {};
        new_val.bytes_sent   = pkt_bytes;
        new_val.packets_sent = 1;
        bpf_map_update_elem(&traffic_costs, &key, &new_val, BPF_NOEXIST);
    }

    /* Classify by zone and update aggregate stats */
    classify_and_count(src_ip, dst_ip, pkt_bytes);

    return 0;  /* TC_ACT_OK */
}

/* Ingress: track inbound traffic */
SEC("classifier/ingress")
int cost_tracker_ingress(struct __sk_buff *skb)
{
    __u32 src_ip = 0, dst_ip = 0;
    __u64 pkt_bytes = 0;

    if (parse_ip_header(skb, &src_ip, &dst_ip, &pkt_bytes) < 0)
        return 0;  /* TC_ACT_OK */

    /* Update per-pair counters (keyed by dst->src for ingress) */
    struct cost_key key = {};
    key.src_ip = dst_ip;  /* local IP is the destination on ingress */
    key.dst_ip = src_ip;  /* remote IP is the source on ingress */

    struct cost_value *val = bpf_map_lookup_elem(&traffic_costs, &key);
    if (val) {
        __sync_fetch_and_add(&val->bytes_recv, pkt_bytes);
        __sync_fetch_and_add(&val->packets_recv, 1);
    } else {
        struct cost_value new_val = {};
        new_val.bytes_recv   = pkt_bytes;
        new_val.packets_recv = 1;
        bpf_map_update_elem(&traffic_costs, &key, &new_val, BPF_NOEXIST);
    }

    /* Classify by zone and update aggregate stats */
    classify_and_count(src_ip, dst_ip, pkt_bytes);

    return 0;  /* TC_ACT_OK */
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
