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
```

### CLI Flags
- `-listen` - DNS server address (default: `127.0.0.1:5353`)
- `-metric` - Prometheus metrics address (default: `:9999`)
- `-log-level` - Log level: debug, info, warn, error (default: `info`)
- `-version` - Show version information

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test -v ./server/...

# Run specific test
go test -v -run TestCacheHit ./server/

# Run E2E tests (require network access)
go test -v ./e2e/...

# Format code before commit
gofmt -w .

# Run linter
go vet ./...
```

## Architecture

**Recursive DNS resolver** implemented as a state machine with caching and IP quality tracking.

### Package Structure
- `cmd/` - Entry point, flag parsing, signal handling, graceful shutdown
- `server/` - Core DNS logic: state machine, cache, IP pool, UDP/TCP server
- `monitor/` - Prometheus metrics (`metric.go`) and zap logger (`log.go`)
- `utils/` - Zone resolution (`GetZoneList`), root servers
- `e2e/` - End-to-end integration tests with mock DNS servers
- `single_machine/` - Docker Compose setup for local deployment

### State Machine (`server/state_machine.go`, `server/state_define.go`)

The DNS resolution flow is a state machine where each state implements the `stateMachine` interface and returns a result code that determines the next state.

**States and Transitions:**
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
  ├─ GET_CNAME → IN_CACHE (follow CNAME chain, tracked for cycles)
  └─ GET_NS → IN_GLUE

ITER:
  ├─ NO_ERROR → CHECK_RESP
  └─ ERROR → return SERVFAIL
```

**Key Implementation Details:**
- `MaxIterations = 50` prevents infinite loops (state_machine.go:13)
- CNAME cycle detection via `visitedDomains` map (state_machine.go:27)
- Original question is preserved and restored in response (state_machine.go:31, :174)
- ITER state handles EDNS0 with 4096 buffer size (state_define.go:249)

### Key Components

**Cache (`server/cache.go`):**
- Uses `patrickmn/go-cache` library with 5min default TTL, 10min cleanup interval
- Cache key format: `domain:qtype` (e.g., `google.com.:1` for A record)
- `getCacheCopy()` returns deep copy to prevent concurrent modification
- Global instance: `globalDnsCache`

**IP Pool (`server/ip_pool.go`):**
- Tracks upstream nameserver quality via latency scoring
- Initial latency: 1000ms, max penalty: 10000ms
- Prefetch mechanism with max 10 concurrent checks (`MAX_PREFETCH_CONCUR`)
- Global instance: `globalIPPool`
- Must call `Shutdown()` for graceful termination

**Server (`server/server.go`):**
- Single `ServeDNS` handler for both UDP and TCP
- UDP truncation handling with TC flag (server.go:88-120)
- Graceful shutdown via context with 5-second timeout

**Metrics (`monitor/metric.go`):**
- `rec53_in_total` - Incoming queries (stage, name, type)
- `rec53_out_total` - Outgoing responses (stage, name, type, code)
- `rec53_latency_ms` - Query latency histogram
- `rec53_ip_quality` - Nameserver latency gauge

## Dependencies

- `github.com/miekg/dns` - DNS library for server and message handling
- `github.com/patrickmn/go-cache` - In-memory cache with TTL
- `go.uber.org/zap` - Structured logging
- `github.com/prometheus/client_golang` - Metrics exposition

## E2E Testing Pattern

The `e2e/` package provides integration testing utilities:

```go
// Create mock authority server
zone := &Zone{
    Origin: "example.com.",
    Records: map[uint16][]dns.RR{
        dns.TypeA: {A("example.com.", "192.0.2.1", 300)},
    },
}
mock := NewMockAuthorityServer(t, zone)
defer mock.Stop()

// Create test resolver
handler := server.NewServer("127.0.0.1:0")
tr, _ := NewTestResolver(handler)
defer tr.Stop()

// Query
resp, err := tr.Query("example.com.", dns.TypeA)
```

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

- `www.huawei.com` resolution may have issues with certain CNAME chains
- Some domains with complex CNAME chains may return SERVFAIL when the final A/AAAA resolution fails

## Documentation

Project documentation in `.rec53/`:
- [`.rec53/README.md`](../.rec53/README.md) - Documentation index
- `.rec53/roadmap/ROADMAP.md` - Version roadmap
- `.rec53/roadmap/REQUIREMENTS.md` - Feature requirements
- `.rec53/progress/PROGRESS.md` - Development progress
- `.rec53/quality/CODE_QUALITY.md` - Code quality analysis and optimization plan
- `.rec53/bugs/` - Bug tracking records
- `.rec53/decisions/` - Architecture Decision Records (ADR)