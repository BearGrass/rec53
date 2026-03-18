## Context

rec53 currently creates a single UDP + TCP listener pair in `server.Run()`. Under cache-hit load, throughput saturates at ~91-95K QPS on a 4C8T machine regardless of goroutine concurrency. pprof confirms the bottleneck is `syscall.Syscall6` at ~25% CPU — kernel-level serialisation on a single socket's `recvfrom`/`sendto` calls.

`miekg/dns` v1.1.52 supports `SO_REUSEPORT` via the `dns.Server.ReusePort` field. On Linux, this sets `SO_REUSEPORT` on the socket, allowing multiple sockets to bind the same `addr:port`. The kernel distributes incoming packets across sockets using a consistent hash on the source 4-tuple, giving each listener its own receive queue.

The `server` struct currently holds `udpSrv *dns.Server` and `tcpSrv *dns.Server` (single pointers), with `NotifyStartedFunc` closures that close `udpReady`/`tcpReady` channels. `Shutdown()` calls `ShutdownContext()` on both servers.

## Goals / Non-Goals

**Goals:**

- Break the single-socket throughput ceiling by running N UDP+TCP listener pairs with `SO_REUSEPORT`
- Make the listener count configurable via `dns.listeners` in `config.yaml`
- Preserve exact current behaviour when `listeners` is 0 or 1 (no `SO_REUSEPORT`, single pair)
- Maintain all existing test compatibility without changes
- Verify gains with dnsperf benchmark comparison (listeners=1 vs listeners=4)

**Non-Goals:**

- Per-listener metrics (all listeners share the same `ServeDNS` handler and global metrics)
- Listener affinity or CPU pinning (`GOMAXPROCS` / `runtime.LockOSThread`)
- Sharded or per-listener caches (shared `globalDnsCache` with existing RWMutex is retained)
- `recvmmsg`/`sendmmsg` batch syscall optimisation (requires bypassing `miekg/dns` Server layer)
- Windows support (`SO_REUSEPORT` is Unix-only; `miekg/dns` silently ignores the flag on unsupported platforms)

## Decisions

### 1. Slice of servers vs single server with N goroutines

**Decision**: Replace `udpSrv/tcpSrv *dns.Server` with `udpSrvs/tcpSrvs []*dns.Server`.

**Rationale**: Each `dns.Server` with `ReusePort: true` calls `listenUDP()` / `listenTCP()` which creates its own socket with `SO_REUSEPORT`. The kernel requires N distinct sockets to distribute packets. A single `dns.Server` with multiple goroutines still reads from one socket.

**Alternative considered**: Manually creating N `net.PacketConn` via `net.ListenConfig{Control: reuseportControl}` and passing them via `dns.Server.PacketConn`. This bypasses `ListenAndServe()` and requires managing listener lifecycle manually. Rejected — `miekg/dns` already handles this cleanly with the `ReusePort` field.

### 2. Ready-channel semantics with multiple listeners

**Decision**: Use `sync.Once` to close `udpReady`/`tcpReady` channels when the **first** listener of each type binds successfully.

**Rationale**: `Run()` currently blocks on `<-s.udpReady; <-s.tcpReady` before returning. With N listeners, waiting for **all** N to bind would delay startup. The first successful bind proves the address is available and the server is accepting packets. Address capture (`s.udpAddr`/`s.tcpAddr`) happens in the first `NotifyStartedFunc` that fires.

**Alternative considered**: Wait for all N listeners. Rejected — adds startup latency proportional to N with no benefit; if one listener fails the error channel reports it.

### 3. Default listeners value

**Decision**: `listeners: 0` (or 1) means single listener pair without `SO_REUSEPORT`. This is the zero-config default and matches current behaviour exactly.

**Rationale**: `SO_REUSEPORT` changes kernel packet distribution semantics. Users who haven't benchmarked should not get surprising behaviour changes. The feature is opt-in.

### 4. Constructor signature change

**Decision**: Add `listeners int` parameter to `NewServerWithFullConfig()` only. `NewServer()` and `NewServerWithConfig()` default to `listeners: 1`.

**Rationale**: Only the production entry point (`cmd/rec53.go`) needs the config-driven listener count. Test constructors (`NewServer()`) retain zero-change compatibility.

## Risks / Trade-offs

**[Risk] `SO_REUSEPORT` kernel version requirement** → Linux 3.9+ (2013). All modern distributions satisfy this. `miekg/dns` silently falls back to normal binding on unsupported kernels.

**[Risk] `sync.Once` ready channel — partial startup failure** → If the first listener binds but subsequent listeners fail (e.g., ephemeral port exhaustion), `Run()` returns success while errors are sent to `errChan`. Mitigation: callers already monitor `errChan` for runtime errors; no change needed.

**[Risk] RWMutex contention on `globalDnsCache` under higher QPS** → More listeners means more concurrent `ServeDNS` goroutines, increasing read-lock contention. At 200K QPS the atomic counter in `sync.RWMutex` may become a measurable bottleneck. Mitigation: monitor via pprof after deployment; cache sharding is a future option if contention is confirmed.

**[Trade-off] No per-listener metrics** → All listeners share one `ServeDNS` handler. Cannot distinguish which listener served a query. Acceptable: the kernel's distribution is opaque anyway, and per-listener metrics add label cardinality cost.
