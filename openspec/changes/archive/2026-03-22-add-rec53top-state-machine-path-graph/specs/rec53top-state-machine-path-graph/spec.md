## ADDED Requirements

### Requirement: rec53top SHALL render a State Machine path graph from transition metrics
`rec53top` SHALL render the `State Machine` detail path view from `rec53_state_machine_transition_total` data so operators can inspect the real request path instead of inferring flow from stage heat alone.

#### Scenario: path graph shows live transition edges
- **WHEN** the TUI has at least two successful scrapes that include state-machine transition metrics
- **THEN** the `State Machine` detail page SHALL show a path-oriented view derived from current-window transition deltas

#### Scenario: path graph remains bounded
- **WHEN** the TUI renders the state-machine path graph
- **THEN** it SHALL show a bounded set of dominant edges, branch points, or terminal exits rather than an unbounded dump of all possible transitions

### Requirement: rec53top SHALL separate current path activity from cumulative path totals
The `State Machine` detail view SHALL distinguish current-window transition activity from bounded since-start transition totals so operators can tell whether a path is hot now or only historically accumulated.

#### Scenario: path graph shows current and cumulative meaning separately
- **WHEN** the TUI renders state-machine path data
- **THEN** it SHALL label or structure current-window transitions separately from since-start transition totals

#### Scenario: cumulative transition totals stay bounded
- **WHEN** many transition labels exist in cumulative totals
- **THEN** the TUI SHALL show only a bounded top-N subset in the since-start section

### Requirement: rec53top SHALL provide dedicated State Machine path drill-down views
The `State Machine` detail page SHALL provide `Summary`, `Path Graph`, and `Failures` drill-down subviews so users can move from the top verdict to the real path and then to terminal failure interpretation without leaving the panel.

#### Scenario: State Machine detail opens on Summary first
- **WHEN** the user opens the `State Machine` detail page
- **THEN** the TUI SHALL land on the `Summary` subview first

#### Scenario: Path Graph subview shows real path data
- **WHEN** the user switches to the `Path Graph` subview
- **THEN** the TUI SHALL emphasize real transition edges, branch hotspots, and terminal exits instead of repeating only stage-frequency summaries

#### Scenario: Failures subview keeps path context
- **WHEN** the user switches to the `Failures` subview
- **THEN** the TUI SHALL relate dominant failure reasons to the path or terminal edges currently visible in the same panel

### Requirement: rec53top SHALL present a stable dominant path verdict without overstating certainty
When live transition data supports a clear dominant path, the `State Machine` detail experience SHALL summarize that path consistently. When live transition data is branch-heavy or ambiguous, the detail experience SHALL identify the branch point and leading competing edges instead of pretending that one complete path is dominant.

#### Scenario: clear live path is summarized consistently
- **WHEN** one live outgoing branch is clearly dominant at each step of the current transition walk
- **THEN** the `Summary` and `Path Graph` subviews SHALL agree on the dominant live path

#### Scenario: ambiguous live branch is shown honestly
- **WHEN** the current transition walk reaches a branch where no single outgoing edge is clearly dominant
- **THEN** the detail view SHALL stop extending the claimed dominant path at that branch
- **AND** SHALL show the leading competing edges as branch context

### Requirement: rec53top SHALL keep path endings and failure reasons interpretable together
The `Failures` subview SHALL help operators reconcile terminal exits from transition metrics with failure-reason counters so that the path view and the failure view reinforce each other.

#### Scenario: failure reason maps cleanly to a terminal exit
- **WHEN** a dominant failure reason corresponds to a canonical terminal exit
- **THEN** the `Failures` subview SHALL name both the failure reason and the matching terminal edge context

#### Scenario: coarse error exit still explains specific failure context
- **WHEN** the path graph shows `error_exit`
- **THEN** the `Failures` subview SHALL explain the most relevant bounded failure-reason bucket behind that terminal edge rather than leaving `error_exit` uninterpreted

### Requirement: rec53top SHALL keep State Machine subviews readable in non-normal panel states
When the `State Machine` panel is `WARMING`, `UNAVAILABLE`, `STALE`, or `DISCONNECTED`, the detail page SHALL keep the same `Summary`, `Path Graph`, and `Failures` subviews while rendering subview-specific explanations for why normal live path interpretation is absent or limited.

#### Scenario: warming state keeps subviews but explains limited live meaning
- **WHEN** the `State Machine` panel is `WARMING`
- **THEN** the detail page SHALL keep the `Summary`, `Path Graph`, and `Failures` subviews available
- **AND** each subview SHALL explain why short-window path meaning is not yet stable

#### Scenario: unavailable or stale state does not masquerade as empty graph
- **WHEN** the `State Machine` panel is `UNAVAILABLE`, `STALE`, or `DISCONNECTED`
- **THEN** the relevant subview SHALL explain why live path or failure interpretation is missing or stale
- **AND** SHALL avoid rendering an empty normal-looking path view as if zero traffic were healthy
