// SPDX-License-Identifier: GPL-2.0-only
// dns_cache.h — shared struct definitions between eBPF program and Go loader.
// This header is included by dns_cache.c and the Go-side struct layout MUST
// match exactly (size, field offset, padding).

#ifndef __DNS_CACHE_H
#define __DNS_CACHE_H

#include <linux/types.h>

// Maximum DNS wire-format qname length (RFC 1035: 255 octets including root label).
#define MAX_QNAME_LEN 255

// Maximum pre-serialized DNS response length stored in BPF map.
// 512 bytes = traditional DNS UDP payload limit (no EDNS0).
#define MAX_DNS_RESPONSE_LEN 512

// Maximum number of qname labels to parse (bounded loop limit).
// RFC 1035 allows up to 127 labels (each at least 1-byte label + 1-byte length).
#define MAX_QNAME_LABELS 127

// BPF map capacities.
#define CACHE_MAP_MAX_ENTRIES 65536
#define STATS_MAP_MAX_ENTRIES 4

// Stats counter indices for xdp_stats per-CPU array map.
#define STAT_HIT   0
#define STAT_MISS  1
#define STAT_PASS  2
#define STAT_ERROR 3

// cache_key is the lookup key for the DNS cache BPF hash map.
// Layout: wire-format qname (zero-padded) + qtype (host byte order).
// Total size: 255 + 1 (padding) + 2 = 258 bytes.
struct cache_key {
    __u8  qname[MAX_QNAME_LEN]; // DNS wire-format qname, lowercase, zero-padded
    __u8  _pad;                  // explicit padding for alignment
    __u16 qtype;                 // DNS query type (host byte order)
} __attribute__((packed));

// cache_value holds a pre-serialized DNS response and its expiration time.
struct cache_value {
    __u64 expire_ts;                       // monotonic clock expiration (seconds)
    __u32 resp_len;                        // actual response wire-format length
    __u8  response[MAX_DNS_RESPONSE_LEN];  // pre-serialized DNS response
} __attribute__((packed));

#endif // __DNS_CACHE_H
