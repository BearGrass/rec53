## ADDED Requirements

### Requirement: Network-bound e2e tests are skippable in short mode
All e2e tests that access real internet DNS servers SHALL be skippable by running `go test -short`. Tests that require network access MUST begin with `if testing.Short() { t.Skip(...) }`.

#### Scenario: Warmup tests skipped with -short flag
- **WHEN** the test suite is run with `go test -short ./e2e/...`
- **THEN** all 5 warmup integration tests are skipped and no real DNS queries are sent to root servers

#### Scenario: Warmup tests run without -short flag
- **WHEN** the test suite is run without the `-short` flag
- **THEN** all warmup tests execute normally and query real root DNS servers

#### Scenario: Non-network tests still run in short mode
- **WHEN** the test suite is run with `go test -short ./e2e/...`
- **THEN** mock-based tests (authority, glue recursion, IP pool, mock server) still execute

### Requirement: Test goroutine channels do not deadlock
Channels used to communicate between test goroutines and the main test goroutine SHALL be buffered sufficiently to prevent send-blocking when the receiver is not ready.

#### Scenario: Concurrent IP selection test completes without deadlock
- **WHEN** `TestIPPoolV2_ConcurrentSelection` runs 10 goroutines each performing 100 GetBestIPsV2 calls
- **THEN** all goroutines complete and the test finishes without hanging

#### Scenario: Error reporting channels accept all possible errors
- **WHEN** every concurrent goroutine in the test encounters an error
- **THEN** all errors are buffered and reported after goroutines finish, without any goroutine blocking on channel send
