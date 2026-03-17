# Metrics

## Prometheus Metrics

Metrics are exposed at `http://<host>:9999/metric` by default.
The address can be changed via the `-metric` CLI flag or the `dns.metric` config field.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `rec53_query_counter` | Counter | `stage`, `name`, `type` | Incoming query count |
| `rec53_response_counter` | Counter | `stage`, `name`, `type`, `code` | Outgoing response count |
| `rec53_latency` | Histogram | `stage`, `name`, `type`, `code` | End-to-end query latency (ms) |
| `rec53_ipv2_p50_latency_ms` | Gauge | `ip` | Median nameserver RTT |
| `rec53_ipv2_p95_latency_ms` | Gauge | `ip` | 95th-percentile nameserver RTT |
| `rec53_ipv2_p99_latency_ms` | Gauge | `ip` | 99th-percentile nameserver RTT |

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
