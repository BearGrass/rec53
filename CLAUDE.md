# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o rec53 ./cmd

# Run (DNS on :5353, metrics on :9999)
./rec53

# Run with custom config
./rec53 -listen 0.0.0.0:53 -metric :9099 -log-level debug
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

### IP Quality Tracking

`server/ip_pool.go` tracks upstream nameserver latency for optimal server selection:
- **IPQuality structure**: Each IP has `isInit` flag (measured vs. initial) and `latency` (in milliseconds)
- **Initial state**: `isInit=true`, `latency=1000ms` (assumed value before measurement)
- **Measured state**: `isInit=false`, `latency=actual RTT` (after prefetch verification)
- **Best IP selection**: Via `getBestIPs()` returning highest-priority and second-best IP
- **Quality improvement**: `UpIPsQuality()` reduces latency by 10% for well-performing IPs
- **Background prefetch**: `PrefetchIPs()` measures candidate servers with semaphore-limited concurrency (max 10)
- **Prefetch candidates**: `GetPrefetchIPs()` selects IPs with latency in range `[best × 0.9, best]`
- **Graceful shutdown**: `Shutdown()` waits for all prefetch goroutines with context timeout

## Dependencies

- `github.com/miekg/dns` - DNS protocol implementation
- `github.com/patrickmn/go-cache` - TTL-based caching
- `github.com/prometheus/client_golang` - Metrics export
- `go.uber.org/zap` - Structured logging
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation

## Conventions

- **State pattern**: Each state is a struct implementing `stateMachine` interface with `getCurrentState()`, `getRequest()`, `getResponse()`, `handle()`
- **Global instances**: `globalIPPool` and cache use package-level globals
- **Error handling**: States return error codes (e.g., `ITER_COMMON_ERROR`, `IN_CACHE_HIT_CACHE`) rather than errors directly for flow control
- **Graceful shutdown**: Server supports context-based shutdown with 5s timeout
- **UDP truncation**: Responses exceeding EDNS0 size are truncated with TC flag set

See [.rec53/CONVENTIONS.md](./.rec53/CONVENTIONS.md) for detailed coding conventions.

## Documentation

- [Architecture](./.rec53/ARCHITECTURE.md)
- [Conventions](./.rec53/CONVENTIONS.md)
- [Roadmap](./.rec53/ROADMAP.md)
- [TODO](./.rec53/TODO.md)
- [Test Plan](./.rec53/TEST_PLAN.md)
- [Backlog](./.rec53/BACKLOG.md)

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

### TODO.md Update Triggers

- Completed a task → mark as done and move to Completed section
- Found a new bug → add BUG item with discovery context
- Found optimization opportunity outside current task → add OPT item
- Task interrupted → update progress checkboxes to latest state
- Introduced technical debt → add DEBT item with source reference

### BACKLOG.md Update Triggers

- During development, discovered a prerequisite feature is needed → add to Unplanned
- Requirement development complete → move from Planned to Completed

### Execution Rules

- Do NOT make a separate round just to update docs. Update in the same task that caused the change.
- After updating, mention what changed in one line, e.g.: "Updated CLAUDE.md Architecture: added internal/middleware/"

See [.rec53/CONVENTIONS.md](./.rec53/CONVENTIONS.md) for detailed coding conventions.

## Documentation

- [Architecture](./.rec53/ARCHITECTURE.md)
- [Conventions](./.rec53/CONVENTIONS.md)
- [Roadmap](./.rec53/ROADMAP.md)
- [TODO](./.rec53/TODO.md)
- [Test Plan](./.rec53/TEST_PLAN.md)
- [Backlog](./.rec53/BACKLOG.md)

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

### TODO.md Update Triggers

- Completed a task → mark as done and move to Completed section
- Found a new bug → add BUG item with discovery context
- Found optimization opportunity outside current task → add OPT item
- Task interrupted → update progress checkboxes to latest state
- Introduced technical debt → add DEBT item with source reference

### BACKLOG.md Update Triggers

- During development, discovered a prerequisite feature is needed → add to Unplanned
- Requirement development complete → move from Planned to Completed

### Execution Rules

- Do NOT make a separate round just to update docs. Update in the same task that caused the change.
- After updating, mention what changed in one line, e.g.: "Updated CLAUDE.md Architecture: added internal/middleware/"

See [.rec53/CONVENTIONS.md](./.rec53/CONVENTIONS.md) for detailed coding conventions.

## Documentation

- [Architecture](./.rec53/ARCHITECTURE.md)
- [Conventions](./.rec53/CONVENTIONS.md)
- [Roadmap](./.rec53/ROADMAP.md)
- [TODO](./.rec53/TODO.md)
- [Test Plan](./.rec53/TEST_PLAN.md)
- [Backlog](./.rec53/BACKLOG.md)

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

### TODO.md Update Triggers

- Completed a task → mark as done and move to Completed section
- Found a new bug → add BUG item with discovery context
- Found optimization opportunity outside current task → add OPT item
- Task interrupted → update progress checkboxes to latest state
- Introduced technical debt → add DEBT item with source reference

### BACKLOG.md Update Triggers

- During development, discovered a prerequisite feature is needed → add to Unplanned
- Requirement development complete → move from Planned to Completed

### Execution Rules

- Do NOT make a separate round just to update docs. Update in the same task that caused the change.
- After updating, mention what changed in one line, e.g.: "Updated CLAUDE.md Architecture: added internal/middleware/"