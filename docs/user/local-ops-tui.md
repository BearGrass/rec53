# Local Ops TUI

`rec53top` is the local terminal dashboard for rec53. It reads the rec53 Prometheus endpoint directly and renders a fixed six-panel view for traffic, cache, snapshot, upstream, XDP, and state-machine health.

## Scope

`v1.1.2` is intentionally a small MVP:

- single target only
- read-only dashboard
- current-state and short-window summaries
- no Prometheus server or Grafana required

It does not try to replace `docs/metrics.md`, `docs/user/observability-dashboard.md`, or future multi-node observability work.

## Run

Build:

```bash
go build -o rec53top ./cmd/rec53top
```

Run against the default local endpoint:

```bash
./rec53top
```

Override the metrics endpoint:

```bash
./rec53top -target http://127.0.0.1:9999/metric
```

Useful flags:

- `-target`: metrics endpoint, default `http://127.0.0.1:9999/metric`
- `-refresh`: dashboard refresh interval, default `2s`
- `-timeout`: scrape timeout, default `1500ms`

If the terminal opens but does not render correctly, first retry with an explicit terminal type:

```bash
TERM=xterm-256color ./rec53top
```

## Keys

- `q`: quit
- `r`: refresh immediately
- `h` or `?`: toggle help and status legend

## Status Model

The TUI uses a small fixed set of states:

- `OK`: the panel has data and no obvious degradation signal is active
- `DEGRADED`: the panel has data and current signals suggest a problem
- `DISABLED`: the feature is intentionally off, most commonly XDP
- `UNAVAILABLE`: the target is reachable but the metric family is missing
- `STALE`: scrape failed after a previous successful sample
- `DISCONNECTED`: the target cannot be reached yet
- `WARMING`: only the first successful sample exists, so short-window rates are not ready

## What Each Panel Shows

- `Traffic`: QPS, p99 latency, and response-code ratios
- `Cache`: hit ratio, positive or negative hit rate, miss rate, entry count, and lifecycle activity
- `Snapshot`: load or save success totals, imported entries, skipped entries, and duration
- `Upstream`: timeout rate, bad-rcode rate, fallback activity, and winner path
- `XDP`: active or disabled state, hit ratio, sync errors, cleanup activity, and entry count
- `State Machine`: most active stage and top failure reasons

## Local Self-Test

1. Start rec53 locally.

```bash
./rec53ctl run
```

2. In another terminal, open the TUI.

```bash
./rec53top
```

3. Generate traffic.

```bash
for i in {1..20}; do dig @127.0.0.1 -p 5353 example.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 github.com >/dev/null; done
for i in {1..10}; do dig @127.0.0.1 -p 5353 nosuchname1234.example. >/dev/null; done
```

4. Verify the first changes:

- the first successful scrape may show `WARMING`; after the next refresh, rate-based fields should become meaningful
- `Traffic` shows non-zero QPS
- `Cache` moves from warming into visible hit or miss rates
- `State Machine` shows active stages such as `cache_lookup`, `forward_lookup`, or `return_resp`
- `Upstream` shows winner-path activity when iterative queries actually touch upstream resolution
- `Upstream` reflects fallback or timeout activity when upstream issues exist
- `XDP` shows `DISABLED` on normal non-XDP deployments instead of pretending to be healthy

If you need deeper analysis than the TUI summary provides, continue with [Metrics](../metrics.md), [Observability Dashboard](observability-dashboard.md), and [Operator Checklist](operator-checklist.md).
