## 1. Investigation & Diagnosis

- [x] 1.1 Reproduce the crash by running `./rec53 --config ./config.yaml` with debug logging
- [x] 1.2 Add detailed debug logging at each initialization step (config load, logger init, metrics init, server init, warmup)
- [x] 1.3 Identify the exact line and component where the crash occurs
- [x] 1.4 Add panic recovery wrapper in main() to catch and log panics with full stack trace

## 2. Configuration Validation

- [x] 2.1 Implement validation function to check required fields (dns.listen, dns.metric, warmup)
- [x] 2.2 Add nil checks and empty string validation for critical config fields
- [x] 2.3 Validate port numbers are in valid range (1-65535)
- [x] 2.4 Add validation for warmup timeout (must be positive duration)
- [x] 2.5 Call validation after config load but before using any config values
- [x] 2.6 Provide clear error messages for each validation failure

## 3. Initialization Robustness

- [x] 3.1 Add error handling in logger initialization with fallback to stderr logging
- [x] 3.2 Add error handling in metrics server initialization (don't crash if port unavailable)
- [x] 3.3 Add error handling in DNS server initialization with validation
- [x] 3.4 Ensure all subsystems fail gracefully instead of panicking

## 4. Warmup Error Handling

- [x] 4.1 Review warmup routine in server/warmup.go for potential panic points
- [x] 4.2 Add panic recovery to warmup goroutines
- [x] 4.3 Add timeout context handling to warmup
- [x] 4.4 Make warmup non-blocking - server should start even if warmup fails
- [x] 4.5 Log warmup failures as warnings, not fatal errors

## 5. Testing & Verification

- [x] 5.1 Create test case for startup with valid config file
- [x] 5.2 Create test case for startup with invalid YAML format
- [x] 5.3 Create test case for startup with missing required fields
- [x] 5.4 Create test case for startup with empty listen address
- [x] 5.5 Create test case for startup with invalid metric address
- [x] 5.6 Run full test suite with race detector: `go test -race ./...`
- [x] 5.7 Manually verify startup works with config file without crashing
- [x] 5.8 Verify error messages are helpful and actionable

## 6. Code Quality & Documentation

- [x] 6.1 Run `gofmt -w .` to format all modified code
- [x] 6.2 Ensure all error messages follow project style guidelines
- [x] 6.3 Update comments in main() and config loading functions
- [x] 6.4 Verify all logs use monitor.Rec53Log with appropriate log levels
