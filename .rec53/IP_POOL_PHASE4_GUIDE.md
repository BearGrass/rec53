# IP Pool Phase 4: Integration & Migration Guide

**Status**: Ready to implement (Phases 1-3 complete, 33 tests pass)  
**Estimated Effort**: 1 week (4-5 days)  
**Target Completion**: TBD

---

## Overview

Phase 4 integrates the new IPQualityV2 algorithm into the DNS resolution flow and adds observability. The core V2 algorithm is complete and tested; Phase 4 focuses on:

1. **Integration** (F-003/11): Replace `getBestIPs()` calls with `GetBestIPsV2()`
2. **Metrics** (F-003/12): Export P50/P95/P99 latency to Prometheus
3. **Testing** (F-003/13-14): Benchmarks and E2E integration tests
4. **Probe Loop** (F-003/6): Background IP recovery mechanism
5. **Feature Flag** (F-003/15): A/B testing support

---

## F-003/11: Migration to GetBestIPsV2

### Overview
Replace the old V1 `getBestIPs()` logic with the new V2 algorithm in `state_define.go`.

### Implementation Steps

**1. Update state_define.go**
- **Location**: `server/state_define.go:329`
- **Current Code**:
  ```go
  bestIP, backupIP := globalIPPool.getBestIPs(ipList)
  ```
- **New Code**:
  ```go
  bestIP, backupIP := globalIPPool.GetBestIPsV2(ipList)
  ```

**2. Add V2 Metric Recording**
When a query succeeds/fails on an IP, record it:
```go
// In iterState.handle() after successful response
if len(response.Answer) > 0 {
    iqv2 := globalIPPool.GetIPQualityV2(ipUsed)
    if iqv2 != nil {
        // Record measured latency
        latency := int32(time.Since(queryStart).Milliseconds())
        iqv2.RecordLatency(latency)
    }
}

// On query failure
if err != nil {
    iqv2 := globalIPPool.GetIPQualityV2(ipUsed)
    if iqv2 != nil {
        iqv2.RecordFailure()
    }
}
```

**3. Backward Compatibility**
- Keep `getBestIPs()` for fallback (optional, can remove after validation)
- Both V1 and V2 maintain separate pools
- No breaking changes to existing code

### Testing Strategy
```bash
# 1. Unit tests for migration
go test ./server/ -run "TestIterState" -v

# 2. E2E tests with mock servers
go test ./e2e/ -v

# 3. Regression tests
go test ./... -race
```

### Success Criteria
- All existing tests pass
- New `GetBestIPsV2()` calls work correctly
- Performance no worse than V1 (should be better)
- No memory leaks

### Risk Mitigation
- Use feature flag for gradual rollout
- Monitor metrics for 24-48 hours before full deployment
- Keep V1 as fallback for quick rollback

---

## F-003/12: Prometheus Metrics Export

### Overview
Add gauges for P50/P95/P99 latency per IP to monitor resolver health.

### Implementation Steps

**1. Define Metrics in monitor/metrics.go**
```go
var (
    ipP50LatencyMs = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "rec53_ip_p50_latency_ms",
            Help: "P50 (median) latency for IP nameserver in milliseconds",
        },
        []string{"ip"},
    )
    ipP95LatencyMs = prometheus.NewGaugeVec(...)
    ipP99LatencyMs = prometheus.NewGaugeVec(...)
    ipConfidence = prometheus.NewGaugeVec(...)
    ipState = prometheus.NewGaugeVec(...)
)
```

**2. Register Metrics**
```go
func init() {
    prometheus.MustRegister(ipP50LatencyMs)
    prometheus.MustRegister(ipP95LatencyMs)
    prometheus.MustRegister(ipP99LatencyMs)
    prometheus.MustRegister(ipConfidence)
    prometheus.MustRegister(ipState)
}
```

**3. Update Metrics Periodically**
- Create background goroutine in `IPPool.Run()`
- Every 10 seconds, iterate all IPs in poolV2
- Set gauge values from GetP50Latency(), etc.

```go
func (ipp *IPPool) updateMetrics() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ipp.ctx.Done():
            return
        case <-ticker.C:
            ipp.l.RLock()
            for ip, iqv2 := range ipp.poolV2 {
                ipP50LatencyMs.WithLabelValues(ip).Set(float64(iqv2.GetP50Latency()))
                ipP95LatencyMs.WithLabelValues(ip).Set(float64(iqv2.GetP95Latency()))
                // ... etc
            }
            ipp.l.RUnlock()
        }
    }
}
```

### Dashboards
Create Grafana dashboard queries:
- `rec53_ip_p50_latency_ms` - sorted by value
- `rec53_ip_p95_latency_ms` - tail latency
- `increase(rec53_ip_queries_total[5m])` - QPS per IP
- `histogram_quantile(0.99, ...)` - P99 across all IPs

### Success Criteria
- Metrics exported to Prometheus correctly
- P50/P95/P99 values match internal calculations
- No performance impact from metric updates

---

## F-003/13: Performance Benchmark

### Overview
Verify that IP selection remains fast as pool grows.

### Implementation

**1. Add Benchmark Test**
```go
func BenchmarkGetBestIPsV2(b *testing.B) {
    ipp := NewIPPool()
    defer ipp.Shutdown(context.Background())
    
    // Create 1000 test IPs
    ips := make([]string, 0, 1000)
    for i := 0; i < 1000; i++ {
        ip := fmt.Sprintf("192.0.2.%d", i%256)
        ips = append(ips, ip)
        
        iqv2 := NewIPQualityV2()
        for j := 0; j < 10; j++ {
            iqv2.RecordLatency(int32(100 + j*10))
        }
        ipp.SetIPQualityV2(ip, iqv2)
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = ipp.GetBestIPsV2(ips)
    }
}
```

**2. Run Benchmark**
```bash
go test -bench=BenchmarkGetBestIPsV2 -benchmem ./server
```

### Success Criteria
- Selection time < 1ms for 1000 IPs
- Memory allocation stable
- No performance regression vs V1

### Optimization Hints
If too slow:
- Cache scores locally (race-safe with timestamp)
- Use partial sort instead of full sort
- Parallel score calculation with sync.Pool

---

## F-003/6: Background Probe Loop

### Overview
Periodically probe SUSPECT IPs to enable auto-recovery.

### Implementation

**1. Add Probe Loop to IPPool**
```go
func (ipp *IPPool) StartProbeLoop(ctx context.Context) {
    ipp.wg.Add(1)
    go ipp.periodicProbeLoop()
}

func (ipp *IPPool) periodicProbeLoop() {
    defer ipp.wg.Done()
    
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ipp.ctx.Done():
            return
        case <-ticker.C:
            ipp.probeAllSuspiciousIPs()
        }
    }
}

func (ipp *IPPool) probeAllSuspiciousIPs() {
    // Find candidates
    ipp.l.RLock()
    candidates := make([]string, 0)
    for ip, iqv2 := range ipp.poolV2 {
        if iqv2.ShouldProbe() {
            candidates = append(candidates, ip)
        }
    }
    ipp.l.RUnlock()
    
    // Probe each candidate (simple DNS root query)
    for _, ip := range candidates {
        req := new(dns.Msg)
        req.SetQuestion(".", dns.TypeA)
        
        client := &dns.Client{Timeout: 3 * time.Second}
        _, _, err := client.Exchange(req, ip+":53")
        
        iqv2 := ipp.GetIPQualityV2(ip)
        if iqv2 != nil {
            if err == nil {
                iqv2.ResetForProbe()  // Recovery success
            }
            // Failure: leave alone, will retry in 30s
        }
    }
}
```

**2. Integrate with Server Startup**
In `server/Run()`:
```go
globalIPPool.StartProbeLoop(s.srv.ctx)
```

### Risk Mitigation
- Rate limit probes with semaphore
- Avoid probe storms (30s interval minimum)
- Skip probes if resolver is overloaded
- Add metric: `rec53_ip_probes_total`

### Success Criteria
- SUSPECT IPs recover within 30-60 seconds
- Probes don't impact normal query handling
- No resource leaks

---

## F-003/15: Feature Flag (Optional)

### Overview
Enable A/B testing of V1 vs V2 before full migration.

### Implementation

**1. Add Config Flag**
```go
type Config struct {
    // ... existing fields
    IPPoolV2Enabled bool `flag:"ip-pool-v2"`
}
```

**2. Conditional Logic**
```go
func (ipp *IPPool) SelectBestIPs(ips []string) (string, string) {
    if config.IPPoolV2Enabled {
        return ipp.GetBestIPsV2(ips)
    }
    return ipp.getBestIPs(ips)  // V1 fallback
}
```

**3. Metrics**
Add label to track which algorithm is active:
```
rec53_ip_selection_algorithm{version="v2"}
```

---

## Testing Checklist

- [ ] Unit tests for V2 methods (Phase 1-3, already done)
- [ ] Integration test: migration in state_define.go
- [ ] E2E test: full DNS resolution flow
- [ ] Performance benchmark: 1000 IPs < 1ms
- [ ] Metrics verification: P50/P95/P99 exported
- [ ] Fault injection test: IP becomes SUSPECT, recovers
- [ ] Concurrency test: 100 goroutines simultaneous queries
- [ ] Regression test: all existing tests pass
- [ ] Memory leak test: long-running with goroutine pprof

---

## Deployment Strategy

### 1. Pre-Deploy
```bash
# Run full test suite with race detector
go test -race ./...

# Benchmark on target hardware
go test -bench=. -benchmem ./server

# Check memory profile
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

### 2. Canary (10% traffic)
- Deploy with feature flag disabled
- Enable for 10% of queries
- Monitor metrics for 24 hours
- Check error rates, latency SLOs

### 3. Rollout (50%)
- Expand to 50% of traffic
- Monitor for 48 hours
- Verify P99 latency improvement

### 4. Full Deployment
- 100% traffic
- Monitor continuously
- Keep V1 as rollback option

### Rollback Plan
If issues detected:
1. Set `IPPoolV2Enabled: false` in config
2. Restart resolver
3. Previous behavior restored immediately
4. No data loss (both pools maintained separately)

---

## Files Modified/Created

### Phase 4 Modifications
- `server/state_define.go` - Replace getBestIPs() calls (+10 lines)
- `server/ip_pool.go` - Add metrics updates, probe loop (+100 lines)
- `monitor/metrics.go` - Add Prometheus gauges (+50 lines)
- `server/ip_pool_test.go` - Add Phase 4 tests (+150 lines)
- `e2e/dns_test.go` - E2E integration tests (+100 lines)

### Total Phase 4 Effort
- Implementation: 150-200 LOC
- Tests: 150-200 LOC
- Effort: 4-5 days

---

## Success Metrics

By end of Phase 4:
- ✅ All 33 existing tests still pass
- ✅ 10+ new Phase 4 tests pass
- ✅ GetBestIPsV2 performance: < 1ms for 1000 IPs
- ✅ Fault recovery time: < 60 seconds
- ✅ P99 latency improvement: > 10%
- ✅ Zero regression in DNS query success rate
- ✅ Feature flag working for safe rollout

---

## References

- **Design**: `.rec53/IP_POOL_DESIGN.md`
- **Roadmap**: `.rec53/IP_POOL_ROADMAP.md`
- **Code**: `server/ip_pool.go` (466 lines, 33 tests)
- **API Docs**: RecordLatency, RecordFailure, GetScore, GetBestIPsV2

---

## Questions?

Contact: AI Development Assistant  
Last Updated: 2026-03-11  
Phase 4 Ready Date: YYYY-MM-DD
