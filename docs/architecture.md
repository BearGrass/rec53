# Architecture

English | [中文](architecture.zh.md)

`docs/architecture.md` is developer-facing. It explains how rec53 resolves DNS queries, how core modules interact, and which implementation constraints matter when you modify the code. For deployment and operator guidance, use the documents under `docs/user/`.

## Scope

rec53 is a node-local recursive DNS resolver. The intended production model is one resolver per machine or per cluster node. The project is optimized around:

- predictable recursive resolution behavior
- low operational overhead for local deployment
- conservative concurrency and cache safety
- clear failure and shutdown behavior

It is not designed as a centralized recursive DNS fleet manager.

## Repository Map

| Path | Purpose |
|---|---|
| `cmd/rec53.go` | CLI flags, config loading, logger/metric/server startup, signal handling |
| `server/` | DNS request handling, state machine, cache, IP pool, warmup, snapshot, XDP |
| `monitor/` | Logger, Prometheus metrics, pprof support |
| `utils/` | Root server data and zone helpers |
| `e2e/` | End-to-end and integration tests |
| `tools/` | Internal benchmark and validation tools |
| `docs/user/` | Operator and deployer documentation |
| `docs/dev/` | Developer workflow and release documentation |

## Request Lifecycle

At a high level, every DNS request follows this path:

```text
client query
  -> server.ServeDNS()
  -> Change(stateMachine)
  -> response classification / iterative steps
  -> response normalization
  -> optional UDP truncation
  -> metrics + write back to client
```

Entry point:

- `server.ServeDNS()` creates the reply message, validates `QDCOUNT`, records metrics, and hands the request to the state machine.
- `Change()` in `server/state_machine.go` drives the request through up to 50 iterations to prevent infinite loops.
- The final response is normalized so the original question is preserved even when CNAME chasing or internal query rewriting occurred.

## Resolution Pipeline

The state machine is the core of rec53. It encodes the resolution order explicitly:

```text
STATE_INIT
  -> HOSTS_LOOKUP
  -> FORWARD_LOOKUP
  -> CACHE_LOOKUP
  -> CLASSIFY_RESP
  -> EXTRACT_GLUE
  -> LOOKUP_NS_CACHE
  -> QUERY_UPSTREAM
  -> RETURN_RESP
```

Priority order:

1. `HOSTS_LOOKUP`
2. `FORWARD_LOOKUP`
3. `CACHE_LOOKUP`
4. iterative resolution

This ordering is intentional:

- hosts entries must short-circuit all other logic
- forwarding rules must bypass local iterative behavior for matching zones
- cache should reduce latency without changing correctness
- only unresolved misses should pay the full iterative cost

## State Responsibilities

| State | Responsibility |
|---|---|
| `STATE_INIT` | Validate request, initialize response header |
| `HOSTS_LOOKUP` | Answer static local records, including NODATA on name hit with type mismatch |
| `FORWARD_LOOKUP` | Route matching zones to configured upstreams |
| `CACHE_LOOKUP` | Read cached response copies by `fqdn:qtype` |
| `CLASSIFY_RESP` | Distinguish final answer, negative answer, CNAME, or referral |
| `EXTRACT_GLUE` | Pull A records for delegated NS hosts from the additional section |
| `LOOKUP_NS_CACHE` | Recover NS IPs from cache or fall back to roots |
| `QUERY_UPSTREAM` | Query selected nameserver IPs with happy-eyeballs style concurrency |
| `RETURN_RESP` | Restore original question and prepend any CNAME chain |

Two loops are especially important:

- Delegation loop: referral -> glue/ns resolution -> upstream query -> classify again
- CNAME loop: follow CNAME target, preserve chain, then re-enter cache/iterative path

Both are guarded:

- `MaxIterations = 50` bounds total state-machine progress
- `visitedDomains` prevents infinite CNAME cycles
- `contextKeyNSResolutionDepth` prevents recursive NS-resolution deadlock

## Cache Model

The DNS cache is global and type-aware.

- Implementation: `server/cache.go`
- Key format: `"fqdn:qtype"`
- Storage backend: `github.com/patrickmn/go-cache`

Important invariants:

- Reads return a shallow copy of `dns.Msg`
- Callers may replace or append RR slices on the returned message
- Callers must not mutate individual RR fields on cached records
- OPT records are stripped before cache write so concurrent `Pack()` calls remain safe

Behavioral rules:

- positive answers are cached by TTL
- negative responses use SOA-derived TTL, falling back to `DefaultNegativeCacheTTL`
- forwarded responses are not cached
- cached responses are copied before use so request-local changes do not corrupt shared state

## IP Pool And Upstream Selection

The IP pool tracks nameserver quality and drives upstream selection.

- Implementation: `server/ip_pool.go`, `server/ip_pool_quality_v2.go`
- Global: `globalIPPool`
- Input: root IPs, glue IPs, recursively resolved NS IPs

Key behavior:

- `GetBestIPsV2` returns the preferred and secondary upstream IP
- `queryHappyEyeballs` races the two candidates where possible
- failures call `RecordFailure`
- successful RTTs call `RecordLatency`
- a background probe loop helps degraded NS entries recover over time

This system is intentionally simple: selection is adaptive, but the code avoids policy-heavy scheduling or distributed coordination.

## Hosts And Forwarding Snapshots

Hosts and forwarding rules are compiled into an immutable snapshot and published through `atomic.Pointer`.

- Implementation: `server/state_shared.go`
- Snapshot contents: `hostsMap`, `hostsNames`, `forwardZones`

Why this exists:

- read path stays lock-free
- config consumers never observe partially updated structures
- tests can install a synthetic snapshot without rebuilding the server

## Warmup, Snapshot, And XDP

These features extend the default path but are not required for a basic deployment.

### Warmup

- `server/warmup.go`
- warms root and selected TLD NS records on startup
- runs in the background and must not block server readiness

### Cache snapshot

- `server/snapshot.go`
- persists cache entries on shutdown and restores them during startup
- should be treated as an operational optimization, not a correctness dependency

### XDP/eBPF fast path

- `server/xdp_loader.go`, `server/xdp_sync.go`, `server/xdp_metrics.go`, `server/xdp/`
- enabled only on supported Linux environments
- attaches before DNS listeners start
- degrades to the Go-only cache path if load or attach fails

XDP must remain optional. The Go path is the release baseline.

## Startup And Shutdown Constraints

The server lifecycle is centered in `server/server.go`.

Startup rules:

- build all UDP/TCP listener structs first
- initialize optional XDP before DNS listeners
- start warmup asynchronously
- publish ready addresses only after a listener actually binds

Shutdown rules:

- cancel warmup before tearing down shared services
- shut down listeners deterministically
- stop background loops before closing shared resources
- save cache snapshot only after active query processing has stopped

When modifying lifecycle code, prefer preserving these guarantees over introducing new abstractions.

## Testing Expectations

The repo relies on a mix of unit, package, and end-to-end tests.

- package-level behavior lives under `server/`, `cmd/`, `monitor/`, `utils/`
- integration tests live under `e2e/`
- run with `-race` for concurrency-sensitive changes

When touching hot-path or lifecycle logic, add or update tests that cover:

- malformed requests
- startup failure behavior
- shutdown cleanup
- cache safety invariants
- forwarding / hosts precedence

See `docs/dev/testing.md` for commands and test guidance.
