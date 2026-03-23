## 1. Slice A: Transition Metric Foundation

- [x] 1.1 Define canonical state, transition, and terminal-exit names for the resolver state machine
- [x] 1.2 Add `rec53_state_machine_transition_total{from,to}` registration and metric helpers in `monitor/`
- [x] 1.3 Instrument `server/state_machine.go` to record every real branch transition and terminal exit without changing resolver behavior

## 2. Resolver Coverage And Validation

- [x] 2.1 Add or update tests that verify normal state-to-state transitions are emitted for cache-hit, forward-hit, and iterative-resolution flows
- [x] 2.2 Add or update tests that verify terminal edges are emitted for success, FORMERR, SERVFAIL, handler-error, CNAME loop-back, and max-iterations paths
- [x] 2.3 Verify transition labels remain bounded and match the canonical naming set
- [x] 2.4 Verify failure-reason buckets and terminal exits stay reconcilable for `formerr`, `servfail`, `max_iterations`, and generic handler/internal errors

## 3. Slice B: TUI Data Model And Parsing

- [x] 3.1 Extend TUI metrics parsing to ingest `rec53_state_machine_transition_total{from,to}` alongside existing stage and failure metrics
- [x] 3.2 Extend dashboard derivation to compute current-window transition deltas, bounded since-start transition totals, and a deterministic dominant-path-or-branch summary for the `State Machine` panel
- [x] 3.3 Add unit tests for transition parsing, bounded ranking, current-vs-cumulative state-machine path derivation, and ambiguous-branch handling

## 4. State Machine Path Graph UI

- [x] 4.1 Add `Summary`, `Path Graph`, and `Failures` subviews to the `State Machine` detail page
- [x] 4.2 Render a bounded hybrid path view that shows one dominant live path plus branch points, side edges, and terminal exits from real transition metrics
- [x] 4.3 Render state-machine failure context so the `Failures` subview ties dominant failure reasons back to the visible path or terminal edges, including `error_exit`
- [x] 4.4 Add TUI rendering and navigation tests for the new state-machine subviews and fallback behavior in warming, unavailable, stale, and disconnected states

## 5. Documentation And Finish

- [x] 5.1 Update `docs/metrics.md` to document `rec53_state_machine_transition_total` and explain its `from`/`to` labels
- [x] 5.2 Update `docs/user/rec53top/pages.md` and `docs/user/local-ops-tui.md` to explain how to read the `State Machine` path graph and its subviews
- [x] 5.3 Run the most relevant Go tests for `monitor/`, `server/`, and `tui/`, then resolve any failures caused by the new path-graph work
