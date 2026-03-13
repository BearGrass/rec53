# Design: Split state_define.go by State

## Decision

Split `server/state_define.go` (996 lines) into 8 files by moving each state struct and its methods to a dedicated file. All files share `package server`; imports are distributed per-file.

## File Boundaries

### `state_shared.go` (new)
Shared types and helpers used by multiple states:
- `contextKeyType` type declaration
- `contextKeyWarmupDeadline` and `contextKeyNSResolutionDepth` constants
- `DefaultNegativeCacheTTL` constant
- `extractSOAFromAuthority()` function
- `hasSOAInAuthority()` function

Imports: `github.com/miekg/dns`

### `state_init.go` (new)
State `STATE_INIT`:
- `stateInitState` struct
- `newStateInitState()`, `newStateInitStateWithContext()`
- Interface methods: `getCurrentState()`, `getRequest()`, `getResponse()`, `getContext()`
- `handle()` method

Imports: `context`, `fmt`, `github.com/miekg/dns`, `rec53/monitor`

### `state_cache_lookup.go` (new)
State `CACHE_LOOKUP`:
- `cacheLookupState` struct + constructors + interface + `handle()`

Imports: `context`, `fmt`, `github.com/miekg/dns`, `rec53/monitor`

### `state_classify_resp.go` (new)
State `CLASSIFY_RESP`:
- `classifyRespState` struct + constructors + interface + `handle()`

Imports: `context`, `fmt`, `github.com/miekg/dns`, `rec53/monitor`

### `state_extract_glue.go` (new)
State `EXTRACT_GLUE`:
- `extractGlueState` struct + constructors + interface + `handle()`

Imports: `context`, `fmt`, `github.com/miekg/dns`

### `state_query_upstream.go` (new)
Port override test hooks + State `QUERY_UPSTREAM`:
- `iterPortOverride`, `SetIterPort()`, `ResetIterPort()`, `getIterPort()`
- `queryUpstreamState` struct + constructors + interface + `handle()`
- Helper functions: `getIPListFromResponse()`, `getNSNamesFromResponse()`, `resolveNSIPs()`, `nsResult`, `resolveNSIPsRecursively()`, `resolveNSIPsConcurrently()`, `updateNSIPsCache()`, `getBestAddressAndPrefetchIPs()`

Imports: `context`, `fmt`, `net`, `sync`, `time`, `github.com/miekg/dns`, `rec53/monitor`

### `state_lookup_ns_cache.go` (new)
State `LOOKUP_NS_CACHE`:
- `lookupNSCacheState` struct + constructors + interface + `handle()`

Imports: `context`, `fmt`, `github.com/miekg/dns`, `rec53/monitor`, `rec53/utils`

### `state_return_resp.go` (new)
State `RETURN_RESP`:
- `returnRespState` struct + constructors + interface + `handle()`

Imports: `context`, `fmt`, `github.com/miekg/dns`

## Deletion

`server/state_define.go` is deleted after all content is moved.

## Verification

`go test -race ./...` must pass without changes to any test file.
