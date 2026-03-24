# Operations

English | [中文](operations.zh.md)

This document covers routine operations for a running rec53 instance.

## Run Modes

Foreground validation:

```bash
./rec53ctl run
```

Service deployment:

```bash
sudo ./rec53ctl install
sudo ./rec53ctl upgrade
sudo ./rec53ctl uninstall
sudo ./rec53ctl uninstall --purge
```

## Logs

Default binary behavior writes logs to `./log/rec53.log`.

Installed systemd service behavior is explicit:

- `rec53ctl run` sends logs to stderr so startup failures are visible in the terminal
- `rec53ctl install` writes runtime logs to `/var/log/rec53/rec53.log` by default
- override the service log path with `LOG_FILE=/your/path/rec53.log sudo ./rec53ctl install`
- log rotation is bounded in-process to one active 1 MB file plus up to 5 backups, so app-managed logs stay capped to a few MB instead of growing without limit

Useful patterns:

```bash
tail -f /var/log/rec53/rec53.log
tail -f ./log/rec53.log
journalctl -u rec53 -f
```

`journalctl` is still useful for service start failures, crashes, and stderr output, but the normal rec53 application log stream is the configured `LOG_FILE`.

## Metrics

Metrics are exposed on `http://<host>:9999/metric` by default.

The same operational HTTP surface also exposes a readiness probe:

```bash
curl -s -i http://127.0.0.1:9999/healthz/ready
```

The response body is intentionally small:

```text
ready=true
phase=warming
```

Read it this way:

- `phase` is intentionally bounded lifecycle context, not a full health taxonomy
- `ready=false` with `phase=cold-start` means listener bind has not completed yet
- `ready=true` means rec53 is ready to accept DNS traffic
- `phase=warming` means warmup is still running, but traffic can already be served
- `phase=steady` means listener startup is complete and warmup is no longer active
- `phase=shutting-down` means rec53 is intentionally leaving service

Snapshot restore stays inside the same model:

- missing snapshot file keeps startup on the normal cold-start path
- snapshot restore failure degrades to cold-cache startup; it does not create a new probe phase
- once listeners bind, readiness still follows `warming` or `steady` based on warmup state

For local validation, `rec53top` can read the same endpoint directly and show a fixed six-panel terminal dashboard without requiring Prometheus or Grafana:

```bash
mkdir -p dist && go build -o dist/rec53top ./cmd/rec53top
./dist/rec53top
```

If the terminal environment does not support the full-screen UI, use:

```bash
./dist/rec53top -plain
```

Use metrics to watch:

- query volume
- response codes
- end-to-end latency
- cache lookup quality
- per-client expensive-path refusals
- snapshot restore/save behavior
- upstream failure reasons and fallback activity
- nameserver quality
- XDP counters when XDP is enabled

## Per-Client Expensive Request Protection

rec53 can protect expensive resolution work on a per-client-IP basis.

The first version only applies when a request leaves the local cheap path:

- forwarding miss that must send an external forwarding query
- cache miss that must continue into iterative resolution

It does not count these cheap paths:

- `hosts` hits
- forwarding hits that finish locally
- cache hits

Configuration:

```yaml
dns:
  expensive_request_limit_mode: "enabled"
  expensive_request_limit: 0
```

Operational meaning:

- only `disabled` and `enabled` are product modes
- `0` uses the built-in default limit: `runtime.NumCPU()`
- one front-end request consumes at most one slot even if it fans out internally
- over-limit requests return `REFUSED`
- logs are rate-limited per client IP and include a suppressed-event count
- the first version is metric/log driven only and does not add a per-IP TUI view

## Hot-Zone Expensive Path Protection

rec53 now also watches whether many requesters are collectively pushing the same zone into the expensive path.

The first version keeps the scope narrow:

- it only protects one hot zone at a time
- it only blocks new expensive-path entry for that zone
- it still allows cheap paths such as `hosts` hits, forwarding hits, and cache hits
- refused hot-zone requests also return `REFUSED`

Business-root selection uses this order:

- matched forwarding zone
- built-in base suffixes plus any operator-added `dns.hot_zone_base_suffixes`
- fallback level-3 domain when neither higher-priority rule matches

Observe mode and protection are intentionally conservative:

- short window: `5s`
- selection uses the most recent `3` windows by simple occupancy summation
- observe mode requires `avg_expensive_concurrency >= 0.75 * NumCPU()` and host CPU `>= 70%`
- the same candidate must persist for `3` consecutive observe windows before protection starts
- protection exits after global expensive occupancy falls back to within `1.05x` of the pre-trigger baseline

### Prometheus Scrape Setup

Example scrape config:

```yaml
scrape_configs:
  - job_name: "rec53"
    metrics_path: /metric
    scrape_interval: 5s
    static_configs:
      - targets:
          - "127.0.0.1:9999"
```

The repository also includes a sample file at [`etc/prometheus.yml`](../../etc/prometheus.yml).

### What To Verify First

After startup, verify:

```bash
curl -s http://127.0.0.1:9999/metric | head
curl -s http://127.0.0.1:9999/healthz/ready
```

Then check in Prometheus:

- target status is `UP`
- `rec53_query_counter` is increasing
- `rec53_response_counter` has expected response codes
- `rec53_latency` has buckets populated after real queries

### Probe Examples

Systemd-friendly local check:

```bash
until curl -fsS http://127.0.0.1:9999/healthz/ready >/tmp/rec53.ready; do sleep 1; done
cat /tmp/rec53.ready
```

Container-style readiness probe:

```yaml
readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 9999
```

Treat the probe as readiness-only:

- use HTTP status for admission decisions
- use the body only to distinguish bounded lifecycle states
- do not treat `phase` as a substitute for logs, metrics, or incident diagnosis

### Core Signals

For first deployments, focus on:

- query rate
- SERVFAIL ratio
- p99 query latency
- cache lookup outcome mix from `rec53_cache_lookup_total`
- policy pressure from `rec53_expensive_request_limit_total`
- snapshot load/save outcomes from `rec53_snapshot_operations_total`
- upstream timeout and bad-rcode trends from `rec53_upstream_failures_total`
- degraded upstream IPs from `rec53_ipv2_p50_latency_ms`
- XDP counters only when XDP is explicitly enabled

See [Metrics](../metrics.md) for metric definitions and PromQL examples, [Local Ops TUI](local-ops-tui.md) for the zero-dependency local dashboard path, [Observability Dashboard](observability-dashboard.md) for the baseline panel layout, and [Operator Checklist](operator-checklist.md) for symptom-first triage.

## pprof

Enable only for debugging:

```yaml
debug:
  pprof_enabled: true
  pprof_listen: "127.0.0.1:6060"
```

Example:

```bash
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

Keep `pprof_listen` on loopback.

## Cache Snapshot

Snapshot can reduce restart cold-start effects:

```yaml
snapshot:
  enabled: true
  file: /var/lib/rec53/cache-snapshot.json
```

Operational guidance:

- make sure the snapshot path is writable
- treat snapshot as a startup optimization, not a source of truth
- verify shutdown completes cleanly so snapshots are actually written
- watch `rec53_snapshot_operations_total`, `rec53_snapshot_entries_total`, and `rec53_snapshot_duration_ms` after restart if cold-start quality changes

## Optional Features

Use these only after the baseline Go path is working:

- `dns.listeners > 1` for `SO_REUSEPORT`
- `snapshot.enabled: true`
- `debug.pprof_enabled: true`
- `xdp.enabled: true`

## Upgrade Strategy

Recommended:

```bash
sudo ./rec53ctl upgrade
```

This keeps the config in place and updates the binary. Validate after upgrade with:

```bash
systemctl status rec53
dig @127.0.0.1 -p 5353 example.com
```

## Install And Uninstall Safety

`rec53ctl` is now conservative by default:

- `install` refuses to overwrite an existing systemd unit or binary unless they are marked as managed by `rec53ctl`
- `uninstall` removes the managed unit and binary, but preserves config and logs by default
- use `sudo ./rec53ctl uninstall --purge` only when you explicitly want to remove managed config and log files too

This reduces the chance of deleting user-managed files or clobbering unrelated system resources.
