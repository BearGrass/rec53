## Context

The rec53 DNS resolver crashes on startup when launched with a config file (`./rec53 --config ./config.yaml`). The initialization sequence involves:
1. Loading YAML configuration from file
2. Overriding config with command-line flags
3. Initializing the logger
4. Initializing the metrics server
5. Creating and starting the DNS server (which includes warmup if enabled)

The crash could occur at any of these points, with potential causes:
- Nil pointer dereferences in config structures
- Missing or invalid YAML fields in config file
- Unhandled errors during server startup
- Panic in warmup routine without recovery
- Uninitialized dependencies passed to subsystems

## Goals / Non-Goals

**Goals:**
- Identify the exact crash location and root cause
- Implement defensive initialization with proper nil checks
- Add validation of critical config fields before use
- Implement panic recovery at initialization boundaries
- Add diagnostic logging at each initialization step
- Ensure graceful failure with helpful error messages

**Non-Goals:**
- Do not change the configuration file format
- Do not modify public APIs of server or monitor packages
- Do not add new configuration options (beyond validation)

## Decisions

1. **Defensive Initialization Strategy**
   - Add nil checks before dereferencing config structures
   - Validate that critical fields (listen address, metric address) are non-empty
   - Check that warmup config is properly initialized before passing to server
   - Rationale: Prevents nil pointer panics from invalid config states

2. **Error Handling Wrapper**
   - Create a recovery mechanism in main() to catch and log panics
   - Add detailed debug logging at each initialization step
   - Rationale: Makes crash diagnosis easier and prevents silent failures

3. **Warmup Robustness**
   - Add timeout and context handling to warmup routine
   - Implement panic recovery within warmup goroutines
   - Rationale: Warmup is a known heavy operation that could panic; should not crash main process

4. **Configuration Validation**
   - Validate port numbers are in valid ranges
   - Check that listen addresses can be parsed
   - Rationale: Early validation catches config errors before they propagate

## Risks / Trade-offs

- **Risk**: Adding panic recovery may hide bugs that should fail fast
  - Mitigation: Log all panics with full stack trace for debugging; fail gracefully rather than silently

- **Risk**: Extensive nil checks could add noise to code
  - Mitigation: Consolidate into a validation function; keep main logic clean

- **Risk**: Warmup failures might leave the server in inconsistent state
  - Mitigation: Make warmup non-blocking; server can operate without it

## Migration Plan

This is a bug fix with no breaking changes. Deployment is straightforward:
1. Build new binary with fixes
2. Replace old binary
3. Run with same config and flags as before
4. If crash still occurs, new logging will help diagnose root cause
