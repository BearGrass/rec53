## Context

`rec53top` already exposes a `State Machine` panel, but the panel is limited to stage-frequency and failure-reason summaries. The resolver records `rec53_state_machine_stage_total{stage}` and `rec53_state_machine_failures_total{reason}`, which are useful for heat and terminal error diagnosis, yet they do not preserve the real `from -> to` edges a request traversed through the resolver loop.

This change crosses the resolver, metrics, TUI, and documentation layers. The state-machine loop in `server/state_machine.go` currently records stage entry on each iteration, and then changes `stm` or returns early depending on branch outcomes. To render a true path graph in `rec53top`, the system must emit canonical transition metrics for both normal state changes and terminal exits such as `formerr`, `servfail`, `success`, and `max_iterations`.

Constraints:
- The resolver must keep its current behavior; this change adds observability, not a new resolution algorithm.
- Transition labels must remain bounded and stable.
- The TUI must stay single-target, session-local, and text-first.
- The path graph must be based on real transition metrics, not heuristics derived from stage heat.

## Goals / Non-Goals

**Goals:**
- Add a canonical transition metric for the resolver state machine with bounded `from` and `to` labels.
- Capture terminal edges so a request path can end visibly in `success`, `formerr`, `servfail`, `error`, or `max_iterations` style exits.
- Extend `rec53top` so the `State Machine` detail page can render a path-oriented drill-down from transition metrics.
- Preserve current detail reading order with `Summary`, `Path Graph`, and `Failures` views.
- Update metrics and user docs so operators understand the new metric and the graph semantics.

**Non-Goals:**
- Reworking the resolver state machine itself.
- Adding multi-target TUI support, persistent history, or fleet-level graphing.
- Building a fully interactive node-link editor or mouse-driven graph widget.
- Emitting unbounded per-domain, per-query, or per-CNAME-instance path telemetry.

## Decisions

### Decision: Add `rec53_state_machine_transition_total{from,to}` as the canonical path metric

The system will add a new Prometheus counter vector that increments on every real state transition and on every modeled terminal exit.

Why:
- The existing stage metric only answers "which states were entered".
- A path graph needs real edges, not inferred adjacency.
- A bounded `from,to` label model fits Prometheus and the current local-ops architecture.

Alternatives considered:
- Reconstruct edges from adjacent stage counts in the TUI: rejected because counts do not preserve ordering or branch identity.
- Emit logs only and parse them client-side: rejected because `rec53top` is metrics-driven and should remain usable without log parsing.

### Decision: Model terminal exits as explicit edges to synthetic bounded nodes

The transition graph will include canonical terminal targets such as `success_exit`, `formerr_exit`, `servfail_exit`, `error_exit`, and `max_iterations_exit`.

Why:
- Real path graphs are incomplete if they only show intra-loop edges and never show how requests end.
- The current resolver returns early from several branches; explicit terminal nodes let the TUI show those endings as first-class path outcomes.

Alternatives considered:
- Keep terminal outcomes in `rec53_state_machine_failures_total` only: rejected because the path graph would appear truncated.
- Encode terminal outcomes as fake stages in `rec53_state_machine_stage_total`: rejected because it overloads stage semantics and weakens existing metrics.

### Decision: Define and reuse a canonical transition list in code

The implementation will centralize allowed state names and terminal-node names so resolver instrumentation, tests, and TUI rendering all use the same identifiers.

Why:
- Transition metrics are only useful if label values are stable.
- TUI graph rendering and documentation both need a trustworthy mapping.

Alternatives considered:
- Build label strings inline at call sites: rejected because it increases drift risk and makes coverage auditing harder.

### Decision: Instrument state changes at branch points in `server/state_machine.go`

Each branch in the resolver loop will record the edge it actually takes immediately before changing state or returning.

Why:
- The loop already acts as the single state transition coordinator.
- This keeps instrumentation close to the authoritative transition logic and avoids duplicated inference elsewhere.

Alternatives considered:
- Instrument inside each individual state handler: rejected because handlers return branch codes, but the authoritative next-state decision is made in the loop.

### Decision: Build the TUI graph from transition deltas plus bounded since-start totals

`rec53top` will parse transition counters alongside existing stage and failure metrics, derive current-window rates from successive scrapes, and present both short-window path activity and bounded cumulative transition totals.

Why:
- Existing detail pages already separate current-window and since-start meaning.
- Operators need to tell whether a path is hot now or only historically accumulated.

Alternatives considered:
- Show only current-window paths: rejected because longer-lived path bias would be hidden.
- Show only cumulative transition totals: rejected because current regressions would be harder to spot.

### Decision: Derive the dominant path deterministically and stop at ambiguous branches

The `Summary` verdict and `Path Graph` subview will derive the "dominant path" from current-window transition deltas using a deterministic walk:

- Start from the canonical entry state `init`.
- Follow the highest live outgoing edge from the current state.
- Stop when no live outgoing edge remains, when a terminal exit is reached, or when the top branch is too close to the next candidate to claim a single dominant path honestly.
- When the walk stops because of ambiguity, render the state as a branch point and show the leading competing edges rather than inventing a single path.

Why:
- The coolest version of this feature is only useful if the same live traffic shape produces the same reading on every refresh.
- Operators should see when the resolver has one clearly dominant path versus when traffic is genuinely split across branches.

Alternatives considered:
- Pick a path heuristically from stage heat: rejected because it is not a real path and can disagree with transition counters.
- Always force a single full path: rejected because it overstates certainty and makes branch-heavy workloads look cleaner than they are.

### Decision: Use a hybrid textual graph with a fixed visual budget

The `Path Graph` subview will use a hybrid layout:

- one main path ladder for the current dominant live path
- a bounded side-edge list for the most important branch alternatives
- a bounded terminal-exit list for live endings
- a separate bounded since-start ranking block

The visual budget should stay intentionally tight so the panel remains readable in a terminal:

- one dominant path
- up to three side branches
- up to three live terminal exits
- a bounded top-N cumulative list rather than a full edge dump

Why:
- A terminal UI benefits from one strong visual story plus a few ranked exceptions.
- This gives the feature some visual punch without turning it into a dense ASCII graph that users have to decode line by line.

Alternatives considered:
- Dense full-DAG ASCII art: rejected because it is hard to scan and too easy to overflow smaller terminals.
- Ranked edge lists only: rejected because they are useful but do not feel like a path view.

### Decision: Keep failure counters and terminal exits aligned but not identical

The `Failures` subview will reconcile two bounded models:

- terminal exits from `rec53_state_machine_transition_total{from,to}` explain where requests ended in the path
- failure counters from `rec53_state_machine_failures_total{reason}` explain why they failed

Expected mapping:

- `formerr` -> `formerr_exit`
- `servfail` -> `servfail_exit`
- `max_iterations` -> `max_iterations_exit`
- handler or internal resolver errors -> `error_exit`

`error_exit` remains a single synthetic terminal node in the path graph. More specific handler-error detail stays in the failure-reason metric family and logs, not in transition labels.

Why:
- Operators need path endings and failure reasons to reinforce each other instead of competing.
- Keeping `error_exit` coarse preserves bounded transition labels while still letting the `Failures` view explain what sits behind that bucket.

Alternatives considered:
- Add many error-specific terminal exits: rejected because it would bloat the path graph and increase label drift risk.
- Ignore reconciliation and show each family independently: rejected because the `Failures` view would feel disconnected from the path graph.

### Decision: Add `Summary`, `Path Graph`, and `Failures` subviews to `State Machine`

The `State Machine` detail page will follow the same drill-down pattern as other supported panels, but the graph-specific content will live in a dedicated subview rather than flattening everything into one page.

Why:
- The graph view is more complex than a summary page and needs room for edges, branch hotspots, and terminal outcomes.
- Keeping `Summary` as the default preserves the established reading order.

Alternatives considered:
- Put the graph in the default summary page: rejected because it would crowd out the current standout diagnosis.
- Create a multi-level navigation tree: rejected because `rec53top` remains intentionally lightweight.

### Decision: Preserve subview behavior in non-normal states

When the panel is `WARMING`, `UNAVAILABLE`, `STALE`, or `DISCONNECTED`, the `State Machine` detail page will keep the same `Summary`, `Path Graph`, and `Failures` subviews visible, but each subview will render a mode-specific explanation instead of pretending to have normal path insight.

Expected behavior:

- `Summary` explains why the panel is not yet fully interpretable.
- `Path Graph` explains why live path edges are absent or stale.
- `Failures` explains whether failure interpretation is unavailable, stale, or intentionally incomplete for the current session state.

Why:
- Keeping subview navigation stable makes the panel feel intentional rather than broken.
- Operators should learn one navigation model regardless of scrape health.

Alternatives considered:
- Hide subviews outside `OK`/`DEGRADED`: rejected because the panel would jump between different navigation models.
- Reuse the exact same fallback body in every subview: rejected because each subview answers a different question.

## Risks / Trade-offs

- [Instrumentation drift] -> Mitigation: centralize canonical transition names and add tests that cover every major branch and terminal exit.
- [Missed terminal edges] -> Mitigation: audit every early return path in `server/state_machine.go`, including FORMERR, SERVFAIL, generic errors, and max-iteration protection.
- [Graph readability in a terminal] -> Mitigation: prefer a bounded textual path graph and ranked transitions over a dense ASCII art DAG.
- [Metric cardinality creep] -> Mitigation: keep labels limited to canonical state and exit names; do not add query name, qtype, or domain labels.
- [Operator confusion between stage heat and path edges] -> Mitigation: keep transition views separate from stage/failure summaries and document the distinction explicitly.

## Migration Plan

1. Add the new transition metric and registration path in `monitor/`.
2. Instrument canonical transitions and terminal exits in `server/state_machine.go`.
3. Extend TUI metrics parsing and dashboard derivation to consume transition counters.
4. Add the `State Machine` path subviews and rendering logic in `tui/`.
5. Update tests for metrics, resolver path coverage, and TUI rendering.
6. Update metrics and user docs.

Recommended implementation slices:

- Slice A: transition metric foundation plus resolver coverage and validation. This slice is useful on its own because it ships the new metric without changing resolver behavior.
- Slice B: TUI derivation, `Summary` / `Path Graph` / `Failures` subviews, and docs. This slice depends on Slice A but remains local to `tui/` and user documentation.

This keeps the overall task medium-large but controllable. It should not be treated as one monolithic UI-and-instrumentation patch.

Rollback strategy:
- If the TUI work is not ready, the transition metric can still ship independently without changing resolver behavior.
- If instrumentation proves noisy or incomplete, the new metric can be ignored by the TUI while existing stage/failure panels continue working.

## Open Questions

- Should the ambiguity guard for "dominant path" use a fixed percentage band, a minimum edge-rate floor, or both?
- Should the `Path Graph` subview visually annotate branch ambiguity inline on the ladder, or keep ambiguity in a separate side-edge block only?
- Should `success_exit` always be modeled from `return_resp`, or should direct successful early returns preserve their actual source state for additional fidelity?
