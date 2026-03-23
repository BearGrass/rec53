## Context

rec53 already has most of the lifecycle mechanics that an operator expects from a node-local resolver:

- DNS listeners only report ready after a real bind succeeds
- warmup runs asynchronously and does not block the service path
- snapshot restore runs before listeners start and can degrade to cold-cache behavior
- graceful shutdown cancels background work before snapshot save
- XDP can degrade to the Go cache path when attach fails

Those behaviors are implemented today across `cmd/rec53.go`, `server/server.go`, and `monitor/`, but they are not surfaced as one consistent runtime contract. External systems currently have to infer readiness from logs, scrape success, or indirect symptoms.

The change is cross-cutting because it spans startup/shutdown sequencing, health exposure, metrics, tests, and operator docs. It also needs to be careful about product semantics: rec53 is a single-node resolver, not a distributed control plane, so the design should expose a small, explicit contract rather than an elaborate orchestration API.

## Goals / Non-Goals

**Goals:**

- Define one minimal runtime state model for `readiness` and bounded lifecycle `phase`.
- Make that model machine-consumable through a simple HTTP readiness probe.
- Keep `warmup` and `cold-start` distinguishable from true service failure.
- Ensure shutdown flips readiness early enough that external systems stop sending new traffic before listeners are torn down.
- Keep the first version small enough to implement and test without redesigning the resolver.

**Non-Goals:**

- Do not implement resource protection, rate limiting, or queue management in this change.
- Do not add a separate liveness contract in this version.
- Do not add degraded-state logic beyond what is strictly necessary to explain readiness and phase transitions.
- Do not turn Prometheus metrics into the source of truth for readiness calculation.
- Do not build a cluster-level or controller-facing health API.
- Do not make runtime health depend on long-window traffic heuristics such as SERVFAIL ratio or p99 latency.

## Decisions

### 1. The first version will model only `readiness` and `phase`

The runtime contract will separate traffic acceptance from lifecycle explanation, but only with the smallest useful pair:

- `readiness`: whether rec53 is ready to accept normal DNS traffic
- `phase`: a bounded lifecycle enum such as `cold-start`, `warming`, `steady`, `shutting-down`

Why:

- `readiness` alone cannot explain whether the server is merely warming or actually unhealthy.
- `phase` alone cannot tell an orchestrator whether it should keep sending traffic.
- This pair is enough to answer the highest-value operational question without introducing a broader health taxonomy that may go unused.

Alternatives considered:

- Add `liveness` and `degraded` now as well. Rejected for v1.2.0 because they broaden the design and testing surface before the repo has a proven need for them.
- Only expose `healthy/unhealthy`. Rejected because it collapses warmup, cold-start, and shutdown into one ambiguous bit.

### 2. `readiness` will mean listener-ready and traffic-accepting, not warmup-complete

The first version will treat warmup as a lifecycle phase, not a readiness gate. Once the DNS listeners are bound and the service is not shutting down, rec53 is `ready=true` even if warmup is still running.

Why:

- Warmup is already designed as a non-blocking optimization.
- Holding readiness until warmup completes would make restarts slower and would create false negative probe failures.
- Operators care more about whether the resolver can answer now than whether its caches are fully warmed.

Alternatives considered:

- Make warmup completion a hard readiness prerequisite. Rejected because it conflicts with the current non-blocking startup design and would overstate the meaning of warmup.

### 3. The readiness probe will share the existing metrics HTTP server

The design will reuse the existing operational HTTP surface bound to `dns.metric` and expose:

- `/metric`
- `/healthz/ready`

The readiness endpoint will return simple HTTP status codes and a compact body suitable for scripts and probes.

Why:

- Reusing the metrics server avoids a new port and keeps operational wiring simple.
- systemd helper scripts, containers, and local operators already know this address.
- The repo already treats the metrics address as the local operations surface.

Alternatives considered:

- Add a dedicated health server. Rejected because it introduces more config and more lifecycle complexity for little gain.
- Expose metrics only and force users to build probes from Prometheus queries. Rejected because readiness checks should not depend on scrape infrastructure.

### 4. Runtime readiness state will be maintained in-process, not inferred from Prometheus collectors

Readiness state will live in a dedicated runtime state holder that startup, shutdown, and selected subsystems can update directly. The readiness endpoint will read from that holder.

Why:

- The readiness endpoint must reflect current state immediately.
- Startup and shutdown transitions need direct, low-latency updates.
- It avoids circular dependencies where readiness depends on the health of the metrics pipeline itself.

Alternatives considered:

- Compute status on every request from listeners and counters. Rejected because it scatters lifecycle logic and makes semantics harder to test.

## Risks / Trade-offs

- [Warmup may still be misunderstood as unhealthy] -> Mitigation: keep `ready=true` during warmup and surface `phase=warming` explicitly in the readiness response and docs.
- [Readiness on the metrics server means one HTTP surface carries both `/metric` and `/healthz/ready`] -> Mitigation: keep the endpoint small and additive; split later only if operational evidence shows a real need.
- [The first version does not provide a full health taxonomy] -> Mitigation: treat this as the minimum useful contract and add `liveness` or `degraded` later only if real operational use-cases appear.

## Migration Plan

1. Add the in-process runtime readiness model with default startup values.
2. Wire startup and shutdown transitions in `cmd/rec53.go` and `server/server.go`.
3. Expose `/healthz/ready` on the operational HTTP server.
4. Add lifecycle and probe tests.
5. Update operator docs with probe examples and state interpretation guidance.

Rollback:

- The readiness endpoint is additive.
- If needed, operators can ignore the new probe and continue using existing `/metric`, logs, and `rec53top`.
- The rollout risk is mostly semantic; fallback is to remove or stop consuming the new health surfaces.

## Open Questions

- Should the readiness body return plain text only, or include a small JSON summary with `phase`?
- Should snapshot load failure when `snapshot.enabled=true` affect only `phase` and logs, or also keep a short textual reason in the readiness body?
