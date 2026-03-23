## MODIFIED Requirements

### Requirement: Local ops TUI presents the six MVP health domains

The local ops TUI SHALL present a fixed MVP dashboard that covers traffic, cache, snapshot, upstream, XDP, and state-machine health so that users can inspect rec53's current operating state without composing PromQL first. For the `State Machine` domain, the TUI SHALL emphasize readable aggregate state and terminal signals rather than attempting to reconstruct one global live path for mixed concurrent traffic.

#### Scenario: Core health domains are visible
- **WHEN** the TUI successfully loads rec53 metrics
- **THEN** it SHALL display summaries for traffic, cache, snapshot, upstream, XDP, and state-machine health in the same session

#### Scenario: Short-window behavior is visible
- **WHEN** the TUI has at least two successful scrapes
- **THEN** it SHALL display derived short-window rates, ratios, or status summaries needed to judge recent behavior rather than only raw counters

#### Scenario: State Machine summary remains counter-oriented
- **WHEN** an operator reads the `State Machine` panel in overview or detail
- **THEN** the TUI SHALL surface recent state-entry and terminal-exit signals that remain readable even when different requests are mixed inside one scrape window
- **AND** SHALL NOT require the operator to trust one aggregated global path as the primary interpretation of the panel
