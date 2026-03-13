## ADDED Requirements

### Requirement: Curated TLD list defined
The system SHALL maintain a curated list of exactly 30 TLDs selected for warmup operations. The list SHALL be hardcoded with an option to override via configuration.

#### Scenario: Default TLD list loads on startup
- **WHEN** rec53 starts without explicit TLD configuration
- **THEN** system loads the built-in curated list of 30 TLDs

#### Scenario: Custom TLD list overrides default
- **WHEN** user configures `warmup.tlds` in config.yaml
- **THEN** system uses the custom list instead of the built-in default

#### Scenario: TLD list contains required tier-1 TLDs
- **WHEN** warmup process initializes
- **THEN** the TLD list SHALL contain all tier-1 TLDs: .com, .cn, .de, .net, .org, .uk, .ru, .nl

#### Scenario: TLD list contains tier-2 TLDs for geographic/strategic coverage
- **WHEN** warmup process initializes
- **THEN** the TLD list SHALL contain at least 22 additional tier-2 TLDs representing major ccTLDs and gTLDs

### Requirement: TLD list composition is auditable
The system SHALL expose the current active TLD list for operational visibility.

#### Scenario: List is documented in code and config template
- **WHEN** operator reviews rec53 codebase or config.yaml template
- **THEN** the curated TLD list is clearly documented with rationale for each tier

#### Scenario: Logs indicate loaded TLD count at startup
- **WHEN** rec53 starts up
- **THEN** system logs at INFO level: "Loaded N TLDs for warmup"

### Requirement: TLD list is easily maintainable
The system SHALL support adding/removing TLDs without code recompilation.

#### Scenario: TLD list can be updated in config
- **WHEN** operator edits `warmup.tlds` in config.yaml and restarts rec53
- **THEN** system loads the new TLD list without code changes

#### Scenario: Built-in list is clearly separated from user overrides
- **WHEN** developer reviews tld_config.go
- **THEN** the default hardcoded list is distinct from any user-provided configuration
