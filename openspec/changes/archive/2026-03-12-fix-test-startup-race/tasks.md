# Tasks: fix-test-startup-race

## Implementation Tasks

- [x] **T1**: Add `udpReady chan struct{}` field to `server` struct in `server/server.go`
- [x] **T2**: In `server.Run()`, initialize `s.udpReady` and set `s.udpSrv.NotifyStartedFunc = func() { close(s.udpReady) }`
- [x] **T3**: Add `WaitUntilReady()` method to `server` in `server/server.go`
- [x] **T4**: Fix `NewMockAuthorityServer` in `e2e/helpers.go` — replace `time.Sleep` with `NotifyStartedFunc` + channel
- [x] **T5**: Fix `NewTestResolver` in `e2e/helpers.go` — replace `time.Sleep` with `NotifyStartedFunc` + channel
- [x] **T6**: Fix `NewMultiZoneMockServer` in `e2e/helpers.go` — replace `time.Sleep` with `NotifyStartedFunc` + channel
- [x] **T7**: Fix `setupResolverWithMockRoot` in `e2e/helpers.go` — replace `time.Sleep(100ms)` with `srv.WaitUntilReady()`
- [x] **T8**: Remove unused `"time"` import from `e2e/helpers.go` if no longer needed
- [x] **T9**: Run `go test -race ./...` and verify no DATA RACE warnings
