## Context

The e2e test suite covers the full DNS resolution stack. Two categories of tests cause resource exhaustion:

1. **Warmup integration tests** (`e2e/warmup_test.go`): 5 test functions unconditionally send DNS queries to real internet root servers. With up to 32 concurrent goroutines and 60-second timeouts, running `go test ./e2e/...` can saturate the machine's network sockets and CPU, causing SSH sessions to drop and the server to become unresponsive.

2. **IP pool concurrency test** (`e2e/ippool_v2_test.go`, `TestIPPoolV2_ConcurrentSelection`): Uses two unbuffered channels (`done` and `errorChan`). Ten goroutines write `true` to `done` and may write to `errorChan` on error. The main goroutine drains `done` first with 10 sequential `<-done` receives, then reads `errorChan`. If a goroutine tries to send on `errorChan` while blocked waiting for `done` to be consumed, a deadlock occurs (unbuffered channel send blocks until the receiver is ready, but the receiver is stuck draining `done` first).

## Goals / Non-Goals

**Goals:**
- Make `go test -short ./e2e/...` safe to run on any machine without network access or resource risk
- Eliminate the deadlock in `TestIPPoolV2_ConcurrentSelection`
- Match the existing `testing.Short()` convention already used in all other e2e network-bound tests

**Non-Goals:**
- Rewriting warmup tests to use mock servers (a larger effort; the tests remain valid as real integration tests when run without `-short`)
- Changing production behavior
- Adding new test coverage

## Decisions

### Decision 1: `testing.Short()` guard for warmup tests

**Choice**: Add `if testing.Short() { t.Skip(...) }` at the top of each warmup test function.

**Rationale**: This is the established convention in the codebase — all other network-bound e2e tests (`resolver_test.go`, `cache_test.go`, `server_test.go`, `error_test.go`) already use this pattern. It requires minimal code change, preserves the tests as real integration tests when run without `-short`, and is immediately understood by any Go developer.

**Alternative considered**: Convert warmup tests to use `MultiZoneMockServer` instead of real root servers. This would make them run-anywhere tests but is a significantly larger change and reduces coverage of the real warmup path.

### Decision 2: Buffered channels to fix `TestIPPoolV2_ConcurrentSelection` deadlock

**Choice**: Replace `make(chan error)` with `make(chan error, 10*100)` (maximum possible errors = 10 goroutines × 100 iterations) and `make(chan bool)` with `make(chan bool, 10)`.

**Rationale**: The deadlock arises because a goroutine attempting to send on an unbuffered `errorChan` blocks if the main goroutine isn't concurrently receiving. With a fully-buffered channel, sends never block regardless of receiver state. Using capacity `10*100` is conservative (most iterations succeed) but guarantees no blocking. Making `done` buffered (`cap=10`) is also prudent to avoid a symmetric issue if the main goroutine's `<-done` loop were ever reordered.

**Alternative considered**: Restructure the goroutine logic to never send errors to a separate channel (inline `t.Errorf`). This is cleaner but requires using `t.Helper()` inside goroutines, which is not idiomatic and has subtle failure-reporting issues across goroutine boundaries.

## Risks / Trade-offs

- [Risk] Buffered `errorChan` means errors are collected silently until after all goroutines finish, rather than failing fast → Mitigation: The test's purpose is a final pass/fail check, not per-iteration interruption; delayed error reporting is acceptable here.
- [Risk] `-short` flag skips warmup real-world coverage in local dev → Mitigation: CI can run without `-short` on a machine with network access; developer machines use `-short` for fast iteration.

## Migration Plan

1. Modify `e2e/warmup_test.go`: add Short guard to 5 functions
2. Modify `e2e/ippool_v2_test.go`: replace unbuffered channels in `TestIPPoolV2_ConcurrentSelection`
3. Verify: `go test -short ./e2e/...` completes cleanly
4. Verify: `go test -race ./e2e/...` (with `-short`) shows no race or deadlock
5. No rollback complexity — changes are confined to test files
