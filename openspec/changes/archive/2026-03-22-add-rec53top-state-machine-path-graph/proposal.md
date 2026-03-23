## Why

`rec53top` currently shows state-machine stage hot spots and failure buckets, but it cannot show the actual state transitions that requests are taking. That leaves operators guessing whether they are looking at a cache-led path, iterative referral path, CNAME loop, or terminal failure edge when investigating resolver behavior.

## What Changes

- Add transition-level state-machine telemetry so rec53 records real `from -> to` edges, including terminal exits such as success, FORMERR, SERVFAIL, and max-iteration protection.
- Add a `State Machine` detail path view in `rec53top` that renders the current dominant request path from real transition metrics instead of inferring a path from stage frequency alone.
- Extend the `State Machine` detail experience with a path-oriented subview layout (`Summary`, `Path Graph`, `Failures`) that keeps the existing detail reading order while making real path data inspectable.
- Document the new transition metric and explain how to read the `State Machine` path graph without confusing stage heat with true transitions.

## Capabilities

### New Capabilities
- `state-machine-transition-metrics`: Record canonical state-machine transition edges, including terminal exits, as Prometheus metrics that other observability surfaces can consume.
- `rec53top-state-machine-path-graph`: Render a real path-oriented detail view for the `State Machine` panel using transition metrics.

### Modified Capabilities
- `local-ops-tui`: The local TUI detail behavior for the `State Machine` panel changes from stage-only interpretation to real path inspection.
- `rec53top-detail-panels`: The `State Machine` detail page gains additional diagnostic value through a path graph and failure-oriented drill-down views.
- `metrics-doc`: The metrics documentation must describe the new transition metric and its labels.

## Impact

- Affected code: `server/state_machine.go`, `monitor/metric.go`, `monitor/var.go`, `tui/`, and `cmd/rec53top`
- Affected docs: `docs/metrics.md`, `docs/user/rec53top/pages.md`, `docs/user/local-ops-tui.md`, and related roadmap references
- Affected tests: state-machine metrics tests, TUI parsing/rendering tests, and path/terminal-edge coverage for resolver flows
