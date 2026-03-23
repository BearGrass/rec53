## Context

rec53 already has the skeleton of the `v1.2.0` runtime contract in code: `monitor/runtime_state.go` defines the bounded phase enum, `monitor/metric.go` exposes `GET /healthz/ready`, and server startup/shutdown paths already toggle readiness around listener bind and graceful shutdown. The remaining problem is not inventing a new model, but closing the gaps between implementation, tests, operator-facing semantics, and roadmap language.

This is cross-cutting because it spans lifecycle code in `server/`, operational HTTP surfaces in `monitor/`, tests that lock the contract in place, and docs that tell operators what the probe means. The design should preserve the current lightweight approach: one readiness probe, one bounded phase enum, and no expansion into a full health taxonomy.

## Goals / Non-Goals

**Goals:**
- Make `readiness` and `phase` semantics explicit and stable for startup, warming, steady state, startup failure, and graceful shutdown.
- Ensure `/healthz/ready` status code and body reflect the same bounded contract.
- Cover edge cases that matter operationally, especially snapshot restore degradation and warmup completion transitions.
- Update roadmap and operator-facing docs so current implementation baseline and remaining work are clearly separated.

**Non-Goals:**
- Add `liveness`, `degraded`, or a broader health taxonomy.
- Introduce cluster-wide coordination, multi-node health, or central control-plane semantics.
- Rework warmup, snapshot, or listener architecture beyond what is needed to make lifecycle signaling correct.
- Expand probe output into a large JSON schema or log-derived diagnostics surface.

## Decisions

### Keep the current two-dimensional contract

The contract remains:
- `readiness`: boolean for normal traffic admission
- `phase`: bounded lifecycle enum

This matches the existing code and is already small enough for systemd, Kubernetes probes, shell scripts, and humans. The alternative was adding more states such as `degraded` or `restoring`, but those blur readiness with quality-of-service and would enlarge the contract before resource-protection work is ready.

### Treat warmup as ready-but-not-steady

After at least one UDP and one TCP listener bind successfully, rec53 is operationally ready even if warmup continues. That means `ready=true` and `phase=warming` is valid and intentional. This preserves the existing non-blocking warmup design and avoids turning optional startup optimization into an availability dependency.

Alternative considered: keep `ready=false` until warmup finishes. Rejected because it would delay traffic admission for a best-effort optimization and create unnecessary restart churn under probes.

### Keep snapshot restore in startup context, not as a separate phase

Snapshot restore behavior affects startup quality, but not the phase vocabulary. Missing or failed snapshot restore should not add a new phase; it should keep rec53 within `cold-start` before bind or `warming`/`steady` after bind depending on warmup progress. This keeps the external contract bounded while allowing docs and logs to explain cold-cache vs restored-cache startup.

Alternative considered: add a `restoring` phase. Rejected because it complicates the probe contract without materially changing operator action.

### Define shutdown semantics as readiness-first

On graceful shutdown, rec53 should flip to `ready=false` and `phase=shutting-down` before the rest of teardown completes. This gives external supervisors a clear stop-serving signal before listeners, warmup, XDP, and snapshot cleanup finish.

### Update roadmap to distinguish baseline from remaining work

The roadmap currently presents parts of `v1.2.0` as entirely pending, but the baseline implementation already exists. The roadmap should be rewritten to say what is already present, what still needs tightening, and what remains explicitly out of scope.

## Risks / Trade-offs

- [Docs diverge from code again] -> Add targeted tests around probe body/status and runtime transitions, then update roadmap/operator docs in the same change.
- [Temptation to widen scope into health taxonomy] -> Keep the change limited to readiness and bounded phase semantics only.
- [Edge cases remain underspecified] -> Write explicit requirements for startup failure, snapshot degradation, warmup completion, and shutdown transitions.
- [Operators over-interpret `phase`] -> Document that `phase` is a lifecycle hint, not a full diagnosis channel.
