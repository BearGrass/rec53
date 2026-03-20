## 1. Metric Contract And Scaffolding

- [x] 1.1 Audit existing metric families in `monitor/var.go` and `monitor/metric.go`, then define the new low-cardinality metric set for cache, snapshot, upstream, XDP, and state machine
- [x] 1.2 Add or extend metric helpers in `monitor/` so business code records bounded `result` / `reason` / `stage` / `path` values without ad hoc label construction
- [x] 1.3 Add or update `monitor/...` tests to lock metric registration, label shape, and bounded-cardinality assumptions

## 2. Batch 1 Runtime Metrics

- [x] 2.1 Instrument cache lookup and cache lifecycle paths with positive hit, negative hit, miss, and entry-lifecycle signals
- [x] 2.2 Instrument snapshot save and load paths with attempt, outcome, restored count, skipped count by reason, and duration signals
- [x] 2.3 Instrument upstream query paths with timeout, bad-rcode, fallback, and Happy Eyeballs winner signals
- [x] 2.4 Add focused server tests that verify Batch 1 metrics change on representative success and failure paths

## 3. Batch 2 Runtime Metrics

- [x] 3.1 Instrument XDP sync and cleanup paths with sync-error, cleanup-deleted, and occupancy or entry-count signals
- [x] 3.2 Instrument state-machine transitions and terminal failures with bounded stage and reason aggregation
- [x] 3.3 Add focused tests for XDP and state-machine observability paths without changing existing resolver semantics

## 4. Documentation And Operator Outputs

- [x] 4.1 Update `docs/metrics.md` with the full metric catalog, label/cardinality rules, and PromQL examples grouped for developers vs operators
- [x] 4.2 Add and maintain `docs/metrics.zh.md` as a synchronized Chinese version of the metrics catalog and PromQL guidance
- [x] 4.3 Update `docs/user/operations.md` and `docs/user/troubleshooting.md` with the first-check signals for cache, snapshot, upstream, and XDP degradation
- [x] 4.4 Create the baseline dashboard layout or documented dashboard panel set covering request basics, cache, snapshot, upstream, XDP, and state-machine health
- [x] 4.5 Write an operator checklist that maps common degraded states to the first metrics and logs to inspect

## 5. Verification And Release Readiness

- [x] 5.1 Run targeted unit tests for `monitor/` and touched `server/` packages, then fix any metric or label regressions
- [x] 5.2 Run `gofmt -w .`, `go vet ./...`, and the most relevant `go test` commands for observability changes
- [x] 5.3 Review the final metric surface against the spec to confirm no raw domain or other unbounded labels were introduced
