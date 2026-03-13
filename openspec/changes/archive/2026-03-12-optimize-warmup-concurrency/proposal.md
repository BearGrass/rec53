## Why

The current warmup process uses a fixed concurrency of 32 goroutines, which causes excessive CPU usage and system freezing on machines with fewer CPU cores (e.g., 4 cores). The hardcoded concurrency creates an 8x oversubscription on typical development machines, leading to context switching overhead and cache pollution. By dynamically calculating concurrency based on available CPU cores, we can achieve optimal performance across different hardware configurations while maintaining system responsiveness.

## What Changes

- Dynamic concurrency calculation based on available CPU cores at runtime
- Formula: `Concurrency = min(runtime.NumCPU() * 2, 8)` allows I/O-bound DNS queries to benefit from 2x CPU core ratio while capping at 8 for safety
- Default warmup configuration uses computed value instead of hardcoded 32
- Configuration remains overridable via `config.yaml` for special deployments
- No changes to warmup query logic or DNS resolution behavior

## Capabilities

### New Capabilities
- `adaptive-warmup-concurrency`: Dynamic warmup concurrency calculation based on system CPU capacity to prevent resource exhaustion across heterogeneous deployments

### Modified Capabilities
- `dns-warmup`: Warmup now respects system CPU constraints to prevent excessive context switching and maintain system responsiveness

## Impact

- **Configuration**: Default concurrency changes from hardcoded 32 to dynamically computed value based on CPU cores
- **Code**: `server/warmup_defaults.go` modified to compute concurrency at init time using `runtime.NumCPU()`
- **User Experience**: Better responsiveness during startup on low-core systems; no breaking changes to public API
- **Backward Compatibility**: Existing `config.yaml` files continue to work; users can explicitly override concurrency if needed
