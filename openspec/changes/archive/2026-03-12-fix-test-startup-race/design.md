# Design: Fix Data Race in Test Server Startup Synchronization

## Architecture

### Root Cause Analysis

`dns.Server.ListenAndServe()` (miekg/dns) binds the socket and sets `PacketConn` inside the goroutine. The existing code uses `time.Sleep(50ms)` and then reads `PacketConn` from the caller goroutine. The Go memory model does not guarantee that the sleep creates a happens-before relationship with the goroutine's writes — hence the race detector fires.

### Available Mechanism

`dns.Server` (miekg/dns v1.1.52) exposes:
```go
// NotifyStartedFunc is called once the server has started listening.
NotifyStartedFunc func()
```
This is called *inside* `ListenAndServe` after `PacketConn` is set, providing the required happens-before guarantee when paired with a channel close.

### Fix for e2e/helpers.go (3 locations)

Replace the `time.Sleep` + `PacketConn` read pattern with:

```go
started := make(chan struct{})
m.server.NotifyStartedFunc = func() { close(started) }
go func() {
    if err := m.server.ListenAndServe(); err != nil {
        t.Logf("server stopped: %v", err)
    }
}()
<-started
m.addr = m.server.PacketConn.LocalAddr().String()
```

Applies to: `NewMockAuthorityServer`, `NewTestResolver`, `NewMultiZoneMockServer`.

### Fix for server/server.go

Add a `udpReady chan struct{}` field to `server`. Set `NotifyStartedFunc` on `udpSrv` to close it. Expose a `WaitUntilReady()` method.

```go
type server struct {
    // ... existing fields ...
    udpReady chan struct{}  // closed when UDP server is ready
}

func (s *server) Run() <-chan error {
    s.udpReady = make(chan struct{})
    s.udpSrv = &dns.Server{Addr: s.listen, Net: "udp", Handler: s}
    s.udpSrv.NotifyStartedFunc = func() { close(s.udpReady) }
    // ... start goroutines ...
}

// WaitUntilReady blocks until the UDP server has started listening.
// Call this in tests instead of time.Sleep after Run().
func (s *server) WaitUntilReady() {
    <-s.udpReady
}
```

In `setupResolverWithMockRoot` (`e2e/helpers.go`), replace `time.Sleep(100ms)` with `srv.WaitUntilReady()`.

### UDPAddr() Safety

After `WaitUntilReady()` returns, `PacketConn` is guaranteed to be set (happens-before via channel), so `UDPAddr()` is safe to call without additional synchronization.

## No Behavioral Changes

- Production code path is identical: `NotifyStartedFunc` is only called once, and the channel is only used for startup synchronization in tests
- `WaitUntilReady()` is only called from test helpers
- All existing test logic remains the same
