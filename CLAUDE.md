# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o rec53 ./cmd

# Generate default config (required on first run)
./generate-config.sh

# Run with config (DNS on :5353, metrics on :9999)
./rec53 --config ./config.yaml

# Run with warmup disabled
./rec53 --config ./config.yaml --no-warmup

# Override config settings with flags
./rec53 --config ./config.yaml -listen 0.0.0.0:53 -metric :9099 -log-level debug
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test -v ./server/...

# Run E2E integration tests
go test -v ./e2e/...

# Skip long-running integration tests
go test -short ./...

# Run specific test
go test -v -run TestResolverIntegration ./e2e/...

# Run with race detector (recommended for concurrent code)
go test -race ./...
```

## Architecture

rec53 is a recursive DNS resolver implemented with a state machine architecture. The core resolution logic lives in `server/state_machine.go`, which orchestrates DNS queries through defined states.

### Key Packages

- **`cmd/`** - Entry point (`rec53.go`), flag parsing, signal handling, graceful shutdown
- **`server/`** - Core DNS logic: state machine, cache, IP pool, UDP/TCP server
- **`monitor/`** - Prometheus metrics and Zap structured logging
- **`utils/`** - Root DNS servers, zone parsing, network utilities
- **`e2e/`** - End-to-end integration tests with mock DNS servers

### State Machine Flow

The resolver uses a state machine pattern (see README.md for diagram):

```
STATE_INIT → IN_CACHE → CHECK_RESP → IN_GLUE → ITER → RET_RESP
                ↑           │          │        │
                └───────────┘          └────────┘
```

Key states:
- **IN_CACHE**: Check if response is cached
- **CHECK_RESP**: Determine if answer, CNAME, or NS referral
- **IN_GLUE/IN_GLUE_CACHE**: Get nameserver addresses from glue/cache
- **ITER**: Query upstream nameserver

### IP Pool Architecture (IPQualityV2)

`server/ip_pool.go` implements **IPQualityV2**, a sliding-window histogram-based IP quality tracking system for optimal upstream nameserver selection with automatic fault recovery:

#### Data Structure (server/ip_pool.go:68-88)
- **Sliding window**: Last 64 RTT samples (milliseconds) in ring buffer
- **Percentiles**: P50 (median), P95, P99 computed incrementally
- **Confidence**: 0-100% based on sample count (≥10 samples = 100%)
- **Failure tracking**: Consecutive failure counter with exponential backoff phases
- **State machine**: ACTIVE(0) → DEGRADED(1) → SUSPECT(2) → RECOVERED(3)

#### Core Algorithms

**Latency Recording** (`RecordLatency()`)
- Adds RTT to ring buffer, shifts old samples out
- Recalculates P50/P95/P99 percentiles
- Resets failure counter on successful response
- Updates Prometheus metrics via `IPQualityV2GaugeSet()`

**Failure Handling** (`RecordFailure()` + exponential backoff)
- Phase 1 (1-3 failures): DEGRADED state, apply 20% latency penalty
- Phase 2 (4-6 failures): SUSPECT state, all metrics set to MAX (10000ms)
- Phase 3 (7+ failures): Remain SUSPECT, eligible for background probing
- Non-blocking: Failure tracking doesn't block query handling

**Probe Strategy** (`ShouldProbe()` + `ResetForProbe()`)
- Identifies SUSPECT candidates for recovery probing
- Throttles probing to prevent overload
- Resets to ACTIVE on successful probe (see F-003/6)

**Composite Scoring** (`GetScore()`)
- Formula: `p50_latency × confidence_multiplier × state_weight`
- Confidence multiplier: Low-confidence IPs (0% confidence) get 2x bonus to encourage sampling
- State weights: ACTIVE(1.0), DEGRADED(1.5), SUSPECT(100.0), RECOVERED(1.1)
- Ensures both quality metrics and exploration of new servers

**IP Selection** (`GetBestIPsV2()`)
- Returns primary and secondary IP based on lowest composite score
- Avoids SUSPECT IPs unless no alternatives
- Encourages sampling of low-confidence IPs for better statistics

#### Background Recovery (F-003/6)

**Probe Loop Integration** (server/server.go:140, server/ip_pool.go:StartProbeLoop)
- `StartProbeLoop()`: Launches background goroutine on server startup
- `periodicProbeLoop()`: Runs every 30 seconds, non-blocking to queries
- `probeAllSuspiciousIPs()`: Probes only SUSPECT IPs via A record query to detect recovery
- Uses RWMutex to prevent blocking normal query path

#### Metrics Export

Prometheus gauges (monitor/var.go:37-60, monitor/metric.go:31-36):
- `rec53_ipv2_p50_latency_ms` - P50 (median) latency per IP
- `rec53_ipv2_p95_latency_ms` - P95 (95th percentile) latency per IP
- `rec53_ipv2_p99_latency_ms` - P99 (99th percentile) latency per IP

Updated on every successful latency recording via `IPQualityV2GaugeSet()` call.

#### Performance
- Selection time: 94-98 µs for 1000 IPs (10x under 1ms target)
- Scales linearly: O(n) with IP count
- Memory: 24KB per 1000 IPs (64 samples × 8 bytes + metadata)

#### Testing
- **Unit tests** (server/ip_pool_test.go): 65+ tests covering latency, percentiles, failure states, recovery, scoring, selection
- **E2E tests** (e2e/ippool_v2_test.go): 8 comprehensive tests covering full resolution flow
- **Benchmarks**: BenchmarkGetBestIPsV2_1000IPs validates performance requirement
- All tests pass with `-race` flag (no concurrency issues)

### NS Warmup on Startup (O-025)

**Configuration-driven approach** that pre-warms root and TLD NS records on startup for improved cache hit rate:

#### Configuration (server/warmup_defaults.go)
- `WarmupConfig` struct with Enabled, Timeout, Concurrency, TLDs fields
- `DefaultWarmupConfig`: warmup disabled by default in NewServer() for test compatibility
- `DefaultTLDs`: 100+ pre-configured TLDs (generic, country codes, regional, new gTLDs)

#### Core Implementation (server/warmup.go)
- `WarmupNSRecords()`: Concurrent warmup using semaphore pattern (default 32 concurrent queries)
  - Queries root (".") + all configured TLDs
  - Per-query 5s timeout prevents hanging
  - Returns WarmupStats (total, succeeded, failed, duration)
- `queryNSRecords()`: Creates synthetic NS queries processed through state machine
  - Reuses existing cache and IP pool infrastructure
  - Metrics automatically recorded via state machine

#### Server Integration (server/server.go)
- `NewServerWithConfig()`: Creates server with warmup config
- `warmupNSOnStartup()`: Background goroutine launched in Run()
  - Non-blocking: doesn't delay server startup
  - 60s overall timeout with per-query 5s timeout
  - Logs completion stats

#### Configuration File (cmd/rec53.go)
- `--config` flag: Required path to YAML config file (no automatic path checking)
- `--no-warmup` flag: Disables warmup even if enabled in config
- Command-line flags override config file values
- Error messages guide users to `./generate-config.sh`

#### Config File Generation (./generate-config.sh)
- Shell script auto-generates `./config.yaml` with 100+ pre-configured TLDs
- Usage: `./generate-config.sh` or `./generate-config.sh -o /path/to/config.yaml`
- YAML structure:
  ```yaml
  dns:
    listen: "127.0.0.1:5353"
    metric: ":9999"
    log_level: "info"
  warmup:
    enabled: true
    timeout: 5s
    concurrency: 32
    tlds: [com, net, org, ...]
  ```

#### Testing
- **Unit tests** (e2e/warmup_test.go): 6 tests covering basic warmup, concurrency, timeouts, cache population, statistics
- Tests use mock DNS hierarchy or skip slow real-server queries
- All tests complete within timeout limits

## Dependencies

- `github.com/miekg/dns` - DNS protocol implementation
- `github.com/patrickmn/go-cache` - TTL-based caching
- `github.com/prometheus/client_golang` - Metrics export
- `go.uber.org/zap` - Structured logging
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation
- `gopkg.in/yaml.v2` - YAML config parsing

## Conventions

- **State pattern**: Each state is a struct implementing `stateMachine` interface with `getCurrentState()`, `getRequest()`, `getResponse()`, `handle()`
- **Global instances**: `globalIPPool` and cache use package-level globals with RWMutex protection
- **Error handling**: States return error codes (e.g., `ITER_COMMON_ERROR`, `IN_CACHE_HIT_CACHE`) rather than errors directly for flow control
- **Graceful shutdown**: Server supports context-based shutdown with 5s timeout
- **UDP truncation**: Responses exceeding EDNS0 size are truncated with TC flag set
- **Concurrency**: IP pool uses RWMutex (reader lock for queries, writer lock for probing) to avoid blocking

See [.rec53/CONVENTIONS.md](./.rec53/CONVENTIONS.md) for detailed coding conventions.

## Documentation

- [Architecture](./.rec53/ARCHITECTURE.md) - System design and state machine details
- [Conventions](./.rec53/CONVENTIONS.md) - Go coding standards and patterns
- [Roadmap](./.rec53/ROADMAP.md) - Feature roadmap
- [TODO](./.rec53/TODO.md) - Current tasks and progress
- [Backlog](./.rec53/BACKLOG.md) - Feature backlog and completed items

## Document Self-Maintenance Rules

### CLAUDE.md Update Triggers

- Added a package or directory → update Architecture section
- Added an external dependency → update Dependencies section
- Changed interfaces or test strategy → update Testing section
- Added commands or build steps → update Build & Run section
- Found CLAUDE.md description inconsistent with code → fix immediately

### README.md Update Triggers

- Added user-facing feature → update feature list
- Changed config format or CLI flags → update usage instructions
- Changed build requirements → update install steps
- Version number changed → update version badge

### Execution Rules

- Do NOT make a separate round just to update docs. Update in the same task that caused the change.
- After updating, mention what changed in one line, e.g.: "Updated CLAUDE.md Architecture: added internal/middleware/"
