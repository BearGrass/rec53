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

### server.IPPool

- **Responsibility**: Nameserver quality tracking, best IP selection, and background prefetch
- **Data structure**:
  - `IPQuality`: Atomic-based tracking with `isInit` flag (initialized before/after measurement) and `latency` (milliseconds)
  - `IPPool`: Concurrent-safe pool with RWMutex, context-based shutdown, prefetch semaphore
- **Core methods**:
  - `getBestIPs(ips)`: Returns (best IP, second-best IP) based on lowest latency
  - `UpIPsQuality(ips)`: Reduces latency by 10% for measured IPs to reward good performers
  - `GetPrefetchIPs(bestIP)`: Identifies candidates in `[bestLatency × 0.9, bestLatency]` range
  - `PrefetchIPs(ips)`: Asynchronously measures candidate IPs with concurrency limit (max 10)
  - `updateIPQuality(ip, latency)`: Updates IP with actual measurement (sets `isInit=false`)
- **State transitions**:
  - New: `isInit=true, latency=1000ms` (assumed)
  - Measured: `isInit=false, latency=actualRTT` (after prefetch)
- **Dependencies**: dns.Client for prefetch queries, context for graceful shutdown

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

## IP Quality Management Lifecycle

The IP quality tracking follows this lifecycle:

```
1. IP INITIALIZATION
   ┌─────────────────────────────────────┐
   │ New IP discovered from nameserver   │
   │ isInit=true, latency=1000ms (assumed)│
   └─────────────────────────────────────┘
           │
           ▼
2. SELECTION & USAGE
   ┌─────────────────────────────────────┐
   │ getBestIPs(): Pick top 2 IPs by latency
   │ Use best IP for iterative query     │
   └─────────────────────────────────────┘
           │
           ▼
3. PREFETCH DISCOVERY (Background)
   ┌─────────────────────────────────────┐
   │ GetPrefetchIPs(): Find candidates   │
   │ in range [best×0.9, best]           │
   │ PrefetchIPs(): Measure them async   │
   │ (max 10 concurrent)                 │
   └─────────────────────────────────────┘
           │
           ├─ Success ─→ updateIPQuality(ip, RTT)
           │             isInit=false, latency=RTT
           │
           └─ Failure ─→ Retain original value
                        Retry in future prefetch
   
4. QUALITY IMPROVEMENT
   ┌─────────────────────────────────────┐
   │ UpIPsQuality(): For measured IPs    │
   │ latency *= 0.9 (10% reduction)      │
   │ Rewards consistent good performers  │
   └─────────────────────────────────────┘

5. GRACEFUL SHUTDOWN
   ┌─────────────────────────────────────┐
   │ Shutdown(): Cancel context          │
   │ Wait for all prefetch goroutines    │
   │ (context timeout: configurable)     │
   └─────────────────────────────────────┘
```

## Concurrent Access Patterns

- **IPQuality**: Uses atomic operations (`atomic.Load/Store`) for lock-free reads/writes
- **IPPool.pool**: Protected by `RWMutex` for safe concurrent map access
- **Prefetch concurrency**: Semaphore-based with channel (max 10 concurrent goroutines)
- **Shutdown coordination**: WaitGroup tracks all goroutines, context cancellation for graceful termination