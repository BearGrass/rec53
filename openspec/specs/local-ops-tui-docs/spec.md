# local-ops-tui-docs Specification

## Purpose
TBD - created by archiving change build-local-ops-tui-v1-1-2. Update Purpose after archive.
## Requirements
### Requirement: TUI usage documentation supports local self-test

rec53 SHALL document how developers and operators can launch the local ops TUI, point it at a metrics endpoint, and verify the MVP panels during local self-test.

#### Scenario: Local launch flow is documented
- **WHEN** a reader opens the TUI documentation
- **THEN** the documentation SHALL show the basic launch command, the default endpoint, and how to override the endpoint

#### Scenario: Self-test flow is documented
- **WHEN** a developer wants to validate the dashboard locally
- **THEN** the documentation SHALL describe a minimal self-test flow that produces query traffic and explains which panels or summaries should change first

### Requirement: TUI documentation states MVP boundaries

rec53 SHALL document the TUI's MVP boundaries so readers do not mistake it for a replacement for Prometheus, Grafana, or future multi-node observability tooling. The documentation SHALL also explain that the `State Machine` panel is an aggregate heat/exit view and that exact per-domain resolver paths belong to dedicated trace/debugging tooling rather than the aggregate TUI.

#### Scenario: Out-of-scope features are explicit
- **WHEN** a reader evaluates the TUI documentation
- **THEN** the documentation SHALL state that the MVP is single-target, read-only, and limited to current-state or short-window summaries

#### Scenario: Relationship to existing metrics docs is explicit
- **WHEN** a reader needs deeper analysis than the TUI provides
- **THEN** the documentation SHALL direct them to the metrics and operator observability docs rather than implying the TUI replaces those references

#### Scenario: State Machine aggregate-vs-trace boundary is documented
- **WHEN** a reader wants to understand what the `State Machine` panel can and cannot explain
- **THEN** the documentation SHALL explain that the panel summarizes mixed aggregate activity
- **AND** SHALL explain that one domain's real resolver path requires a dedicated trace/debug flow
