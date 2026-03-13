# Proposal: Refactor state_define.go — Split by State

## Problem

`server/state_define.go` is 996 lines containing 7 distinct state structs plus shared helpers all in one file. This makes it hard to navigate, review, or modify a single state without scrolling through unrelated code.

## Proposed Solution

Split `server/state_define.go` into multiple focused files — one per state — plus a shared helpers file. Delete the original monolithic file.

**File mapping:**

| File | Contents |
|------|----------|
| `state_shared.go` | `contextKeyType`, context key constants, `DefaultNegativeCacheTTL`, `extractSOAFromAuthority`, `hasSOAInAuthority` |
| `state_init.go` | `stateInitState` (lines 59–119) |
| `state_cache_lookup.go` | `cacheLookupState` (lines 121–182) |
| `state_classify_resp.go` | `classifyRespState` (lines 184–300) |
| `state_extract_glue.go` | `extractGlueState` (lines 302–367) |
| `state_query_upstream.go` | `iterPortOverride` helpers (lines 369–390) + `queryUpstreamState` (lines 392–875) |
| `state_lookup_ns_cache.go` | `lookupNSCacheState` (lines 877–943) |
| `state_return_resp.go` | `returnRespState` (lines 945–996) |

## Constraints

- **Zero logic changes** — pure file reorganisation
- All existing tests pass without modification (`go test -race ./...`)
- Same package (`package server`)
- Imports redistributed to each file as needed
