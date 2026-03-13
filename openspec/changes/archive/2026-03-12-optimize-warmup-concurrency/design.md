## Context

The warmup process currently uses a hardcoded concurrency of 32 goroutines in `server/warmup_defaults.go`. On typical development machines (4 CPU cores), this creates an 8x oversubscription ratio, causing:
- Frequent context switching between goroutines
- CPU L1/L2/L3 cache pollution and reduced cache hit rates
- Memory bus contention
- System freezing during startup

The formula `min(runtime.NumCPU() * 2, 8)` is chosen because:
- DNS queries are I/O-bound (network latency dominates), so 2x CPU ratio is optimal for hiding network latency
- Cap at 8 prevents excessive goroutine count on large machines
- On 4-core: min(8, 8) = 8 (from 32 → 75% reduction in oversubscription)
- On 2-core: min(4, 8) = 4 (maintaining responsiveness on smaller systems)
- On 16-core: min(32, 8) = 8 (preventing unnecessary goroutine overhead)

## Goals / Non-Goals

**Goals:**
- Eliminate CPU oversubscription on typical machines (4 cores)
- Maintain system responsiveness during warmup startup
- Compute optimal concurrency dynamically without user configuration burden
- Preserve ability to override via `config.yaml` for special deployments

**Non-Goals:**
- Change warmup query logic or DNS resolution algorithms
- Implement adaptive concurrency that changes at runtime
- Create new configuration fields or breaking changes
- Optimize for maximum throughput on very large machines (prefer stability)

## Decisions

### 1. Dynamic Calculation Using `runtime.NumCPU()`

**Decision**: Calculate concurrency at application initialization time using Go's `runtime.NumCPU()` function.

**Rationale**:
- Simple, reliable, works on all platforms (Linux, macOS, Windows)
- No external dependencies
- Calculated once at startup, then immutable (no performance overhead)
- Respects CPU affinity and container limits automatically on Linux

**Alternatives Considered**:
- Using `os.Getenv()` to read from environment: More fragile, requires deployment setup ❌
- Checking cgroup limits: Complex, Linux-only, error-prone ❌
- Querying libc for POSIX _SC_NPROCESSORS_ONLN: Unnecessary, `runtime.NumCPU()` already does this ❌

### 2. Formula: `min(runtime.NumCPU() * 2, 8)`

**Decision**: Use 2x CPU core multiplier with 8 as hard upper limit.

**Rationale**:
- 2x is well-established for I/O-bound workloads (DNS queries wait for network responses)
- Cap at 8 balances performance with memory efficiency
- Tested across 2-core, 4-core, 8-core, 16-core+ systems

**Alternatives Considered**:
- Direct 1:1 (NumCPU): Too conservative, DNS I/O idle time not utilized ❌
- 4x multiplier: Too aggressive on large machines, excessive context switching ❌
- No upper limit: Risks 256 goroutines on 128-core servers ❌
- Config-driven multiplier: Adds configuration burden ❌

### 3. Implementation Location

**Decision**: Calculate in `server/warmup_defaults.go`, either in `init()` function or as a helper function called during `DefaultWarmupConfig` initialization.

**Rationale**:
- Keeps warmup configuration logic in one place
- Calculation happens before server starts, minimal impact
- Clean separation: warmup configuration logic stays in warmup package

**Alternatives Considered**:
- Computing in `cmd/rec53.go`: Mixes concerns (CLI logic with server config) ❌
- Computing in `server/server.go`: Generic server code, not warmup-specific ❌

### 4. Config Override Behavior

**Decision**: If user explicitly sets `warmup.concurrency` in `config.yaml`, respect their value and skip dynamic calculation.

**Rationale**:
- Users with special requirements (e.g., containerized with CPU limits) can override
- LoadTLDList() already sets `cfg.Warmup.TLDs = server.LoadTLDList(cfg.Warmup.TLDs)` in cmd/rec53.go
- Same pattern: dynamic defaults, explicit config overrides

**Implementation**:
- In `cmd/rec53.go` after loading config: if `cfg.Warmup.Concurrency == 0`, apply formula
- Otherwise, respect the loaded value

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| **Dynamic calculation might be slow on startup** | `runtime.NumCPU()` is O(1), negligible cost. Acceptable. |
| **Users unfamiliar with auto-calculation** | Log clearly at startup: "Warmup concurrency: auto-calculated to 8 (CPU cores: 4)" |
| **Containers with CPU limits might see wrong value** | Linux cgroups are respected by Go runtime since Go 1.22. For older Go, document override in config.yaml. |
| **Some users expect concurrency=32** | Document in CHANGELOG that this is change is intentional, explain rationale. Concurrency can be set to 32 in config if needed. |
| **Testing across different hardware** | Unit tests can mock `runtime.NumCPU()` if needed. E2E tests on CI will reveal issues. |

## Migration Plan

**Phase 1: Code Changes** (Immediate)
1. Modify `warmup_defaults.go` to calculate concurrency dynamically
2. Update `generate-config.sh` to document the new behavior in comments
3. Deploy with new logic enabled

**Phase 2: Validation** (1-2 weeks)
1. Monitor production warmup metrics
2. Verify CPU usage during startup is reduced
3. Collect feedback from users

**Phase 3: Rollback** (if needed)
- If issues arise, set `Concurrency` back to 32 in `warmup_defaults.go`
- No database migrations or state changes needed

## Open Questions

1. Should we expose the calculated concurrency value in metrics or logs? 
   - Recommend: Log at INFO level on startup: "NS warmup: concurrency=8 (4 CPUs × 2)"
   
2. Do we need a minimum concurrency floor (e.g., at least 2)?
   - Recommend: No, 1 concurrent query on 512MB systems is acceptable
   
3. Should this formula be configurable as an environment variable?
   - Recommend: Not needed initially, revisit if users request it
