# Configuration

English | [中文](configuration.zh.md)

This document covers the main configuration blocks used for a deployable rec53 setup. Start simple and enable optional features only after the baseline path is stable.

## Minimal Configuration

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

## `dns`

```yaml
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"
  upstream_timeout: 1500ms
  hot_zone_base_suffixes: []
  listeners: 0
```

Fields:

- `listen`: DNS bind address
- `metric`: Prometheus metrics listen address
- `log_level`: `debug`, `info`, `warn`, `error`
- `upstream_timeout`: timeout for one upstream iterative query
- `hot_zone_base_suffixes`: optional additive suffix list for hot-zone business-root detection; defaults still include the built-in curated suffix set
- `listeners`: `0` or `1` means classic single listener pair, `>1` enables `SO_REUSEPORT`

Recommendations:

- keep `listen` on loopback during first rollout
- keep `listeners: 0` or `1` unless you have measured contention
- increase `upstream_timeout` only for high-latency networks
- append private suffixes here only when your deployment uses names outside the built-in public-style suffix set

## `warmup`

```yaml
warmup:
  enabled: true
  timeout: 5s
  duration: 5s
  concurrency: 0
  tlds: []
```

Recommendations:

- keep `enabled: true` for the default path
- keep `concurrency: 0` unless you have a reason to override
- leave `tlds` empty to use the curated built-in list

## `hosts`

Static local authority records are answered before forwarding, cache, or iterative lookup.

```yaml
hosts:
  - name: db.internal
    type: A
    value: 10.0.0.5
    ttl: 300
  - name: alias.internal
    type: CNAME
    value: real.internal
```

Supported types:

- `A`
- `AAAA`
- `CNAME`

## `forwarding`

Forward queries for selected zones to dedicated upstream resolvers.

```yaml
forwarding:
  - zone: corp.example.com
    upstreams:
      - 192.168.1.1:53
      - 192.168.1.2:53
```

Rules:

- longest suffix wins
- upstreams are tried in order
- forwarded answers are not cached
- there is no iterative fallback after all forwarding upstreams fail

## Optional: `snapshot`

```yaml
snapshot:
  enabled: true
  file: /var/lib/rec53/cache-snapshot.json
```

Use this only after the default run path is stable. It helps restart latency but is not required for correctness.

## Optional: `debug`

```yaml
debug:
  pprof_enabled: true
  pprof_listen: "127.0.0.1:6060"
```

Never expose `pprof` publicly.

## Optional: `xdp`

```yaml
xdp:
  enabled: false
  interface: ""
```

Treat XDP as platform-specific:

- Linux only
- requires suitable kernel and privileges
- should not be the first deployment step

## Validation Notes

If startup fails, validate first:

- `dns.listen`
- `dns.metric`
- forwarding upstream `host:port` values
- `xdp.interface` when `xdp.enabled: true`

Then check [Troubleshooting](troubleshooting.md).
