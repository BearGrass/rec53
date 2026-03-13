## 1. Investigate monitor package initialization API

- [x] 1.1 Read `monitor/var.go` to confirm the declaration of `Rec53Metric` and `Rec53Log`
- [x] 1.2 Read `monitor/metric.go` (or equivalent) to understand what `InitMetric()` / `NewMetric()` does and whether it binds an HTTP listener
- [x] 1.3 Confirm whether calling `InitMetric()` multiple times causes a Prometheus duplicate-registration panic (check for `sync.Once` or `MustRegister`)

## 2. Create centralized e2e test setup

- [x] 2.1 Create `e2e/main_test.go` with a `TestMain(m *testing.M)` function
- [x] 2.2 Inside `TestMain`, initialize `monitor.Rec53Log` to `zap.NewNop().Sugar()` (replaces scattered per-file `init()` calls)
- [x] 2.3 Inside `TestMain`, initialize `monitor.Rec53Metric` using the appropriate constructor (no HTTP listener needed for tests)
- [x] 2.4 Call `os.Exit(m.Run())` at the end of `TestMain` to ensure proper exit code propagation

## 3. Clean up per-file init() duplication

- [x] 3.1 Remove or guard the `Rec53Log` assignments in individual e2e `init()` functions that are now redundant (e2e/error_test.go, e2e/server_test.go, etc.) — only if `TestMain` fully covers them

## 4. Verify fixes

- [x] 4.1 Run `go test -race -v -run TestMalformedQueries ./e2e/...` — confirm `valid_A_query` passes
- [x] 4.2 Run `go test -race -v -run TestServerUDPAndTCP ./e2e/...` — confirm `UDP_query` no longer times out (may still need network for `RcodeSuccess`)
- [x] 4.3 Run `go test -race -v -run TestWarmupNSRecords ./e2e/...` — confirm `stats.Succeeded > 0`
- [x] 4.4 Run the full suite `go test -race -timeout 120s ./...` — confirm no regressions
