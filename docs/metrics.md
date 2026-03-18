# Metrics

## Prometheus Metrics

Metrics are exposed at `http://<host>:9999/metric` by default.
The address can be changed via the `-metric` CLI flag or the `dns.metric` config field.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `rec53_query_counter` | Counter | `stage`, `type` | Incoming query count |
| `rec53_response_counter` | Counter | `stage`, `type`, `code` | Outgoing response count |
| `rec53_latency` | Histogram | `stage`, `type`, `code` | End-to-end query latency (ms) |
| `rec53_ipv2_p50_latency_ms` | Gauge | `ip` | Median nameserver RTT |
| `rec53_ipv2_p95_latency_ms` | Gauge | `ip` | 95th-percentile nameserver RTT |
| `rec53_ipv2_p99_latency_ms` | Gauge | `ip` | 99th-percentile nameserver RTT |

> **Breaking change (v0.5.0):** The `name` label (raw FQDN) was removed from
> `rec53_query_counter`, `rec53_response_counter`, and `rec53_latency` to
> eliminate unbounded cardinality and reduce per-query allocation overhead.
> If your PromQL queries or Grafana dashboards reference `name`, remove that
> label selector. Aggregation by domain is no longer available from these
> metrics; use DNS query logs if per-domain visibility is needed.

## Useful Queries

```promql
# Query rate
rate(rec53_query_counter[1m])

# Error rate (SERVFAIL)
rate(rec53_response_counter{code="SERVFAIL"}[1m]) / rate(rec53_response_counter[1m])

# P99 end-to-end latency
histogram_quantile(0.99, rate(rec53_latency_bucket[5m]))

# Degraded nameservers (P50 > 500ms)
rec53_ipv2_p50_latency_ms > 500
```
