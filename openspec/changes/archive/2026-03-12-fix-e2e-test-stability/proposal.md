## Why

Running `go test ./e2e/...` causes the server to become unresponsive (high CPU/network load, SSH disconnect) because warmup tests access real internet root DNS servers without any `testing.Short()` guard, and a concurrent IP pool test has a potential deadlock due to unbuffered channels. These issues make the test suite unsafe to run on shared or resource-constrained machines.

## What Changes

- Add `testing.Short()` skip guards to all 5 warmup integration tests in `e2e/warmup_test.go` so they are excluded when running `go test -short ./e2e/...`
- Fix potential deadlock in `e2e/ippool_v2_test.go`: replace unbuffered `errorChan` and `done` channel with buffered alternatives to prevent goroutine send-blocking when the main goroutine is waiting on `<-done`

## Capabilities

### New Capabilities

- `e2e-test-safety`: Requirements for e2e tests to be runnable safely in resource-constrained environments — all network-bound tests must be skippable via `-short` flag, and test goroutine channels must not deadlock

### Modified Capabilities

<!-- No existing spec-level capability requirements are changing -->

## Impact

- `e2e/warmup_test.go`: 5 test functions modified (Short guard added)
- `e2e/ippool_v2_test.go`: `TestIPPoolV2_ConcurrentSelection` modified (buffered channels)
- No production code changes
- No API or behavior changes
- `go test -short ./e2e/...` becomes safe to run without network access or resource risk
