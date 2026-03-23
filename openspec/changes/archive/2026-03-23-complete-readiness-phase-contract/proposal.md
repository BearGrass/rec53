## Why

rec53 already exposes the first version of `readiness / phase`, but the contract is still only partially closed: code, tests, docs, and roadmap are not fully aligned, and operators still lack a single stable definition for startup, warming, steady service, and graceful shutdown. This is worth finishing now because runtime contract clarity is more valuable than additional TUI polish or broader resilience work.

## What Changes

- Complete the runtime readiness contract so `cold-start`, `warming`, `steady`, and `shutting-down` have explicit operator-facing semantics.
- Tighten the mapping between listener bind, warmup, snapshot restore behavior, startup failure, and graceful shutdown.
- Refine the readiness endpoint contract and tests so probe status and body stay aligned with the lifecycle model.
- Update operator-facing docs and roadmap so `v1.2.0` reflects implemented baseline vs remaining work.

## Capabilities

### New Capabilities

- `runtime-phase-ops-docs`: operator and roadmap documentation for readiness and lifecycle phase behavior

### Modified Capabilities

- `runtime-health-contract`: clarify lifecycle semantics and edge cases for startup, warmup, restore, and shutdown
- `health-probe-endpoints`: tighten the readiness probe contract and response expectations
- `dns-server-startup`: align startup/shutdown behavior with the finalized readiness contract

## Impact

- Affected code: `monitor/`, `server/`, related tests in `monitor/` and `server/`
- Affected docs: roadmap and operator-facing health/probe guidance
- External surface: `GET /healthz/ready` semantics and documented lifecycle meanings
