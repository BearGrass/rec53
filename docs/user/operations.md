# Operations

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

For local validation, `rec53top` can read the same endpoint directly and show a fixed six-panel terminal dashboard without requiring Prometheus or Grafana:

```bash
go build -o rec53top ./cmd/rec53top
./rec53top
```

If the terminal environment does not support the full-screen UI, use:

```bash
./rec53top -plain
```

Use metrics to watch:

- query volume
- response codes
- end-to-end latency
- cache lookup quality
- snapshot restore/save behavior
- upstream failure reasons and fallback activity
- nameserver quality
- XDP counters when XDP is enabled

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
```

Then check in Prometheus:

- target status is `UP`
- `rec53_query_counter` is increasing
- `rec53_response_counter` has expected response codes
- `rec53_latency` has buckets populated after real queries

### Core Signals

For first deployments, focus on:

- query rate
- SERVFAIL ratio
- p99 query latency
- cache lookup outcome mix from `rec53_cache_lookup_total`
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
