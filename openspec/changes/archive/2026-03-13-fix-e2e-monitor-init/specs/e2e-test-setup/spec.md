## ADDED Requirements

### Requirement: E2E package initializes monitor singletons before any test
The e2e test package SHALL initialize `monitor.Rec53Metric` and `monitor.Rec53Log` to non-nil values before any test function executes, using a `TestMain` entry point.

#### Scenario: Rec53Metric is non-nil when ServeDNS is called
- **WHEN** any e2e test starts a DNS server and sends a query
- **THEN** `monitor.Rec53Metric.InCounterAdd` SHALL execute without panicking

#### Scenario: Rec53Log is non-nil when server logs
- **WHEN** any e2e test runs server code that calls `monitor.Rec53Log.Debugf` or similar
- **THEN** the log call SHALL execute without panicking

#### Scenario: TestMain delegates to m.Run
- **WHEN** the e2e `TestMain` completes initialization
- **THEN** it SHALL call `m.Run()` and pass through the exit code so all tests execute normally

### Requirement: Previously-failing tests pass after setup fix
After the monitor singletons are properly initialized, the three previously-failing tests SHALL pass when network is available.

#### Scenario: TestMalformedQueries valid_A_query receives a response
- **WHEN** `TestMalformedQueries/valid_A_query` sends a valid A record query to the local server
- **THEN** the server SHALL return a DNS response (not time out) and no error SHALL be returned from `client.Exchange`

#### Scenario: TestWarmupNSRecords reports at least one success
- **WHEN** `TestWarmupNSRecords` runs `server.WarmupNSRecords` for 4 domains
- **THEN** `stats.Succeeded` SHALL be greater than 0 and the test SHALL not report all queries as failures
