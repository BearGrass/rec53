## MODIFIED Requirements

### Requirement: Readiness probe

The system SHALL expose a machine-consumable HTTP readiness probe on the operational HTTP server bound to the configured metrics listen address.

The probe surface SHALL include at least `GET /healthz/ready`.

#### Scenario: Readiness probe succeeds after listener bind
- **WHEN** the DNS listeners are ready to accept traffic
- **THEN** `GET /healthz/ready` SHALL return a success status code

#### Scenario: Readiness probe fails during cold-start
- **WHEN** rec53 has not yet completed a successful DNS listener bind
- **THEN** `GET /healthz/ready` SHALL return a non-success status code

#### Scenario: Readiness probe fails during shutdown
- **WHEN** rec53 has begun graceful shutdown
- **THEN** `GET /healthz/ready` SHALL return a non-success status code before listener teardown completes

### Requirement: Probe responses MUST expose bounded runtime context

The health probe surface SHALL expose bounded runtime context sufficient for scripts and operators to distinguish lifecycle states without parsing logs.

The response body SHALL include at least the current `ready` value and current `phase`.

#### Scenario: Cold-start probe body identifies not-ready startup
- **WHEN** rec53 is in cold-start before listener bind
- **THEN** the readiness response body SHALL identify `ready=false`
- **AND** it SHALL identify `phase=cold-start`

#### Scenario: Warming probe body identifies ready-but-warming state
- **WHEN** rec53 is ready to serve but startup warmup is still in progress
- **THEN** the health response body SHALL identify `ready=true`
- **AND** it SHALL identify `phase=warming`

#### Scenario: Shutdown probe body identifies shutdown phase
- **WHEN** rec53 has begun graceful shutdown
- **THEN** the readiness response body SHALL identify `ready=false`
- **AND** it SHALL identify `phase=shutting-down`
