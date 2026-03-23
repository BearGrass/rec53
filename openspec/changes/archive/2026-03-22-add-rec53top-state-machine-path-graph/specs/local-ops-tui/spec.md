## MODIFIED Requirements

### Requirement: Local ops TUI presents the six MVP health domains

The local ops TUI SHALL present a fixed MVP dashboard that covers traffic, cache, snapshot, upstream, XDP, and state-machine health so that users can inspect rec53's current operating state without composing PromQL first. When transition metrics are available, the `State Machine` detail experience SHALL also expose a real path-oriented reading of resolver flow in the same single-target session.

#### Scenario: Core health domains are visible
- **WHEN** the TUI successfully loads rec53 metrics
- **THEN** it SHALL display summaries for traffic, cache, snapshot, upstream, XDP, and state-machine health in the same session

#### Scenario: Short-window behavior is visible
- **WHEN** the TUI has at least two successful scrapes
- **THEN** it SHALL display derived short-window rates, ratios, or status summaries needed to judge recent behavior rather than only raw counters

#### Scenario: State Machine detail can expose real path flow
- **WHEN** the TUI loads transition metrics for the state machine
- **THEN** the `State Machine` detail page SHALL allow the operator to inspect the real request path from those transitions without leaving the local TUI session
