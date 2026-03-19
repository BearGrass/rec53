# Metrics

Prometheus is the primary observability interface for rec53. Operators should use it to validate deployment health and runtime behavior. Developers should treat the metric set and labels as part of the public operational surface.

## Endpoint And Scrape Configuration

Metrics are exposed at `http://<host>:9999/metric` by default.
The address can be changed via the `-metric` CLI flag or the `dns.metric` config field.

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

## Operator Checklist

For first deployment and day-2 operations, start with these checks:

- the Prometheus target is `UP`
- `rec53_query_counter` increases under real traffic
- `rec53_response_counter{code="SERVFAIL"}` does not dominate responses
- `rec53_latency` histogram has data and acceptable tail latency
- `rec53_ipv2_p50_latency_ms` does not show persistent high RTT upstreams
- `rec53_xdp_*` metrics are only used when XDP is enabled

Useful PromQL:

```promql
# Query rate
rate(rec53_query_counter[1m])

# Error rate (SERVFAIL)
rate(rec53_response_counter{code="SERVFAIL"}[1m]) / rate(rec53_response_counter[1m])

# P99 end-to-end latency
histogram_quantile(0.99, rate(rec53_latency_bucket[5m]))

# Degraded nameservers (P50 > 500ms)
rec53_ipv2_p50_latency_ms > 500

# XDP cache hit ratio (when XDP is enabled)
rec53_xdp_cache_hits_total / (rec53_xdp_cache_hits_total + rec53_xdp_cache_misses_total)

# XDP error ratio
rec53_xdp_errors_total / (rec53_xdp_cache_hits_total + rec53_xdp_cache_misses_total + rec53_xdp_pass_total + rec53_xdp_errors_total)
```

## Metric Reference

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `rec53_query_counter` | Counter | `stage`, `type` | Incoming query count |
| `rec53_response_counter` | Counter | `stage`, `type`, `code` | Outgoing response count |
| `rec53_latency` | Histogram | `stage`, `type`, `code` | End-to-end query latency (ms) |
| `rec53_ipv2_p50_latency_ms` | Gauge | `ip` | Median nameserver RTT |
| `rec53_ipv2_p95_latency_ms` | Gauge | `ip` | 95th-percentile nameserver RTT |
| `rec53_ipv2_p99_latency_ms` | Gauge | `ip` | 99th-percentile nameserver RTT |
| `rec53_xdp_cache_hits_total` | Gauge | — | XDP BPF cache hits (sum across all CPUs) |
| `rec53_xdp_cache_misses_total` | Gauge | — | XDP BPF cache misses |
| `rec53_xdp_pass_total` | Gauge | — | Packets passed to Go stack (non-DNS, malformed, etc.) |
| `rec53_xdp_errors_total` | Gauge | — | XDP BPF processing errors |

> **XDP metrics note:** The `rec53_xdp_*` metrics are Gauges (not Counters) because Go
> reads absolute totals from BPF per-CPU arrays and sets the gauge each tick (5s interval).
> These metrics are only populated when XDP is enabled; otherwise they remain at 0.

## Label Stability And Cardinality

> **Breaking change (v0.5.0):** The `name` label (raw FQDN) was removed from
> `rec53_query_counter`, `rec53_response_counter`, and `rec53_latency` to
> eliminate unbounded cardinality and reduce per-query allocation overhead.
> If your PromQL queries or Grafana dashboards reference `name`, remove that
> label selector. Aggregation by domain is no longer available from these
> metrics; use DNS query logs if per-domain visibility is needed.

This matters for both operators and developers:

- operators should avoid building dashboards that assume per-domain labels exist
- developers should avoid adding unbounded labels on hot-path metrics
- any metric or label change needs release-note visibility because dashboards and alerts depend on it

## Developer Guidance

When changing metrics:

- prefer stable, bounded labels
- treat metrics as part of the operator contract
- update this document and `CHANGELOG.md` when label sets or semantics change
- verify whether the change affects PromQL, alerts, or Grafana dashboards

When adding observability for new behavior:

- keep the default path readable first
- avoid high-cardinality dimensions in per-query metrics
- document whether a metric is always present or feature-gated, such as XDP-only gauges
