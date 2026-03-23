## ADDED Requirements

### Requirement: Operators can request a real resolver path for a specified domain

rec53 SHALL provide a bounded debugging capability that records and returns the real resolver path for a specified domain or query so operators and developers can inspect one request's actual state sequence instead of inferring it from aggregate counters.

#### Scenario: Domain trace returns the observed state sequence
- **WHEN** an operator requests a trace for a specified domain or query
- **THEN** rec53 SHALL return the ordered resolver states reached for that traced work
- **AND** SHALL include the final terminal result for that traced work

### Requirement: Domain trace stays scoped to targeted debugging

The domain-trace capability SHALL remain a targeted debugging workflow rather than a replacement for aggregate observability or a high-cardinality always-on telemetry stream.

#### Scenario: Trace remains request-scoped rather than global
- **WHEN** the system exposes domain-trace data
- **THEN** it SHALL scope that data to explicitly requested debugging activity
- **AND** SHALL NOT require continuously publishing every domain's full resolver path as aggregate observability data

#### Scenario: Trace explains one domain more directly than aggregate state counters
- **WHEN** an operator needs to answer why one domain slowed down or failed
- **THEN** the trace output SHALL provide a more direct explanation of that domain's state sequence and terminal outcome than the aggregate `State Machine` TUI panel alone
