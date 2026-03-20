## ADDED Requirements

### Requirement: Observability outputs distinguish developer and operator audiences

rec53 SHALL document observability outputs in a way that explicitly separates developer diagnosis goals from operator or deployment-user health checks.

#### Scenario: Developer-facing questions are explicit
- **WHEN** a developer reads the observability guidance
- **THEN** the guidance SHALL identify which metrics help answer regression, behavior-change, or performance-comparison questions

#### Scenario: Operator-facing questions are explicit
- **WHEN** an operator or deployment user reads the observability guidance
- **THEN** the guidance SHALL identify which metrics help answer whether the resolver is healthy, degraded, or likely misconfigured

### Requirement: Operator observability includes a baseline dashboard view

rec53 SHALL provide a baseline dashboard definition or documented dashboard layout that covers request volume, response quality, cache effectiveness, snapshot behavior, upstream health, XDP status, and state-machine failure concentration.

#### Scenario: Dashboard covers runtime health domains
- **WHEN** an operator opens the baseline dashboard
- **THEN** the dashboard SHALL include panels or documented views for cache, snapshot, upstream, XDP, and state-machine health in addition to request and latency basics

#### Scenario: Dashboard supports rapid triage
- **WHEN** cache misses spike or upstream failures increase
- **THEN** the baseline dashboard SHALL make those signals visible without requiring the operator to start from raw metric family discovery

### Requirement: Operator observability includes a troubleshooting checklist

rec53 SHALL provide an operator checklist that maps common degraded states to the first metrics and logs that should be inspected.

#### Scenario: Cache effectiveness regression checklist exists
- **WHEN** cache hit quality drops or misses rise unexpectedly
- **THEN** the checklist SHALL point operators to the relevant cache metrics and the next supporting signals to inspect

#### Scenario: Snapshot restore issue checklist exists
- **WHEN** startup quality degrades because snapshot restore fails or restores too few entries
- **THEN** the checklist SHALL point operators to snapshot load outcome, skipped-entry, and duration signals

#### Scenario: Upstream or XDP degradation checklist exists
- **WHEN** operators see rising timeouts, fallback activity, or XDP degradation
- **THEN** the checklist SHALL identify the upstream and XDP signals that should be checked before deeper code-level debugging
