## ADDED Requirements

### Requirement: Metrics functions use WithLabelValues

All Prometheus metrics helper functions (`InCounterAdd`, `OutCounterAdd`,
`LatencyHistogramObserve`) SHALL use `WithLabelValues(...)` instead of
`With(prometheus.Labels{...})` to avoid per-call map allocation.

#### Scenario: InCounterAdd does not allocate a map
- **WHEN** `InCounterAdd` is called with stage and qtype arguments
- **THEN** the implementation calls `InCounter.WithLabelValues(stage, qtype)` directly without constructing a `prometheus.Labels` map

#### Scenario: OutCounterAdd does not allocate a map
- **WHEN** `OutCounterAdd` is called with stage, qtype, and code arguments
- **THEN** the implementation calls `OutCounter.WithLabelValues(stage, qtype, code)` directly without constructing a `prometheus.Labels` map

#### Scenario: LatencyHistogramObserve does not allocate a map
- **WHEN** `LatencyHistogramObserve` is called with stage, qtype, code, and latency arguments
- **THEN** the implementation calls `LatencyHistogramObserver.WithLabelValues(stage, qtype, code).Observe(latency)` directly without constructing a `prometheus.Labels` map

### Requirement: Remove name label from query and response metrics

The `name` (raw FQDN) label SHALL be removed from the following Prometheus metric
families: `rec53_query_counter`, `rec53_response_counter`, `rec53_latency`. This
eliminates unbounded cardinality growth.

#### Scenario: InCounter label vector excludes name
- **WHEN** `InCounter` is defined in `monitor/var.go`
- **THEN** its label names SHALL be `["stage", "type"]` with no `name` label

#### Scenario: OutCounter label vector excludes name
- **WHEN** `OutCounter` is defined in `monitor/var.go`
- **THEN** its label names SHALL be `["stage", "type", "code"]` with no `name` label

#### Scenario: LatencyHistogramObserver label vector excludes name
- **WHEN** `LatencyHistogramObserver` is defined in `monitor/var.go`
- **THEN** its label names SHALL be `["stage", "type", "code"]` with no `name` label

### Requirement: Metrics function signatures drop name parameter

The `name string` parameter SHALL be removed from `InCounterAdd`, `OutCounterAdd`,
and `LatencyHistogramObserve`. Callers SHALL NOT pass domain names to metrics functions.

#### Scenario: InCounterAdd signature
- **WHEN** `InCounterAdd` is called
- **THEN** it accepts exactly two string parameters: `stage` and `qtype`

#### Scenario: OutCounterAdd signature
- **WHEN** `OutCounterAdd` is called
- **THEN** it accepts exactly three string parameters: `stage`, `qtype`, and `code`

#### Scenario: LatencyHistogramObserve signature
- **WHEN** `LatencyHistogramObserve` is called
- **THEN** it accepts three string parameters (`stage`, `qtype`, `code`) and one `float64` (`latency`)

#### Scenario: Server call sites updated
- **WHEN** any call to `InCounterAdd`, `OutCounterAdd`, or `LatencyHistogramObserve` exists in `server/`
- **THEN** the call SHALL NOT pass a domain name argument

### Requirement: IPQualityV2 gauges unchanged

The `IPQualityV2_P50`, `IPQualityV2_P95`, `IPQualityV2_P99` gauge metrics and the
`IPQualityV2GaugeSet` function SHALL NOT be modified. Their `ip` label is bounded
by the number of upstream nameserver IPs, not by domain count.

#### Scenario: IPQualityV2GaugeSet signature preserved
- **WHEN** `IPQualityV2GaugeSet` is called
- **THEN** its signature remains `(ip string, p50, p95, p99 float64)` unchanged

### Requirement: Test assertions updated for removed label

All test assertions in `monitor/metric_test.go` and `monitor/metric_bench_test.go`
that reference the `name` label SHALL be updated to reflect the new label schema.

#### Scenario: metric_test.go compiles and passes
- **WHEN** `go test ./monitor/...` is run
- **THEN** all tests pass with no references to a `name` label in counter/histogram assertions

#### Scenario: metric_bench_test.go compiles and passes
- **WHEN** `go test -bench . ./monitor/...` is run
- **THEN** all benchmarks compile and run without passing a `name` argument

### Requirement: Documentation reflects breaking change

`docs/metrics.md`, `README.md`, and `README.zh.md` SHALL document the removal of the
`name` label as a breaking change.

#### Scenario: metrics.md updated
- **WHEN** a user reads `docs/metrics.md`
- **THEN** no PromQL example references a `name` label on `rec53_query_counter`, `rec53_response_counter`, or `rec53_latency`

#### Scenario: README notes breaking change
- **WHEN** a user reads the metrics section of `README.md` or `README.zh.md`
- **THEN** the removal of `name` label is documented as a breaking schema change
