# Architecture

## Overview

rec53 is a recursive DNS resolver implemented in Go with a state machine architecture. It performs iterative DNS resolution from root servers, featuring IP quality tracking for optimal upstream server selection, TTL-based caching, and Prometheus metrics for monitoring.

## Directory Structure

```
rec53/
├── cmd/                    # Entry point and CLI
│   ├── rec53.go            # Main application, flag parsing, signal handling
│   ├── loglevel.go         # Log level parsing utilities
│   └── *_test.go           # Command package tests
├── server/                 # Core DNS resolution logic
│   ├── server.go           # UDP/TCP DNS server, request handling
│   ├── state_machine.go    # State machine orchestration
│   ├── state_define.go     # State constants and return codes
│   ├── state.go            # State handler implementations
│   ├── cache.go            # DNS response cache with TTL
│   ├── ip_pool.go          # Nameserver quality tracking
│   └── *_test.go           # Server package tests
├── monitor/                # Observability
│   ├── metric.go           # Prometheus metrics
│   ├── log.go              # Zap structured logging
│   └── var.go              # Global metric instances
├── utils/                  # Utilities
│   ├── root.go             # Root DNS servers configuration
│   ├── zone.go             # Zone parsing utilities
│   └── net.go              # Network utilities
├── e2e/                    # End-to-end integration tests
│   ├── helpers.go          # Test utilities and mock servers
│   ├── resolver_test.go    # Resolver integration tests
│   ├── cache_test.go       # Cache behavior tests
│   ├── server_test.go      # Server lifecycle tests
│   └── error_test.go       # Error handling tests
├── etc/                    # Configuration
│   └── prometheus.yml      # Prometheus config for Docker
└── single_machine/         # Docker Compose deployment
    └── docker-compose.yml
```

## Core Flow

```
DNS Query
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  Server.ServeDNS (server.go)                                │
│  - Receives UDP/TCP query                                   │
│  - Creates state machine with request/response              │
│  - Executes Change() to run state machine                   │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  State Machine (state_machine.go)                           │
│                                                             │
│  STATE_INIT → IN_CACHE → CHECK_RESP → IN_GLUE → ITER ─┐    │
│                   ↑           │          │         │  │    │
│                   └───────────┘          └─────────┘  │    │
│                                                       │    │
│  States:                                              │    │
│  - STATE_INIT: Validate request (FORMERR if invalid), initialize response │    │
│  - IN_CACHE: Check if answer is cached                │    │
│  - CHECK_RESP: Determine if ANS/CNAME/NS referral     │    │
│  - IN_GLUE: Get nameserver addresses from glue        │    │
│  - IN_GLUE_CACHE: Get nameserver from cache or root   │    │
│  - ITER: Query upstream nameserver (best IP selected) │    │
│  - RET_RESP: Return final response                     │    │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│  IP Pool (ip_pool.go)                                       │
│  - Tracks latency for each upstream nameserver              │
│  - Selects best IP based on measured RTT                    │
│  - Prefetches quality for candidate servers                 │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
DNS Response
```

## Key Components

### server.Server

- **Responsibility**: UDP/TCP DNS listener, request routing
- **Interface**: `NewServer(addr)`, `Run()`, `ServeDNS()`, `Shutdown(ctx)`
- **Dependencies**: miekg/dns, state machine, metrics

### server.StateMachine

- **Responsibility**: Orchestrates DNS resolution through states
- **Interface**: `Change(stm) (*dns.Msg, error)`
- **Dependencies**: Cache, IP pool, all state handlers

### server.IPPool (IPQualityV2)

- **Responsibility**: Nameserver quality tracking with sliding-window histograms, intelligent selection, and background fault recovery
- **Data structures**:
  - `IPQualityV2`: Per-IP quality tracking with:
    - Ring buffer: Last 64 RTT samples (milliseconds)
    - Percentiles: P50, P95, P99 computed incrementally
    - Confidence: 0-100% based on sample count (≥10 samples = 100%)
    - State machine: ACTIVE(0) → DEGRADED(1) → SUSPECT(2) → RECOVERED(3)
    - Failure counter: Tracks consecutive failures for exponential backoff
  - `IPPool`: Concurrent-safe pool with RWMutex protecting IP quality map
- **Core methods**:
  - `RecordLatency(ip, rtt)`: Records RTT in ring buffer, updates percentiles, resets failure counter
  - `RecordFailure(ip)`: Increments failure counter, applies exponential backoff phases:
    - Phase 1 (1-3 failures): DEGRADED, 20% latency penalty
    - Phase 2 (4-6 failures): SUSPECT, all metrics set to MAX (10000ms)
    - Phase 3 (7+ failures): Remains SUSPECT, eligible for probing
  - `GetScore(ip)`: Composite score = p50 × confidence_multiplier × state_weight
    - Low-confidence IPs (0%) get 2x bonus to encourage sampling
    - State weights: ACTIVE(1.0), DEGRADED(1.5), SUSPECT(100.0), RECOVERED(1.1)
  - `GetBestIPsV2(ips)`: Returns (best, secondary) by lowest composite score
  - `ShouldProbe(ip)`: Identifies SUSPECT candidates for recovery probing
  - `ResetForProbe(ip)`: Resets to ACTIVE state on successful probe
  - `IPQualityV2GaugeSet(ip, quality)`: Exports P50/P95/P99 to Prometheus
- **Background recovery**:
  - `StartProbeLoop()`: Launches background goroutine on server startup
  - `periodicProbeLoop()`: Runs every 30 seconds, non-blocking to queries
  - `probeAllSuspiciousIPs()`: Probes only SUSPECT IPs via A record query
  - Uses RWMutex to prevent blocking normal query path
- **Dependencies**: dns.Client for probing, context for graceful shutdown, RWMutex for concurrency

### server.Cache

- **Responsibility**: DNS response caching with TTL
- **Interface**: `getCacheCopyByType()`, `setCacheCopyByType()`
- **Dependencies**: patrickmn/go-cache

### monitor.Metric

- **Responsibility**: Prometheus metrics export
- **Interface**: `InCounterAdd()`, `OutCounterAdd()`, `LatencyHistogramObserve()`
- **Dependencies**: prometheus/client_golang

## Design Constraints

- Single binary deployment
- Must handle both UDP and TCP protocols
- Graceful shutdown with 5-second timeout
- Max 50 state machine iterations (CNAME loop protection)
- EDNS0 support with 4096-byte UDP buffer

## Known Limitations

- DNSSEC validation not implemented
- DoT/DoH not supported

## IP Quality Management Lifecycle (IPQualityV2)

The IP quality tracking follows this lifecycle with automatic fault recovery:

```
1. IP DISCOVERY
   ┌──────────────────────────────┐
   │ New IP from NS delegation    │
   │ State: ACTIVE (0)            │
   │ Confidence: 0% (no samples)  │
   │ Score: 1000 × 2.0 × 1.0 = 2000 (encouraged for sampling)
   └──────────────────────────────┘
          │
          ▼
2. LATENCY RECORDING (On Query Success)
   ┌──────────────────────────────┐
   │ RecordLatency(ip, rtt)       │
   │ - Add RTT to ring buffer     │
   │ - Update P50/P95/P99         │
   │ - Increment sample count     │
   │ - Reset failure counter = 0  │
   │ - Export metrics to Prometheus
   └──────────────────────────────┘
          │
          ▼
3. FAILURE TRACKING (On Query Failure)
   ┌──────────────────────────────┐
   │ RecordFailure(ip)            │
   │ Exponential backoff phases:  │
   │                              │
   │ Phase 1 (1-3 failures):      │
   │   State: DEGRADED (1)        │
   │   Score: p50 × 1.0 × 1.5    │
   │   Effect: 20% latency penalty│
   │                              │
   │ Phase 2 (4-6 failures):      │
   │   State: SUSPECT (2)         │
   │   Score: 10000 × 1.0 × 100.0│
   │   Effect: Avoided in selection
   │                              │
   │ Phase 3 (7+ failures):       │
   │   State: SUSPECT (2)         │
   │   Eligible for background    │
   │   probing every 30 seconds   │
   └──────────────────────────────┘
          │
          ▼
4. BACKGROUND RECOVERY (Every 30 seconds)
   ┌──────────────────────────────┐
   │ periodicProbeLoop()          │
   │ - Identify SUSPECT IPs       │
   │ - Query A record to each     │
   │ - On success:                │
   │   ResetForProbe(ip) →        │
   │   State: ACTIVE (0)          │
   │   Failure counter reset      │
   │ - Non-blocking to queries    │
   └──────────────────────────────┘
          │
          ▼
5. COMPOSITE SCORING & SELECTION
   ┌──────────────────────────────┐
   │ GetScore(ip):                │
   │ score = p50 ×                │
   │         confidence_mult ×    │
   │         state_weight         │
   │                              │
   │ GetBestIPsV2(ips):           │
   │ - Return lowest score IP     │
   │ - Balanced best+explore      │
   └──────────────────────────────┘
```

### Score Calculation Examples

| State | Confidence | P50ms | Conf Mult | State Weight | Score |
|-------|------------|-------|-----------|--------------|-------|
| ACTIVE | 0% | 100 | 2.0 | 1.0 | 200 (encouraged) |
| ACTIVE | 100% | 100 | 1.0 | 1.0 | 100 (preferred) |
| DEGRADED | 100% | 100 | 1.0 | 1.5 | 150 (penalized) |
| SUSPECT | 100% | 10000 | 1.0 | 100.0 | 1,000,000 (avoided) |
| RECOVERED | 100% | 100 | 1.0 | 1.1 | 110 (slightly penalized) |

## Concurrent Access Patterns

- **IPQualityV2**: Lock-free design with atomic operations for latency/confidence
- **IPPool.pool**: Protected by `RWMutex` for safe concurrent map access
  - Reader lock: Query latency recording (RecordLatency, RecordFailure)
  - Writer lock: Background probing only (periodicProbeLoop)
  - Design: Non-blocking to queries, probing happens out-of-band
- **Background probing**: Goroutine-based with periodic 30-second intervals
- **Shutdown coordination**: Context-based cancellation with graceful goroutine termination