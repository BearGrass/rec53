# Observability Dashboard

This document defines the baseline rec53 dashboard layout. It is intended for operators first, but developers can use the same layout as a stable starting point when comparing behavior across releases.

## Panel Order

Use this order from top to bottom:

1. traffic and response quality
2. cache effectiveness
3. snapshot startup quality
4. upstream health
5. XDP health
6. state-machine concentration

## Recommended Panels

### 1. Traffic And Response Quality

- Query rate: `rate(rec53_query_counter[1m])`
- Response code mix: `sum by (code) (rate(rec53_response_counter[5m]))`
- P99 latency: `histogram_quantile(0.99, sum by (le) (rate(rec53_latency_bucket[5m])))`

### 2. Cache Effectiveness

- Cache lookup outcomes: `sum by (result) (rate(rec53_cache_lookup_total[5m]))`
- Cache miss ratio: `rate(rec53_cache_lookup_total{result="miss"}[5m]) / rate(rec53_cache_lookup_total[5m])`
- Current cache entries: `rec53_cache_entries`
- Cache lifecycle activity: `sum by (event) (rate(rec53_cache_lifecycle_total[5m]))`

### 3. Snapshot Startup Quality

- Snapshot operation results: `sum by (operation, result) (increase(rec53_snapshot_operations_total[1h]))`
- Snapshot entry outcomes on load: `sum by (result) (increase(rec53_snapshot_entries_total{operation="load"}[1h]))`
- Snapshot duration: `histogram_quantile(0.99, sum by (le, operation, result) (rate(rec53_snapshot_duration_ms_bucket[1h])))`

### 4. Upstream Health

- Upstream failure reasons: `sum by (reason) (rate(rec53_upstream_failures_total[5m]))`
- Bad upstream rcodes: `sum by (rcode) (rate(rec53_upstream_failures_total{reason="bad_rcode"}[5m]))`
- Fallback outcomes: `sum by (result) (rate(rec53_upstream_fallback_total[5m]))`
- Happy Eyeballs winner path: `sum by (path) (rate(rec53_upstream_winner_total[5m]))`
- Degraded upstream IPs: `rec53_ipv2_p50_latency_ms > 500`

### 5. XDP Health

- XDP status: `rec53_xdp_status`
- XDP hit ratio: `rec53_xdp_cache_hits_total / (rec53_xdp_cache_hits_total + rec53_xdp_cache_misses_total)`
- XDP sync errors: `sum by (reason) (rate(rec53_xdp_cache_sync_errors_total[5m]))`
- XDP cleanup deleted: `rate(rec53_xdp_cleanup_deleted_total[5m])`
- XDP entries: `rec53_xdp_entries`

### 6. State-Machine Concentration

- Top stages entered: `topk(10, increase(rec53_state_machine_stage_total[10m]))`
- Terminal failures by reason: `sum by (reason) (increase(rec53_state_machine_failures_total[10m]))`

## Reading Order During Incidents

- If response quality is bad, start at traffic and response panels.
- If latency rises with low cache effectiveness, move to cache and upstream panels.
- If restart quality is bad, inspect snapshot panels before upstream panels.
- If XDP is enabled but hit ratio is low or sync errors rise, treat XDP as degraded and compare with the Go-path cache panels.
- If none of the above explains the issue, inspect state-machine failure concentration to find the dominant failing phase.
