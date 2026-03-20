# rec53

English | [中文](README.zh.md)

rec53 is a lightweight recursive DNS resolver written in Go for node-local and host-local deployment. It is designed to replace the OS resolver on a single machine or cluster node, not to act as a centralized recursive DNS cluster.

## Positioning

- Recommended baseline: iterative resolution, hosts authority, forwarding rules, cache, warmup, metrics
- Optional enhancements: cache snapshot, pprof, SO_REUSEPORT multi-listener
- Platform-specific feature: XDP/eBPF cache fast path on Linux

## Release Scope

`v1.0.0` is intended for personal users and simple IT environments:

- single-machine and node-local recursive DNS
- development hosts, home labs, and small internal deployments
- explicit operator-managed systemd deployment on Linux

It is not positioned as:

- a public open resolver
- a centralized recursive DNS fleet
- a high-availability enterprise DNS platform

Keep XDP optional for now. The Go path is the `v1.0.0` baseline.

## Quick Start

Recommended workflow:

```bash
# 1. Generate a config template
./generate-config.sh

# 2. Review and edit config.yaml

# 3. Build
./rec53ctl build

# 4. Run in foreground
./rec53ctl run

# 5. Verify DNS answers
dig @127.0.0.1 -p 5353 example.com
dig @127.0.0.1 -p 5353 example.com AAAA
```

Manual run:

```bash
go build -o rec53 ./cmd
./rec53 --config ./config.yaml
```

## Minimum Configuration

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"

warmup:
  enabled: true
  timeout: 5s
  duration: 5s
```

Recommended operator path:

- Start with `rec53ctl run` for local validation
- Use `rec53ctl install` for systemd-based deployment
- Keep XDP disabled until the Go path is stable in your environment

## Core Features

- Full iterative resolution from root servers
- Local hosts authority for `A` / `AAAA` / `CNAME`
- Forwarding rules with longest-suffix match
- UDP and TCP DNS listeners
- TTL cache with negative caching
- NS warmup to reduce cold-start latency
- Prometheus metrics and optional pprof
- Graceful shutdown and optional cache snapshot restore

## Operations

`rec53ctl` is the recommended operational entrypoint:

```bash
./rec53ctl build
./rec53ctl run
sudo ./rec53ctl install
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

Installed services write application logs to `/var/log/rec53/rec53.log` by default. Foreground `rec53ctl run` sends logs to stderr for immediate visibility.

Key CLI flags:

| Flag | Default | Description |
|---|---|---|
| `--config` | required | YAML config file |
| `-listen` | `127.0.0.1:5353` | DNS listen address |
| `-metric` | `:9999` | Metrics address |
| `-log-level` | `info` | `debug`, `info`, `warn`, `error` |
| `-no-warmup` | `false` | Disable NS warmup |
| `-rec53.log` | `./log/rec53.log` | Log file path |
| `-version` | `false` | Print version and exit |

## Documentation

User docs:

- [Quick Start](docs/user/quick-start.md)
- [Configuration](docs/user/configuration.md)
- [Operations](docs/user/operations.md)
- [Troubleshooting](docs/user/troubleshooting.md)
- [Observability Dashboard](docs/user/observability-dashboard.md)
- [Operator Checklist](docs/user/operator-checklist.md)

Developer docs:

- [Developer Docs Index](docs/dev/README.md)
- [Architecture](docs/architecture.md)
- [Contributing](docs/dev/contributing.md)
- [Testing](docs/dev/testing.md)
- [Release Checklist](docs/dev/release.md)

Reference docs:

- [Metrics](docs/metrics.md)
- [Metrics (Chinese)](docs/metrics.zh.md)
- [Testing Docs Index](docs/testing/README.md)
- [Benchmarks](docs/testing/benchmarks.md)
- [Physical NIC XDP Benchmark Report (Chinese)](docs/testing/xdp-physical-benchmark-2026-03-19.zh.md)
- [Performance Regression Notes](docs/testing/perf-regression.md)
- [Conventions](.rec53/CONVENTIONS.md)
- [Roadmap](.rec53/ROADMAP.md)
