# Metrics

English | [中文](metrics.zh.md)

Prometheus is the primary observability interface for rec53. This document is the metric contract for both contributors and operators: metric names, label sets, and semantics are treated as part of the public operational surface.

The synchronized Chinese version lives at [docs/metrics.zh.md](./metrics.zh.md).

## Endpoint And Scrape Configuration

Metrics are exposed at `http://<host>:9999/metric` by default. The address can be changed via the `-metric` CLI flag or the `dns.metric` config field.

Example scrape configuration:

```yaml
scrape_configs:
  - job_name: "rec53"
    metrics_path: /metric
    scrape_interval: 5s
    static_configs:
      - targets:
          - "127.0.0.1:9999"
```

The repository sample at `etc/prometheus.yml` uses the same pattern.

## Cardinality Rules

All new observability metrics in `v1.1.1` follow these rules:

- use only bounded labels such as `stage`, `type`, `code`, `result`, `reason`, or `path`
- do not add raw domain names, request IDs, full upstream lists, or free-form error strings as labels
- treat per-IP labels as exceptional and limited to bounded upstream sets such as `rec53_ipv2_*`

This matters for both audiences:

- developers should treat labels as a compatibility contract, not temporary debug output
- operators should assume dashboards and alerts remain safe to aggregate over time

## Operator Health Checks

For first deployment and day-2 operations, start here:

- `rec53_query_counter` is increasing under real traffic
- `rec53_response_counter{code="SERVFAIL"}` is not dominating responses
- `rec53_cache_lookup_total` still shows healthy hit categories instead of only misses
- `rec53_snapshot_operations_total` does not show repeated load or save failures
- `rec53_upstream_failures_total` is not dominated by `timeout` or `bad_rcode`
- `rec53_xdp_status` and `rec53_xdp_*` are only interpreted when XDP is enabled

Use [Observability Dashboard](./user/observability-dashboard.md) for the recommended panel layout and [Operator Checklist](./user/operator-checklist.md) for incident-first lookup order.

## Developer Diagnostic Entry Points

Use these views when a code change, performance result, or behavior regression needs explanation:

- request and response totals to confirm load shape
- cache lookup outcome mix to explain latency or upstream pressure changes
- snapshot load and save outcomes to explain restart quality differences
- upstream failure reasons and winner paths to explain timeout or tail-latency changes
- state-machine stage and failure counters to see which resolution phase changed
- XDP sync and cleanup metrics to explain Go path vs fast-path divergence

## PromQL Examples

### Operator Examples

```promql
# Query rate
rate(rec53_query_counter[1m])

# SERVFAIL ratio
rate(rec53_response_counter{code="SERVFAIL"}[5m]) / rate(rec53_response_counter[5m])

# P99 end-to-end latency
histogram_quantile(0.99, sum by (le) (rate(rec53_latency_bucket[5m])))

# Cache miss share
rate(rec53_cache_lookup_total{result="miss"}[5m]) / rate(rec53_cache_lookup_total[5m])

# Snapshot failures in the last 15m
increase(rec53_snapshot_operations_total{result="failure"}[15m])

# Upstream timeout rate
rate(rec53_upstream_failures_total{reason="timeout"}[5m])

# XDP cache hit ratio when XDP is enabled
rec53_xdp_cache_hits_total / (rec53_xdp_cache_hits_total + rec53_xdp_cache_misses_total)
```

### Developer Examples

```promql
# Compare positive vs negative cache hits
sum by (result) (rate(rec53_cache_lookup_total[5m]))

# Snapshot skipped-entry reasons after restart
increase(rec53_snapshot_entries_total{operation="load"}[30m])

# Bad upstream rcodes by code
sum by (rcode) (rate(rec53_upstream_failures_total{reason="bad_rcode"}[5m]))

# Happy Eyeballs winner path distribution
sum by (path) (rate(rec53_upstream_winner_total[5m]))

# State-machine stages most frequently entered
topk(10, increase(rec53_state_machine_stage_total[10m]))

# State-machine transitions by real path edge
sum by (from, to) (increase(rec53_state_machine_transition_total[10m]))

# Terminal state-machine failures by reason
sum by (reason) (increase(rec53_state_machine_failures_total[10m]))
```

## Metric Reference

### Core Request Metrics

| Metric | Type | Labels | Audience | Description |
|--------|------|--------|----------|-------------|
| `rec53_query_counter` | Counter | `stage`, `type` | Both | Incoming query count |
| `rec53_response_counter` | Counter | `stage`, `type`, `code` | Both | Outgoing response count |
| `rec53_latency` | Histogram | `stage`, `type`, `code` | Both | End-to-end query latency in milliseconds |

### Cache Metrics

| Metric | Type | Labels | Audience | Description |
|--------|------|--------|----------|-------------|
| `rec53_cache_lookup_total` | Counter | `result` | Both | Cache lookup outcomes such as `positive_hit`, `negative_hit`, `delegation_hit`, and `miss` |
| `rec53_cache_entries` | Gauge | — | Operator | Current number of Go cache entries |
| `rec53_cache_lifecycle_total` | Counter | `event` | Developer | Cache lifecycle activity such as `write`, `delete_expired`, and `flush` |

### Snapshot Metrics

| Metric | Type | Labels | Audience | Description |
|--------|------|--------|----------|-------------|
| `rec53_snapshot_operations_total` | Counter | `operation`, `result` | Both | Snapshot save/load attempts by bounded result such as `success`, `failure`, or `not_found` |
| `rec53_snapshot_entries_total` | Counter | `operation`, `result` | Both | Snapshot entry counts such as `saved`, `imported`, `skipped_expired`, `skipped_corrupt`, `skipped_non_dns`, `skipped_pack_error` |
| `rec53_snapshot_duration_ms` | Histogram | `operation`, `result` | Developer | Snapshot save/load duration in milliseconds |

### Upstream Metrics

| Metric | Type | Labels | Audience | Description |
|--------|------|--------|----------|-------------|
| `rec53_upstream_failures_total` | Counter | `reason`, `rcode` | Both | Upstream failures classified by bounded reason such as `timeout`, `transport_error`, `context_canceled`, or `bad_rcode` |
| `rec53_upstream_fallback_total` | Counter | `result` | Both | Alternate-upstream fallback outcomes such as `success`, `failure`, or `unavailable` |
| `rec53_upstream_winner_total` | Counter | `path` | Developer | Winning path in upstream selection such as `single`, `primary`, or `secondary` |
| `rec53_ipv2_p50_latency_ms` | Gauge | `ip` | Both | Median nameserver RTT |
| `rec53_ipv2_p95_latency_ms` | Gauge | `ip` | Developer | 95th-percentile nameserver RTT |
| `rec53_ipv2_p99_latency_ms` | Gauge | `ip` | Developer | 99th-percentile nameserver RTT |

### XDP Metrics

| Metric | Type | Labels | Audience | Description |
|--------|------|--------|----------|-------------|
| `rec53_xdp_status` | Gauge | — | Operator | XDP fast-path status, `0` disabled or unavailable and `1` active |
| `rec53_xdp_cache_hits_total` | Gauge | — | Operator | XDP BPF cache hits, exported as absolute totals from per-CPU counters |
| `rec53_xdp_cache_misses_total` | Gauge | — | Operator | XDP BPF cache misses |
| `rec53_xdp_pass_total` | Gauge | — | Developer | Packets passed to the Go stack |
| `rec53_xdp_errors_total` | Gauge | — | Developer | XDP BPF processing errors |
| `rec53_xdp_cache_sync_errors_total` | Counter | `reason` | Both | Go-to-BPF cache sync failures by bounded reason such as `key_build`, `value_build`, or `update` |
| `rec53_xdp_cleanup_deleted_total` | Counter | — | Operator | Total expired XDP entries removed by cleanup |
| `rec53_xdp_entries` | Gauge | — | Operator | Current active XDP cache entry count after cleanup reconciliation |

> `rec53_xdp_cache_hits_total`, `rec53_xdp_cache_misses_total`, `rec53_xdp_pass_total`, and `rec53_xdp_errors_total` are Gauges because Go periodically reads absolute totals from BPF per-CPU arrays and sets the gauge on each collection interval.

### State Machine Metrics

| Metric | Type | Labels | Audience | Description |
|--------|------|--------|----------|-------------|
| `rec53_state_machine_stage_total` | Counter | `stage` | Developer | State-machine stage transitions by bounded stage name |
| `rec53_state_machine_transition_total` | Counter | `from`, `to` | Developer | Real state-machine edges by bounded source and destination, including terminal exits such as `success_exit`, `servfail_exit`, `formerr_exit`, `error_exit`, and `max_iterations_exit` |
| `rec53_state_machine_failures_total` | Counter | `reason` | Developer | Terminal state-machine failure categories such as `query_upstream_error`, `cname_cycle`, or `max_iterations` |

`rec53_state_machine_transition_total` is the path-oriented companion to `rec53_state_machine_stage_total`:

- `stage_total` answers which states were entered most often
- `transition_total` answers which real `from -> to` edges requests actually took
- terminal exits are modeled as bounded synthetic `to` nodes so paths do not appear truncated

These metrics are still aggregate-scoped. For one real request-scoped path, use `./rec53 --config ./config.yaml --trace-domain example.com --trace-type A`.

Canonical state labels currently include:

- `state_init`
- `hosts_lookup`
- `forward_lookup`
- `cache_lookup`
- `classify_resp`
- `extract_glue`
- `lookup_ns_cache`
- `query_upstream`
- `return_resp`

Canonical terminal exits currently include:

- `success_exit`
- `formerr_exit`
- `servfail_exit`
- `error_exit`
- `max_iterations_exit`

## Label Stability And Compatibility Notes

> **Breaking change (v0.5.0):** The `name` label (raw FQDN) was removed from `rec53_query_counter`, `rec53_response_counter`, and `rec53_latency` to eliminate unbounded cardinality and reduce per-query allocation overhead.

If your PromQL, dashboards, or alerts still reference `name`, remove that selector. Per-domain aggregation is not provided by these metrics; use DNS query logs if that level of detail is required.

## Guidance For Contributors

When changing metrics:

- keep labels bounded and reusable
- prefer counters or histograms for trend analysis and gauges only for current-state surfaces
- update this document and `docs/metrics.zh.md` together
- check whether dashboard panels, PromQL examples, and operator docs also need updates
