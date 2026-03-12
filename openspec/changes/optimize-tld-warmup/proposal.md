## Why

Current warmup process loads thousands of TLDs, causing server crashes and excessive memory consumption during startup. This prevents rec53 from operating reliably in resource-constrained environments. By focusing on 30 curated TLDs representing 85%+ of global domain registrations, we can eliminate startup crashes while maintaining coverage for the most critical TLD spaces.

## What Changes

- Replace dynamic TLD discovery with a curated static list of 30 high-value TLDs
- Reduce warmup memory footprint by 80-90%, enabling reliable startup in resource-constrained environments
- Simplify TLD maintenance by explicitly managing a known-good set rather than attempting comprehensive coverage
- Reduce warmup time proportionally to fewer TLDs being probed
- Maintain coverage for .com, .cn, .net, .org and other major registries representing the vast majority of real-world DNS queries

## Capabilities

### New Capabilities
- `curated-tld-list`: Configuration and management of the optimized 30-TLD list for warmup operations
- `warmup-memory-optimization`: Memory-efficient warmup process that prevents server crashes on startup

### Modified Capabilities
- `dns-warmup`: Existing warmup process now uses curated TLD list instead of comprehensive TLD enumeration

## Impact

- **Configuration**: New `config.yaml` field to specify curated TLD list (or use built-in defaults)
- **Server startup**: Warmup phase will complete faster with lower memory usage
- **Code changes**: `server/warmup.go`, TLD configuration loading logic
- **Testing**: E2E tests need updated expectations for fewer TLDs
- **No breaking changes to public API**: External behavior unchanged; internal optimization only
