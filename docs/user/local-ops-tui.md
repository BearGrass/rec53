# Local Ops TUI

`rec53top` is the local terminal dashboard for rec53. It reads the rec53 Prometheus endpoint directly and renders a fixed six-panel view for traffic, cache, snapshot, upstream, XDP, and state-machine health.

This page is the operational guide: how to launch it, which keys and states matter, how to self-test it locally, and how to read the detailed panels once you are already using the TUI.

For positioning, use cases, boundaries, and the stable overview entrypoint, start with [rec53top Overview](rec53top.md).

## Run

Recommended:

```bash
./rec53ctl top
```

Manual build:

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
- `-plain`: print periodic plain-text summaries instead of starting the full-screen TUI

If the terminal opens but does not render correctly, first retry with an explicit terminal type:

```bash
TERM=xterm-256color ./rec53top
```

If the terminal still does not support the full-screen UI, use the plain compatibility mode:

```bash
./rec53top -plain
```

`-plain` prints periodic plain-text summaries using the same dashboard model, but avoids the full-screen terminal UI dependency.

## Keys

- `q`: quit
- `r`: refresh immediately
- `h` or `?`: toggle help and status legend
- `1` to `6`: open detail view for Traffic, Cache, Snapshot, Upstream, XDP, or State Machine
- `0` or `Esc`: return to the overview dashboard

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

## Detail View

The full-screen TUI can expand one panel at a time into a detail page. This is still intentionally lightweight: it does not add drill-down navigation or historical charts, but the detail page is no longer just a longer copy of the overview card.

Each detail page now follows the same reading order:

- `status`: current panel state
- `What stands out now`: the current dominant signal, abnormal condition, or the reason the panel is not yet interpretable
- `Key metrics`: the main raw values behind that conclusion
- breakdown sections such as response mix, lookup mix, winner mix, or failure reasons when that panel has them
- `Next checks`: where to look next in rec53top or logs

Recommended use:

- stay in overview for first-check triage
- press `1` to `6` when one panel already looks suspicious and you want the current standout condition plus the most relevant breakdown or next-check hint
- press `0` or `Esc` to return to the overview

Non-normal states are also explained directly in detail view:

- `WARMING`: short-window rates are not stable yet because only one successful scrape exists
- `UNAVAILABLE`: required metric families for that panel are missing from the scrape
- `DISABLED`: the feature is intentionally off, most commonly XDP
- `DISCONNECTED`: rec53top has not reached a successful scrape yet
- `STALE`: the latest scrape failed, so the panel is showing older data with scrape troubleshooting hints instead of normal reading guidance

`-plain` stays overview-only by design.

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
- detail view (`1` to `6`) shows a `What stands out now` summary instead of only repeating the overview numbers
- degraded or unavailable panels show `Next checks` that point to the next likely panel or troubleshooting direction
- `State Machine` shows active stages such as `cache_lookup`, `forward_lookup`, or `return_resp`
- `Upstream` shows winner-path activity when iterative queries actually touch upstream resolution
- `Upstream` reflects fallback or timeout activity when upstream issues exist
- `XDP` shows `DISABLED` on normal non-XDP deployments instead of pretending to be healthy

If you need deeper analysis than the TUI summary provides, continue with [Metrics](../metrics.md), [Observability Dashboard](observability-dashboard.md), and [Operator Checklist](operator-checklist.md).
