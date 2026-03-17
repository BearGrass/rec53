# Architecture

## Overview

rec53 is a recursive DNS resolver implemented in Go with a state machine architecture. It performs iterative DNS resolution from root servers, featuring local hosts authority, forwarding rules, IP quality tracking for optimal upstream server selection, TTL-based caching, and Prometheus metrics for monitoring.

## Directory Structure

```
rec53/
├── cmd/                    # Entry point and CLI
│   ├── rec53.go            # main(), flag parsing, config loading, signal handling
│   ├── loglevel.go         # log level parsing
│   └── *_test.go           # Command package tests
├── server/                 # Core DNS resolution logic
│   ├── server.go           # UDP/TCP server, ServeDNS(), truncation, warmup lifecycle
│   ├── state_machine.go    # Change() loop, CNAME chain, iteration guard
│   ├── state_define.go     # State constants, return codes, state constructors
│   ├── state.go            # State handler implementations (handle() methods)
│   ├── state_hosts.go      # HOSTS_LOOKUP state: local authority lookup
│   ├── state_forward.go    # FORWARD_LOOKUP state: forwarding rules
│   ├── state_shared.go     # Global accessors for hosts/forwarding config
│   ├── hosts_config.go     # HostEntry type, hosts compilation to dns.Msg map
│   ├── forward_config.go   # ForwardZone type, zone sorting
│   ├── cache.go            # TTL cache wrapper (go-cache)
│   ├── ip_pool.go          # IPQualityV2 ring buffer, scoring, probe loop
│   ├── warmup.go           # WarmupNSRecords(), TLD list
│   └── *_test.go           # Server package tests
├── monitor/                # Observability
│   ├── metric.go           # Prometheus metric methods, HTTP server
│   ├── log.go              # Zap logger initialization, level control
│   └── var.go              # Global metric/log singletons, metric definitions
├── utils/                  # Utilities
│   ├── root.go             # Root DNS server addresses (13 roots)
│   ├── zone.go             # Zone parsing helpers
│   └── net.go              # Network utilities
├── e2e/                    # Integration tests
│   ├── helpers.go          # MockAuthorityServer, test utilities
│   ├── resolver_test.go    # End-to-end resolution tests
│   ├── cache_test.go       # Cache behavior tests
│   ├── server_test.go      # Server lifecycle tests
│   ├── error_test.go       # Error handling tests
│   └── hosts_forward_test.go # Hosts authority & forwarding E2E tests
├── etc/                    # Configuration
│   └── prometheus.yml      # Prometheus config for Docker
└── single_machine/         # Docker Compose deployment
    └── docker-compose.yml
```

## Request Lifecycle

```
Client UDP/TCP query
        │
        ▼
  server.ServeDNS()           ← server/server.go
  - guard QDCOUNT == 0
  - save originalQuestion
  - InCounterAdd(request)
  - newStateInitState()
        │
        ▼
  Change(stm)                 ← server/state_machine.go
  - state machine loop (max 50 iterations)
  - accumulates cnameChain
        │
        ▼
  reply = result
  - restore originalQuestion
  - UDP: truncateResponse() if needed
  - OutCounterAdd / LatencyHistogramObserve
  - w.WriteMsg(reply)
```

## Component Map

| Component | File | Role |
|-----------|------|------|
| `server` | `server/server.go` | UDP/TCP listener, request entry point |
| `Change()` | `server/state_machine.go` | State machine loop orchestrator |
| State handlers | `server/state_define.go`, `state.go`, `state_hosts.go`, `state_forward.go` | Per-state `handle()` logic |
| `globalHostsMap` | `server/state_shared.go` | Pre-compiled hosts → `*dns.Msg` map |
| `globalForwardZones` | `server/state_shared.go` | Sorted forwarding zones (longest-suffix first) |
| `globalDnsCache` | `server/cache.go` | TTL response cache |
| `globalIPPool` | `server/ip_pool.go` | Nameserver latency tracking & selection |
| `WarmupNSRecords` | `server/warmup.go` | Startup IP pool bootstrap |
| `SaveSnapshot` / `LoadSnapshot` | `server/snapshot.go` | NS cache persistence and restore |
| `Rec53Metric` | `monitor/metric.go` | Prometheus counters / histograms / gauges |
| `Rec53Log` | `monitor/log.go` | Zap structured logger |

---

## Core Subsystem: State Machine

### Overview

All DNS resolution happens inside the `Change()` loop in `server/state_machine.go`. Each call to `Change()` drives a state machine through up to **50 transitions** (CNAME loop guard). Each state is a struct that implements:

```go
type stateMachine interface {
    getCurrentState() int
    getRequest()      *dns.Msg
    getResponse()     *dns.Msg
    handle(req, resp *dns.Msg) (int, error)
}
```

`handle()` returns `(nextStateCode, error)`. The loop continues until it receives `RETURN_RESP` or an error.

### States

| State | Constant | Purpose |
|-------|----------|---------|
| `STATE_INIT` | `0` | Validate request; initialize response header |
| `HOSTS_LOOKUP` | `1` | Look up query in pre-compiled hosts map (local authority) |
| `FORWARD_LOOKUP` | `2` | Match query against forwarding zones; forward to upstream if matched |
| `CACHE_LOOKUP` | `3` | Look up query in `globalDnsCache` |
| `CLASSIFY_RESP` | `4` | Classify current response: Answer / CNAME / NS referral |
| `EXTRACT_GLUE` | `5` | Extract nameserver IPs from glue records in current response |
| `LOOKUP_NS_CACHE` | `6` | Fall back to cache or root servers if no glue IPs found |
| `QUERY_UPSTREAM` | `7` | Send query to best and secondary nameserver concurrently (Happy Eyeballs); first response wins. Retries on secondary if primary returns a bad rcode (SERVFAIL/REFUSED/FORMERR/NOTIMPL) |
| `RETURN_RESP` | `8` | Prepend CNAME chain; write final response |

### Query Processing Chain

```
STATE_INIT → HOSTS_LOOKUP → FORWARD_LOOKUP → CACHE_LOOKUP → ... → RETURN_RESP
```

**Priority order**: Hosts (local authority) > Forwarding rules > Cache > Iterative resolution.

- **HOSTS_LOOKUP**: If the query matches a hosts entry (exact name + type), returns an authoritative response (AA=true) immediately. If the name exists but type doesn't match, returns NODATA (NOERROR + empty answer). On miss, falls through to `FORWARD_LOOKUP`.
- **FORWARD_LOOKUP**: If the query's domain matches a forwarding zone (longest-suffix wins), forwards to configured upstreams sequentially. Results are NOT cached. All upstreams fail → SERVFAIL (no iterative fallback). On no zone match, falls through to `CACHE_LOOKUP`.

### Transition Diagram

Four paths run through the state machine:

```
                      ┌─────────────────────────────────────────────────┐
                      │           Loop A: iterative delegation          │
                      │   (each NS referral → drill one level deeper)   │
                      │                                                 │
                      │  ┌──────────────────────────────────────┐       │
                      │  │        Loop B: CNAME chain           │       │
                      │  │  (each CNAME target re-resolved)     │       │
                      │  │                                      │       │
    ┌─────────────┐   │  │                                      │       │
    │  STATE_INIT │   │  │                                      │       │
    └──────┬──────┘   │  │                                      │       │
           │ always   │  │                                      │       │
           ▼          │  │                                      │       │
    ┌──────────────┐  │  │                                      │       │
    │ HOSTS_LOOKUP │  │  │                                      │       │
    └──────┬───────┘  │  │                                      │       │
           │ hit ──────┼──┼───────────────────────────────────┐ │       │
           │ miss     │  │                                    │ │       │
           ▼          │  │                                    │ │       │
    ┌───────────────┐ │  │                                    │ │       │
    │FORWARD_LOOKUP │ │  │                                    │ │       │
    └──────┬────────┘ │  │                                    │ │       │
           │ hit ──────┼──┼───────────────────────────────────┤ │       │
           │ miss     │  │                                    │ │       │
           ▼          │  │                                    │ │       │
    ┌─────────────┐   │  │   hit                              │ │       │
    │ CACHE_LOOKUP│───┼──┼──────────────────┐                 │ │       │
    └──────┬──────┘   │  │                  ▼                 ▼ │       │
           │ miss     │  │         ┌──────────────────┐       │ │       │
           ▼          │  │         │  CLASSIFY_RESP   │       │ │       │
    ┌─────────────┐   │  │         └────────┬─────────┘       │ │       │
    │ EXTRACT_GLUE│◄──┼──┼──────────────────┤ NS referral     │ │       │
    └──────┬──────┘   │  │                  │                 │ │       │
           │          │  │                  │ CNAME ──────────┘ │       │
           │ glue IPs │  │                  │                   │       │
           │ found    │  │                  │ answer / negative │       │
           │          │  │                  ▼                   │       │
           │          │  │         ┌──────────────────┐         │       │
           │          │  │         │   RETURN_RESP    │ ──► (done)      │
           │          │  │         └──────────────────┘                 │
           │ no glue  │  │                                              │
           ▼          │  │                                              │
    ┌──────────────┐  │  │                                              │
    │LOOKUP_NS_CACHE│ │  │                                              │
    └──────┬───────┘  │  │                                              │
           │ hit or   │  │                                              │
           │ miss     │  │                                              │
           │ (roots)  │  │                                              │
           ▼          │  │                                              │
    ┌──────────────┐  │  │                                              │
    │QUERY_UPSTREAM│──┴──┘  success → CLASSIFY_RESP ──────────────────┘
    └──────┬───────┘         (new NS referral closes Loop A)
           │
           │ error → SERVFAIL (terminal)
```

**Fast paths — HOSTS_LOOKUP and FORWARD_LOOKUP** (short-circuit before iterative)

`HOSTS_LOOKUP` checks the pre-compiled hosts map. On exact match (name + type), it returns immediately via `RETURN_RESP` with AA=true. On name match but type mismatch, it returns NODATA. On complete miss, it falls through to `FORWARD_LOOKUP`.

`FORWARD_LOOKUP` checks forwarding zones using longest-suffix match. On match, it queries configured upstreams sequentially and returns the result via `RETURN_RESP`. Forwarded results are never cached. On all-upstreams-fail, it returns SERVFAIL without iterative fallback. On no zone match, it falls through to `CACHE_LOOKUP`.

**Loop A — iterative delegation** (main loop, up to 50 iterations)

Each time `QUERY_UPSTREAM` receives an NS referral from an upstream authoritative server (Ns + Extra present, no Answer), `CLASSIFY_RESP` recognises it as an NS referral and transitions to `EXTRACT_GLUE`. The loop continues until a server at some level returns a final answer.

```
EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP →(NS referral)→ EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP → …
   (root)         (root)           (TLD NS)         (TLD)           (TLD)         (auth)             (answer!)
```

**Loop B — CNAME chain tracking** (each CNAME target triggers a full resolution pass)

When `CLASSIFY_RESP` detects a CNAME, it appends the CNAME record to `cnameChain`, updates the Question to the target, and transitions back to `CACHE_LOOKUP` to re-run the full resolution pipeline until a non-CNAME record is obtained.

```
CLASSIFY_RESP →(CNAME a→b)→ CACHE_LOOKUP →(miss)→ EXTRACT_GLUE → QUERY_UPSTREAM → CLASSIFY_RESP
               →(CNAME b→c)→ CACHE_LOOKUP → …
               →(answer c)→  RETURN_RESP  (prepend cnameChain: [a→b, b→c] + answer)
```

**`LOOKUP_NS_CACHE` fallback path** (branch of Loop A, not an independent loop)

When `EXTRACT_GLUE` finds no glue records, `LOOKUP_NS_CACHE` looks up the parent zone's NS + glue in cache, or falls back to root servers. Both cache hit and miss proceed to `QUERY_UPSTREAM` to continue Loop A.

```
EXTRACT_GLUE →(no glue)→ LOOKUP_NS_CACHE →(hit: cached zone)→ QUERY_UPSTREAM
                                          →(miss: root servers)→ QUERY_UPSTREAM
```

### CNAME Chain Handling

`CLASSIFY_RESP` detects CNAME records in the Answer section and appends them to `cnameChain []dns.RR` (stored in the state machine). The next query is re-issued for the CNAME target via `CACHE_LOOKUP`. At `RETURN_RESP`, the accumulated chain is prepended to the final Answer.

**Cycle detection**: a `visitedDomains` map prevents infinite CNAME loops.

**B-004 fix**: `isNSRelevantForCNAME` preserves NS delegation records when they belong to the zone of the original query rather than the CNAME target — preventing incorrect referral loops.

### NS Resolution Without Glue

When `LOOKUP_NS_CACHE` cannot find nameserver IPs in cache or from roots, `resolveNSIPsConcurrently` launches parallel recursive state machine calls (one per NS hostname). A depth guard via `contextKeyNSResolutionDepth` prevents deadlock when NS hostnames are themselves delegated.

### Return Codes

Return codes are defined in `server/state_machine.go` and `server/state_define.go`:

| Code | Meaning |
|------|---------|
| `HOSTS_HIT` | Hosts exact match — go to `RETURN_RESP` |
| `HOSTS_NODATA` | Hosts name match, type mismatch — go to `RETURN_RESP` (NODATA) |
| `HOSTS_MISS` | Hosts miss — go to `FORWARD_LOOKUP` |
| `FORWARD_HIT` | Forwarding zone match, upstream success — go to `RETURN_RESP` |
| `FORWARD_FAIL` | Forwarding zone match, all upstreams failed — SERVFAIL |
| `FORWARD_MISS` | No forwarding zone match — go to `CACHE_LOOKUP` |
| `CACHE_LOOKUP_HIT` | Cache hit — go to `CLASSIFY_RESP` |
| `CACHE_LOOKUP_MISS` | Cache miss — go to `EXTRACT_GLUE` |
| `CLASSIFY_RESP_GET_ANS` | Final answer ready — go to `RETURN_RESP` |
| `CLASSIFY_RESP_GET_CNAME` | CNAME found — re-enter `CACHE_LOOKUP` |
| `CLASSIFY_RESP_GET_NS` | NS referral — go to `EXTRACT_GLUE` |
| `EXTRACT_GLUE_EXIST` | Glue IPs found — go to `QUERY_UPSTREAM` |
| `EXTRACT_GLUE_NOT_EXIST` | No glue — go to `LOOKUP_NS_CACHE` |
| `QUERY_UPSTREAM_COMMON_ERROR` | Upstream query failed |
| `RETURN_RESP_NO_ERROR` | Terminal state, return response |

---

## Core Subsystem: Cache

### Design

The cache is a thin wrapper around [`patrickmn/go-cache`](https://github.com/patrickmn/go-cache) with these guarantees:

- **Key format**: `"name.:qtype_number"` — e.g. `"example.com.:1"` for A, `"example.com.:28"` for AAAA
- **Deep copy on read and write**: every cached `*dns.Msg` is stored and retrieved via `msg.Copy()` to prevent callers from mutating cached data
- **TTL from DNS response**: extracted from `Answer[0].Header().Ttl` (positive responses) or `Ns[0].Header().Ttl` (NS referrals); defaults to 5 minutes
- **go-cache parameters**: default TTL 5 min, cleanup interval 10 min

### Negative Caching

NXDOMAIN and NODATA (empty answer, no error) responses are cached using the SOA `Minttl` field from the Authority section. If no SOA is present, a 60-second default TTL is used. This prevents repeated iterative resolution for non-existent domains.

### Cache API

```go
// Read — always returns a deep copy; nil if not cached
msg := getCacheCopyByType(name, qtype)

// Write — stores a deep copy; ttl from msg or default 5 min
setCacheCopyByType(name, qtype, msg)
```

### Thread Safety

`go-cache` provides its own internal locking. The `getCacheCopyByType`/`setCacheCopyByType` wrappers do not add additional locking. The deep-copy discipline ensures no data races even under concurrent reads.

---

## Core Subsystem: IP Pool (IPQualityV2)

### Overview

`globalIPPool` tracks latency quality for every nameserver IP encountered during resolution. It uses a **64-sample sliding window ring buffer** per IP and exports P50/P95/P99 percentiles to Prometheus. Selection uses a **composite score** that balances measured latency, confidence, and fault state.

### Per-IP Data Structure

```go
type IPQualityV2 struct {
    samples      [64]float64   // ring buffer of RTT samples (ms)
    sampleCount  int           // total samples recorded (capped at 64)
    head         int           // next write position in ring buffer
    p50, p95, p99 float64      // computed percentiles
    failCount    int           // consecutive failure counter
    state        int           // ACTIVE / DEGRADED / SUSPECT / RECOVERED
}
```

### Lifecycle

```
New IP discovered
    │  state=ACTIVE, confidence=0%, score=2000 (encouraged for sampling)
    ▼
RecordLatency(ip, rtt)
    │  add rtt to ring buffer, recompute P50/P95/P99, reset failCount=0
    ▼
Query success ──► state stays ACTIVE; confidence increases toward 100%
Query failure ──► RecordFailure(ip)
                      failCount 1-3: state=DEGRADED  (score ×1.5)
                      failCount 4-6: state=SUSPECT   (score ×100, p50=10000)
                      failCount 7+:  state=SUSPECT   (eligible for probe)
                          │
                          ▼ every 30 s (background)
                      periodicProbeLoop()
                          probe A record → success → ResetForProbe()
                                                      state=ACTIVE, failCount=0
```

### Composite Score Formula

```
score = p50_ms × confidence_multiplier × state_weight

confidence_multiplier:
  0%  confidence → 2.0   (new IPs are tried aggressively)
  100% confidence → 1.0  (fully measured IPs are judged on latency alone)

state_weight:
  ACTIVE    → 1.0
  RECOVERED → 1.1   (slight penalty: recently recovered)
  DEGRADED  → 1.5   (moderate penalty: some failures)
  SUSPECT   → 100.0 (avoided: severe failures)
```

### Score Examples

| State | Confidence | P50 (ms) | Conf Mult | State Weight | Score |
|-------|------------|----------|-----------|--------------|-------|
| ACTIVE | 0% | 100 | 2.0 | 1.0 | **200** (new, encouraged) |
| ACTIVE | 100% | 100 | 1.0 | 1.0 | **100** (preferred) |
| ACTIVE | 100% | 50 | 1.0 | 1.0 | **50** (best) |
| RECOVERED | 100% | 100 | 1.0 | 1.1 | **110** (slightly penalized) |
| DEGRADED | 100% | 100 | 1.0 | 1.5 | **150** (penalized) |
| SUSPECT | 100% | 10000 | 1.0 | 100.0 | **1,000,000** (avoided) |

### Selection API

```go
// Returns (best, secondary) by lowest composite score
best, secondary := globalIPPool.GetBestIPsV2(ips)

// Record a successful query
globalIPPool.RecordLatency(ip, rtt_ms)

// Record a failed query
globalIPPool.RecordFailure(ip)
```

### Concurrent Access

- `IPQualityV2` fields are accessed lock-free via atomic operations in the hot path
- `IPPool.pool` (the map of IP → `*IPQualityV2`) is protected by `sync.RWMutex`:
  - `RLock` for reads during query path (`RecordLatency`, `RecordFailure`, `GetScore`)
  - `Lock` only in background probe loop (`ResetForProbe`)
- Background probe goroutine runs every 30 s; non-blocking to the query path

### Warmup Bootstrap

On startup, `WarmupNSRecords()` resolves NS records for a configurable TLD list. All resolved nameserver IPs are fed into `globalIPPool` via `RecordLatency`, giving the pool measured baselines before the first user query arrives. This eliminates the cold-start penalty where all IPs have 0% confidence.

---

## Core Subsystem: Full Cache Snapshot

### Overview

`server/snapshot.go` implements optional full cache persistence. On graceful shutdown, `SaveSnapshot()` serialises **all** cache entries from `globalDnsCache` (A/AAAA answers, CNAME chains, NS delegations, and any other cached `dns.Msg`) to a JSON file. On the next startup, `LoadSnapshot()` reads that file and restores unexpired entries into `globalDnsCache` **before `server.Run()` is called**, guaranteeing the cache is warm before the first DNS query arrives. This eliminates cold-start latency for both delegation lookups and direct answer queries.

### Startup Sequence

```
cmd/main()
  ├─ NewServerWithFullConfig(...)     ← creates server with snapshotCfg
  ├─ server.LoadSnapshot(cfg.Snapshot) ← synchronous, < 5ms, before any listener starts
  └─ rec53.Run()
       ├─ goroutine: warmupNSOnStartup()   ← Round 1 TLD warmup (unchanged)
       ├─ goroutine: udp.ListenAndServe()
       └─ goroutine: tcp.ListenAndServe()
```

`LoadSnapshot` runs on the calling goroutine and returns before `Run()` starts any listener. There is no race between cache writes and incoming queries.

### Snapshot File Format

Each entry is a JSON object:

```json
{ "key": "github.com.:2", "msg_b64": "<wire-format base64>", "saved_at": 1710000000 }
```

- `key` — cache key in `"name.:qtype"` format
- `msg_b64` — `dns.Msg.Pack()` wire bytes, base64-encoded; preserves all RR fields including TTL
- `saved_at` — Unix timestamp (seconds) when the snapshot was written

On restore, remaining TTL = `rr.Ttl - (now - saved_at)`. `remainingTTL()` scans all three message sections (Answer, Ns, Extra) and returns the maximum remaining TTL found. Entries where all RRs are expired are skipped.

### Configuration

```yaml
snapshot:
  enabled: false
  file: ""   # e.g. /var/lib/rec53/cache-snapshot.json
```

`enabled: false` (default) is a complete no-op. `file: ""` disables the feature even when `enabled: true`. Write failures on shutdown are logged as errors but do not affect the `Shutdown()` return value.

---

## Design Constraints

- Single binary deployment
- Must handle both UDP and TCP protocols
- Graceful shutdown with 5-second timeout
- Max 50 state machine iterations (CNAME loop protection)
- EDNS0 support with 4096-byte UDP buffer

## Known Limitations

- DNSSEC validation not implemented
- DoT/DoH not supported
