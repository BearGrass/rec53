## 1. Add Short Guards to Warmup Tests

- [x] 1.1 Add `testing.Short()` skip guard to `TestWarmupNSRecords` in `e2e/warmup_test.go`
- [x] 1.2 Add `testing.Short()` skip guard to `TestWarmupNSRecords_Concurrency` in `e2e/warmup_test.go`
- [x] 1.3 Add `testing.Short()` skip guard to `TestWarmupNSRecords_Timeout` in `e2e/warmup_test.go`
- [x] 1.4 Add `testing.Short()` skip guard to `TestWarmupNSRecords_LargeTLDList` in `e2e/warmup_test.go`
- [x] 1.5 Add `testing.Short()` skip guard to `TestWarmupStats` in `e2e/warmup_test.go`

## 2. Fix Deadlock in IP Pool Concurrency Test

- [x] 2.1 Replace unbuffered `errorChan` with buffered `make(chan error, 10*100)` in `TestIPPoolV2_ConcurrentSelection` in `e2e/ippool_v2_test.go`
- [x] 2.2 Replace unbuffered `done` channel with buffered `make(chan bool, 10)` in `TestIPPoolV2_ConcurrentSelection` in `e2e/ippool_v2_test.go`

## 3. Verification

- [x] 3.1 Run `go test -short ./e2e/...` and confirm all 5 warmup tests are skipped and no panics/hangs occur
- [x] 3.2 Run `go test -race -short ./e2e/...` and confirm no race conditions or deadlocks are detected
- [x] 3.3 Run `go test -v -run TestIPPoolV2_ConcurrentSelection ./e2e/...` and confirm it completes successfully
