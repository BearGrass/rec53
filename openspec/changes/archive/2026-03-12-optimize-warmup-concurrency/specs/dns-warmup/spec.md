## MODIFIED Requirements

### Requirement: Warmup respects system CPU constraints
The system SHALL calculate warmup concurrency dynamically to prevent CPU oversubscription and maintain system responsiveness. Concurrency is calculated as `min(runtime.NumCPU() * 2, 8)` and can be overridden via config.yaml.

#### Scenario: Warmup does not cause CPU oversubscription
- **WHEN** rec53 starts warmup on a 4-core machine
- **THEN** warmup uses 8 concurrent goroutines instead of hardcoded 32, reducing context switching overhead

#### Scenario: System remains responsive during warmup
- **WHEN** rec53 is warming up with dynamically calculated concurrency
- **THEN** the system remains responsive to incoming DNS queries and other operations

#### Scenario: Explicit config overrides dynamic calculation
- **WHEN** user explicitly sets warmup.concurrency in config.yaml
- **THEN** system respects the explicit value and does not apply dynamic formula
