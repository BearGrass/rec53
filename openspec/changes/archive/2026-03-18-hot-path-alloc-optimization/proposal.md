## Why

pprof profiling under sustained load (~100K QPS, `tools/dnsperf -c 128`) reveals that
the cache-hit serving path spends ~24% of cumulative `alloc_space` on metrics reporting
and ~26-27% on cache copy operations. The metrics path has two compounding problems:
the `name` label (raw FQDN) creates **unbounded Prometheus cardinality** (every unique
domain spawns 9+ time series), and every `With(prometheus.Labels{...})` call allocates
a map. The `getCacheKey` function uses `fmt.Sprintf` on every cache lookup, adding
unnecessary allocation overhead on the hottest code path. Reducing these allocations
improves throughput headroom and lowers GC pressure without architectural risk.

## What Changes

- **BREAKING**: Remove `name` (raw FQDN) label from `rec53_query_counter`,
  `rec53_response_counter`, and `rec53_latency` metrics. This is a schema-breaking
  change — existing dashboards or alerts filtering on `name` will break. Within this
  repository, no Grafana dashboards, alerting rules, recording rules, or documented
  PromQL queries filter on `name`; however, external deployments cannot be ruled out.
- Replace `With(prometheus.Labels{...})` with `WithLabelValues(...)` in all metrics
  call sites, eliminating per-call map allocation.
- Replace `fmt.Sprintf("%s:%d", ...)` in `getCacheKey` with string concatenation +
  `strconv.FormatUint`, removing the `fmt` overhead on every cache lookup.
- Update metric function signatures (`InCounterAdd`, `OutCounterAdd`,
  `LatencyHistogramObserve`) to remove the `name` parameter.
- Update all call sites in `server/` to stop passing domain name to metrics functions.
- Re-profile with `tools/dnsperf` + pprof to validate allocation reduction and record
  before/after comparison in `docs/benchmarks.md`.

## Capabilities

### New Capabilities

- `metrics-label-optimization`: Removal of unbounded `name` label from query/response/latency
  metrics and mechanical replacement of `With()` → `WithLabelValues()` to eliminate per-call
  map allocations.
- `cache-key-optimization`: Replace `fmt.Sprintf` in `getCacheKey` with lower-overhead string
  concatenation.

### Modified Capabilities

_(none — no existing spec-level behavioral requirements are changing)_

## Impact

- **`monitor/var.go`**: Label vectors for `InCounter`, `OutCounter`, `LatencyHistogramObserver`
  lose the `name` dimension.
- **`monitor/metric.go`**: `InCounterAdd`, `OutCounterAdd`, `LatencyHistogramObserve` signatures
  drop `name` parameter; internals switch to `WithLabelValues`.
- **`monitor/metric_test.go`**: ~12 assertions that reference `name` label must be updated.
- **`monitor/metric_bench_test.go`**: 3 call sites that pass `name` must be updated.
- **`server/server.go`** (lines 98, 123, 124): Call sites passing `r.Question[0].Name` updated.
- **`server/state_query_upstream.go`** (lines 432, 455): Forwarding metrics call sites updated.
- **`server/cache.go`** (line 20-22): `getCacheKey` implementation replaced.
- **`docs/metrics.md`**: PromQL examples updated to reflect removed `name` label.
- **`docs/benchmarks.md`**: Before/after pprof comparison added.
- **`README.md` / `README.zh.md`**: Metrics section updated for breaking change.
