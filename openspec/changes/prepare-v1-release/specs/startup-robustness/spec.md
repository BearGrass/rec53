## MODIFIED Requirements

### Requirement: Graceful error handling during startup
The system SHALL not panic during initialization even when unexpected errors occur, and SHALL report startup failures clearly to the operator. Startup code SHALL avoid indefinite waits when listener creation fails before readiness is signalled.

#### Scenario: DNS server initialization fails gracefully
- **WHEN** DNS server startup encounters a listener bind or startup error
- **THEN** the error SHALL be surfaced to the caller or logs without panic
- **AND** the startup path SHALL NOT block forever waiting for readiness signals that can no longer arrive

#### Scenario: Metrics or logger initialization succeeds
- **WHEN** system starts with valid config
- **THEN** logger and metrics initialization SHALL complete without crashing the main process

### Requirement: Diagnostic logging during initialization
The system SHALL log enough information during startup to identify which major component failed or degraded, including config loading, metrics startup, DNS startup, and optional feature degradation.

#### Scenario: XDP 降级可诊断
- **WHEN** XDP initialization fails but the DNS server continues in degraded mode
- **THEN** logs SHALL indicate the failure reason and the fact that Go-only cache mode is being used

#### Scenario: 启动完成日志存在
- **WHEN** system successfully initializes all critical components
- **THEN** an info-level log entry SHALL confirm the listen and metric addresses in use

### Requirement: Warmup robustness
The system SHALL handle warmup cancellation, timeout, and panic without crashing the main DNS server process or blocking service startup.

#### Scenario: Warmup timeout or cancellation
- **WHEN** warmup exceeds its deadline or the server begins shutdown
- **THEN** warmup SHALL stop promptly and the DNS server SHALL continue or shut down cleanly

#### Scenario: Warmup panic
- **WHEN** warmup routine panics
- **THEN** the panic SHALL be contained and logged without crashing the process
