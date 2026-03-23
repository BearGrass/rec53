## 1. Runtime Contract Tightening

- [x] 1.1 Audit current `readiness / phase` transitions in `server/` and `monitor/` against the new spec, then close any semantic gaps in startup, warmup completion, startup failure, and shutdown handling.
- [x] 1.2 Ensure `/healthz/ready` exposes the finalized bounded contract consistently in both HTTP status and response body.
- [x] 1.3 Add or refine targeted tests for cold-start, warming, steady, shutdown, and startup-failure semantics.

## 2. Restore And Lifecycle Edge Cases

- [x] 2.1 Verify snapshot-missing and snapshot-restore-failure behavior remains within the bounded startup contract, and adjust lifecycle handling if needed.
- [x] 2.2 Add or update tests covering restore-path and warmup-completion transitions without widening the health model.

## 3. Docs And Roadmap Alignment

- [x] 3.1 Update operator-facing docs for `readiness / phase`, including probe usage and lifecycle meaning.
- [x] 3.2 Rewrite the `v1.2.0` roadmap section to distinguish implemented baseline behavior from remaining readiness-contract work.
- [x] 3.3 Run formatting and the smallest relevant test set, then widen verification to affected packages.
