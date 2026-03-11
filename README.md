# rec53

A recursive DNS resolver implemented in Go with state machine architecture, IP quality tracking, and Prometheus metrics.

## Features

- **Recursive DNS Resolution** - Full iterative resolution from root servers
- **UDP/TCP Support** - Listens on both protocols simultaneously
- **Smart Caching** - LRU cache with TTL-based expiration (5 min default)
- **IP Quality Tracking** - Monitors upstream nameserver latency for optimal server selection
- **IP Prefetch** - Proactively checks nameserver quality for better performance
- **Prometheus Metrics** - Built-in metrics endpoint for monitoring
- **Graceful Shutdown** - Clean shutdown with timeout handling

## Quick Start

```bash
# Build
go build -o rec53 ./cmd

# Run (DNS on :5353, metrics on :9999)
./rec53

# Run with custom configuration
./rec53 -listen 0.0.0.0:53 -metric :9099 -log-level debug

# Test resolution
dig @127.0.0.1 -p 5353 google.com
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `127.0.0.1:5353` | DNS server listen address |
| `-metric` | `:9999` | Prometheus metrics address |
| `-log-level` | `info` | Log level: debug, info, warn, error |
| `-version` | `false` | Show version information |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         cmd/rec53.go                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ Flag Parsing│→ │   Server    │→ │  Graceful Shutdown      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       server/                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              State Machine (state_machine.go)             │   │
│  │                                                           │   │
│  │  STATE_INIT → IN_CACHE → CHECK_RESP → IN_GLUE → ITER →   │   │
│  │      │           │            │           │        │      │   │
│  │      │      ┌────┴────┐       │      ┌────┴───┐    │      │   │
│  │      │      │         │       │      │        │    │      │   │
│  │      ▼      ▼         ▼       ▼      ▼        ▼    ▼      │   │
│  │  [Init]  [Hit/Miss] [Ans/CNAME/NS] [Exist/Cache] [Query]  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Cache     │  │   IP Pool   │  │    DNS Server (UDP/TCP) │  │
│  │ (cache.go)  │  │(ip_pool.go) │  │      (server.go)        │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       monitor/                                  │
│  ┌─────────────────────┐  ┌─────────────────────────────────┐   │
│  │  Prometheus Metrics │  │         Zap Logger              │   │
│  │    (metric.go)      │  │         (log.go)                │   │
│  └─────────────────────┘  └─────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### State Machine Flow

```
                    ┌──────────────┐
                    │  STATE_INIT  │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
              ┌────→│   IN_CACHE   │←─────────────────────────┐
              │     └──────┬───────┘                          │
              │            │                                  │
         CNAME │     ┌──────┴──────┐                          │
              │     │             │                          │
              │   Hit            Miss                         │
              │     │             │                          │
              │     ▼             ▼                          │
              │ ┌────────┐  ┌──────────────┐                  │
              │ │CHECK_  │  │   IN_GLUE    │                  │
              │ │RESP    │  └──────┬───────┘                  │
              │ └───┬────┘         │                          │
              │     │       ┌──────┴──────┐                   │
              │     │       │             │                   │
              │     │    Exist      Not Exist                 │
              │     │       │             │                   │
              │     │       ▼             ▼                   │
              │     │  ┌────────┐  ┌──────────────┐           │
              │     │  │  ITER  │  │ IN_GLUE_CACHE│           │
              │     │  └───┬────┘  └──────┬───────┘           │
              │     │      │              │                   │
              │     │      │        ┌─────┴─────┐             │
              │     │      │        │           │             │
              │     │      │      Hit         Miss            │
              │     │      │        │           │             │
              │     │      │        └─────┬─────┘             │
              │     │      │              │                   │
              │     │      ▼              ▼                   │
              │     │  ┌──────────────────────┐               │
              │     │  │        ITER          │───────────────┤
              │     │  └──────────┬───────────┘               │
              │     │             │                           │
              │     │      Success                            │
              │     │             │                           │
              │     │             ▼                           │
              │     │      ┌──────────────┐                   │
              │     └──────│  CHECK_RESP  │                   │
              │            └──────┬───────┘                   │
              │                   │                           │
              │        ┌──────────┼──────────┐                │
              │        │          │          │                │
              │     Answer     CNAME       NS                 │
              │        │          │          │                │
              │        ▼          └──────────┘                │
              │  ┌──────────┐                                 │
              │  │RET_RESP  │─────────────────────────────────┘
              │  └──────────┘
              │       │
              └───────┘ (Done)
```

### Key Components

| Component | File | Description |
|-----------|------|-------------|
| State Machine | `server/state_machine.go` | Core DNS resolution logic |
| States | `server/state_define.go`, `server/state.go` | State definitions and handlers |
| Cache | `server/cache.go` | DNS response cache with TTL |
| IP Pool | `server/ip_pool.go` | Nameserver quality tracking & prefetch |
| Server | `server/server.go` | UDP/TCP DNS server |
| Metrics | `monitor/metric.go` | Prometheus integration |
| Logger | `monitor/log.go` | Zap structured logging |

## IP Quality Algorithm (IPQualityV2)

The resolver implements **IPQualityV2**, a sliding-window histogram-based system for intelligent nameserver selection with automatic fault recovery:

### Key Features

1. **Sliding Window Histogram** - Maintains last 64 RTT samples per IP for percentile calculation
   - P50 (median) - Primary metric for server selection
   - P95, P99 - Monitoring percentiles for detecting outliers

2. **Exponential Backoff Failure Handling** - Graceful degradation on server failures
   - Phase 1 (1-3 failures): DEGRADED state with 20% latency penalty
   - Phase 2 (4-6 failures): SUSPECT state with MAX latency (10000ms)
   - Phase 3 (7+ failures): Eligible for automatic recovery probing

3. **Confidence-Based Selection** - Encourages sampling of new servers
   - Confidence: 0-100% based on sample count (≥10 samples = 100%)
   - Low-confidence IPs get 2x score multiplier to boost exploration
   - Balances best-known performance with discovery of better servers

4. **Composite Scoring Formula**
   ```
   score = p50_latency × confidence_multiplier × state_weight
   
   Confidence multiplier: 2.0 (0% confidence) → 1.0 (100% confidence)
   State weights: ACTIVE(1.0), DEGRADED(1.5), SUSPECT(100.0), RECOVERED(1.1)
   ```

5. **Automatic Recovery Probing** - Background goroutine probes SUSPECT IPs
   - Runs every 30 seconds non-blocking to queries
   - Identifies recovery candidates via A record queries
   - Resets to ACTIVE state on successful probe

6. **Prometheus Metrics Export**
   - `rec53_ipv2_p50_latency_ms` - Median latency per IP
   - `rec53_ipv2_p95_latency_ms` - 95th percentile per IP
   - `rec53_ipv2_p99_latency_ms` - 99th percentile per IP

### Performance Characteristics

- **Selection Speed**: 94-98 µs for 1000 IPs (10x under 1ms target)
- **Memory Usage**: ~24KB per 1000 IPs (64 samples × 8 bytes + metadata)
- **Fault Recovery Time**: 30-60 seconds for SUSPECT IPs via background probing

## Monitoring

### Prometheus Metrics

Metrics available at `http://localhost:9999/metric`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `rec53_in_total` | Counter | stage, name, type | Incoming queries |
| `rec53_out_total` | Counter | stage, name, type, code | Outgoing responses |
| `rec53_latency_ms` | Histogram | stage, name, type, code | Query latency |
| `rec53_ipv2_p50_latency_ms` | Gauge | ip | Median nameserver latency |
| `rec53_ipv2_p95_latency_ms` | Gauge | ip | 95th percentile nameserver latency |
| `rec53_ipv2_p99_latency_ms` | Gauge | ip | 99th percentile nameserver latency |

### Grafana Dashboard

Use the Prometheus data source to visualize:
- Query rate by domain/type
- Response code distribution
- P50/P99 latency
- Top queried domains
- Nameserver quality over time

## Docker Deployment

```bash
# Build image
docker build -t rec53 .

# Run standalone
docker run -d -p 5353:5353/udp -p 5353:5353 -p 9999:9999 rec53

# Run with Docker Compose (includes Prometheus)
cd single_machine && docker-compose up -d
```

### Docker Compose Services

| Service | Port | Description |
|---------|------|-------------|
| rec53 | 5353 (UDP/TCP), 9999 | DNS server + metrics |
| prometheus | 9090 | Metrics collection |
| node-exporter | 9100 | Host metrics |

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Format code
gofmt -w .

# Run linter
go vet ./...
```

## Known Issues

- `www.huawei.com` resolution may have issues with certain CNAME chains
- Some domains with complex CNAME chains may return SERVFAIL when the final A/AAAA resolution fails

## Roadmap

See [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) for planned features:
- DNSSEC validation
- DoT/DoH support
- Concurrent queries
- Query rate limiting

## Documentation

- [`.rec53/README.md`](.rec53/README.md) - Project documentation index
- [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) - Roadmap and requirements

## References

- [miekg/dns](https://github.com/miekg/dns) - DNS library for Go
- [Unbound](https://nlnetlabs.nl/projects/unbound/about/) - Reference state machine architecture
- [RFC 1034](https://datatracker.ietf.org/doc/html/rfc1034) - DNS concepts
- [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035) - DNS implementation