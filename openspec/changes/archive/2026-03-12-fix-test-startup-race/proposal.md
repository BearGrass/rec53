# Proposal: Fix Data Race in Test Server Startup Synchronization

## Problem

`go test -race` reports DATA RACE warnings in the e2e and server test infrastructure. The root cause is `time.Sleep`-based startup synchronization: a goroutine calls `dns.Server.ListenAndServe()` which sets `PacketConn` internally, while the main goroutine reads `PacketConn` after a fixed sleep. This is a classic TOCTOU race — the sleep is not a memory barrier.

Affected locations:
1. `e2e/helpers.go` — `NewMockAuthorityServer` (line 57)
2. `e2e/helpers.go` — `NewTestResolver` (line 367)
3. `e2e/helpers.go` — `NewMultiZoneMockServer` (line 492)
4. `e2e/helpers.go` — `setupResolverWithMockRoot` (line 759, reads from `srv.UDPAddr()`)
5. `server/server.go` — `Run()` / `UDPAddr()` (lines 166–168, 241)

## Solution

Use `dns.Server.NotifyStartedFunc` (available in miekg/dns v1.1.52) instead of `time.Sleep`. This callback is invoked by `ListenAndServe` after the socket is bound and `PacketConn` is set, providing a proper happens-before relationship.

**Fix pattern:**
```go
started := make(chan struct{})
srv.NotifyStartedFunc = func() { close(started) }
go func() { srv.ListenAndServe() }()
<-started  // blocks until PacketConn is fully set — no race
addr = srv.PacketConn.LocalAddr().String()
```

For `server.Run()` / `server.UDPAddr()`: add a `started chan struct{}` field to `server`, close it in `NotifyStartedFunc`, and block in a new `WaitUntilReady()` method (called by tests instead of `time.Sleep`).

## Scope

- No behavioral changes to production DNS resolution
- Only test helpers and server startup synchronization are affected
- Eliminates all `time.Sleep` startup delays in tests (faster + correct)
