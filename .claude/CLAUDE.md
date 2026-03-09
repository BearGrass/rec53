# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o rec53 cmd/rec53.go

# Build with version info
go build -ldflags "-X main.version=X.X.X -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o rec53 cmd/rec53.go

# Run (DNS server on port 5353, metrics on 9999)
./rec53

# Run with custom ports
./rec53 -listen 0.0.0.0:53 -metric :9099 -log-level debug

# Build Docker image
docker build -t rec53 .

# Run with Docker Compose (includes Prometheus, node-exporter)
cd single_machine && docker-compose up -d

# Test
go test ./...

# Test specific package
go test -v ./server/...
```

### CLI Flags
- `-listen` - DNS server address (default: `127.0.0.1:5353`)
- `-metric` - Prometheus metrics address (default: `:9999`)
- `-log-level` - Log level: debug, info, warn, error (default: `info`)
- `-version` - Show version information

## Architecture

**Recursive DNS resolver** implemented as a state machine with caching and IP quality tracking.

### Directory Structure
- `cmd/` - Entry point, flag parsing, signal handling, graceful shutdown
- `server/` - Core DNS logic: state machine (`state_machine.go`, `state_define.go`, `state.go`), cache, IP pool
- `monitor/` - Prometheus metrics and zap logger
- `utils/` - Zone resolution (`GetZoneList`), root servers
- `single_machine/` - Docker Compose setup for local deployment

### State Machine (`server/state_machine.go`, `server/state_define.go`)

The DNS resolution flow is a state machine where each state returns a result code that determines the next state.

**State Flow:**
```
STATE_INIT → IN_CACHE
                ├─ HIT_CACHE → CHECK_RESP
                └─ MISS_CACHE → IN_GLUE
                                  ├─ EXIST → ITER
                                  └─ NOT_EXIST → IN_GLUE_CACHE
                                                   ├─ HIT_CACHE → ITER
                                                   └─ MISS_CACHE → ITER

CHECK_RESP:
  ├─ GET_ANS → RET_RESP (done)
  ├─ GET_CNAME → IN_CACHE (follow CNAME chain)
  └─ GET_NS → IN_GLUE

ITER:
  ├─ NO_ERROR → CHECK_RESP
  └─ ERROR → return SERVFAIL
```

**Key States:**
- `IN_CACHE` - Check cache for direct answer
- `IN_GLUE` - Check if NS/glue records available
- `IN_GLUE_CACHE` - Walk zone hierarchy to find cached glue
- `ITER` - Send query to upstream nameserver

### Key Components
- **Cache** (`server/cache.go`): go-cache library with 5min TTL, stores `*dns.Msg` by domain name
- **IP Pool** (`server/ip_pool.go`): Tracks upstream nameserver quality
  - Latency-based scoring (lower = better)
  - Prefetch mechanism for proactive quality checks
  - Concurrency-limited prefetch (max 10 concurrent)
- **Metrics** (`monitor/metric.go`): Prometheus on `:9999/metric`

## Coding Conventions

```bash
# Format code before commit
gofmt -w .
```

### Go Style
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Exported functions must have doc comments
- Handle errors explicitly, do not ignore

### Naming
- Package: lowercase, no underscores
- Exported: PascalCase
- Private: camelCase

### Error Handling
- Never ignore errors
- Wrap with context: `fmt.Errorf("context: %w", err)`
- Error messages: lowercase, no trailing period

## Reference Implementations

| Project | Feature to Reference |
|---------|---------------------|
| BIND 9 | Authoritative + Recursive, DNSSEC |
| Unbound | State machine architecture, DNSSEC validation |
| CoreDNS | Plugin system, Go patterns |

## Known Issues

- `www.huawei.com` resolution bug (see README.md)

## Documentation

Detailed docs in `.rec53/`:
- `requirements/REQUIREMENTS.md` - Feature requirements
- `requirements/ROADMAP.md` - Version roadmap
- `requirements/PROGRESS.md` - Development progress