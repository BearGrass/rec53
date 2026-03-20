# Operator Checklist

Use this checklist when rec53 looks degraded. Start with the symptom you can already observe, then inspect the first metrics before diving into logs or code.

## High SERVFAIL Ratio

Check first:

- `rate(rec53_response_counter{code="SERVFAIL"}[5m]) / rate(rec53_response_counter[5m])`
- `sum by (reason) (rate(rec53_upstream_failures_total[5m]))`
- `sum by (reason) (increase(rec53_state_machine_failures_total[10m]))`

Then inspect:

- `journalctl -u rec53 -n 100 --no-pager`
- `/var/log/rec53/rec53.log`

## Latency Regression

Check first:

- `histogram_quantile(0.99, sum by (le) (rate(rec53_latency_bucket[5m])))`
- `sum by (result) (rate(rec53_cache_lookup_total[5m]))`
- `sum by (path) (rate(rec53_upstream_winner_total[5m]))`
- `rec53_ipv2_p50_latency_ms`

Then inspect:

- whether cache misses rose
- whether upstream timeouts or fallback activity also rose

## Cache Effectiveness Drop

Check first:

- `sum by (result) (rate(rec53_cache_lookup_total[5m]))`
- `rec53_cache_entries`
- `sum by (event) (rate(rec53_cache_lifecycle_total[5m]))`

Then inspect:

- whether cache entries are unexpectedly low
- whether cleanup or flush activity spiked
- whether upstream failures are forcing more cold-path work

## Snapshot Restore Looks Wrong

Check first:

- `increase(rec53_snapshot_operations_total{operation="load"}[1h])`
- `increase(rec53_snapshot_entries_total{operation="load"}[1h])`
- `histogram_quantile(0.99, sum by (le) (rate(rec53_snapshot_duration_ms_bucket{operation="load"}[1h])))`

Then inspect:

- snapshot path permissions
- shutdown behavior from the previous process
- snapshot load logs for corrupt entries or missing files

## Upstream Timeouts Or Unstable Responses

Check first:

- `sum by (reason) (rate(rec53_upstream_failures_total[5m]))`
- `sum by (rcode) (rate(rec53_upstream_failures_total{reason="bad_rcode"}[5m]))`
- `sum by (result) (rate(rec53_upstream_fallback_total[5m]))`
- `rec53_ipv2_p50_latency_ms`

Then inspect:

- network reachability to upstreams or root servers
- whether one or more upstream IPs are consistently degraded
- whether fallback is succeeding or also failing

## XDP Looks Broken

Check first:

- `rec53_xdp_status`
- `rec53_xdp_cache_hits_total`
- `rec53_xdp_cache_misses_total`
- `sum by (reason) (rate(rec53_xdp_cache_sync_errors_total[5m]))`
- `rec53_xdp_entries`

Then inspect:

- logs for attach or degrade-to-Go-path messages
- interface name and privilege configuration
- whether the Go cache path still looks healthy

## Still Unclear

If no single domain explains the issue:

- inspect `topk(10, increase(rec53_state_machine_stage_total[10m]))`
- inspect `sum by (reason) (increase(rec53_state_machine_failures_total[10m]))`
- compare the current dashboard with a known-good deployment window
