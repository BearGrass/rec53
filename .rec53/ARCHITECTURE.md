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
│  - STATE_INIT: Initialize response, set reply         │    │
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

- **Responsibility**: Nameserver quality tracking and selection
- **Interface**: `getBestIPs()`, `updateIPQuality()`, `PrefetchIPs()`
- **Dependencies**: dns.Client for prefetch queries

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