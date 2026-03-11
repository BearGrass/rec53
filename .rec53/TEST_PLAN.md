# Test Plan

## Overview

- Coverage baseline: 28.6% - 82.6% (varies by package)
- Coverage target: 80%
- Last updated: 2026-03-11

### Current Coverage by Package

| Package | Coverage | Status |
|---------|----------|--------|
| utils | 82.6% | ✅ Target met |
| server | 69.0% | ⚠️ Below target |
| monitor | 58.1% | ❌ Below target |
| cmd | 47.1% | ❌ Below target |
| e2e | 28.6% | ❌ Integration tests |

## Batch Schedule

Tests are organized by dependency order:
1. Foundation/Utility Layer - No dependencies
2. Core Logic Layer - Depends on foundation
3. Interface/Handler Layer - Depends on core
4. Integration - Full stack tests

### Batch 1: Foundation/Utility Layer

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| utils/zone.go | utils/zone_test.go | Zone parsing, edge cases | ✅ Done |
| utils/root.go | utils/root_test.go | Root glue generation | ✅ Done |

### Batch 2: Core Logic Layer

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| server/cache.go | server/cache_test.go | Cache set/get, TTL expiration, type keys | ✅ Done |
| server/ip_pool.go | server/ip_pool_test.go | IP quality, best selection, prefetch | ✅ Done |
| server/state_define.go | server/state_define_test.go | State constants | ✅ Done |
| server/state_machine.go | server/state_machine_test.go | State transitions, CNAME loop | ⚠️ Needs more tests |
| server/state.go | - | State handlers | ❌ Not started |

### Batch 3: Interface/Handler Layer

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| server/server.go | server/server_test.go | UDP/TCP handling, truncation | ✅ Done |
| monitor/metric.go | monitor/metric_test.go | Counter, histogram operations | ✅ Done |
| monitor/log.go | monitor/log_test.go | Log levels, initialization | ✅ Done |
| cmd/loglevel.go | cmd/log_level_test.go | Level parsing | ✅ Done |
| cmd/rec53.go | cmd/signal_test.go | Signal handling | ✅ Done |

### Batch 4: Integration

| Source file | Test file | Key test points | Status |
|-------------|-----------|-----------------|--------|
| e2e/helpers.go | - | Mock server utilities | ✅ Done |
| - | e2e/resolver_test.go | Full resolution flow | ⚠️ Flaky |
| - | e2e/cache_test.go | Cache behavior | ⚠️ Flaky |
| - | e2e/server_test.go | Server lifecycle | ✅ Done |
| - | e2e/error_test.go | Error handling | ✅ Done |

## Notes

- E2E tests require network access and may be flaky
- Consider adding `-short` flag to skip integration tests in CI
- Use mock servers for deterministic testing