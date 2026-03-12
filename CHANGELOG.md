# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- Architecture documentation (ARCHITECTURE.md)
- Test plan documentation (TEST_PLAN.md)
- Task management documentation (TODO.md)

### Changed
- Test coverage improved from ~1% to ~60%
  - monitor: 3.2% → 58.1% (metric_test.go)
  - server: 75.2% → 76.8% (state_machine_test.go, state_define_test.go)
- Add iterState unit tests with IP quality and cache operations
- Optimize warmup concurrency: dynamically calculate based on CPU cores using `min(NumCPU() * 2, 8)` formula instead of hardcoded 32, reducing CPU oversubscription on smaller machines while allowing efficient I/O-bound parallelism

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

[Unreleased]: https://github.com/username/rec53/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/username/rec53/releases/tag/v0.1.0