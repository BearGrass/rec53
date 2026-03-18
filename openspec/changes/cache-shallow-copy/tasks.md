## 1. OPT stripping on cache write

- [ ] 1.1 `server/cache.go` — Add `stripOPT(msg *dns.Msg)` function that removes all `*dns.OPT` records from `msg.Extra` while preserving non-OPT records
- [ ] 1.2 `server/cache.go` — Modify `setCacheCopy` to call `stripOPT` on the deep copy before passing to `setCache`
- [ ] 1.3 `server/cache_test.go` — Add `TestStripOPT` covering: OPT removed, non-OPT preserved, no-OPT no-op, multiple OPT stripped

## 2. Shallow copy on cache read

- [ ] 2.1 `server/cache.go` — Add `shallowCopyMsg(m *dns.Msg) *dns.Msg` that allocates a new `dns.Msg` with copied slice headers but shared RR pointers
- [ ] 2.2 `server/cache.go` — Modify `getCacheCopy` to call `shallowCopyMsg` instead of `msg.Copy()`
- [ ] 2.3 `server/cache_test.go` — Add `TestShallowCopyMsg` verifying: independent slice headers, shared RR pointers (same address), all fields preserved
- [ ] 2.4 `server/cache_test.go` — Add `TestShallowCopySliceIsolation` verifying: append/nil on returned slice does not affect cached entry

## 3. Concurrency safety tests

- [ ] 3.1 `server/cache_test.go` — Add `TestCacheConcurrentReadPack`: 100 goroutines read same key, each appends RRs to a response, calls `Pack()`, verify no race (run with `-race`)
- [ ] 3.2 `server/cache_test.go` — Add `TestCacheConcurrentReadWrite`: N readers + 1 writer on same key, verify no race and readers always get valid messages

## 4. Functional equivalence tests

- [ ] 4.1 `server/cache_test.go` — Add `TestShallowVsDeepCopyWireFormat`: compare `Pack()` output of shallow copy vs deep copy for representative messages (A, NS delegation, CNAME chain, NXDOMAIN)
- [ ] 4.2 `server/cache_test.go` — Add `TestWriterMutationDoesNotAffectCache`: write entry, mutate caller's message, verify cached entry unchanged

## 5. Safety invariant documentation

- [ ] 5.1 `server/cache.go` — Add block comment above `getCacheCopy` documenting the cache safety invariant (OPT stripped, shared RR pointers, no RR mutation)
- [ ] 5.2 `.rec53/CONVENTIONS.md` — Add "Cache Read Safety" section: never modify individual RR fields from cache-read values

## 6. Build & test verification

- [ ] 6.1 `go build ./...` — confirm compilation
- [ ] 6.2 `go vet ./...` — no warnings
- [ ] 6.3 `go test -race -timeout 120s ./... -count=1` — all tests pass with race detector

## 7. Benchmarks

- [ ] 7.1 `server/cache_bench_test.go` — Add `BenchmarkShallowVsDeepCopy` comparing allocs/op and ns/op
- [ ] 7.2 Run `BenchmarkCacheGetHit` with `-benchmem -count=5` — record before/after; verify allocs/op decreased

## 8. Re-profile validation (dual-metric gate)

- [ ] 8.1 Run `tools/dnsperf -c 64 -d 20s` × 3 runs; record median QPS and P99 (must not regress vs v0.5.0 baseline ~111K QPS)
- [ ] 8.2 Run pprof `alloc_space` with denoised focus during load; record cache-copy-path alloc_space % (must show measurable reduction vs v0.5.0 baseline ~31.53%)
- [ ] 8.3 Update `docs/benchmarks.md` with before/after comparison table
