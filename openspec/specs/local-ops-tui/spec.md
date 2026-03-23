# local-ops-tui Specification

## Purpose
TBD - created by archiving change build-local-ops-tui-v1-1-2. Update Purpose after archive.
## Requirements
### Requirement: Local ops TUI reads rec53 metrics directly

rec53 SHALL provide a local terminal dashboard command that reads the rec53 Prometheus metrics endpoint directly, without requiring a Prometheus server, Grafana, or external datastore. The command SHALL default to `http://127.0.0.1:9999/metric` and SHALL allow the target endpoint to be overridden explicitly.

#### Scenario: Default local target works
- **WHEN** an operator launches the TUI without specifying a target
- **THEN** the TUI SHALL attempt to scrape `http://127.0.0.1:9999/metric`

#### Scenario: Explicit target override works
- **WHEN** an operator launches the TUI with an explicit metrics endpoint
- **THEN** the TUI SHALL scrape that endpoint instead of the default local address

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

### Requirement: Local ops TUI degrades explicitly for unavailable data

The local ops TUI SHALL distinguish unreachable targets, missing metric families, and intentionally absent XDP metrics, and SHALL surface those states explicitly instead of silently treating them as healthy zero values.

#### Scenario: Unreachable target is explicit
- **WHEN** the metrics endpoint cannot be reached
- **THEN** the TUI SHALL show that the target is disconnected or stale
- **AND** SHALL surface the latest scrape error without crashing

#### Scenario: Missing metric family is explicit
- **WHEN** a required metric family is absent from the scrape result
- **THEN** the corresponding panel SHALL show an unavailable state rather than a zero-value success state

#### Scenario: XDP-disabled deployments remain readable
- **WHEN** XDP metrics are absent but the rest of the scrape succeeds
- **THEN** the XDP panel SHALL identify the state as disabled or unsupported rather than degraded sync failure

### Requirement: Local ops TUI remains a single-target read-only MVP

The first local ops TUI release SHALL remain scoped to a single target, a read-only dashboard, and current-state or short-window summaries. It MUST NOT require persistent history, multi-target aggregation, or interactive drill-down for the MVP to be considered complete.

#### Scenario: Single-target scope is preserved
- **WHEN** a user launches the MVP TUI
- **THEN** the session SHALL monitor exactly one configured metrics endpoint at a time

#### Scenario: No history backend is required
- **WHEN** a user runs the MVP TUI on a machine with only rec53 exposed locally
- **THEN** the TUI SHALL still provide its supported dashboard behavior without any separate timeseries backend

### Requirement: local ops TUI docs SHALL explain default navigation

The operational TUI guide SHALL describe the default navigation path for overview focus, opening detail, moving through any supported drill-down subviews, and returning to overview so the interface can be used without memorizing only numeric shortcuts.

#### Scenario: User reads the navigation section
- **WHEN** a user reads the TUI operational guide
- **THEN** the guide SHALL explain overview focus movement
- **AND** SHALL explain how to open detail from the current focus
- **AND** SHALL explain how to move between drill-down subviews when the current panel supports them
- **AND** SHALL still document the numeric shortcuts as compatible fast paths
