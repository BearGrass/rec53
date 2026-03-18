## Context

rec53 is a lightweight endpoint-side recursive DNS resolver. Under sustained cache-hit
load (~100K QPS), pprof `alloc_space` profiling shows three hot-path allocation sources:

| Source | alloc_space % | Root cause |
|--------|--------------|------------|
| Metrics reporting (`InCounterAdd` / `OutCounterAdd` / `LatencyHistogramObserve`) | ~24% | `With(prometheus.Labels{...})` allocates a map per call; `name` label = unbounded cardinality |
| Cache copy (`getCacheCopy` / `dns.Msg.Copy`) | ~26-27% | Deep copy on every read (addressed conditionally, not in this change) |
| `getCacheKey` | included in cache path | `fmt.Sprintf("%s:%d", ...)` allocates on every lookup |

This change targets the metrics and cache-key paths. Cache COW is deferred to a follow-up
change, contingent on re-profiling showing >20% denoised `alloc_space` from `Copy` after
these optimizations land.

Current metrics schema uses raw FQDN as the `name` label on 3 metric families
(`rec53_query_counter`, `rec53_response_counter`, `rec53_latency`). Every unique domain
creates 9+ time series (counter + histogram buckets). Within this repository, no Grafana
dashboards, alerting rules, recording rules, or documented PromQL queries filter on
`name`; external deployments cannot be ruled out (see D1 compatibility caveat).

## Goals / Non-Goals

**Goals:**

- Eliminate per-call map allocation in all 6 metrics call sites (switch to `WithLabelValues`)
- Remove unbounded `name` label to cap Prometheus cardinality at O(stages × qtypes × codes)
  instead of O(stages × domains × qtypes × codes)
- Replace `fmt.Sprintf` in `getCacheKey` with lower-overhead string concatenation
- Validate with **dual-metric acceptance gate**: (a) dnsperf QPS and P99 must not regress,
  AND (b) pprof denoised `alloc_space` for metrics path must show measurable reduction.
  Neither metric alone is sufficient — alloc reduction without throughput improvement, or
  throughput improvement without alloc reduction, both indicate a measurement artefact.
- Maintain full backward compatibility for all non-metrics interfaces

**Non-Goals:**

- Cache COW / zero-copy read path (deferred, conditional on re-profile results)
- `sync.Pool` for `dns.Msg` (rejected in v0.4.0 — lifecycle complexity too high)
- Changing `IPQualityV2` gauge labels (already bounded by IP count, not domain count)
- Adding new metrics dimensions or aggregation layers
- Prometheus `_v2` metric transition period (no in-repo consumers; direct removal with release-note callout)

## Decisions

### D1: Direct removal of `name` label (not `_v2` transition)

**Choice**: Remove `name` from `InCounter`, `OutCounter`, `LatencyHistogramObserver`
label vectors in a single change. No `_v2` parallel metrics.

**Alternatives considered**:
- *`_v2` transition*: Register new metrics without `name`, deprecate old ones, remove after
  N releases. Rejected: no in-repo consumers found (no dashboards, no alerts, no
  recording rules). The `rec53_ip_quality` V1→V2 migration set precedent for direct delete.
- *Replace with bucketed label* (e.g., TLD or second-level domain): Adds implementation
  complexity for a label nobody queries. Rejected.

**Rationale**: Simplest path. Prior art in codebase (`rec53_ip_quality` V1→V2 direct delete).

**Compatibility caveat**: "Zero consumers" is verified only within this repository (no
dashboards, alerts, or recording rules). We cannot rule out external Prometheus/Grafana
setups that query on `name`. Mitigation: treat this as an explicit one-time breaking
change with prominent release-note callout (version header + migration note in README,
README.zh, and CHANGELOG). A runtime compat switch was considered but rejected as
over-engineering for rec53's endpoint-side, small-user-base positioning.

### D2: `WithLabelValues(...)` over `With(prometheus.Labels{...})`

**Choice**: Mechanical replacement of all `With()` calls to `WithLabelValues()` with
positional string arguments matching the label order in `var.go`.

**Rationale**: `WithLabelValues` skips map allocation entirely — it does a direct slice
lookup on the label index. This is the standard Prometheus Go client optimization for
hot paths. The label order is defined once in `var.go` and used consistently.

### D3: `getCacheKey` string concatenation

**Choice**: Replace `fmt.Sprintf("%s:%d", name, qtype)` with
`name + ":" + strconv.FormatUint(uint64(qtype), 10)`.

**Alternatives considered**:
- *`strings.Builder`*: Overhead of builder allocation negates benefit for short strings.
- *Pre-computed key in request context*: Over-engineering for a 2-line function.

**Rationale**: Go compiler optimizes `+` concatenation of small strings into a single
allocation. `strconv.FormatUint` avoids the reflection overhead of `fmt`.

### D4: Signature changes propagated top-down

**Choice**: Remove `name string` parameter from `InCounterAdd`, `OutCounterAdd`,
`LatencyHistogramObserve`. Update all call sites in `server/` package. Update all test
files in `monitor/`.

**Rationale**: Clean API — callers no longer need to extract `r.Question[0].Name` for
metrics. Fewer parameters = less cognitive load and fewer opportunities for misuse.

## Risks / Trade-offs

**[Risk] Breaking Prometheus schema** → Mitigation: "Zero consumers" verified within repo
only; cannot rule out external setups. Treat as explicit one-time break: prominent
release-note callout in README, README.zh, and CHANGELOG with migration guidance
("remove `name` from your PromQL queries"). No runtime compat switch (over-engineering
for this project's scale).

**[Risk] Label order mismatch in `WithLabelValues`** → Mitigation: Label order is defined
exactly once in `var.go` (`[]string{"stage", "type", "code"}` after removal of `name`).
Each call site is mechanically mapped. Existing tests with hardcoded label assertions will
catch any mismatch immediately.

**[Risk] `getCacheKey` format change breaks existing cache entries** → Mitigation: The
output format is identical (`"domain.:1"` etc.) — only the implementation changes from
`fmt.Sprintf` to string concatenation. A unit test will verify format equivalence.

**[Risk] Re-profile shows cache COW is still dominant** → Mitigation: This is expected
and acceptable. Cache COW is explicitly deferred as a conditional follow-up, gated on
>20% denoised `alloc_space` from `Copy` in the new baseline.

## Open Questions

_(none — all decisions were agreed in the v0.5.0 exploration session)_
