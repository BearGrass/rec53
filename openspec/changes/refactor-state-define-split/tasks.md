# Tasks: refactor-state-define-split

## Implementation Tasks

- [x] Create `server/state_shared.go` with context keys, constants, and SOA helpers
- [x] Create `server/state_init.go` with `stateInitState`
- [x] Create `server/state_cache_lookup.go` with `cacheLookupState`
- [x] Create `server/state_classify_resp.go` with `classifyRespState`
- [x] Create `server/state_extract_glue.go` with `extractGlueState`
- [x] Create `server/state_query_upstream.go` with port override hooks and `queryUpstreamState`
- [x] Create `server/state_lookup_ns_cache.go` with `lookupNSCacheState`
- [x] Create `server/state_return_resp.go` with `returnRespState`
- [x] Delete `server/state_define.go`
- [x] Run `go test -race ./...` and confirm all tests pass
