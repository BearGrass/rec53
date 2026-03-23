## MODIFIED Requirements

### Requirement: Runtime readiness state model

The system SHALL maintain a minimal runtime state model with two explicit dimensions:

- `readiness`: whether rec53 is ready to accept normal DNS traffic
- `phase`: a bounded lifecycle state

The `phase` dimension SHALL use a bounded enum that includes at least `cold-start`, `warming`, `steady`, and `shutting-down`.

#### Scenario: Process begins in cold-start
- **WHEN** rec53 starts and DNS listeners have not yet completed a successful bind
- **THEN** the runtime health model SHALL report `phase=cold-start`
- **AND** `readiness` SHALL be `false`

#### Scenario: Service enters warming after listeners are ready
- **WHEN** the DNS listeners are bound successfully and configured warmup is still running
- **THEN** the runtime health model SHALL report `phase=warming`
- **AND** `readiness` SHALL be `true`

#### Scenario: Service reaches steady state without warmup
- **WHEN** the DNS listeners are bound successfully and warmup is disabled
- **THEN** the runtime health model SHALL report `phase=steady`
- **AND** `readiness` SHALL be `true`

#### Scenario: Service reaches steady state after warmup completes
- **WHEN** rec53 is already in `phase=warming` with `readiness=true` and startup warmup completes successfully or times out without aborting service
- **THEN** the runtime health model SHALL transition to `phase=steady`
- **AND** `readiness` SHALL remain `true`

#### Scenario: Service begins graceful shutdown
- **WHEN** rec53 begins graceful shutdown after receiving a stop signal or programmatic shutdown request
- **THEN** the runtime health model SHALL report `phase=shutting-down`
- **AND** `readiness` SHALL become `false` before listener teardown completes

### Requirement: Runtime health mapping MUST cover startup and restore paths

The system SHALL apply consistent runtime health mapping to startup, snapshot restore, and shutdown behavior so operators can distinguish cold-start from failure.

#### Scenario: Snapshot file missing does not block readiness
- **WHEN** `snapshot.enabled=true` and the configured snapshot file is not present at startup
- **THEN** rec53 SHALL continue startup in `phase=cold-start` before bind and `phase=warming` or `phase=steady` after bind according to warmup state
- **AND** snapshot file absence alone SHALL NOT force `readiness=false`

#### Scenario: Snapshot restore failure degrades to cold-cache startup
- **WHEN** snapshot restore encounters an error and rec53 continues serving with an empty or partially restored cache
- **THEN** the runtime health model SHALL preserve startup progress semantics instead of reporting the node dead
- **AND** the runtime state SHALL remain machine-distinguishable from normal steady-state service through `phase`

#### Scenario: Startup bind failure never becomes ready
- **WHEN** DNS listener bind fails before rec53 reaches serving state
- **THEN** the runtime health model SHALL NOT report `readiness=true`
