## Why

Three e2e tests (`TestMalformedQueries/valid_A_query`, `TestServerUDPAndTCP/UDP_query`, `TestWarmupNSRecords`) consistently time out with i/o timeout errors or report zero successes because `monitor.Rec53Metric` is never initialized in the e2e test package, causing nil-pointer panics inside `ServeDNS` and `iterState.handle` that silently drop queries.

## What Changes

- Add a `TestMain` (or package-level `init`) to the `e2e` package that initialises `monitor.Rec53Metric` with a no-op/in-memory metric instance before any test runs.
- Ensure `monitor.Rec53Log` is also consistently set in the same setup so no logger is nil either.
- Verify all three previously-failing tests pass with `-race` and `-timeout 120s`.

## Capabilities

### New Capabilities

- `e2e-test-setup`: Centralized test initialization for the e2e package — ensures all monitor singletons (`Rec53Metric`, `Rec53Log`) are properly initialized before any test executes.

### Modified Capabilities

## Impact

- `e2e/` package only — no production code changes.
- No API or dependency changes.
- Fixes the `rec53/e2e` test suite so `go test -race ./...` passes.
