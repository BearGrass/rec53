# Spec: e2e-test-safety — Race-Free Server Startup

## Capability

End-to-end and integration tests must not exhibit data races when starting mock DNS servers or the rec53 resolver. Startup synchronization must use proper memory-ordering primitives rather than fixed-duration sleeps.

## Requirements

### REQ-1: MockAuthorityServer startup synchronization
`NewMockAuthorityServer` must use `dns.Server.NotifyStartedFunc` + channel to wait for `PacketConn` to be set before reading the address. No `time.Sleep`.

### REQ-2: NewTestResolver startup synchronization  
`NewTestResolver` must use `dns.Server.NotifyStartedFunc` + channel. No `time.Sleep`.

### REQ-3: NewMultiZoneMockServer startup synchronization
`NewMultiZoneMockServer` must use `dns.Server.NotifyStartedFunc` + channel. No `time.Sleep`.

### REQ-4: server.Run() / server.UDPAddr() race elimination
`server.Run()` must set `NotifyStartedFunc` on `udpSrv` and close a `udpReady` channel. `UDPAddr()` may be called safely only after `WaitUntilReady()`.

### REQ-5: WaitUntilReady API
`server` must expose `WaitUntilReady()` which blocks until UDP server has started. `setupResolverWithMockRoot` must call this instead of `time.Sleep(100ms)`.

### REQ-6: No behavioral regression
All existing e2e and unit tests must pass with `-race` flag. DNS resolution behavior is unchanged.

## Acceptance Criteria

- `go test -race ./...` completes without DATA RACE warnings related to `PacketConn` reads
- `go test ./...` passes (no functional regressions)
- No `time.Sleep` calls remain in server startup synchronization paths
