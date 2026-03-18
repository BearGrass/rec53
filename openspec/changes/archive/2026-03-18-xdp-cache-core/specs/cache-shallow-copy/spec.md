## MODIFIED Requirements

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
5. XDP sync: When XDP is enabled, `setCacheCopy` and `setCacheCopyByType` SHALL, after storing the entry in Go cache, synchronously write a pre-serialized copy to the BPF cache_map. The XDP sync step SHALL use the already-copied (post-OPT-strip) message for `Pack()`, NOT the caller's original message. XDP sync failure SHALL NOT affect Go cache write success.

#### Scenario: Race test validates invariant
- **WHEN** a race test (`-race`) runs with N concurrent readers and 1 writer on the same cache key
- **THEN** zero data races SHALL be detected

#### Scenario: Pack() is side-effect-free on cached messages
- **WHEN** a cached message (with OPT stripped) is shallow-copied and `Pack()` is called
- **THEN** no field of any RR in the cached entry SHALL be modified by `Pack()`

#### Scenario: XDP sync uses post-copy message
- **WHEN** `setCacheCopyByType` is called with XDP enabled
- **THEN** the message passed to `dns.Msg.Pack()` for BPF map SHALL be the deep-copied, OPT-stripped version
- **AND** the original caller's message SHALL NOT be packed or modified

#### Scenario: XDP sync failure does not block Go cache
- **WHEN** `setCacheCopy` is called and BPF map update returns `-ENOSPC`
- **THEN** Go cache write SHALL still succeed
- **AND** error SHALL be logged at Debug level
