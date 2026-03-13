## ADDED Requirements

### Requirement: Warmup memory footprint is significantly reduced
The system SHALL reduce warmup memory consumption by 80-90% compared to dynamic TLD enumeration.

#### Scenario: Memory usage stays within resource constraints
- **WHEN** rec53 starts warmup with 30 TLDs in a 512MB RAM environment
- **THEN** warmup completes without memory allocation errors

#### Scenario: Peak memory during warmup is predictable
- **WHEN** rec53 runs warmup with the curated TLD list
- **THEN** peak memory usage is documented and does not exceed baseline + 50MB

#### Scenario: Memory is released after warmup completes
- **WHEN** warmup process finishes and transitions to normal query mode
- **THEN** memory footprint stabilizes at steady-state levels

### Requirement: Warmup time is proportionally reduced
The system SHALL complete warmup faster due to probing fewer TLDs.

#### Scenario: Warmup time is proportional to TLD count
- **WHEN** rec53 runs warmup with 30 TLDs instead of thousands
- **THEN** total warmup duration is approximately (30/N) of the original duration, where N = previous TLD count

#### Scenario: Warmup time is acceptable for startup
- **WHEN** rec53 starts with warmup enabled
- **THEN** warmup completes within 30 seconds on typical infrastructure

### Requirement: Warmup process does not crash on startup
The system SHALL complete startup without crashes in resource-constrained environments.

#### Scenario: Startup succeeds in 512MB RAM environment
- **WHEN** rec53 starts in a container with 512MB RAM limit
- **THEN** process completes startup without out-of-memory errors

#### Scenario: Crash recovery is not needed
- **WHEN** rec53 starts normally
- **THEN** no crashes occur and process remains healthy for continuous operation

### Requirement: Warmup failure modes are handled gracefully
The system SHALL continue operating even if warmup encounters issues with individual TLDs.

#### Scenario: Single TLD probe failure does not block warmup
- **WHEN** probing fails for one TLD in the curated list
- **THEN** warmup continues with remaining TLDs and logs the failure

#### Scenario: Partial warmup completion is acceptable
- **WHEN** warmup completes with N of 30 TLDs successfully probed
- **THEN** system enters normal operation mode and serves queries via fallback for unprobed TLDs

### Requirement: Warmup can be disabled for faster startup
The system SHALL support disabling warmup entirely for scenarios requiring minimal startup time.

#### Scenario: No-warmup mode skips TLD probing
- **WHEN** user starts rec53 with `--no-warmup` flag
- **THEN** warmup process is skipped entirely and system enters query mode immediately
