# [F-003/6] Background Probe Loop Implementation Plan

**Status**: Ready for Implementation  
**Target Completion**: 2026-03-12  
**Effort Estimate**: 1 day  
**Dependencies**: Phases 1-3 complete (33 tests pass)

---

## Overview

Implement background probing mechanism to enable automatic recovery of SUSPECT IPs. This step creates goroutines that periodically query SUSPECT nameservers, allowing transient faults to be identified and recovered from.

### Reference Documentation
- **IP_POOL_PHASE4_GUIDE.md:218-289** — Full implementation guide
- **IP_POOL_DESIGN.md** — Architecture & concurrency patterns
- **CONVENTIONS.md** — Go coding style, concurrency patterns

---

## Implementation Steps

### F-003/6a: Implement `StartProbeLoop()` method in server/ip_pool.go

**Purpose**: Initialize and launch background probe goroutine.

**Location**: `server/ip_pool.go` (after `Shutdown()` method)

**Signature**:
```go
func (ipp *IPPool) StartProbeLoop(ctx context.Context)
```

**Behavior**:
- Increment WaitGroup counter
- Launch `periodicProbeLoop()` as goroutine with context
- Should be idempotent (safe to call multiple times)

**Key Points**:
- Must use `ipp.wg.Add(1)` before goroutine
- Pass `ctx` to goroutine (for graceful shutdown)
- Called from `server.Run()` after server initialization

**Testing**:
- Verify goroutine is launched (check WaitGroup count)
- Verify context propagation on shutdown

---

### F-003/6b: Implement `periodicProbeLoop()` method in server/ip_pool.go

**Purpose**: Main loop for periodic IP probing with 30-second interval.

**Location**: `server/ip_pool.go` (after `StartProbeLoop()`)

**Signature**:
```go
func (ipp *IPPool) periodicProbeLoop()
```

**Behavior**:
1. Defer: `ipp.wg.Done()` to signal completion
2. Create ticker with 30-second interval
3. Loop forever:
   - Wait for either:
     - Context cancellation → exit loop
     - Ticker tick → call `probeAllSuspiciousIPs()`
4. Cleanup: Stop ticker, return

**Key Points**:
- **Interval**: 30 seconds (per Phase 4 guide, line 235)
- **Context handling**: `select` on `ipp.ctx.Done()` for shutdown
- **Logging**: Use `monitor.Rec53Log.Debugf()` for probe loop lifecycle (start, stop, errors)

**Testing**:
- Verify loop exits on context cancellation
- Verify ticker fires at expected intervals
- No goroutine leaks on shutdown

---

### F-003/6c: Implement `probeAllSuspiciousIPs()` method in server/ip_pool.go

**Purpose**: Query SUSPECT IPs to probe for recovery.

**Location**: `server/ip_pool.go` (after `periodicProbeLoop()`)

**Signature**:
```go
func (ipp *IPPool) probeAllSuspiciousIPs()
```

**Algorithm**:

1. **Find Candidates** (RLock):
   ```
   RLock poolV2
   For each IP → iqv2:
     If iqv2.ShouldProbe() == true:
       Add to candidates list
   RUnlock
   ```

2. **Probe Each Candidate** (sequential, no lock):
   ```
   For each candidate IP:
     - Create DNS query: SetQuestion(".", dns.TypeA)
     - Create client with 3-second timeout
     - Exchange(query, ip:53)
     - If error == nil:
       - iqv2 = GetIPQualityV2(ip)
       - iqv2.ResetForProbe()  ← marks IP as recovered
     - If error != nil:
       - Log probe failure (optional)
       - Leave IP in SUSPECT state (will retry in 30s)
   ```

3. **Logging**:
   - Debug: "Probing {count} SUSPECT IPs"
   - Debug: "IP {ip} recovered from SUSPECT state" (on success)
   - Error: "Probe of {ip} failed: {err}" (on failure)

**Key Points**:
- **No locking during probe**: Avoid blocking normal queries
- **Root query**: Query "." (root zone) with Type A
- **Timeout**: 3 seconds per probe (prevents slow IPs from blocking)
- **Error handling**: Failures are silent (IP stays SUSPECT, retry in 30s)
- **Metric**: Increment `rec53_ip_probes_total` counter (defer to F-003/12)

**Testing**:
- Mock IP state as SUSPECT, verify probe changes state
- Verify concurrent queries are not blocked during probes
- Verify timeout is enforced (no hung probes)

---

### F-003/6d: Integrate probe loop startup in server/server.go Run() method

**Purpose**: Integrate probe loop into server lifecycle.

**Location**: `server/server.go` in `Server.Run()` method

**Current Location** (approx): After server starts listening

**Changes**:
```go
// After s.srv.ListenAndServe() goroutine starts or similar initialization
globalIPPool.StartProbeLoop(s.srv.ctx)  // or server context
```

**Context Source**:
- Use same context as DNS server
- Verify context is properly handled on shutdown
- Should be canceled during graceful shutdown

**Key Points**:
- **Timing**: Start probe loop after server is ready to handle queries
- **Context**: Must use same context as server for synchronized shutdown
- **Error handling**: No errors expected (probe loop is fire-and-forget)

**Testing**:
- Verify probe loop starts on server startup
- Verify probe loop stops on server shutdown
- Verify probe loop doesn't interfere with normal queries

---

### F-003/6e: Write unit tests for probe loop in server/ip_pool_test.go

**Purpose**: Verify probe loop behavior and concurrency.

**Location**: `server/ip_pool_test.go` (new test section)

**Test Cases** (minimum 5):

1. **TestStartProbeLoop_LaunchesGoroutine**
   - Verify goroutine is launched when `StartProbeLoop()` called
   - Check WaitGroup count increases
   - Verify cleanup on shutdown (WaitGroup count returns to 0)

2. **TestPeriodicProbeLoop_ExitsOnContext**
   - Create context with cancel
   - Call `periodicProbeLoop()` (or start with `StartProbeLoop()`)
   - Cancel context after 100ms
   - Verify goroutine exits within 1 second
   - No goroutine leaks

3. **TestPeriodicProbeLoop_TickerInterval**
   - Mock ticker or use real ticker with 100ms interval for test
   - Verify `probeAllSuspiciousIPs()` called on each tick
   - Verify probe is called at least 2 times in 300ms window

4. **TestProbeAllSuspiciousIPs_IdentifiesCandidates**
   - Create 3 IPs with states: ACTIVE, DEGRADED, SUSPECT
   - Call `probeAllSuspiciousIPs()`
   - Verify only SUSPECT IP is probed
   - Verify ACTIVE/DEGRADED IPs are skipped

5. **TestProbeAllSuspiciousIPs_RecoveryOnSuccess**
   - Create IP in SUSPECT state
   - Mock successful DNS response
   - Call `probeAllSuspiciousIPs()`
   - Verify `ResetForProbe()` called (state changes to RECOVERED)
   - Verify successive probe attempts ShouldProbe() == false for RECOVERED IP

6. **TestProbeAllSuspiciousIPs_TimeoutHandling**
   - Create IP with slow/unresponsive mock server
   - Set probe timeout to 100ms
   - Verify probe times out without blocking
   - Verify IP remains in SUSPECT state

7. **TestProbeAllSuspiciousIPs_ConcurrencyWithQueries**
   - Start probe loop
   - Simulate normal DNS queries in parallel goroutine
   - Verify queries complete without waiting for probes
   - Verify no mutex contention (use go test -race)

---

## Implementation Checklist

- [ ] **F-003/6a**: `StartProbeLoop()` implemented (10 lines)
- [ ] **F-003/6b**: `periodicProbeLoop()` implemented (20 lines)
- [ ] **F-003/6c**: `probeAllSuspiciousIPs()` implemented (40 lines)
- [ ] **F-003/6d**: Integration in `server/server.go` (1-2 lines)
- [ ] **F-003/6e**: Tests added to `server/ip_pool_test.go` (200+ lines)
- [ ] **Verification**: `go test ./server -race -v` passes all F-003/6 tests
- [ ] **Regression**: `go test ./... -race` passes (no breakage)
- [ ] **Code Review**: Check CONVENTIONS.md for style compliance
- [ ] **Documentation**: Update TODO.md with completion status

---

## Testing Strategy

### Unit Tests
```bash
# Run only F-003/6 tests
go test -run "TestProbe|TestStartProbe" ./server -v

# Run with race detector
go test -race -run "TestProbe|TestStartProbe" ./server -v
```

### Integration Tests
```bash
# Run IP pool tests
go test ./server -race -v -run "IPPool"

# Full test suite
go test ./... -race
```

### Manual Testing (if applicable)
```bash
# Build and verify no new warnings
go build -o rec53 ./cmd

# Run resolver and check logs for probe activity
./rec53 -log-level debug
# Look for "Probing X SUSPECT IPs" in logs
```

---

## Success Criteria

- ✅ All 7 test cases pass
- ✅ `go test ./server -race` passes
- ✅ `go test ./...` passes (no regressions)
- ✅ No goroutine leaks (verified with pprof)
- ✅ Probe loop exits cleanly on shutdown
- ✅ SUSPECT IPs recover within 30-60 seconds
- ✅ Normal queries not impacted by probes
- ✅ Code follows CONVENTIONS.md style

---

## Risk Mitigation

| Risk | Mitigation |
|------|-----------|
| Probe goroutine leak | Use WaitGroup.Done() deferred, test with `-race` |
| Slow probes blocking queries | Separate goroutine, non-blocking map access (RLock only) |
| Context not propagated | Use same context as server, test context cancellation |
| Excessive probe traffic | 30-second interval, limited to SUSPECT IPs only |
| DNS query timeout issues | 3-second timeout, non-fatal on failure |

---

## References

- **Phase 4 Guide**: `.rec53/IP_POOL_PHASE4_GUIDE.md:218-289`
- **Design**: `.rec53/IP_POOL_DESIGN.md`
- **Conventions**: `.rec53/CONVENTIONS.md`
- **Code Location**: `server/ip_pool.go`, `server/server.go`, `server/ip_pool_test.go`

---

## Notes

- F-003/6 is independent and can be implemented in parallel with F-003/11, F-003/12
- Probe loop enables fault recovery for Phase 2 acceptance criteria (recovery time < 60s)
- Integration with F-003/12 (metrics) is optional but recommended for observability
- Feature flag (F-003/15) can enable/disable probe loop if needed for A/B testing

**Last Updated**: 2026-03-11  
**Ready for Implementation**: YES
