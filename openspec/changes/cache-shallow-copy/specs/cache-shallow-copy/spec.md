## ADDED Requirements

### Requirement: Shallow copy on cache read
`getCacheCopy` and `getCacheCopyByType` SHALL return a shallow copy of the cached
`*dns.Msg` — a new struct with new slice headers (`Question`, `Answer`, `Ns`, `Extra`)
that share the same underlying RR pointers as the cached entry. Individual RR structs
SHALL NOT be deep-copied.

#### Scenario: Shallow copy returns independent slice headers
- **WHEN** `getCacheCopyByType("example.com.", dns.TypeA)` returns a cached message with 3 Answer RRs
- **THEN** the returned message's `Answer` slice has length 3
- **AND** modifying the returned `Answer` slice (append, truncate, nil) SHALL NOT affect the cached entry's `Answer` slice
- **AND** the individual RR pointers (`Answer[0]`, `Answer[1]`, `Answer[2]`) SHALL be identical (same pointer) to the cached entry's RR pointers

#### Scenario: Shallow copy preserves all message fields
- **WHEN** a cached message has `MsgHdr.Id=1234`, `Compress=true`, 2 Answer RRs, 3 Ns RRs, 1 Extra RR, and 1 Question
- **THEN** the shallow copy SHALL have the same `MsgHdr` values, `Compress=true`, and the same RR counts in each section

#### Scenario: Shallow copy under concurrent access
- **WHEN** 100 goroutines concurrently read the same cache key via `getCacheCopyByType`
- **AND** each goroutine appends the returned RRs to a separate response and calls `Pack()`
- **THEN** no data race SHALL be detected (under `-race`)
- **AND** each goroutine's `Pack()` output SHALL be a valid DNS wire-format message

### Requirement: OPT record stripping on cache write
`setCacheCopy` and `setCacheCopyByType` SHALL remove all `*dns.OPT` records from
`msg.Extra` before storing the entry in the cache. Non-OPT records in `Extra` (e.g.,
glue A/AAAA records) SHALL be preserved.

#### Scenario: OPT record removed from cached entry
- **WHEN** `setCacheCopyByType("example.com.", dns.TypeA, msg, 300)` is called with a message
  whose `Extra` contains `[*dns.A{glue}, *dns.OPT{edns0}]`
- **THEN** the cached entry's `Extra` SHALL contain only `[*dns.A{glue}]`
- **AND** the caller's original message SHALL still have both records in `Extra` (unmodified)

#### Scenario: No OPT record present
- **WHEN** `setCacheCopy` is called with a message whose `Extra` contains no `*dns.OPT` records
- **THEN** the cached entry's `Extra` SHALL be identical to the original (no records removed)

#### Scenario: Multiple OPT records stripped
- **WHEN** `setCacheCopy` is called with a message whose `Extra` contains 2 `*dns.OPT` records
  and 1 `*dns.AAAA` record
- **THEN** the cached entry's `Extra` SHALL contain only the `*dns.AAAA` record

### Requirement: Cache safety invariant
Cached `*dns.Msg` entries SHALL be treated as immutable after storage. The following
invariants SHALL hold:

1. Write-side deep copy: `setCacheCopy` SHALL `value.Copy()` before storing, ensuring
   the cached entry is independent from the caller's message.
2. OPT stripping: No `*dns.OPT` records SHALL exist in cached entries.
3. Read-side shallow copy: Readers get new slice headers but shared RR pointers.
4. No RR mutation: No code path SHALL modify individual RR struct fields (e.g.,
   `rr.Header().Ttl`, `a.A`) on a message obtained from `getCacheCopy` or
   `getCacheCopyByType`.

#### Scenario: Race test validates invariant
- **WHEN** a race test (`-race`) runs with N concurrent readers and 1 writer on the same cache key
- **THEN** zero data races SHALL be detected

#### Scenario: Pack() is side-effect-free on cached messages
- **WHEN** a cached message (with OPT stripped) is shallow-copied and `Pack()` is called
- **THEN** no field of any RR in the cached entry SHALL be modified by `Pack()`

### Requirement: Write-side deep copy retained
`setCacheCopy` and `setCacheCopyByType` SHALL continue to perform `value.Copy()` (deep
copy) on the input message before stripping OPT and storing. The write-side copy
discipline SHALL NOT be relaxed.

#### Scenario: Writer mutation does not affect cache
- **WHEN** `setCacheCopyByType("test.", dns.TypeA, msg, 300)` is called
- **AND** the caller subsequently modifies `msg.Answer[0].(*dns.A).A` to a different IP
- **THEN** the cached entry's Answer RR SHALL still contain the original IP

### Requirement: Functional equivalence
The observable behavior of cache reads SHALL be identical to the deep-copy baseline.
Specifically:
- All RR data (Name, Type, Class, TTL, Rdata) in the returned message SHALL be
  byte-identical to what `msg.Copy()` would have returned.
- `Pack()` on the returned message SHALL produce the same wire-format output as
  `Pack()` on a deep-copied message (modulo OPT records, which are stripped).

#### Scenario: Wire-format equivalence
- **WHEN** the same cache entry is read twice — once via shallow copy, once via deep copy
- **THEN** `Pack()` on both SHALL produce identical wire-format output

#### Scenario: Benchmark does not regress
- **WHEN** `BenchmarkCacheGetHit` is run with `-benchmem`
- **THEN** `allocs/op` SHALL be less than or equal to the deep-copy baseline
- **AND** `ns/op` SHALL not regress by more than 10%
