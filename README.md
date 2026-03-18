# rec53

English | [中文](README.zh.md)

A recursive DNS resolver implemented in Go with state machine architecture, IP quality tracking, and Prometheus metrics.
rec53 is positioned as a lightweight, endpoint-side recursive resolver for personal devices and production cluster nodes (including host machines). It is intended to replace the OS-provided resolver at the node level to improve local DNS capability and offload centralized enterprise or ISP recursive DNS infrastructure, rather than serving as a centralized recursive cluster itself.

## Features

- **Full Iterative Resolution** — resolves from root servers, no upstream forwarding
- **Hosts Local Authority** — serve static A/AAAA/CNAME records from config, with AA flag, before any cache or upstream lookup
- **Forwarding Rules** — forward queries for specific domain suffixes to designated upstream DNS servers (longest-suffix match)
- **UDP/TCP Support** — dual-protocol listeners on the same port
- **SO_REUSEPORT Multi-Listener** — bind N UDP+TCP listener pairs on the same address via `SO_REUSEPORT` for kernel-level load balancing (Linux; `dns.listeners` config)
- **State Machine Architecture** — clean, auditable resolution pipeline with 9 states
- **IPQualityV2** — sliding-window latency histograms with automatic fault recovery
- **Happy Eyeballs Concurrency** — simultaneous queries to best and secondary nameserver; first response wins
- **Bad Rcode Failover** — automatic retry on secondary NS when primary returns SERVFAIL / REFUSED / FORMERR
- **EDNS0 & UDP Truncation** — 4096-byte EDNS0 buffer; TC flag with progressive answer trimming on UDP overflow
- **TTL-based Caching** — deep-copy safe cache with negative caching (NXDOMAIN/NODATA)
- **NS Warmup** — pre-populates IP pool on startup for low-latency cold start
- **Cache Snapshot** — persists full DNS cache on shutdown and restores it before first query, eliminating cold-start latency after restart
- **Prometheus Metrics** — per-query and per-nameserver observability
- **XDP/eBPF Cache Fast Path** — cache hits served directly from the kernel via `XDP_TX` (zero syscalls, zero Go runtime overhead); requires Linux kernel >= 5.15 and CAP_BPF
- **Graceful Shutdown** — context-based cancellation with 5-second timeout

---

> **v0.5.0 Breaking Change:** The `name` label (raw FQDN) has been removed from
> Prometheus metrics `rec53_query_counter`, `rec53_response_counter`, and
> `rec53_latency`. This eliminates unbounded label cardinality and reduces
> hot-path allocations. **Migration:** remove `name` from any PromQL queries or
> Grafana dashboard selectors that reference these metrics.
> See [CHANGELOG.md](CHANGELOG.md) for details.

## Quick Start

### Using rec53ctl (recommended)

`rec53ctl` is the single-entry operational script covering the full lifecycle of rec53.
Recommended workflow: generate a config template with `./generate-config.sh`, review and edit `config.yaml`, then use `rec53ctl` to run or install the service.

```bash
# 1. Generate default config (first run only)
./generate-config.sh

# 2. Build binary (outputs to dist/rec53)
./rec53ctl build

# 3. Run in foreground with terminal log output
./rec53ctl run

# 4. Install as systemd service (requires root)
sudo ./rec53ctl install

# 5. Upgrade running service (build + hot-swap + auto-rollback)
sudo ./rec53ctl upgrade

# 6. Uninstall service and files (requires root)
sudo ./rec53ctl uninstall
```

Key options:

```bash
# Force overwrite existing /etc/rec53/config.yaml on install
sudo ./rec53ctl install --force-config

# Upgrade without recompiling (use pre-built dist/rec53)
SKIP_BUILD=1 sudo ./rec53ctl upgrade

# Run with a custom config file
CONFIG_FILE=./my-config.yaml ./rec53ctl run
```

Override default paths via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `INSTALL_DIR` | `/usr/local/bin` | Binary installation directory |
| `CONFIG_DIR` | `/etc/rec53` | Config directory |
| `BINARY_NAME` | `rec53` | Binary file name |
| `SERVICE_NAME` | `rec53` | Systemd service name |
| `BUILD_OUTPUT` | `dist/rec53` | Build output path |

### Manual (without rec53ctl)

Manual usage follows the same config flow: generate `config.yaml` first, adjust it for your environment, then start `rec53` with `--config`.

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

# View logs (written to file, not stdout)
tail -f ./log/rec53.log
```

---

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | *(required)* | Path to YAML config file |
| `-listen` | `127.0.0.1:5353` | DNS listen address |
| `-metric` | `:9999` | Prometheus metrics address |
| `-log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `-no-warmup` | `false` | Disable NS warmup on startup |
| `-rec53.log` | `./log/rec53.log` | Log file path (`/dev/stderr` writes to terminal) |
| `-version` | `false` | Print version and exit |

> **Note**: `-listen`, `-metric`, and `-log-level` only override the config file when their value differs from the default. For example, `-log-level info` cannot override a config file that sets `log_level: debug`.

---

## Configuration

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"
  # upstream_timeout: 1500ms  # Per-query timeout for iterative resolution.
                              # Default: 1.5s. Increase to 3-5s on high-latency networks.
                              # Minimum: 100ms.
  # listeners: 0             # Number of UDP+TCP listener pairs bound via SO_REUSEPORT.
                              # 0 or 1 = single pair (classic). >1 = N parallel pairs.
                              # Recommended: match CPU core count. Linux only; ignored elsewhere.

warmup:
  enabled: true
  timeout: 5s        # per-query timeout during warmup
  duration: 5s       # total warmup budget
  concurrency: 0     # 0 = auto (min(NumCPU*2, 8)); >0 = manual override
  tlds:              # leave empty to use curated 30-TLD defaults
    - com
    - net
    - org

# Static local DNS records — answered authoritatively (AA=true) before cache or iterative.
# Priority: hosts > forwarding > cache > iterative.
# Supported types: A, AAAA, CNAME. TTL defaults to 60s if omitted.
hosts:
  - name: db.internal
    type: A
    value: 10.0.0.5
    ttl: 300
  - name: ipv6.internal
    type: AAAA
    value: "fd00::1"
  - name: alias.internal
    type: CNAME
    value: real.internal

# Forward queries for specific domain suffixes to dedicated upstream DNS servers.
# Longest-suffix match wins. Results are NOT cached. All upstreams tried in order;
# SERVFAIL returned if all fail (no iterative fallback).
forwarding:
  - zone: corp.example.com
    upstreams:
      - 192.168.1.1:53
      - 192.168.1.2:53
  - zone: internal
    upstreams:
      - 10.0.0.53:53

# Cache snapshot: persist all DNS cache entries on shutdown and restore on startup.
# Eliminates cold-start latency (typically 300ms+) caused by rebuilding the cache
# after a restart. Disabled by default; file must be set to enable.
# snapshot:
#   enabled: false
#   file: ""   # e.g. /var/lib/rec53/cache-snapshot.json  or  ~/.rec53/cache-snapshot.json

# XDP/eBPF cache fast path: cache hits served directly from the kernel via XDP_TX.
# Zero syscalls, zero Go runtime overhead, zero memory copies for cache hits.
# Requirements: Linux kernel >= 5.15, CAP_BPF or root, clang >= 14 (build time only).
# xdp:
#   enabled: false
#   interface: ""   # e.g. eth0, ens33
```

| `snapshot` field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Enable snapshot save/restore |
| `file` | string | `""` | Path to snapshot file; empty string disables even if `enabled: true` |

| `xdp` field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Enable XDP/eBPF DNS cache fast path |
| `interface` | string | `""` | Network interface to attach XDP program (required when enabled) |

By default, rec53 warms up 30 high-traffic TLDs covering 85%+ of global registrations. To use a custom list, specify `warmup.tlds`. Leave empty for the curated defaults.

---

## Profiling / pprof

rec53 includes a built-in pprof HTTP endpoint for heap, CPU, and goroutine profiling. It is **off by default** and runs on a separate HTTP server from the metrics endpoint.

**Configuration** (`config.yaml`):

```yaml
debug:
  pprof_enabled: true              # default: false
  pprof_listen: "127.0.0.1:6060"   # default: 127.0.0.1:6060
```

> **Security**: Never bind `pprof_listen` to `0.0.0.0` in production. pprof exposes internal runtime data. Use SSH tunneling for remote access: `ssh -L 6060:127.0.0.1:6060 user@server`.

**Usage**:

```bash
# Heap profile (memory allocations)
go tool pprof http://127.0.0.1:6060/debug/pprof/heap

# CPU profile (30-second sample)
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30

# Goroutine dump
go tool pprof http://127.0.0.1:6060/debug/pprof/goroutine

# Browse all profiles in browser
open http://127.0.0.1:6060/debug/pprof/
```

The pprof server participates in graceful shutdown — it stops accepting new requests when rec53 receives SIGINT/SIGTERM.

---

## Docker

```bash
# Build image
docker build -t rec53 .

# Run standalone (mount log directory to access logs from host)
docker run -d \
  -p 5353:5353/udp \
  -p 5353:5353/tcp \
  -p 9999:9999 \
  -v $(pwd)/log:/dist/log \
  rec53

# Run with Docker Compose (includes Prometheus + node-exporter)
cd single_machine && docker-compose up -d
```

---

## Known Limitations

- DNSSEC validation not implemented
- DoT / DoH not supported
- `www.huawei.com` and similar complex CNAME chains may return SERVFAIL when the final A/AAAA resolution fails
- Logs are written **only to file** (default `./log/rec53.log`), not to stdout/stderr by default. Use `-rec53.log /dev/stderr` to write logs to the terminal instead. In Docker, mount the log directory (e.g. `-v $(pwd)/log:/dist/log`) to access logs from the host.

---

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — system design, state machine, cache, IP pool
- [`docs/benchmarks.md`](docs/benchmarks.md) — latency, QPS, memory benchmarks
- [`docs/recursive-dns-test-plan.md`](docs/recursive-dns-test-plan.md) — complete recursive DNS test plan (functional + performance + release gates)
- [`docs/perf-regression.md`](docs/perf-regression.md) — performance regression workflow and acceptance criteria
- [`tools/validate-perf.sh`](tools/validate-perf.sh) — one-command perf validation (dnsperf + pprof, Linux dev workflow)
- [`docs/metrics.md`](docs/metrics.md) — Prometheus metrics and PromQL examples
- [`docs/sdns-comparison.md`](docs/sdns-comparison.md) — feature comparison with sdns
- [`.rec53/CONVENTIONS.md`](.rec53/CONVENTIONS.md) — code conventions and patterns
- [`.rec53/ROADMAP.md`](.rec53/ROADMAP.md) — roadmap and planned features

## References

- [miekg/dns](https://github.com/miekg/dns) — DNS protocol library for Go
- [RFC 1034](https://datatracker.ietf.org/doc/html/rfc1034) — DNS concepts and facilities
- [RFC 1035](https://datatracker.ietf.org/doc/html/rfc1035) — DNS implementation and specification
- [RFC 2308](https://datatracker.ietf.org/doc/html/rfc2308) — Negative caching of DNS queries
