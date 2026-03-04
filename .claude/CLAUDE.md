# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o rec53 cmd/rec53.go

# Run (DNS server on port 5353, metrics on 9999)
./rec53

# Build Docker image
docker build -t rec53 .

# Run with Docker Compose (includes Prometheus, node-exporter)
cd single_machine && docker-compose up -d

# Test
go test ./...
```

## Architecture

**Recursive DNS resolver** implemented as a state machine with caching and IP quality tracking.

### Directory Structure
- `cmd/` - Main entry point
- `server/` - Core DNS logic (state machine, cache, IP pool)
- `monitor/` - Prometheus metrics and logging
- `utils/` - Utility functions (zone resolution, network health checks)
- `single_machine/` - Docker Compose setup for local deployment

### State Machine (`server/server.go`, `server/state_define.go`)

1. **STATE_INIT** → **IN_CACHE** → **CHECK_RESP** → **IN_GLUE** → **ITER** → **RET_RESP**

### Key Components
- **Cache** (`server/cache.go`): LRU cache, 5min TTL
- **IP Pool** (`server/ip_pool.go`): Quality tracking, prefetch
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