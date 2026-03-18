## Context

rec53's cache subsystem uses a double-copy discipline: both reads (`getCacheCopy`)
and writes (`setCacheCopy`) perform `dns.Msg.Copy()` — a deep copy that clones every
RR struct. This guarantees isolation: cached entries are never aliased by readers or
writers.

After v0.5.0 reduced metrics-path allocations to ~3.8%, the cache read-copy path
(`dns.Msg.Copy`) became the dominant allocation source at **31.53%** of denoised
`alloc_space` (~6.88 GB cumulative at ~111K QPS). Profile breakdown:

| Source | alloc_space % |
|--------|---------------|
| `dns.(*Msg).Copy` (deep copy) | 31.53% |
| `dns.(*A).copy` | 6.41% |
| `dns.(*Msg).CopyTo` | 23.81% (cum) |
| `dns.(*NS).copy` | 2.95% |
| `dns.(*CNAME).copy` | 2.58% |

Audit of all 3 cache read call sites shows no downstream code modifies individual
RR fields — all mutations are at the slice header level (append, nil, truncate).
The only known mutation during `Pack()` is `OPT.SetExtendedRcode()`, which modifies
`OPT.Hdr.Ttl`. Stripping OPT before caching eliminates this risk entirely.

## Goals / Non-Goals

**Goals:**

- Eliminate per-RR deep copy allocations on cache reads by sharing RR pointers
- Strip OPT records from cached messages to make `Pack()` side-effect-free
- Establish and enforce a cache safety invariant (documented + race-tested)
- Validate with the same dual-metric gate as v0.5.0: (a) dnsperf QPS/P99 must not
  regress, (b) pprof alloc_space for cache-copy path must show measurable reduction

**Non-Goals:**

- Eliminating write-side deep copy (retained for safety — callers may alias after write)
- Changing the go-cache library or cache key format
- TTL decrement on cache reads (current behavior: return stored TTL as-is)
- `sync.Pool` for `dns.Msg` (rejected in v0.4.0 — lifecycle complexity too high)
- Changing any observable behavior (metrics, config, API, response content)

## Decisions

### D1: Shallow copy on cache reads

**Choice**: Replace `msg.Copy()` in `getCacheCopy` with `shallowCopyMsg()` — a function
that allocates a new `dns.Msg` struct and new slice headers (Question, Answer, Ns, Extra)
but shares the underlying RR pointers with the cached entry.

**Alternatives considered**:

- *Zero-copy (return cached pointer directly)*: Eliminates all read-side allocation but
  requires callers to never modify even slice headers. Current code does `append(s.response.Answer, msgInCache.Answer...)`,
  which is safe with shallow copy but would corrupt the cached slice with zero-copy.
  Rejected: too fragile.
- *Reference-counted COW*: Readers get a reference; mutation triggers a copy. Over-
  engineered for this codebase — no code mutates individual RRs, so the "W" in COW
  never triggers. Rejected: unnecessary complexity.
- *Immutable wrapper type (`frozenMsg`)*: Compile-time safety against RR mutation. Would
  require changing all interfaces to accept `frozenMsg` instead of `*dns.Msg`. Rejected:
  ~120 lines of type scaffolding for a benefit achievable with documentation + race test.

**Rationale**: Shallow copy preserves the existing programming model (callers get a
`*dns.Msg` they can freely modify at the slice level) while eliminating the dominant
allocation cost. The safety invariant is enforced at runtime via `-race` testing.

### D2: OPT stripping on cache writes

**Choice**: Strip all `*dns.OPT` records from `msg.Extra` in `setCacheCopy` after the
deep copy but before storing in the cache.

**Alternatives considered**:

- *Strip on read instead of write*: Would add overhead per read and still leave OPT in
  the cached object (concurrent `Pack()` before stripping = race). Rejected.
- *Don't strip, keep deep copy for Extra only*: Partial deep copy adds complexity and
  only addresses one slice. Rejected.

**Rationale**: OPT records are EDNS0 transport-layer negotiation metadata, not DNS
answer data. They are per-query and per-hop; caching them is semantically incorrect
anyway. Stripping on write ensures the cached entry is safe for unlimited concurrent
`Pack()` calls without any per-read overhead.

### D3: Write-side deep copy retained

**Choice**: Keep `value.Copy()` in `setCacheCopy`. Do not optimize the write path.

**Rationale**: Write-side deep copy is essential for 2 of 6 call sites where the source
message is aliased into `s.response` after caching (`queryUpstreamState` lines 569, 580).
The remaining 4 sites write local/unaliased messages where copy is technically redundant,
but the savings are negligible (writes are far less frequent than reads) and the safety
benefit of unconditional write-side copy outweighs micro-optimization.

### D4: Unified `getCacheCopy` for all read sites

**Choice**: All 3 read sites (`cacheLookupState`, `lookupNSCacheState`, `resolveNSIPs`)
use the same `getCacheCopy` with shallow copy. No specialized zero-copy accessor for
`resolveNSIPs`.

**Rationale**: Shallow copy overhead is 5 small allocations (1 struct + 4 slices).
Adding a separate `getCacheRef` accessor for `resolveNSIPs` saves these 5 allocations
but introduces a second cache read pattern with different safety properties. The
maintenance cost exceeds the allocation savings.

## Risks / Trade-offs

**[Risk] Future code mutates a cached RR field** → Mitigation: Block comment in
`cache.go` documents the invariant; `TestCacheConcurrentReadPack` with `-race` catches
any violation. `AGENTS.md` or `.rec53/CONVENTIONS.md` should note: "never modify
individual RR fields from cache-read values."

**[Risk] `miekg/dns` adds new `Pack()` mutations in future versions** → Mitigation:
Currently pinned at v1.1.52. Any upgrade should re-audit `Pack()` for RR mutations.
The `TestCacheConcurrentReadPack` race test will catch regressions.

**[Risk] OPT stripping changes response content** → Mitigation: OPT records in cache
are semantically incorrect (per-query EDNS0 metadata should not be cached). Stripping
them improves correctness. Cached entries typically have 0 or 1 OPT records; other
Extra RRs (glue A/AAAA records for NS delegation) are preserved.

**[Risk] Shallow copy allows callers to corrupt cached slices via `append` beyond
capacity** → Mitigation: `shallowCopyMsg` creates new slice headers with `make([]T, len)`
+ `copy()`. The new slice has `cap == len`, so any `append` by the caller triggers a
new backing array. The cached slice is not affected.

## Open Questions

_(none — all decisions agreed during brainstorming)_
