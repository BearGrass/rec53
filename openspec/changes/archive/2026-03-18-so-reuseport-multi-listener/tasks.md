## 1. Server struct and constructors

- [x] 1.1 Add `listeners int` field to `server` struct in `server/server.go`
- [x] 1.2 Replace `udpSrv *dns.Server` / `tcpSrv *dns.Server` with `udpSrvs []*dns.Server` / `tcpSrvs []*dns.Server`
- [x] 1.3 Add `listeners int` parameter to `NewServerWithFullConfig()` constructor; normalize 0 → 1
- [x] 1.4 Set `listeners: 1` in `NewServer()` and `NewServerWithConfig()` (test-compat defaults)

## 2. Run() multi-listener startup

- [x] 2.1 Create N `dns.Server` instances per protocol with `ReusePort: n > 1` in `Run()`
- [x] 2.2 Implement `sync.Once`-based `NotifyStartedFunc` to close `udpReady`/`tcpReady` exactly once from the first listener to bind
- [x] 2.3 Capture `udpAddr`/`tcpAddr` from the first listener that fires `NotifyStartedFunc`
- [x] 2.4 Start 2×N goroutines (N UDP + N TCP) with indexed error reporting (`"udp listener[%d]"`)
- [x] 2.5 Size `errChan` to `2*n` capacity
- [x] 2.6 Log listener count and SO_REUSEPORT status after ready channels are closed

## 3. Shutdown() multi-listener teardown

- [x] 3.1 Loop `ShutdownContext()` over all entries in `udpSrvs` and `tcpSrvs` slices
- [x] 3.2 Verify `wg.Wait()` blocks until all 2×N goroutines exit
- [x] 3.3 Retain existing warmup cancel → IP pool shutdown → snapshot save order

## 4. Configuration

- [x] 4.1 Add `Listeners int` field with `yaml:"listeners"` tag to `DNSConfig` in `cmd/rec53.go`
- [x] 4.2 Add validation in `validateConfig`: reject `Listeners < 0`
- [x] 4.3 Pass `cfg.DNS.Listeners` to `NewServerWithFullConfig()` call
- [x] 4.4 Add commented `listeners:` field to `config.yaml` with usage explanation
- [x] 4.5 Add commented `listeners:` field to `generate-config.sh` template

## 5. Verification

- [x] 5.1 `go build ./...` passes
- [x] 5.2 `go vet ./...` passes
- [x] 5.3 `go test -race ./...` passes (all existing tests unchanged)
- [x] 5.4 Manual smoke test: start rec53 with `listeners: 4`, verify `dig` queries succeed
- [x] 5.5 dnsperf benchmark: compare listeners=1 vs listeners=4 (c=64, -d 10s), record QPS and latency

## 6. Documentation

- [x] 6.1 Update `README.md` — add `dns.listeners` config explanation and SO_REUSEPORT section
- [x] 6.2 Update `README.zh.md` — mirror the same changes
- [x] 6.3 Update `docs/benchmarks.md` — add multi-listener benchmark comparison data
- [x] 6.4 Update `docs/architecture.md` — mention multi-listener in server description
