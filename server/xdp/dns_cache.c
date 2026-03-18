// SPDX-License-Identifier: GPL-2.0-only
// dns_cache.c — XDP eBPF program for DNS cache fast path.
//
// Intercepts DNS queries at the network driver layer, returns cache hits
// directly via XDP_TX (zero syscalls, zero Go runtime overhead), and
// transparently passes cache misses to the existing Go recursive resolver.
//
// Supported: IPv4 UDP port 53, single-question queries, responses <= 512 bytes.
// Everything else is passed through to the kernel network stack (XDP_PASS).

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#include "dns_cache.h"

// ---------------------------------------------------------------------------
// BPF map definitions
// ---------------------------------------------------------------------------

// cache_map: DNS cache hash map. Key = {wire-format qname + qtype},
// Value = {expire_ts, resp_len, pre-serialized response}.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, CACHE_MAP_MAX_ENTRIES);
    __type(key, struct cache_key);
    __type(value, struct cache_value);
} cache_map SEC(".maps");

// xdp_stats: per-CPU statistics counters (hit/miss/pass/error).
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, STATS_MAP_MAX_ENTRIES);
    __type(key, __u32);
    __type(value, __u64);
} xdp_stats SEC(".maps");

// scratch_map: per-CPU scratch space for qname extraction.
// Moving the tmp_qname buffer (255 bytes) off the stack and into a per-CPU
// array map keeps us within the 512-byte BPF stack limit.
// Entry 0 holds a cache_key-sized buffer used during qname parsing.
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct cache_key);
} scratch_map SEC(".maps");

// ---------------------------------------------------------------------------
// Helper: increment a per-CPU stats counter
// ---------------------------------------------------------------------------
static __always_inline void stats_inc(__u32 idx)
{
    __u64 *val = bpf_map_lookup_elem(&xdp_stats, &idx);
    if (val)
        *val += 1;  // per-CPU map: XDP runs with preemption disabled, no concurrent access
}

// ---------------------------------------------------------------------------
// Helper: write a byte to key->qname[off] with BPF verifier-safe offset clamping.
//
// The BPF verifier tracks loop-accumulated smax for qname_offset (up to 320)
// and cannot prove the value is < 255 from branch conditions alone.  The asm
// volatile barrier prevents clang from eliminating the & 0xFF mask (clang
// would otherwise see the branch-proven range and remove the mask as a no-op).
// ---------------------------------------------------------------------------
static __always_inline void safe_qname_write(struct cache_key *key, __u32 off, __u8 val)
{
    asm volatile("" : "+r"(off) :);
    key->qname[off & 0xFF] = val;
}

// ---------------------------------------------------------------------------
// Helper: compute IPv4 header checksum (RFC 1071)
// ---------------------------------------------------------------------------
static __always_inline __u16 ipv4_csum(struct iphdr *iph)
{
    __u32 sum = 0;
    __u16 *p = (__u16 *)iph;

    // IPv4 header is 20 bytes = 10 x 16-bit words (no options).
    iph->check = 0;

    #pragma unroll
    for (int i = 0; i < 10; i++)
        sum += p[i];

    sum = (sum >> 16) + (sum & 0xFFFF);
    sum += (sum >> 16);
    return (__u16)~sum;
}

// ---------------------------------------------------------------------------
// DNS header (RFC 1035 Section 4.1.1) — 12 bytes
// ---------------------------------------------------------------------------
struct dns_header {
    __u16 id;       // Transaction ID
    __u16 flags;    // QR(1) OPCODE(4) AA TC RD RA Z(3) RCODE(4)
    __u16 qdcount;  // Number of questions
    __u16 ancount;
    __u16 nscount;
    __u16 arcount;
} __attribute__((packed));

#define DNS_HEADER_LEN sizeof(struct dns_header)

// QR bit is the most significant bit of the flags field (network byte order).
#define DNS_QR_MASK 0x80  // first byte of flags, bit 7

// ---------------------------------------------------------------------------
// Main XDP program
// ---------------------------------------------------------------------------
SEC("xdp")
int xdp_dns_cache(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    // -----------------------------------------------------------------------
    // 1. Parse Ethernet header
    // -----------------------------------------------------------------------
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        goto pass;

    // Only IPv4
    if (eth->h_proto != bpf_htons(ETH_P_IP))
        goto pass;

    // -----------------------------------------------------------------------
    // 2. Parse IPv4 header (no options — IHL must be 5)
    // -----------------------------------------------------------------------
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        goto pass;

    // Only UDP
    if (iph->protocol != IPPROTO_UDP)
        goto pass;

    // We only handle standard 20-byte IPv4 headers (no IP options).
    if (iph->ihl != 5)
        goto pass;

    // -----------------------------------------------------------------------
    // 3. Parse UDP header, check destination port 53
    // -----------------------------------------------------------------------
    struct udphdr *udph = (void *)((char *)iph + sizeof(struct iphdr));
    if ((void *)(udph + 1) > data_end)
        goto pass;

    if (udph->dest != bpf_htons(53))
        goto pass;

    // -----------------------------------------------------------------------
    // 4. Parse DNS header
    // -----------------------------------------------------------------------
    struct dns_header *dnsh = (void *)(udph + 1);
    if ((void *)dnsh + DNS_HEADER_LEN > data_end)
        goto pass;

    // Must be a query (QR == 0)
    __u8 *flags_byte = (__u8 *)&dnsh->flags;
    if (flags_byte[0] & DNS_QR_MASK)
        goto pass;

    // Must have exactly one question
    if (dnsh->qdcount != bpf_htons(1))
        goto pass;

    // -----------------------------------------------------------------------
    // 5. Extract qname (wire format) with bounded loop + inline lowercase
    // -----------------------------------------------------------------------
    __u8 *qname_start = (__u8 *)dnsh + DNS_HEADER_LEN;
    if ((void *)qname_start + 1 > data_end)
        goto pass;

    // Build qname in a per-CPU scratch map to avoid exceeding the 512-byte
    // BPF stack limit. The scratch_map entry holds a cache_key struct; we
    // write the qname into it, then set qtype after extraction.  Using a
    // per-CPU map means no locking is needed and each XDP invocation gets
    // its own buffer.  The zero-initialization comes from the map itself
    // (BPF array maps are zero-filled), so the verifier sees full init
    // when we pass the pointer to bpf_map_lookup_elem(cache_map, ...).
    __u32 scratch_idx = 0;
    struct cache_key *key = bpf_map_lookup_elem(&scratch_map, &scratch_idx);
    if (!key) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    // Zero the key before reuse (per-CPU maps persist across invocations).
    __builtin_memset(key, 0, sizeof(*key));

    __u32 qname_offset = 0;

    // Bounded loop: iterate label by label. Each label is 1-byte length + N
    // bytes of data. The root label is a zero byte.
    // Kernel 5.3+ bounded loop — verifier sees the constant upper bound.
    for (int i = 0; i < MAX_QNAME_LABELS; i++) {
        // Read label length
        if ((void *)(qname_start + qname_offset + 1) > data_end)
            goto pass;

        __u8 label_len = qname_start[qname_offset];

        // Root label (end of qname)
        if (label_len == 0) {
            if (qname_offset >= MAX_QNAME_LEN)
                goto pass;
            safe_qname_write(key, qname_offset, 0);
            qname_offset++;
            break;
        }

        // RFC 1035: label length must be <= 63.
        // This also rejects compressed pointers (>= 0xC0) and any other
        // invalid label length values.
        if (label_len > 63)
            goto pass;

        // Validate: length byte + label data + current offset <= MAX_QNAME_LEN
        if (qname_offset + 1 + label_len > MAX_QNAME_LEN)
            goto pass;

        // Store label length byte.
        safe_qname_write(key, qname_offset, label_len);
        qname_offset++;

        // Bounds-check the entire label in the packet: need label_len
        // bytes starting at qname_start + qname_offset.
        __u8 *label_start = qname_start + qname_offset;
        if ((void *)(label_start + label_len) > data_end)
            goto pass;

        // Copy label bytes with inline lowercase conversion.
        // The BPF verifier cannot track that the outer bounds check
        // (label_start + label_len <= data_end) implies label_start[j]
        // is safe when j < label_len. We must add a per-byte bounds
        // check and prevent clang from optimizing it away via a compiler
        // barrier (asm volatile).
        for (__u8 j = 0; j < 63 && j < label_len; j++) {
            __u8 *cur = label_start + j;
            // Compiler barrier: prevents clang from eliminating
            // the following bounds check as "redundant".
            asm volatile("" : "+r"(cur) :);
            if ((void *)(cur + 1) > data_end)
                goto pass;
            __u8 ch = *cur;
            if (ch >= 'A' && ch <= 'Z')
                ch += 0x20;
            safe_qname_write(key, qname_offset + (__u32)j, ch);
        }
        qname_offset += label_len;
    }

    // The key is already fully populated in the scratch map — qname bytes
    // were written during the extraction loop and the struct was zeroed
    // beforehand, so all padding bytes are zero.  No extra copy needed.

    // -----------------------------------------------------------------------
    // 6. Extract qtype (2 bytes after qname)
    // -----------------------------------------------------------------------
    __u8 *qtype_ptr = qname_start + qname_offset;
    if ((void *)(qtype_ptr + 4) > data_end)  // qtype(2) + qclass(2)
        goto pass;

    // qtype is in network byte order in the packet; store in host byte order
    // for map lookup (Go side stores in host byte order).
    key->qtype = (qtype_ptr[0] << 8) | qtype_ptr[1];

    // -----------------------------------------------------------------------
    // 7. BPF map cache lookup + TTL expiration check
    // -----------------------------------------------------------------------
    struct cache_value *val = bpf_map_lookup_elem(&cache_map, key);
    if (!val) {
        stats_inc(STAT_MISS);
        return XDP_PASS;
    }

    // Check TTL: expire_ts is monotonic seconds.
    __u64 now_ns = bpf_ktime_get_ns();
    __u64 now_s  = now_ns / 1000000000ULL;
    if (now_s > val->expire_ts) {
        stats_inc(STAT_MISS);
        return XDP_PASS;
    }

    // Validate response length
    __u32 resp_len = val->resp_len;
    if (resp_len == 0 || resp_len > MAX_DNS_RESPONSE_LEN) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    // -----------------------------------------------------------------------
    // 8. Build XDP_TX response
    // -----------------------------------------------------------------------

    // Save original header fields before any modification.
    __u16 orig_txid    = dnsh->id;
    __u8  orig_eth_src[ETH_ALEN];
    __u8  orig_eth_dst[ETH_ALEN];
    __builtin_memcpy(orig_eth_src, eth->h_source, ETH_ALEN);
    __builtin_memcpy(orig_eth_dst, eth->h_dest, ETH_ALEN);
    __u32 orig_ip_src  = iph->saddr;
    __u32 orig_ip_dst  = iph->daddr;
    __u16 orig_udp_src = udph->source;

    // Calculate desired total packet length:
    //   ETH(14) + IP(20) + UDP(8) + DNS response(resp_len)
    // Use resp_len (not MAX_DNS_RESPONSE_LEN) so that bpf_xdp_adjust_tail
    // requests only the needed growth — large deltas can fail on some
    // configurations (e.g. generic XDP on loopback).
    int desired_len = sizeof(struct ethhdr) + sizeof(struct iphdr)
                    + sizeof(struct udphdr) + (__u32)resp_len;
    int current_len = data_end - data;
    int delta       = desired_len - current_len;

    // Adjust packet tail size
    if (bpf_xdp_adjust_tail(ctx, delta)) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    // After adjust_tail, all pointers are invalidated — re-derive them.
    data     = (void *)(long)ctx->data;
    data_end = (void *)(long)ctx->data_end;

    eth = data;
    if ((void *)(eth + 1) > data_end) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    udph = (void *)((char *)iph + sizeof(struct iphdr));
    if ((void *)(udph + 1) > data_end) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    __u8 *dns_payload = (__u8 *)(udph + 1);
    // Bounds check: ensure the full resp_len region is within the packet.
    // resp_len has been validated: 0 < resp_len <= MAX_DNS_RESPONSE_LEN (512).
    // We use the compile-time constant for the verifier's benefit; if the
    // packet was correctly resized, the actual resp_len region fits.
    if ((void *)(dns_payload + resp_len) > data_end) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    // 8a. Swap Ethernet addresses: new dst = original src, new src = original dst
    __builtin_memcpy(eth->h_dest, orig_eth_src, ETH_ALEN);
    __builtin_memcpy(eth->h_source, orig_eth_dst, ETH_ALEN);

    // 8b. Swap IP addresses + update IP header fields
    iph->saddr   = orig_ip_dst;
    iph->daddr   = orig_ip_src;
    iph->tot_len = bpf_htons(sizeof(struct iphdr) + sizeof(struct udphdr) + resp_len);
    iph->ttl     = 64;
    iph->check   = ipv4_csum(iph);

    // 8c. Swap UDP ports + update UDP header fields
    udph->source = bpf_htons(53);
    udph->dest   = orig_udp_src;
    udph->len    = bpf_htons(sizeof(struct udphdr) + resp_len);
    udph->check  = 0;  // UDP checksum optional for IPv4

    // 8d. Copy pre-serialized DNS response into packet.
    // resp_len was validated: 0 < resp_len <= MAX_DNS_RESPONSE_LEN (512).
    // We use bpf_xdp_store_bytes() which supports variable-length writes to
    // XDP packet data from a source buffer. This avoids needing a compile-time
    // constant for __builtin_memcpy and allows the packet to be sized exactly
    // to resp_len.
    __u32 dns_offset = sizeof(struct ethhdr) + sizeof(struct iphdr)
                     + sizeof(struct udphdr);
    if (bpf_xdp_store_bytes(ctx, dns_offset, val->response, resp_len)) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }

    // 8e. Patch Transaction ID: first 2 bytes of DNS response = original query TxID.
    // dnsh->id is in network byte order; copy it as-is.
    //
    // Verifier appeasement: bpf_xdp_store_bytes() invalidates all packet
    // pointers, so dns_payload (derived before the helper call) is no longer
    // proven in-bounds even though we already checked resp_len bytes at
    // line 357.  Without this re-check the verifier rejects the program with
    // "invalid access to packet" (confirmed by removing it and loading).
    if ((void *)(dns_payload + 2) > data_end) {
        stats_inc(STAT_ERROR);
        return XDP_PASS;
    }
    __builtin_memcpy(dns_payload, &orig_txid, sizeof(orig_txid));

    // Cache hit served via XDP_TX
    stats_inc(STAT_HIT);
    return XDP_TX;

pass:
    stats_inc(STAT_PASS);
    return XDP_PASS;
}

char LICENSE[] SEC("license") = "GPL";
