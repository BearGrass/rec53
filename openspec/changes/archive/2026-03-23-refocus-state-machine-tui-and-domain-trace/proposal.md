## Why

The current `State Machine` direction in `rec53top` tries to explain resolver behavior through aggregated path graphics, but the result is hard to read in a square terminal view, especially when requests are concurrent, windows cut across in-flight work, and the resolver can loop back through states. We need to refocus the TUI on signals that are easy to trust at a glance, and move high-value path diagnosis to a domain-scoped debugging capability that can answer what a specific query actually did.

## What Changes

- Simplify the `State Machine` panel in `rec53top` so it emphasizes recent per-state counters, terminal-exit counters, and a small bounded failure summary instead of aggregated live-path graphics.
- Update `State Machine` detail semantics so the panel answers "which states are heating up" and "which exits are growing" rather than trying to reconstruct one global path for mixed concurrent traffic.
- Update TUI docs and page descriptions so operators understand the new `State Machine` scope and are directed to a domain-scoped trace flow when they need request-level explanation.
- Introduce a new domain-scoped debugging capability that records and exposes the real resolver path for a specified domain or query, including the sequence of states reached and the final terminal outcome.

## Capabilities

### New Capabilities
- `domain-resolution-trace`: Record and expose the real resolver path for a specified domain or query so operators and developers can inspect one request's actual state sequence and terminal result.

### Modified Capabilities
- `local-ops-tui`: Change the `State Machine` panel requirements so the TUI focuses on readable state and terminal counters instead of aggregated path graphics.
- `rec53top-detail-panels`: Change the diagnostic expectations for `State Machine` detail so it provides clearer counter-oriented interpretation rather than global path reconstruction.
- `local-ops-tui-docs`: Update the TUI guidance so readers understand the simplified `State Machine` panel and when to use domain-scoped trace tooling instead.

## Impact

- Affected code: `tui/`, related state-machine metric derivation, and any server/monitor code needed to support domain-scoped trace capture.
- Affected docs: `docs/user/local-ops-tui*.md`, `docs/user/rec53top/pages*.md`, roadmap, and any new usage guide for domain trace.
- Affected behavior: `State Machine` becomes easier to read in the TUI, while request-level path diagnosis moves to a more explicit debugging interface.
