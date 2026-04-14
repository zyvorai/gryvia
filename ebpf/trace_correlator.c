// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
//
// trace_correlator.c - Distributed trace correlation via tc ingress.
//
// Parses incoming TCP packets for HTTP headers containing W3C traceparent
// headers.  Extracts trace-id and span-id and stores them keyed by the
// connection's 4-tuple, enabling correlation of eBPF-observed network
// events with application-level distributed traces.

#include "headers/common.h"

/* Maximum payload bytes to scan for traceparent header */
#define MAX_PAYLOAD_SCAN 512

/* traceparent header format:
 * traceparent: 00-<32 hex trace-id>-<16 hex span-id>-<2 hex flags>
 * Example: traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
 */

/* ---- structs ---------------------------------------------------------- */

struct trace_context {
    __u64 trace_id_hi;
    __u64 trace_id_lo;
    __u64 span_id;
    __u64 parent_span_id;
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u32 pid;
    __u64 timestamp;
};

/* 4-tuple key for trace context map */
struct trace_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
};

/* ---- BPF maps --------------------------------------------------------- */

/* Trace context map: 4-tuple -> trace context */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, struct trace_key);
    __type(value, struct trace_context);
} trace_ctx_map SEC(".maps");

/* Ring buffer for trace correlation events */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 128 * 1024);
} trace_events SEC(".maps");

/* ---- helpers ---------------------------------------------------------- */

/* Convert a single hex character to its numeric value */
static __always_inline int hex_to_val(char c)
{
    if (c >= '0' && c <= '9')
        return c - '0';
    if (c >= 'a' && c <= 'f')
        return c - 'a' + 10;
    if (c >= 'A' && c <= 'F')
        return c - 'A' + 10;
    return -1;
}

/* Parse 16 hex characters into a __u64 value from packet data.
 * Returns 0 on success, -1 on failure.
 * data points to start of hex string, data_end is packet boundary. */
static __always_inline int parse_hex64(void *data, void *data_end,
                                        int offset, __u64 *result)
{
    __u64 val = 0;
    unsigned char buf[16];
    void *start = data + offset;

    if (start + 16 > data_end)
        return -1;

    /* Read 16 bytes from packet */
    #pragma unroll
    for (int i = 0; i < 16; i++) {
        unsigned char *p = start + i;
        if ((void *)(p + 1) > data_end)
            return -1;
        buf[i] = *p;
    }

    #pragma unroll
    for (int i = 0; i < 16; i++) {
        int v = hex_to_val(buf[i]);
        if (v < 0)
            return -1;
        val = (val << 4) | (__u64)v;
    }

    *result = val;
    return 0;
}

/* Search for "traceparent:" (case-insensitive for 't') in payload.
 * Only scans first MAX_PAYLOAD_SCAN bytes of TCP payload. */
static __always_inline int find_traceparent(void *payload, void *data_end,
                                             int payload_len)
{
    /* "traceparent:" is 13 chars */
    static const char needle[] = "traceparent:";
    int needle_len = 12;

    if (payload_len > MAX_PAYLOAD_SCAN)
        payload_len = MAX_PAYLOAD_SCAN;

    /* Limit search to prevent verifier issues */
    if (payload_len < needle_len)
        return -1;

    int search_limit = payload_len - needle_len;
    if (search_limit > MAX_PAYLOAD_SCAN - needle_len)
        search_limit = MAX_PAYLOAD_SCAN - needle_len;

    #pragma unroll
    for (int i = 0; i < 128; i++) {
        if (i > search_limit)
            break;

        void *pos = payload + i;
        if (pos + needle_len > data_end)
            break;

        unsigned char c0, c1, c2, c3, c4;
        c0 = *((unsigned char *)(pos));
        c1 = *((unsigned char *)(pos + 1));
        c2 = *((unsigned char *)(pos + 2));
        c3 = *((unsigned char *)(pos + 3));
        c4 = *((unsigned char *)(pos + 4));

        /* Quick check first 5 chars: "trace" or "Trace" */
        if ((c0 == 't' || c0 == 'T') &&
            c1 == 'r' && c2 == 'a' && c3 == 'c' && c4 == 'e') {
            /* Check remaining "parent:" */
            unsigned char c5, c6, c7, c8, c9, c10, c11;
            if (pos + 12 > data_end)
                break;
            c5 = *((unsigned char *)(pos + 5));
            c6 = *((unsigned char *)(pos + 6));
            c7 = *((unsigned char *)(pos + 7));
            c8 = *((unsigned char *)(pos + 8));
            c9 = *((unsigned char *)(pos + 9));
            c10 = *((unsigned char *)(pos + 10));
            c11 = *((unsigned char *)(pos + 11));

            if (c5 == 'p' && c6 == 'a' && c7 == 'r' && c8 == 'e' &&
                c9 == 'n' && c10 == 't' && c11 == ':') {
                return i + needle_len;
            }
        }
    }
    return -1;
}

/* ---- tc classifier ---------------------------------------------------- */

SEC("classifier/ingress")
int trace_correlator_ingress(struct __sk_buff *skb)
{
    void *data     = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    /* Parse Ethernet header */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return 0;  /* TC_ACT_OK */

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return 0;

    /* Parse IPv4 header */
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return 0;

    if (iph->protocol != IPPROTO_TCP)
        return 0;

    __u32 ip_hdr_len = iph->ihl * 4;
    if (ip_hdr_len < sizeof(struct iphdr))
        return 0;

    /* Parse TCP header */
    struct tcphdr *tcph = (void *)iph + ip_hdr_len;
    if ((void *)(tcph + 1) > data_end)
        return 0;

    __u32 tcp_hdr_len = tcph->doff * 4;
    if (tcp_hdr_len < sizeof(struct tcphdr))
        return 0;

    /* Calculate payload start and length */
    void *payload = (void *)tcph + tcp_hdr_len;
    if (payload >= data_end)
        return 0;

    int payload_len = data_end - payload;
    if (payload_len <= 0)
        return 0;

    /* Search for traceparent header in payload */
    int header_offset = find_traceparent(payload, data_end, payload_len);
    if (header_offset < 0)
        return 0;

    /* Skip optional whitespace after "traceparent:" */
    void *value_start = payload + header_offset;
    if (value_start + 1 > data_end)
        return 0;

    unsigned char ws = *((unsigned char *)value_start);
    if (ws == ' ')
        value_start++;

    /* Expected format: "00-<32 hex>-<16 hex>-<2 hex>" = 55 chars */
    if (value_start + 55 > data_end)
        return 0;

    /* Verify version prefix "00-" */
    unsigned char v0, v1, v2;
    v0 = *((unsigned char *)value_start);
    v1 = *((unsigned char *)(value_start + 1));
    v2 = *((unsigned char *)(value_start + 2));
    if (v0 != '0' || v1 != '0' || v2 != '-')
        return 0;

    /* Parse trace-id (32 hex chars = 2x __u64) at offset 3 */
    __u64 trace_id_hi = 0, trace_id_lo = 0, span_id = 0;

    if (parse_hex64(value_start, data_end, 3, &trace_id_hi) < 0)
        return 0;
    if (parse_hex64(value_start, data_end, 19, &trace_id_lo) < 0)
        return 0;

    /* Verify dash separator at offset 35 */
    if (value_start + 36 > data_end)
        return 0;
    unsigned char dash = *((unsigned char *)(value_start + 35));
    if (dash != '-')
        return 0;

    /* Parse span-id (16 hex chars = __u64) at offset 36 */
    if (parse_hex64(value_start, data_end, 36, &span_id) < 0)
        return 0;

    /* Store trace context keyed by 4-tuple */
    struct trace_key key = {};
    key.src_ip   = iph->saddr;
    key.dst_ip   = iph->daddr;
    key.src_port = bpf_ntohs(tcph->source);
    key.dst_port = bpf_ntohs(tcph->dest);

    struct trace_context tctx = {};
    tctx.trace_id_hi = trace_id_hi;
    tctx.trace_id_lo = trace_id_lo;
    tctx.span_id     = span_id;
    tctx.src_ip      = iph->saddr;
    tctx.dst_ip      = iph->daddr;
    tctx.src_port    = key.src_port;
    tctx.dst_port    = key.dst_port;
    tctx.timestamp   = bpf_ktime_get_ns();

    bpf_map_update_elem(&trace_ctx_map, &key, &tctx, BPF_ANY);

    /* Emit event to ring buffer */
    struct trace_context *ev;
    ev = bpf_ringbuf_reserve(&trace_events, sizeof(*ev), 0);
    if (!ev)
        return 0;

    *ev = tctx;
    bpf_ringbuf_submit(ev, 0);

    return 0;  /* TC_ACT_OK - always pass the packet */
}

char LICENSE[] SEC("license") = "Dual BSD/GPL";
