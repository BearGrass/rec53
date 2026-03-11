# Roadmap

## Version History

| Version | Date | Highlights |
|---------|------|------------|
| dev | 2026-03 | Graceful shutdown, comprehensive tests, E2E test suite |
| - | 2026-03 | IP quality tracking with prefetch |
| - | 2026-03 | Prometheus metrics integration |
| - | 2026-03 | Docker Compose deployment |

No git tags found. Version history derived from commit history.

## Current Version: dev

### Features

- Recursive DNS resolution from root servers
- UDP/TCP dual protocol support
- LRU cache with TTL-based expiration (5 min default)
- IP quality tracking for optimal upstream server selection
- IP prefetch for candidate servers
- Prometheus metrics endpoint
- Graceful shutdown with 5-second timeout
- CNAME loop detection
- EDNS0 support (4096-byte buffer)

### Known Issues

- [ ] E2E tests for `www.huawei.com` may timeout due to network issues or complex DNS infrastructure (B-004 fix implemented, logic verified by unit tests)

## Next Version: v1.0.0 (Planned)

### Planned

- [ ] DNSSEC validation
- [ ] DoT/DoH support
- [ ] Concurrent queries to multiple nameservers
- [ ] Query rate limiting

### Under Consideration

- DNS over QUIC support
- Response policy zones (RPZ)
- Custom forwarding rules
- IPv6-only operation improvements
- 不需要支持DNAME记录

## Future

### Long-term Goals

- Full DNSSEC validation chain
- High-availability clustering
- Query logging and analytics
- Web-based dashboard