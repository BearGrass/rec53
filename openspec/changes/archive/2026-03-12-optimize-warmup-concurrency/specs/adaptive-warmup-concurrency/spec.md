## ADDED Requirements

### Requirement: Warmup concurrency calculated based on available CPU cores
The system SHALL calculate warmup concurrency dynamically at startup using the formula: `min(runtime.NumCPU() * 2, 8)`. This ensures optimal resource utilization without oversubscription on machines with varying CPU core counts.

#### Scenario: Dynamic concurrency on 4-core machine
- **WHEN** rec53 starts on a machine with 4 CPU cores
- **THEN** warmup uses concurrency = min(4 * 2, 8) = 8 goroutines

#### Scenario: Dynamic concurrency on 2-core machine
- **WHEN** rec53 starts on a machine with 2 CPU cores
- **THEN** warmup uses concurrency = min(2 * 2, 8) = 4 goroutines

#### Scenario: Dynamic concurrency capped on large machines
- **WHEN** rec53 starts on a machine with 16 or more CPU cores
- **THEN** warmup uses concurrency = min(16 * 2, 8) = 8 goroutines (capped at 8)

#### Scenario: Concurrency can be overridden via configuration
- **WHEN** user sets `warmup.concurrency: 16` in config.yaml
- **THEN** system respects the explicit config value and does not apply dynamic calculation

### Requirement: Warmup startup logs report actual concurrency level
The system SHALL log the calculated concurrency value at INFO level during warmup startup to provide operational visibility.

#### Scenario: Startup log shows calculated concurrency
- **WHEN** rec53 starts warmup
- **THEN** logs contain message like "Starting NS warmup with 31 TLDs, concurrency: 8"

#### Scenario: Startup log shows CPU core count
- **WHEN** rec53 starts warmup on any machine
- **THEN** logs clearly indicate the number of available CPU cores used in calculation
