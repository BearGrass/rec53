package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"rec53/monitor"

	"go.uber.org/zap"
)

func init() {
	// Initialize a no-op logger for tests
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// TestFaultRecovery_FullLifecycle verifies the complete fault recovery lifecycle:
// ACTIVE → DEGRADED → SUSPECT → (probe) → RECOVERED → ACTIVE
func TestFaultRecovery_FullLifecycle(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	testIP := "192.0.2.10"

	// Phase 1: Start with healthy ACTIVE IP
	iq := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq.RecordLatency(50) // Establish good baseline
	}
	ipp.SetIPQualityV2(testIP, iq)

	if iq.GetState() != IP_STATE_ACTIVE {
		t.Fatalf("Phase 1: expected ACTIVE state, got %d", iq.GetState())
	}
	t.Logf("Phase 1: IP is ACTIVE with p50=%dms", iq.GetP50Latency())

	// Phase 2: Record 1-3 failures → DEGRADED
	iq.RecordFailure()
	if iq.GetState() != IP_STATE_DEGRADED {
		t.Errorf("Phase 2: expected DEGRADED after 1 failure, got %d", iq.GetState())
	}
	if iq.ShouldProbe() {
		t.Errorf("Phase 2: DEGRADED IP should not be probed")
	}
	t.Logf("Phase 2: IP is DEGRADED after 1 failure")

	// Phase 3: Record 4-6 failures → SUSPECT
	for i := 0; i < 3; i++ {
		iq.RecordFailure()
	}
	if iq.GetState() != IP_STATE_SUSPECT {
		t.Errorf("Phase 3: expected SUSPECT after 4 failures, got %d", iq.GetState())
	}
	// Check that latency penalties are applied
	if iq.GetP50Latency() != int32(MAX_IP_LATENCY) || iq.GetP95Latency() != int32(MAX_IP_LATENCY) {
		t.Errorf("Phase 3: SUSPECT IP should have MAX latency, got p50=%d p95=%d",
			iq.GetP50Latency(), iq.GetP95Latency())
	}
	t.Logf("Phase 3: IP is SUSPECT with max latency penalties")

	// Phase 4: Wait for probe eligibility (5 seconds throttle)
	time.Sleep(5100 * time.Millisecond)
	if !iq.ShouldProbe() {
		t.Errorf("Phase 4: SUSPECT IP should be probe-eligible after 5s")
	}
	t.Logf("Phase 4: IP is probe-eligible")

	// Phase 5: Simulate successful probe → RECOVERED
	iq.ResetForProbe()
	if iq.GetState() != IP_STATE_RECOVERED {
		t.Errorf("Phase 5: expected RECOVERED after probe, got %d", iq.GetState())
	}
	if iq.ShouldProbe() {
		t.Errorf("Phase 5: RECOVERED IP should not be probed immediately")
	}
	t.Logf("Phase 5: IP is RECOVERED")

	// Phase 6: Record success → back to ACTIVE
	iq.RecordLatency(60)
	if iq.GetState() != IP_STATE_ACTIVE {
		t.Errorf("Phase 6: expected ACTIVE after successful latency, got %d", iq.GetState())
	}
	iq.mu.RLock()
	failCount := iq.failCount
	iq.mu.RUnlock()
	if failCount != 0 {
		t.Errorf("Phase 6: failure count should reset to 0, got %d", failCount)
	}
	t.Logf("Phase 6: IP fully recovered to ACTIVE")
}

// TestFaultRecovery_MultipleIPsCompetition verifies IP selection during mixed health states
func TestFaultRecovery_MultipleIPsCompetition(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Setup: 3 IPs with different health states
	healthyIP := "192.0.2.1"  // ACTIVE, low latency
	degradedIP := "192.0.2.2" // DEGRADED, medium latency
	suspectIP := "192.0.2.3"  // SUSPECT, max latency

	// Healthy IP: fast and stable
	iq1 := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq1.RecordLatency(30)
	}
	ipp.SetIPQualityV2(healthyIP, iq1)

	// Degraded IP: slower, with 1 failure
	iq2 := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq2.RecordLatency(80)
	}
	iq2.RecordFailure() // DEGRADED
	ipp.SetIPQualityV2(degradedIP, iq2)

	// Suspect IP: 4 failures
	iq3 := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq3.RecordLatency(50)
	}
	for i := 0; i < 4; i++ {
		iq3.RecordFailure() // SUSPECT
	}
	ipp.SetIPQualityV2(suspectIP, iq3)

	// Verify selection prefers healthy > degraded > suspect
	best, second := ipp.GetBestIPsV2([]string{healthyIP, degradedIP, suspectIP})
	if best != healthyIP {
		t.Errorf("expected healthy IP as best, got %s", best)
	}
	if second != degradedIP {
		t.Errorf("expected degraded IP as second, got %s", second)
	}
	t.Logf("Selection correctly prioritizes: healthy=%s, degraded=%s, suspect=%s",
		best, second, suspectIP)

	// Verify suspect IP is avoided unless no alternatives
	best, second = ipp.GetBestIPsV2([]string{suspectIP})
	if best != suspectIP {
		t.Errorf("when only suspect IP available, should return it, got %s", best)
	}
}

// TestFaultRecovery_ScoreProgression verifies composite score changes through lifecycle
func TestFaultRecovery_ScoreProgression(t *testing.T) {
	iq := NewIPQualityV2()

	// Establish baseline
	for i := 0; i < 20; i++ {
		iq.RecordLatency(50)
	}

	// Record scores at each state
	activeScore := iq.GetScore()
	t.Logf("ACTIVE score: %.2f (p50=%d, state_weight=1.0)", activeScore, iq.GetP50Latency())

	// Transition to DEGRADED
	iq.RecordFailure()
	degradedScore := iq.GetScore()
	t.Logf("DEGRADED score: %.2f (p50=%d, state_weight=1.5)", degradedScore, iq.GetP50Latency())

	if degradedScore <= activeScore {
		t.Errorf("DEGRADED score should be worse (higher) than ACTIVE: %.2f vs %.2f",
			degradedScore, activeScore)
	}

	// Transition to SUSPECT
	for i := 0; i < 3; i++ {
		iq.RecordFailure()
	}
	suspectScore := iq.GetScore()
	t.Logf("SUSPECT score: %.2f (p50=%d, state_weight=100.0)", suspectScore, iq.GetP50Latency())

	if suspectScore <= degradedScore {
		t.Errorf("SUSPECT score should be much worse than DEGRADED: %.2f vs %.2f",
			suspectScore, degradedScore)
	}

	// Simulate probe and recovery
	iq.ResetForProbe()
	recoveredScore := iq.GetScore()
	t.Logf("RECOVERED score: %.2f (p50=%d, state_weight=1.1)", recoveredScore, iq.GetP50Latency())

	if recoveredScore >= suspectScore {
		t.Errorf("RECOVERED score should be better than SUSPECT: %.2f vs %.2f",
			recoveredScore, suspectScore)
	}

	// Record success to return to ACTIVE
	iq.RecordLatency(55)
	finalScore := iq.GetScore()
	t.Logf("Final ACTIVE score: %.2f (p50=%d, state_weight=1.0)", finalScore, iq.GetP50Latency())
}

// TestFaultRecovery_ConcurrentFailuresAndRecoveries simulates real-world chaos
func TestFaultRecovery_ConcurrentFailuresAndRecoveries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Create 10 IPs
	ips := make([]string, 10)
	for i := 0; i < 10; i++ {
		ips[i] = fmt.Sprintf("192.0.2.%d", i+10)
		iq := NewIPQualityV2()
		for j := 0; j < 20; j++ {
			iq.RecordLatency(int32(50 + i*5)) // Varying baseline latencies
		}
		ipp.SetIPQualityV2(ips[i], iq)
	}

	// Simulate concurrent query activity with mixed success/failure
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ip := ips[idx]
			iq := ipp.GetIPQualityV2(ip)
			if iq == nil {
				return
			}

			// Simulate mixed query results
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// 70% success, 30% failure
					if idx < 7 {
						iq.RecordLatency(int32(50 + idx*5))
					} else {
						iq.RecordFailure()
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state distribution
	activeCount := 0
	degradedCount := 0
	suspectCount := 0

	for _, ip := range ips {
		iq := ipp.GetIPQualityV2(ip)
		if iq == nil {
			continue
		}
		state := iq.GetState()
		switch state {
		case IP_STATE_ACTIVE, IP_STATE_RECOVERED:
			activeCount++
		case IP_STATE_DEGRADED:
			degradedCount++
		case IP_STATE_SUSPECT:
			suspectCount++
		}
		iq.mu.RLock()
		failCount := iq.failCount
		iq.mu.RUnlock()
		t.Logf("IP %s: state=%d, p50=%dms, failures=%d",
			ip, state, iq.GetP50Latency(), failCount)
	}

	// Most IPs should remain healthy with 70% success rate
	if activeCount < 5 {
		t.Errorf("expected at least 5 healthy IPs, got %d", activeCount)
	}

	t.Logf("Final distribution: active=%d, degraded=%d, suspect=%d",
		activeCount, degradedCount, suspectCount)
}

// TestFaultRecovery_ProbeThrottling verifies probe throttle prevents excessive probing
func TestFaultRecovery_ProbeThrottling(t *testing.T) {
	iq := NewIPQualityV2()

	// Create SUSPECT IP
	for i := 0; i < 20; i++ {
		iq.RecordLatency(50)
	}
	for i := 0; i < 4; i++ {
		iq.RecordFailure() // SUSPECT
	}

	// Immediately after becoming SUSPECT, should not probe (throttled)
	if iq.ShouldProbe() {
		t.Errorf("SUSPECT IP should be throttled immediately after last failure")
	}

	// Wait just under 5 seconds - still throttled
	time.Sleep(4 * time.Second)
	if iq.ShouldProbe() {
		t.Errorf("SUSPECT IP should still be throttled at 4s")
	}

	// Wait past 5 seconds - now probe-eligible
	time.Sleep(1100 * time.Millisecond)
	if !iq.ShouldProbe() {
		t.Errorf("SUSPECT IP should be probe-eligible after 5s")
	}

	// After probe reset, should not probe again immediately
	iq.ResetForProbe() // Now RECOVERED
	if iq.ShouldProbe() {
		t.Errorf("RECOVERED IP should not be probed immediately")
	}
}

// TestFaultRecovery_ConfidenceBonus verifies low-confidence IPs get selection bonus
func TestFaultRecovery_ConfidenceBonus(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	establishedIP := "192.0.2.100" // High confidence
	newIP := "192.0.2.101"         // Low confidence

	// Established IP: 20 samples, 100% confidence
	iq1 := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq1.RecordLatency(50)
	}
	ipp.SetIPQualityV2(establishedIP, iq1)

	// New IP: only 2 samples, 20% confidence
	iq2 := NewIPQualityV2()
	iq2.RecordLatency(80) // Higher latency
	iq2.RecordLatency(80)
	ipp.SetIPQualityV2(newIP, iq2)

	// Despite higher latency, new IP should get bonus due to low confidence
	score1 := iq1.GetScore()
	score2 := iq2.GetScore()

	confidence1 := iq1.GetConfidence()
	confidence2 := iq2.GetConfidence()

	t.Logf("Established IP: confidence=%d%%, p50=%dms, score=%.2f",
		confidence1, iq1.GetP50Latency(), score1)
	t.Logf("New IP: confidence=%d%%, p50=%dms, score=%.2f",
		confidence2, iq2.GetP50Latency(), score2)

	// Low confidence should provide ~2x bonus to encourage sampling
	if confidence2 >= 50 {
		t.Skipf("Skipping: new IP has confidence=%d%%, need <50%% for bonus test", confidence2)
	}

	// New IP should be competitive despite higher latency
	// With low confidence (20%), the multiplier is ~1.8x
	// So 80ms * 1.8 = 144, which should be < 50 * 3 (not too much worse)
	if score2 > score1*3 {
		t.Errorf("Low-confidence IP should still be competitive, got scores: %.2f vs %.2f",
			score2, score1)
	}
}

// TestFaultRecovery_RecoveredToActiveTransition verifies clean recovery
func TestFaultRecovery_RecoveredToActiveTransition(t *testing.T) {
	iq := NewIPQualityV2()

	// Setup: Create SUSPECT IP
	for i := 0; i < 20; i++ {
		iq.RecordLatency(50)
	}
	for i := 0; i < 4; i++ {
		iq.RecordFailure()
	}

	if iq.GetState() != IP_STATE_SUSPECT {
		t.Fatalf("Setup failed: expected SUSPECT, got %d", iq.GetState())
	}

	// Simulate successful probe
	iq.ResetForProbe()
	if iq.GetState() != IP_STATE_RECOVERED {
		t.Fatalf("Probe failed: expected RECOVERED, got %d", iq.GetState())
	}

	// First successful query after probe should return to ACTIVE
	iq.RecordLatency(55)

	if iq.GetState() != IP_STATE_ACTIVE {
		t.Errorf("expected transition to ACTIVE after first success, got %d", iq.GetState())
	}

	iq.mu.RLock()
	failCount := iq.failCount
	iq.mu.RUnlock()
	if failCount != 0 {
		t.Errorf("failure count should reset to 0, got %d", failCount)
	}

	// Verify latency metrics are updated normally
	if iq.GetP50Latency() < 50 || iq.GetP50Latency() > 60 {
		t.Errorf("expected p50 around 50-55ms, got %dms", iq.GetP50Latency())
	}
}

// TestFaultRecovery_AvoidSuspectIPsWithAlternatives verifies suspect avoidance logic
func TestFaultRecovery_AvoidSuspectIPsWithAlternatives(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	healthyIP := "192.0.2.1"
	suspectIP := "192.0.2.2"

	// Healthy IP
	iq1 := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq1.RecordLatency(50)
	}
	ipp.SetIPQualityV2(healthyIP, iq1)

	// Suspect IP
	iq2 := NewIPQualityV2()
	for i := 0; i < 20; i++ {
		iq2.RecordLatency(40) // Even better baseline latency
	}
	for i := 0; i < 4; i++ {
		iq2.RecordFailure() // But now SUSPECT
	}
	ipp.SetIPQualityV2(suspectIP, iq2)

	// Should prefer healthy over suspect, even with worse baseline
	best, second := ipp.GetBestIPsV2([]string{healthyIP, suspectIP})

	if best != healthyIP {
		t.Errorf("expected healthy IP as best despite worse baseline, got %s", best)
	}

	if second != suspectIP {
		t.Errorf("expected suspect IP as second, got %s", second)
	}

	t.Logf("Correctly avoided suspect IP: best=%s (ACTIVE), second=%s (SUSPECT)",
		best, second)
}
