## Why

rec53 already has a fairly solid internal lifecycle: listeners bind before the server reports started, warmup runs in the background, XDP can degrade to the Go path, and graceful shutdown stops background work before snapshot save. What it still lacks is an operator-facing runtime readiness contract that tells external systems whether the node is ready for DNS traffic and whether it is still warming, already steady, or in the middle of shutdown.

This gap matters now because rec53 has already reached the point where it is deployed and observed through `rec53ctl`, Prometheus, and local troubleshooting flows. Without explicit runtime readiness semantics, cold-start, snapshot restore differences, warmup, and graceful shutdown are all left to logs and human interpretation.

## What Changes

- Define a minimal runtime contract centered on `readiness` and bounded lifecycle `phase`.
- Map startup, warmup, snapshot restore, steady-state service, and graceful shutdown to those runtime states.
- Add a simple readiness probe suitable for systemd, containers, and scripts.
- Add lifecycle-focused tests that verify startup failure, warming, ready state, and shutting-down behavior against the new contract.
- Update operator-facing documentation so health probes and restart behavior are interpreted consistently across `rec53ctl`, systemd, and Prometheus-based monitoring.

## Capabilities

### New Capabilities
- `runtime-health-contract`: Define the runtime state model and required transitions for `readiness` and bounded lifecycle `phase`.
- `health-probe-endpoints`: Expose an operator-consumable readiness probe and bounded phase context for local automation.

### Modified Capabilities
- `dns-server-startup`: Startup and shutdown behavior must align with the new runtime health contract so listener readiness, warmup, and graceful shutdown present consistent external semantics.

## Impact

- Affected code: `cmd/rec53.go`, `server/server.go`, `monitor/metric.go`, and any helper modules used to compute or expose runtime readiness state.
- Affected operator surfaces: metrics HTTP server, readiness probe consumers, `rec53ctl`-documented service workflows, and future container/systemd probes.
- Affected docs: `docs/user/operations*.md`, `docs/user/troubleshooting*.md`, configuration or deployment guidance, and roadmap references for `v1.2.0`.
- Affected tests: startup/shutdown tests, readiness-probe tests, and lifecycle-state validation around warmup, snapshot restore, and graceful shutdown.
