# XDP DNS Cache Fast-Path

## Problem

rec53 at 172K QPS spends 22.5% of CPU time in `sendmsg` syscalls, 29% in GC/memory allocation, and 8% in goroutine stack growth. The miekg/dns Server model creates a goroutine per incoming UDP packet — at 172K QPS that's 172K goroutine creates/destroys per second. For cache hits (the majority of production traffic), the entire Go runtime overhead is unnecessary: the answer is already known and just needs to be returned.

## Solution

Add an XDP/eBPF layer that intercepts DNS queries at the network driver level and serves cache hits entirely in kernel space — zero syscalls, zero memory copies, zero goroutine overhead. Cache misses pass through to the existing Go resolver unchanged.

## Architecture

```
                          ┌─────────────────────────────┐
                          │    Go Process (rec53)        │
                          │                              │
                          │  ┌────────────────────────┐  │
                          │  │ XDP Loader (cilium/ebpf)│  │
                          │  │  - Load/attach XDP prog │  │
                          │  │  - Sync cache → BPF map │  │
                          │  │  - TTL expiry cleanup   │  │
                          │  └────────┬───────────────┘  │
                          │           │ BPF map ops       │
                          │           ▼                   │
NIC/lo ──► XDP hook ──► eBPF program                     │
              │           │  1. Parse ETH/IP/UDP/DNS     │
              │           │  2. Extract qname + qtype    │
              │           │  3. Lookup BPF hashmap       │
              │           │                              │
              │           ├── HIT ──► Build response     │
              │           │           Swap headers       │
              │           │           XDP_TX ◄───────────┘
              │           │           (kernel-only, no userspace)
              │           │
              │           └── MISS ──► XDP_PASS
              │                          │
              ▼                          ▼
         (packet dropped           Kernel protocol stack
          or returned)                   │
                                         ▼
                                  miekg/dns Server
                                         │
                                         ▼
                                  Go iterative resolver
                                         │
                                         ▼
                                  setCacheCopy() ──► sync to BPF map
```

## Design Decisions

### D1: BPF Map Key Design

**Choice:** Fixed-size struct with lowercased, wire-format domain name.

```c
#define MAX_QNAME_LEN 255

struct cache_key {
    __u8  qname[MAX_QNAME_LEN + 1]; // wire-format: len-label-len-label-0
    __u16 qtype;
    __u8  _pad[1];                   // align to 4 bytes
};
```

**Rationale:**
- DNS names are case-insensitive (RFC 4343). Both the eBPF program and the Go sync path must lowercase domain names when constructing cache keys. **Note:** the existing Go cache (`getCacheKey`) does NOT lowercase names — the XDP sync path must add explicit `strings.ToLower()` normalization when converting Go cache keys to BPF map keys. The eBPF side lowercases during wire-format extraction (see D6).
- Wire format (length-prefixed labels) is the natural format already in the packet — no string conversion needed in eBPF.
- Fixed-size key avoids variable-length complications in BPF maps.
- 255 bytes covers the maximum DNS name length (RFC 1035 §2.3.4).
- The Go sync path must convert presentation-format names (e.g., `"www.google.com."`) to wire format (e.g., `\x03www\x06google\x03com\x00`) and lowercase them before using as BPF map keys.

### D2: BPF Map Value Design

**Choice:** Pre-serialized DNS response bytes (ready to copy into packet).

```c
#define MAX_DNS_RESPONSE_LEN 512

struct cache_value {
    __u8  response[MAX_DNS_RESPONSE_LEN]; // complete DNS response payload (after UDP header)
    __u16 resp_len;                        // actual response length
    __u32 expire_ts;                       // expiry as monotonic seconds since boot
};
```

**Rationale:**
- Pre-serialized responses avoid DNS message construction in eBPF (which would be complex and error-prone given eBPF constraints).
- The Go side serializes the DNS response once via `dns.Msg.Pack()` when writing to the BPF map. The eBPF side just copies bytes.
- 512 bytes covers the majority of DNS responses. Responses larger than 512 bytes are not cached in the XDP layer and fall through to Go. This matches the traditional DNS UDP message limit (RFC 1035 §4.2.1). EDNS0 responses larger than 512 bytes require userspace handling.
- **Clock domain (CRITICAL):** `expire_ts` uses monotonic seconds since boot to match `bpf_ktime_get_ns() / 1000000000` in the eBPF program. The Go side MUST compute this using the same monotonic clock via `unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)`, NOT `time.Now().Unix()` (which is wall-clock / UTC epoch). Using wall-clock time would cause a constant offset of ~system-uptime seconds, making all entries appear either permanently expired or permanently valid. Go sync computes: `expire_ts = uint32(monotonic_now_sec + ttl_seconds)`.
- Future enhancement: use `BPF_MAP_TYPE_PERCPU_HASH` with larger values if EDNS0 large response caching is needed.

### D3: Cache Synchronization

**Choice:** Go is the single writer; eBPF is read-only. Unidirectional sync.

```
Go setCacheCopy() ──► Pack() response ──► bpf_map_update_elem()
Go TTL cleanup loop ──► bpf_map_delete_elem()
eBPF program ──► bpf_map_lookup_elem() (read-only)
```

**Rationale:**
- Avoids bidirectional sync complexity.
- Go's `globalDnsCache` remains the source of truth. The BPF map is a derived, pre-serialized projection.
- TTL expiry is managed by Go (periodic cleanup goroutine), not eBPF (eBPF has limited timer support).
- The eBPF program also checks `expire_ts` inline as a secondary guard against serving stale entries between Go cleanup cycles.
- **Clock domain consistency:** Both Go and eBPF use monotonic clock (seconds since boot). Go uses `unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)` to compute `expire_ts = monotonic_now_sec + ttl_seconds`. eBPF uses `bpf_ktime_get_ns() / 1000000000` to get the current monotonic second. Technically, `bpf_ktime_get_ns()` calls `ktime_get_mono_fast_ns()` which may have sub-second granularity differences from userspace `CLOCK_MONOTONIC` — this is immaterial given TTL resolution in whole seconds (typically 60-3600s).

### D4: XDP Attach Mode

**Choice:** Auto-detect with fallback: native → generic.

```go
// Try native first, fall back to generic
err := link.AttachXDP(link.XDPOptions{
    Program:   prog,
    Interface: ifIndex,
    Flags:     link.XDPDriverMode,
})
if err != nil {
    err = link.AttachXDP(link.XDPOptions{
        Program:   prog,
        Interface: ifIndex,
        Flags:     link.XDPGenericMode,
    })
}
```

**Rationale:**
- loopback only supports generic mode. Physical NICs may support native (driver) mode.
- Auto-detection allows the same binary to work in both development (loopback) and production (physical NIC) environments.
- Log the actual mode used for operational visibility.

### D5: Response Construction in eBPF

**Choice:** Header swap + pre-serialized payload copy.

The eBPF program constructs the response by:
0. **Pre-validation:** Before any response construction, verify the DNS header has `QDCOUNT == 1` (network byte order). Queries with QDCOUNT != 1 are non-standard; fall through to `XDP_PASS`. Also bounds-check `val->resp_len <= MAX_DNS_RESPONSE_LEN` after `bpf_map_lookup_elem` to satisfy the eBPF verifier for subsequent memory operations.
1. Swapping Ethernet src/dst MAC addresses
2. Swapping IP src/dst addresses
3. Swapping UDP src/dst ports
4. **Resizing the packet buffer with `bpf_xdp_adjust_tail()`** — the cached response will almost always differ in size from the incoming query. This MUST happen BEFORE writing the new payload. Calculate `delta = (ETH+IP+UDP+resp_len) - original_pkt_len` and call `bpf_xdp_adjust_tail(xdp_md, delta)`. If `delta > 0` (response is larger than query), the call grows the packet; if `delta < 0`, it shrinks. If `bpf_xdp_adjust_tail()` fails (e.g., insufficient headroom), fall through to `XDP_PASS`.
5. **Re-validating data/data_end pointers** — after `bpf_xdp_adjust_tail()`, all previously cached pointers into the packet are invalidated. Must reload `data = (void *)(long)xdp_md->data` and `data_end = (void *)(long)xdp_md->data_end` and re-verify bounds before any subsequent writes.
6. Replacing the DNS payload with the pre-serialized cached response (via `__builtin_memcpy` or bounded `bpf_probe_read_kernel`). The cached `cache_value.response[]` contains a complete DNS response with a placeholder transaction ID from the original resolution.
7. Adjusting IP total length and UDP length fields to reflect the new payload size
8. Updating the DNS header: copy query ID from request → response (overwriting the placeholder ID in the cached response), set QR=1, RD=1, RA=1
9. **Setting IP TTL** to a fixed value (e.g., 64) — the incoming query's TTL may be low from intermediate hops and should not be reused as-is in the response.
10. Recalculating IP checksum (incremental or full recomputation)
11. Setting UDP checksum to 0 (optional for IPv4 UDP, RFC 768)
12. Return `XDP_TX`

**Rationale:**
- `bpf_xdp_adjust_tail()` is required because the incoming query packet and the cached response are different sizes. Without resizing, the response would either be truncated (response > query) or carry trailing garbage bytes (response < query), causing the client to reject the packet.
- Pointer re-validation after `bpf_xdp_adjust_tail()` is mandatory — the eBPF verifier enforces this. Any access to packet data without re-validation after a tail adjustment will be rejected by the verifier.
- Minimal computation in eBPF hot path — the response is pre-serialized, so construction is just a memcpy.
- Pre-serialized response means no DNS message parsing/construction in eBPF.
- Only the DNS transaction ID needs to be patched per-query (unique per client request).
- UDP checksum can be zeroed for IPv4 (valid per RFC 768). For IPv6, checksum is mandatory — handle in a future iteration.

### D6: Query Name Extraction in eBPF

**Choice:** Direct wire-format parsing with bounded loop (kernel 5.3+).

```c
// Extract query name from DNS wire format
// DNS names are a sequence of length-prefixed labels ending with 0
// Example: \x03www\x06google\x03com\x00
static __always_inline int extract_qname(void *dns_data, void *data_end,
                                          struct cache_key *key) {
    __u8 *p = dns_data + 12; // skip DNS header (12 bytes)
    int pos = 0;

    // Bounded loop — kernel 5.3+ supports bounded loops natively without
    // #pragma unroll. The verifier accepts this because MAX_QNAME_LEN is
    // a compile-time constant upper bound. Do NOT use #pragma unroll here:
    // unrolling a 255-iteration loop would bloat the BPF program by ~10x
    // and risk hitting the 1M instruction limit.
    for (int i = 0; i < MAX_QNAME_LEN && pos < MAX_QNAME_LEN; i++) {
        if (p + 1 > (__u8 *)data_end) return -1;
        __u8 ch = *p;
        if (ch == 0) {
            key->qname[pos] = 0; // null terminator
            break;
        }
        // Lowercase ASCII uppercase letters (0x41-0x5A → 0x61-0x7A).
        // Safe to apply uniformly to all bytes: DNS label length bytes are
        // always 0-63 (0x00-0x3F) which never overlap with uppercase ASCII
        // (0x41-0x5A), so length prefixes pass through unchanged.
        key->qname[pos] = (ch >= 'A' && ch <= 'Z') ? ch + 32 : ch;
        p++;
        pos++;
    }
    // Extract qtype (2 bytes after qname null terminator)
    p++; // skip the 0x00 terminator
    if (p + 2 > (__u8 *)data_end) return -1;
    key->qtype = ((__u16)p[0] << 8) | p[1]; // network byte order → host
    return 0;
}
```

**Rationale:**
- DNS query names in the Question section are never compressed (compression only appears in responses), so no pointer-following is needed.
- Linux 5.3+ eBPF verifier natively supports bounded loops with compile-time constant upper bounds — `#pragma unroll` is NOT used because unrolling 255 iterations would bloat the BPF program to ~10K+ instructions and risk hitting the verifier's 1M instruction limit. The bounded `for` loop compiles to compact branch instructions that the verifier can prove terminates.
- Case-insensitive comparison via inline lowercasing.
- Total extraction cost: ~255 byte reads max, typically 20-30 bytes.
- **Minimum kernel requirement:** 5.3 for bounded loop support (rec53 already requires 5.15+ for other BPF features).

### D7: Feature Toggle

**Choice:** Config-driven with graceful degradation.

```yaml
# config.yaml
xdp:
  enabled: false          # off by default
  interface: "lo"         # network interface for XDP attach
  cache_size: 65536       # max BPF map entries
  sync_interval: 100ms    # Go → BPF map sync frequency
  # mode: auto            # auto | native | generic (future)
```

**Rationale:**
- XDP requires CAP_BPF/root. Default off to avoid startup failures on unprivileged environments.
- Interface must be specified because rec53 could listen on different interfaces.
- Cache size is separate from Go cache size — BPF map has fixed max entries.
- **`sync_interval` semantics:** This controls the frequency of the periodic TTL cleanup / reconciliation goroutine, NOT the latency of new cache entries. New entries are written to the BPF map **immediately** (inline in the `setCacheCopy()` write path) for minimal propagation delay. The periodic goroutine only handles: (a) deleting expired entries, (b) reconciling Go-side evictions.
- **BPF map full handling:** When `bpf_map.Update()` returns `-ENOSPC` (map at `cache_size` capacity), the entry is silently dropped from the XDP cache — the Go cache still serves it. The periodic cleanup goroutine will reclaim expired entries. A `rec53_xdp_cache_sync_errors_total` counter tracks these failures for alerting.

### D8: Metrics and Observability

**Choice:** BPF per-CPU counters exported to Prometheus.

```c
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 4);
    __type(key, __u32);
    __type(value, __u64);
} xdp_stats SEC(".maps");

// Indices
#define XDP_STATS_HIT    0
#define XDP_STATS_MISS   1
#define XDP_STATS_PASS   2  // non-DNS or malformed
#define XDP_STATS_ERROR  3
```

Go periodically reads these counters and exposes them as Prometheus metrics:
- `rec53_xdp_cache_hits_total`
- `rec53_xdp_cache_misses_total`
- `rec53_xdp_pass_total`
- `rec53_xdp_errors_total`
- `rec53_xdp_cache_entries` (gauge) — current number of entries in the BPF map, updated each cleanup cycle
- `rec53_xdp_cache_sync_errors_total` — count of failed `bpf_map.Update()` calls (e.g., `-ENOSPC` when map is full)

## File Layout

```
server/
├── xdp/
│   ├── dns_cache.c          # eBPF/XDP program (~300 lines C)
│   ├── dns_cache.h          # Shared struct definitions (key, value, stats)
│   └── Makefile             # clang -target bpf compilation
├── xdp_loader.go            # Go: load eBPF, attach XDP, manage maps
├── xdp_sync.go              # Go: cache sync goroutine (Go cache → BPF map)
├── xdp_metrics.go           # Go: read BPF stats → Prometheus
└── xdp_loader_test.go       # Integration tests
```

## Build Requirements

- `clang` (>= 14) with BPF target support
- Linux kernel >= 5.15 with `CONFIG_BPF=y`, `CONFIG_XDP_SOCKETS=y`
- `CAP_BPF` + `CAP_NET_ADMIN` capabilities (or root)
- `cilium/ebpf` Go library (v0.12+)

Build the eBPF object file:
```bash
clang -O2 -g -target bpf -c server/xdp/dns_cache.c -o server/xdp/dns_cache.o
```

The Go build uses `//go:generate` and `bpf2go` from cilium/ebpf to embed the compiled eBPF object into the Go binary, eliminating runtime dependency on the .o file.

## Cache Sync Protocol

### Write path (Go → BPF map)

When `setCacheCopy()` is called (on cache miss resolution):

1. `dns.Msg.Pack()` serializes the response to wire format
2. If `len(packed) > 512`, skip XDP cache (too large for fast-path)
3. Construct `cache_key` from lowercase wire-format qname + qtype
4. Compute `expire_ts` using monotonic clock: `unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)` → `expire_ts = uint32(ts.Sec + ttl_seconds)`
5. Construct `cache_value` with packed response bytes + `expire_ts`
6. `bpf_map.Update(key, value, BPF_ANY)` — atomic upsert. If returns `-ENOSPC`, increment `rec53_xdp_cache_sync_errors_total` and skip (Go cache still serves it)

### TTL expiry (Go goroutine)

A background goroutine runs every `sync_interval`:

1. Get current monotonic time via `unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)`
2. Iterate all BPF map entries via `bpf_map.Iterate()`
3. Delete entries where `expire_ts < uint32(ts.Sec)`
4. Also reconcile: delete BPF entries not in Go cache (handles Go-side cache eviction)
5. Update `rec53_xdp_cache_entries` gauge with current map entry count

### Read path (eBPF)

1. Extract key from incoming packet
2. `bpf_map_lookup_elem(&cache_map, &key)`
3. If found AND `expire_ts > bpf_ktime_get_ns() / 1000000000`: HIT → construct response → XDP_TX
4. If not found OR expired: MISS → XDP_PASS

## Limitations

1. **IPv4 only** (initial version). IPv6 XDP support is a future enhancement requiring different header parsing and mandatory UDP checksum calculation.
2. **Max response size 512 bytes.** Larger responses (EDNS0) bypass XDP cache and use Go path. This covers the vast majority of simple A/AAAA/CNAME responses.
3. **No DNS response compression in eBPF.** Responses are pre-packed by Go with compression enabled, so they're already compressed in the BPF map value.
4. **No EDNS0 OPT handling in eBPF.** Queries with OPT records are passed to Go. Simple queries (no OPT) are the majority of loopback/internal traffic.
5. **loopback only supports generic XDP** — performance benefit is less than physical NIC. Full benefit requires native XDP on a supported NIC driver.
6. **Single interface per XDP instance.** Multi-interface requires multiple attach calls (handled by config list, future).

## Performance Expectations

| Scenario | Current (Go-only) | With XDP cache (generic/lo) | With XDP cache (native/NIC) |
|----------|-------------------|----------------------------|----------------------------|
| Cache hit QPS | ~170K | ~300-500K (est.) | ~800K-1M+ (est.) |
| Cache hit latency | ~600μs P50 | ~100-200μs P50 (est.) | ~10-50μs P50 (est.) |
| Cache miss QPS | ~170K | ~170K (unchanged) | ~170K (unchanged) |
| CPU usage (cache hit) | 330% (3.3 cores) | <100% (est.) | <50% (est.) |

*Estimates based on published CloudFlare XDP DNS benchmarks scaled to our workload.*

## Testing Strategy

1. **Unit tests:** BPF map operations via cilium/ebpf mock/test helpers
2. **Integration tests:** Load XDP program on veth pair, send DNS queries, verify XDP_TX responses match Go-generated responses byte-for-byte
3. **Performance tests:** dnsperf benchmark with XDP enabled vs disabled, measure QPS and latency
4. **Correctness tests:** Verify cache miss fallback works (XDP_PASS → Go resolver returns correct answer)
5. **TTL tests:** Verify expired entries are not served by XDP, verify cleanup removes stale entries
6. **Regression tests:** Existing e2e tests pass with XDP enabled (should be transparent)

## Rollback Plan

XDP is an additive, opt-in feature (`xdp.enabled: false` by default). To rollback:
1. Set `xdp.enabled: false` in config
2. Restart rec53
3. The XDP program is automatically detached on shutdown

No existing code paths are modified. The Go resolver continues to function identically with or without XDP.

## Success Criteria

1. XDP cache hit path returns correct DNS responses (bit-for-bit match with Go path, modulo transaction ID)
2. dnsperf QPS increases by at least 50% for cache-hit workloads with XDP enabled
3. P99 latency does not regress for cache-miss workloads
4. All existing e2e tests pass with XDP enabled
5. XDP metrics (hit/miss/error counters) are correctly exported to Prometheus
