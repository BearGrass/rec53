## Why

rec53 v0.5.0 reduced metrics-path allocations from ~24% to ~3.8% of denoised
`alloc_space`. With metrics optimized, `dns.Msg.Copy` in the cache read path is
now the dominant allocation source at **31.53%** (6.88 GB cumulative during sustained
~111K QPS load). Every cache hit performs a full deep copy — cloning every RR struct —
even though no downstream code modifies individual RR fields. This is the next
high-value optimization target identified by the v0.5.0 dual-metric validation.

## What Changes

- Replace `msg.Copy()` (deep copy) in `getCacheCopy` with a **shallow copy** that
  copies slice headers but shares RR pointers — eliminates per-RR allocation on every
  cache read.
- Strip **OPT records** from `dns.Msg.Extra` before storing in cache — removes the
  only known `Pack()`-induced mutation (`SetExtendedRcode` modifying `OPT.Hdr.Ttl`),
  making shared RR pointers safe for concurrent `Pack()` calls.
- Add a **cache safety invariant** (documented + enforced by race test) ensuring cached
  RRs are never mutated after storage.

## Capabilities

### New Capabilities
- `cache-shallow-copy`: Shallow copy cache read path with OPT stripping and safety invariant.

### Modified Capabilities
<!-- None — this is a pure internal optimization. Cache key format, external API,
     metrics labels, and all observable behavior remain unchanged. -->

## Impact

- **`server/cache.go`**: New `shallowCopyMsg` and `stripOPT` functions; modified
  `getCacheCopy` and `setCacheCopy`.
- **`server/cache_test.go`**: New race test, OPT strip test, shallow copy correctness test.
- **`server/cache_bench_test.go`**: Updated `BenchmarkCacheGetHit`; new
  `BenchmarkShallowVsDeepCopy`.
- **No breaking changes**: No API, metrics, config, or behavioral changes.
- **Dependencies**: None added or removed.
