## Why

Concurrency scaling benchmarks (`docs/benchmarks.md`) prove that rec53 throughput saturates at ~91-95K QPS regardless of goroutine concurrency (c=32 through c=256). pprof confirms `syscall.Syscall6` (`recvfrom`/`sendto`) at ~25% CPU — the bottleneck is the single UDP/TCP socket's kernel-level read/write serialisation, not application logic. Increasing CPU frequency yields only ~15-25% improvement; adding cores has no effect under the current single-socket architecture.

`SO_REUSEPORT` allows multiple sockets to bind the same address, with the kernel distributing incoming packets across them. `miekg/dns` v1.1.52 already supports this natively via `dns.Server.ReusePort`. This is the lowest-cost path to breaking the single-socket ceiling.

## What Changes

- The `server` struct changes from holding a single UDP+TCP listener pair to holding N pairs, configurable via `dns.listeners` in `config.yaml`.
- `Run()` creates N `dns.Server` instances per protocol with `ReusePort: true` (when N > 1), each getting its own kernel receive queue.
- `Shutdown()` gracefully stops all N listener pairs.
- `DNSConfig` gains a `Listeners int` field; default 0/1 preserves current single-listener behaviour (no `SO_REUSEPORT`).
- Config validation rejects negative values.
- Startup log includes listener count when SO_REUSEPORT is active.

## Capabilities

### New Capabilities

- `reuseport-listener`: SO_REUSEPORT multi-listener support — configurable N UDP+TCP listener pairs on the same address, kernel-level packet distribution, graceful shutdown of all pairs.

### Modified Capabilities

- `dns-server-startup`: `Run()` now starts N listener pairs instead of 1; `Shutdown()` stops all N; ready-channel semantics change to `sync.Once` (first listener to bind signals readiness).

## Impact

- **Code**: `server/server.go` (struct, `Run()`, `Shutdown()`, constructors), `cmd/rec53.go` (`DNSConfig`, constructor call, log)
- **Config**: `config.yaml` and `generate-config.sh` gain `listeners` field
- **Dependencies**: None — uses existing `miekg/dns` v1.1.52 `ReusePort` support
- **Tests**: No changes — `NewServer()` defaults to `listeners=1`, existing tests unaffected
- **Docs**: `README.md`, `README.zh.md`, `docs/benchmarks.md`, `docs/architecture.md`
