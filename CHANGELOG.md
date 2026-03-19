# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [1.0.0] - 2026-03-19

### Release Scope

- `v1.0.0` is positioned for personal users and simple IT environments: single-machine or node-local recursive DNS, explicit operator-managed deployment, and the default Go path as the production baseline.
- It is not positioned as a public open resolver, centralized recursive DNS fleet, or enterprise high-availability DNS platform.

### Changed

- Reorganized the documentation for the v1.0.0 release track: `README.md` and `README.zh.md` now act as concise entry points, user-facing docs live under `docs/user/`, and developer-facing docs live under `docs/dev/`.
- Refocused `docs/architecture.md` on developer-facing architecture, lifecycle, cache, and concurrency boundaries instead of mixing deployment guidance with implementation details.
- Expanded Prometheus documentation in `docs/metrics.md` and `docs/user/operations.md` to cover scrape setup, first-deployment verification, core PromQL checks, and metric-label stability guidance for both operators and developers.
- `server.Run()` no longer waits forever for readiness when listener startup fails before `NotifyStartedFunc` fires; startup errors now surface without blocking the caller.
- `server.Shutdown()` now cancels warmup and XDP background work before waiting on the shared wait group, preventing shutdown hangs when background goroutines depend on cancellation to exit.
- Tightened configuration validation in `cmd/rec53.go`: invalid `dns.log_level` values now fail fast, `debug.pprof_listen` is validated when pprof is enabled, and config file read errors are reported more explicitly.
- Updated startup/config tests in `server/server_test.go`, `cmd/config_validation_test.go`, and `cmd/signal_test.go` to cover the new fail-fast and non-blocking lifecycle behavior.
- Hardened `rec53ctl` lifecycle behavior for safer deployment: installs now refuse to overwrite unmanaged unit/binary targets, installed services use an explicit `LOG_FILE` path (`/var/log/rec53/rec53.log` by default), and `uninstall` preserves config/logs unless `--purge` is requested.
- Added regression tests for `rec53ctl` install/uninstall safety and for logger parent-directory creation so the default and installed log paths remain predictable.
- Replaced deep copy (`dns.Msg.Copy()`) on cache reads with a shallow copy (`shallowCopyMsg`) that shares RR slice elements. Reduces cache-read allocations from 5 allocs/op to 3 allocs/op (−40%) and latency from 234 ns to 175 ns (−25%) per `getCacheCopy` call. Overall QPS improvement: ~111K → ~119K (+7.6%).
- Strip OPT (EDNS0 pseudo-RR) from messages before caching to prevent `Pack()` side-effects (`OPT.SetExtendedRcode` mutates `OPT.Hdr.Ttl`), making shallow-copied messages safe for concurrent packing.
- Added `Cache Read Safety` section to coding conventions (`.rec53/CONVENTIONS.md`): callers of `getCacheCopy` / `getCacheCopyByType` MUST NOT modify individual RR fields on returned values; slice-level operations (append, replace, truncate) are safe.

## [0.5.0] - 2026-03-18

### Changed

- **BREAKING:** Removed `name` label (raw FQDN) from Prometheus metrics `rec53_query_counter`, `rec53_response_counter`, and `rec53_latency`. This eliminates unbounded label cardinality and reduces per-query allocation overhead on the hot path. **Migration:** remove `name` from any PromQL queries, alerting rules, or Grafana dashboards that reference these metrics. Per-domain aggregation is no longer available from metrics; use DNS query logs if needed.
- Switched metric recording from `With(prometheus.Labels{...})` to `WithLabelValues(...)`, eliminating a per-call map allocation on every DNS query.
- Switched `IPQualityV2GaugeSet` internals to `WithLabelValues` (no signature change).
- Replaced `fmt.Sprintf` in `getCacheKey` with string concatenation + `strconv.FormatUint`, reducing from 1 alloc/op (16 B) to 0 allocs/op and ~4× faster.

## [0.4.1] - 2026-03-18

### Added
- **Hosts local authority**: serve static A/AAAA/CNAME records from config with AA flag, before any cache or upstream lookup. Name + type match returns authoritative response; name match with type mismatch returns NODATA.
- **Forwarding rules**: forward queries for specific domain suffixes to designated upstream DNS servers. Longest-suffix match, sequential upstream failover, SERVFAIL on all-fail (no iterative fallback). Forwarded results are not cached.
- New `HOSTS_LOOKUP` and `FORWARD_LOOKUP` states in the state machine: `STATE_INIT → HOSTS_LOOKUP → FORWARD_LOOKUP → CACHE_LOOKUP → ... → RETURN_RESP`
- `NewServerWithFullConfig()` constructor for injecting hosts and forwarding config
- Architecture documentation (ARCHITECTURE.md)
- Test plan documentation (TEST_PLAN.md)
- Task management documentation (TODO.md)

### Changed
- Test coverage improved from ~1% to ~60%
  - monitor: 3.2% → 58.1% (metric_test.go)
  - server: 75.2% → 76.8% (state_machine_test.go, state_define_test.go)
- Add iterState unit tests with IP quality and cache operations
- Optimize warmup concurrency: dynamically calculate based on CPU cores using `min(NumCPU() * 2, 8)` formula instead of hardcoded 32, reducing CPU oversubscription on smaller machines while allowing efficient I/O-bound parallelism
- Optimize warmup TLD list: replace exhaustive TLD enumeration with a curated list of 30 high-traffic TLDs covering 85%+ of global registrations, reducing startup memory footprint by ~80-90% while maintaining coverage for the most common domains; custom TLD override via `warmup.tlds` in config.yaml

## [0.1.0] - 2026-03-04

### Added
- Basic recursive DNS resolution
- UDP/TCP support
- Cache mechanism with type-aware keys
- Prometheus metrics endpoint
- Graceful shutdown
- CNAME chain tracking with cycle detection
- Glue records handling
- IP quality evaluation and prefetch
- EDNS0 support (4096 buffer size)
- UDP response truncation (TC flag)

### Fixed
- Concurrent data race in IPQuality (atomic.Bool)
- Concurrent access in IP Pool (RWMutex)
- Prefetch goroutine lifecycle management (context + semaphore)
- Cache key collision (include query type)
- CNAME infinite loop (MaxIterations limit + visitedDomains)
- Question section mismatch on CNAME follow
- Response message deep copy to prevent cache corruption

### Changed
- Go version upgraded to 1.21+
- Log level setting fixed (zap.AtomicLevel)

[Unreleased]: https://github.com/BearGrass/rec53/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/BearGrass/rec53/releases/tag/v1.0.0
[0.5.0]: https://github.com/BearGrass/rec53/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/BearGrass/rec53/compare/v0.1.0...v0.4.1
[0.1.0]: https://github.com/BearGrass/rec53/releases/tag/v0.1.0
