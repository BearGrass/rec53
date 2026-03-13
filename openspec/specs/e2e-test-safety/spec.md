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

### REQ-4: server.Run() startup synchronization
`server.Run()` must set `NotifyStartedFunc` on both `udpSrv` and `tcpSrv`. Each callback stores the bound address into `udpAddr`/`tcpAddr` fields and closes the corresponding ready channel. `Run()` blocks until both channels are closed before returning, ensuring `UDPAddr()` and `TCPAddr()` are safe to call immediately after `Run()` returns.

### REQ-5: Race-free address accessors
`UDPAddr()` and `TCPAddr()` must return pre-stored string fields (set before the ready channels are closed), not read `PacketConn`/`Listener` directly. This avoids any concurrent read of fields still being written by the DNS library goroutines.

### REQ-6: No behavioral regression
All existing e2e and unit tests must pass with `-race` flag. DNS resolution behavior is unchanged.

### REQ-7: Network-bound e2e tests are skippable in short mode
All e2e tests that access real internet DNS servers must be skippable by running `go test -short`. Tests that require network access must begin with `if testing.Short() { t.Skip(...) }`.

- When run with `-short`, all warmup integration tests are skipped and no real DNS queries are sent to root servers.
- When run without `-short`, all warmup tests execute normally.
- Mock-based tests (authority, glue recursion, IP pool, mock server) still execute in short mode.

### REQ-8: Test goroutine channels do not deadlock
Channels used to communicate between test goroutines and the main test goroutine must be buffered sufficiently to prevent send-blocking when the receiver is not ready.

- `TestIPPoolV2_ConcurrentSelection` must complete without hanging when running 10 goroutines × 100 calls each.
- Error-reporting channels must accept all possible errors without any goroutine blocking on send.

## Acceptance Criteria

- `go test -race ./...` completes without DATA RACE warnings related to `PacketConn` or `Listener` reads
- `go test ./...` passes (no functional regressions)
- No `time.Sleep` calls remain in server startup synchronization paths
- `server.Run()` blocks until both UDP and TCP servers are ready before returning
