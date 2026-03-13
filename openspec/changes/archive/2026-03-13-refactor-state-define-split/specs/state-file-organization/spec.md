## ADDED Requirements

### Requirement: State structs are organized one-per-file
The server package SHALL organize each DNS resolution state struct in its own dedicated file. Shared helpers used by multiple states SHALL reside in `state_shared.go`. The monolithic `state_define.go` SHALL NOT exist.

#### Scenario: Each state has its own file
- **WHEN** a developer navigates to the `server/` package
- **THEN** each state struct (`stateInitState`, `cacheLookupState`, `classifyRespState`, `extractGlueState`, `queryUpstreamState`, `lookupNSCacheState`, `returnRespState`) SHALL be found in its own dedicated file (`state_init.go`, `state_cache_lookup.go`, `state_classify_resp.go`, `state_extract_glue.go`, `state_query_upstream.go`, `state_lookup_ns_cache.go`, `state_return_resp.go`)

#### Scenario: Shared helpers are in state_shared.go
- **WHEN** a developer looks for shared context keys, SOA helpers, or shared constants
- **THEN** `contextKeyType`, context key constants, `DefaultNegativeCacheTTL`, `extractSOAFromAuthority`, and `hasSOAInAuthority` SHALL be found in `server/state_shared.go`

#### Scenario: Monolithic file does not exist
- **WHEN** the `server/` package is compiled
- **THEN** `server/state_define.go` SHALL NOT exist; all code previously in that file SHALL be distributed across the per-state files and `state_shared.go`

#### Scenario: No logic changes
- **WHEN** `go test -race ./...` is run after the file split
- **THEN** all tests SHALL pass without modification to any test file
