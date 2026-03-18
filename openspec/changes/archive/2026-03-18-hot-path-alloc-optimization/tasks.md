## 1. Metrics label removal & WithLabelValues

- [x] 1.1 `monitor/var.go` — Remove `"name"` from label vectors of `InCounter`, `OutCounter`, `LatencyHistogramObserver`
- [x] 1.2 `monitor/metric.go` — Remove `name string` param from `InCounterAdd`, `OutCounterAdd`, `LatencyHistogramObserve`; switch bodies to `WithLabelValues(...)`
- [x] 1.3 `monitor/metric.go` — Switch `IPQualityV2GaugeSet` to `WithLabelValues` (signature unchanged, only internal map removal)
- [x] 1.4 `monitor/metric_test.go` — Update all assertions: remove `name` label references, adjust call sites to new signatures
- [x] 1.5 `monitor/metric_bench_test.go` — Update benchmark call sites to new signatures (drop `name` arg)

## 2. Server call-site updates

- [x] 2.1 `server/server.go` — Update `ServeDNS` call sites: remove `r.Question[0].Name` argument from `InCounterAdd`, `OutCounterAdd`, `LatencyHistogramObserve`
- [x] 2.2 `server/state_query_upstream.go` — Update forwarding metrics call sites: remove domain name argument from `InCounterAdd`, `OutCounterAdd`
- [x] 2.3 Grep all `server/` files for remaining `InCounterAdd\|OutCounterAdd\|LatencyHistogramObserve` calls to ensure none still pass `name`

## 3. getCacheKey optimization

- [x] 3.1 `server/cache.go` — Replace `fmt.Sprintf("%s:%d", name, qtype)` with `name + ":" + strconv.FormatUint(uint64(qtype), 10)`; remove `"fmt"` import if no longer needed
- [x] 3.2 `server/cache_test.go` or inline — Add/update test verifying `getCacheKey` output format matches expected `"domain.:qtype"` pattern for representative inputs
- [x] 3.3 Add `BenchmarkCacheKey` (if not already present); record before/after allocs/op and ns/op (must not regress)

## 4. Build & test verification

- [x] 4.1 `go build ./...` — confirm compilation
- [x] 4.2 `go vet ./...` — no warnings
- [x] 4.3 `go test -race -timeout 120s ./... -count=1` — all tests pass with race detector
- [x] 4.4 `go test -bench 'BenchmarkCacheGetHit|BenchmarkStateMachineCacheHit|BenchmarkCacheKey' -benchmem ./server/...` — record allocs/op baseline comparison

## 5. Documentation updates

- [x] 5.1 `docs/metrics.md` — Remove `name` from label tables and PromQL examples
- [x] 5.2 `README.md` — Add prominent breaking change note about `name` label removal (version header + migration guidance: "remove `name` from PromQL queries")
- [x] 5.3 `README.zh.md` — Mirror the same breaking change note in Chinese
- [x] 5.4 `CHANGELOG.md` — Add v0.5.0 entry with **BREAKING** tag documenting `name` label removal and migration steps (create file if not present)
- [x] 5.5 `docs/benchmarks.md` — Add before/after pprof comparison section for v0.5.0

## 6. Re-profile validation (dual-metric gate)

- [x] 6.1 Run `tools/dnsperf -c 64 -d 20s` × 3 runs; record median QPS and P99 (must not regress vs v0.4.1 baseline ~97K QPS)
- [x] 6.2 Run pprof `alloc_space` with denoised focus during load; record metrics-path alloc_space % (must show measurable reduction vs v0.4.1 baseline ~24%)
- [x] 6.3 Update `docs/benchmarks.md` with dual-metric comparison table (QPS/P99 + alloc_space before/after) and commands used
- [x] 6.4 Evaluate whether cache COW follow-up is warranted (gate: >20% denoised alloc_space from `dns.Msg.Copy`)
