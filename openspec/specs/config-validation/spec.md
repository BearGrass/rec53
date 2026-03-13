### Requirement: Config file format validation
The system SHALL validate that the configuration file is valid YAML and contains all required fields before attempting to initialize any subsystems.

#### Scenario: Valid config file
- **WHEN** user provides a valid YAML config file with required fields (dns.listen, dns.metric, warmup)
- **THEN** configuration is successfully parsed and no validation errors are reported

#### Scenario: Invalid YAML format
- **WHEN** user provides a config file with invalid YAML syntax
- **THEN** system reports a clear error message indicating YAML parse failure and exits gracefully

#### Scenario: Missing required field
- **WHEN** user provides a config file missing a required field (e.g., no dns.listen)
- **THEN** system reports which field is missing and exits gracefully with helpful message

### Requirement: Configuration field value validation
The system SHALL validate that critical config field values are non-empty and in valid formats before use.

#### Scenario: Valid listen address
- **WHEN** dns.listen is a valid address (e.g., "127.0.0.1:5353")
- **THEN** configuration is accepted and address can be parsed correctly

#### Scenario: Empty listen address
- **WHEN** dns.listen is empty or only whitespace
- **THEN** system reports that listen address is required and exits gracefully

#### Scenario: Valid metric address
- **WHEN** dns.metric is a valid address (e.g., ":9999")
- **THEN** configuration is accepted and address can be parsed correctly

#### Scenario: Invalid metric address
- **WHEN** dns.metric is malformed (e.g., "invalid:address:format")
- **THEN** system reports validation error and exits gracefully

### Requirement: Warmup configuration validation
The system SHALL validate that warmup config timeout values are valid and non-negative.

#### Scenario: Valid warmup timeout
- **WHEN** warmup.timeout is set to a valid duration (e.g., "5s")
- **THEN** configuration is accepted and timeout is parsed correctly

#### Scenario: Invalid warmup timeout
- **WHEN** warmup.timeout is set to invalid value (e.g., negative duration)
- **THEN** system reports validation error and sets safe default instead of crashing
