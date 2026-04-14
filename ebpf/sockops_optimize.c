// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// sockops_optimize.c - Socket optimization via sockops and sk_msg redirect.
//
// Intercepts socket operations to detect local (same-node) connections and
// redirects traffic between them via bpf_msg_redirect_hash, bypassing the
// kernel TCP/IP stack for improved latency and throughput on intra-node
// communication.

#include "headers/common.h"
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/* ---- structs ---------------------------------------------------------- */

struct sock_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
};

/* Trusted service pair for selective bypass */
struct trusted_pair {
    __u32 ip_a;
    __u32 ip_b;
    __u16 port;
    __u16 _pad;
};

/* Bypass statistics indices */
#define STAT_REDIRECTED_BYTES   0
#define STAT_REDIRECTED_PKTS    1
#define STAT_ESTABLISHED_LOCAL  2
#define STAT_TOTAL_ESTABLISHED  3
#define NUM_BYPASS_STATS        4

/* ---- BPF maps --------------------------------------------------------- */

/* Socket hash map for message redirection */
struct {
    __uint(type, BPF_MAP_TYPE_SOCKHASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, struct sock_key);
    __type(value, int);
} sock_hash SEC(".maps");

/* Bypass statistics (percpu for lock-free updates) */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, NUM_BYPASS_STATS);
    __type(key, __u32);
    __type(value, __u64);
} bypass_stats SEC(".maps");

/* Trusted service pairs (populated from userspace) */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, struct trusted_pair);
    __type(value, __u8);
} trusted_pairs SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

static __always_inline void update_stat(__u32 idx, __u64 delta)
{
    __u64 *cnt = bpf_map_lookup_elem(&bypass_stats, &idx);
    if (cnt)
        *cnt += delta;
}

static __always_inline void extract_key_from_ops(struct bpf_sock_ops *skops,
                                                  struct sock_key *key)
{
    key->src_ip   = skops->local_ip4;
    key->dst_ip   = skops->remote_ip4;
    key->src_port = skops->local_port;
    /* remote_port is in network byte order in sockops */
    key->dst_port = bpf_ntohl(skops->remote_port) >> 16;
}

/* ---- sockops program -------------------------------------------------- */

SEC("sockops")
int bpf_sockops(struct bpf_sock_ops *skops)
{
    __u32 op = skops->op;

    switch (op) {
    case BPF_SOCK_OPS_ACTIVE_ESTABLISHED_CB:
    case BPF_SOCK_OPS_PASSIVE_ESTABLISHED_CB: {
        /* Only handle IPv4 */
        if (skops->family != AF_INET)
            return 0;

        update_stat(STAT_TOTAL_ESTABLISHED, 1);

        /* Check if both endpoints are local (same /8 or loopback).
         * In a real deployment, userspace would populate trusted_pairs
         * with known local pod IP ranges. For now, we add all IPv4
         * connections to the sockhash and let sk_msg decide. */
        __u32 local_ip  = skops->local_ip4;
        __u32 remote_ip = skops->remote_ip4;

        /* Skip if not same subnet (basic check: same /16 prefix) */
        if ((local_ip & 0xFFFF0000) != (remote_ip & 0xFFFF0000) &&
            local_ip != 0x0100007F && remote_ip != 0x0100007F)
            return 0;

        struct sock_key key = {};
        extract_key_from_ops(skops, &key);

        /* Add socket to sockhash for potential bypass */
        bpf_sock_hash_update(skops, &sock_hash, &key, BPF_ANY);

        update_stat(STAT_ESTABLISHED_LOCAL, 1);

        /* Also add the reverse key so the peer can find us */
        struct sock_key rev_key = {};
        rev_key.src_ip   = key.dst_ip;
        rev_key.dst_ip   = key.src_ip;
        rev_key.src_port = key.dst_port;
        rev_key.dst_port = key.src_port;
        bpf_sock_hash_update(skops, &sock_hash, &rev_key, BPF_ANY);

        /* Enable sockops callbacks for state changes */
        bpf_sock_ops_cb_flags_set(skops,
            skops->bpf_sock_ops_cb_flags | BPF_SOCK_OPS_STATE_CB_FLAG);

        break;
    }

    case BPF_SOCK_OPS_STATE_CB: {
        /* Connection state change - remove from sockhash on close */
        if (skops->args[1] == BPF_TCP_CLOSE ||
            skops->args[1] == BPF_TCP_CLOSE_WAIT) {
            struct sock_key key = {};
            extract_key_from_ops(skops, &key);
            bpf_map_delete_elem(&sock_hash, &key);

            /* Also remove reverse key */
            struct sock_key rev_key = {};
            rev_key.src_ip   = key.dst_ip;
            rev_key.dst_ip   = key.src_ip;
            rev_key.src_port = key.dst_port;
            rev_key.dst_port = key.src_port;
            bpf_map_delete_elem(&sock_hash, &rev_key);
        }
        break;
    }

    default:
        break;
    }

    return 0;
}

/* ---- sk_msg program --------------------------------------------------- */

SEC("sk_msg")
int bpf_skmsg_redirect(struct sk_msg_md *msg)
{
    /* Build the reverse key to find the peer socket */
    struct sock_key key = {};
    key.src_ip   = msg->remote_ip4;
    key.dst_ip   = msg->local_ip4;
    key.src_port = bpf_ntohl(msg->remote_port) >> 16;
    key.dst_port = msg->local_port;

    /* Update stats */
    __u64 bytes = msg->size;
    update_stat(STAT_REDIRECTED_BYTES, bytes);
    update_stat(STAT_REDIRECTED_PKTS, 1);

    /* Redirect to peer socket, bypassing TCP/IP stack */
    return bpf_msg_redirect_hash(msg, &sock_hash, &key, BPF_F_INGRESS);
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
