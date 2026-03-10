# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- Architecture documentation (ARCHITECTURE.md)
- Test plan documentation (TEST_PLAN.md)
- Task management documentation (TODO.md)

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