# rec53

English | [中文](README.zh.md)

A recursive DNS resolver implemented in Go with state machine architecture, IP quality tracking, and Prometheus metrics.

## Features

- **Full Iterative Resolution** — resolves from root servers, no upstream forwarding
- **UDP/TCP Support** — dual-protocol listeners on the same port
- **State Machine Architecture** — clean, auditable resolution pipeline with 7 states
- **IPQualityV2** — sliding-window latency histograms with automatic fault recovery
- **TTL-based Caching** — deep-copy safe cache with negative caching (NXDOMAIN/NODATA)
- **NS Warmup** — pre-populates IP pool on startup for low-latency cold start
- **Prometheus Metrics** — per-query and per-nameserver observability
- **Graceful Shutdown** — context-based cancellation with 5-second timeout

---

## Quick Start

```bash
# Build
go build -o rec53 ./cmd

# Generate default config (first run)
./generate-config.sh

# Run with config
./rec53 --config ./config.yaml

# Run with overrides
./rec53 --config ./config.yaml -listen 0.0.0.0:53 -metric :9099 -log-level debug

# Test resolution
dig @127.0.0.1 -p 5353 google.com
dig @127.0.0.1 -p 5353 google.com AAAA
```

---

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | *(required)* | Path to YAML config file |
| `-listen` | `127.0.0.1:5353` | DNS listen address (overrides config) |
| `-metric` | `:9999` | Prometheus metrics address (overrides config) |
| `-log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `-no-warmup` | `false` | Disable NS warmup on startup |
| `-version` | `false` | Print version and exit |

CLI flags take precedence over config file values.

---

## Configuration

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"

warmup:
  enabled: true
  timeout: 5s        # per-query timeout during warmup
  duration: 5s       # total warmup budget
  concurrency: 0     # 0 = auto (min(NumCPU*2, 8)); >0 = manual override
  tlds:              # leave empty to use curated 30-TLD defaults
    - com
    - net
    - org
```

By default, rec53 warms up 30 high-traffic TLDs covering 85%+ of global registrations. To use a custom list, specify `warmup.tlds`. Leave empty for the curated defaults.

---

## Docker

```bash
# Build image
docker build -t rec53 .

# Run standalone
docker run -d \
  -p 5353:5353/udp \
  -p 5353:5353/tcp \
  -p 9999:9999 \
  rec53

# Run with Docker Compose (includes Prometheus + node-exporter)
cd single_machine && docker-compose up -d
```

---

## Known Limitations

- DNSSEC validation not implemented
- DoT / DoH not supported
- `www.huawei.com` and similar complex CNAME chains may return SERVFAIL when the final A/AAAA resolution fails

---

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — system design, state machine, cache, IP pool
- [`docs/benchmarks.md`](docs/benchmarks.md) — latency, QPS, memory benchmarks
- [`docs/metrics.md`](docs/metrics.md) — Prometheus metrics and PromQL examples
- [`docs/sdns-comparison.md`](docs/sdns-comparison.md) — feature comparison with sdns
- [`.rec53/CONVENTIONS.md`](.rec53/CONVENTIONS.md) — code conventions and patterns
- [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) — roadmap and planned features

## References

- [miekg/dns](https://github.com/miekg/dns) — DNS protocol library for Go
- [RFC 1034](https://datatracker.ietf.org/doc/html/rfc1034) — DNS concepts and facilities
- [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035) — DNS implementation and specification
- [RFC 2308](https://datatracker.ietf.org/doc/html/rfc2308) — Negative caching of DNS queries
