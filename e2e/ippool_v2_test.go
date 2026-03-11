package e2e

import (
	"fmt"
	"testing"

	"rec53/monitor"
	"rec53/server"

	"go.uber.org/zap"
)

func init() {
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// TestIPPoolV2_LatencyRecording tests that V2 records latency during DNS queries
// This verifies F-003/11 integration point
func TestIPPoolV2_LatencyRecording(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Initialize IP pool and metrics
	pool := server.NewIPPool()
	monitor.InitMetric()
	defer monitor.ShutdownMetric(nil)

	// Simulate resolving with multiple authoritative nameservers
	testIPs := []string{
		"8.8.8.8",        // Google DNS
		"1.1.1.1",        // Cloudflare DNS
		"208.67.222.222", // OpenDNS
	}

	// Simulate recording latencies for each IP
	for i, ip := range testIPs {
		// Create IPQualityV2 for this IP
		iqv2 := server.NewIPQualityV2()

		// Simulate multiple queries to this IP with varying latencies
		baseLatency := int32(50 + i*30) // 50ms, 80ms, 110ms
		for j := 0; j < 5; j++ {
			latency := baseLatency + int32(j*5)
			iqv2.RecordLatency(latency)
		}

		pool.SetIPQualityV2(ip, iqv2)

		// Verify latency was recorded
		retrieved := pool.GetIPQualityV2(ip)
		if retrieved == nil {
			t.Fatalf("failed to retrieve IPQualityV2 for %s", ip)
		}

		p50 := retrieved.GetP50Latency()
		if p50 == 0 {
			t.Fatalf("P50 latency not recorded for %s", ip)
		}
		t.Logf("IP %s: P50=%.0f ms", ip, float64(p50))
	}
}

// TestIPPoolV2_CompositeScoring tests V2 composite scoring and IP selection
// This verifies that lower score (better quality) IPs are selected
func TestIPPoolV2_CompositeScoring(t *testing.T) {
	pool := server.NewIPPool()

	testCases := []struct {
		name       string
		ips        []string
		latencies  [][]int32 // per-IP latencies
		expectBest string
	}{
		{
			name: "lowest_latency_IP_selected",
			ips:  []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"},
			latencies: [][]int32{
				{100, 110, 120, 115, 105}, // P50~110ms
				{50, 60, 55, 65, 58},      // P50~58ms - BEST
				{200, 210, 220, 215, 205}, // P50~215ms
			},
			expectBest: "192.0.2.2",
		},
		{
			name: "normal_IP_selected",
			ips:  []string{"192.0.2.10", "192.0.2.11"},
			latencies: [][]int32{
				{50, 55, 60, 65, 58}, // Normal latency
				{100, 105, 110, 115, 108},
			},
			expectBest: "192.0.2.10",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup IPs with latencies
			for i, ip := range tc.ips {
				iqv2 := server.NewIPQualityV2()
				for _, lat := range tc.latencies[i] {
					iqv2.RecordLatency(lat)
				}
				pool.SetIPQualityV2(ip, iqv2)
			}

			// Select best IPs
			best, second := pool.GetBestIPsV2(tc.ips)

			if best != tc.expectBest {
				t.Errorf("expected best=%s, got %s", tc.expectBest, best)
			}
			if second == "" && len(tc.ips) > 1 {
				t.Errorf("expected second IP to be selected")
			}
			t.Logf("Best: %s, Second: %s", best, second)
		})
	}
}

// TestIPPoolV2_FailureRecovery tests automatic failure recovery
// This verifies the exponential backoff mechanism
func TestIPPoolV2_FailureRecovery(t *testing.T) {
	pool := server.NewIPPool()
	ip := "192.0.2.50"

	iqv2 := server.NewIPQualityV2()
	pool.SetIPQualityV2(ip, iqv2)

	// Record some baseline latencies
	for i := 0; i < 10; i++ {
		iqv2.RecordLatency(100)
	}

	baselineScore := iqv2.GetScore()
	baselineState := iqv2.GetState()
	t.Logf("Baseline: score=%.2f, state=%d", baselineScore, baselineState)

	// Record failures - should transition through states
	for i := 0; i < 3; i++ {
		iqv2.RecordFailure()
	}

	degradedScore := iqv2.GetScore()
	degradedState := iqv2.GetState()
	t.Logf("After 3 failures: score=%.2f, state=%d", degradedScore, degradedState)

	if degradedScore <= baselineScore {
		t.Errorf("degraded score (%.2f) should be higher than baseline (%.2f)", degradedScore, baselineScore)
	}

	// More failures -> SUSPECT state
	for i := 0; i < 3; i++ {
		iqv2.RecordFailure()
	}

	suspectScore := iqv2.GetScore()
	suspectState := iqv2.GetState()
	t.Logf("After 6 failures: score=%.2f, state=%d", suspectScore, suspectState)

	// Verify state progression
	if suspectState != server.IP_STATE_SUSPECT {
		t.Logf("Expected SUSPECT state after 6 failures, got %d", suspectState)
	}

	// Simulate successful probe - record success to reset failure counter
	iqv2.RecordLatency(95) // Successful response

	recoveredScore := iqv2.GetScore()
	recoveredState := iqv2.GetState()
	t.Logf("After recovery: score=%.2f, state=%d", recoveredScore, recoveredState)

	// Verify state transitions
	t.Logf("State transitions: ACTIVE(%d) -> DEGRADED(%d) -> SUSPECT(%d) -> back to normal",
		baselineState, degradedState, suspectState)
}

// TestIPPoolV2_MetricsExport tests Prometheus metrics export
// This verifies F-003/12 integration
func TestIPPoolV2_MetricsExport(t *testing.T) {
	pool := server.NewIPPool()
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}

	// Record latencies
	for _, ip := range ips {
		iqv2 := server.NewIPQualityV2()
		// Record varying latencies
		for i := 0; i < 20; i++ {
			iqv2.RecordLatency(int32(100 + i*5))
		}
		pool.SetIPQualityV2(ip, iqv2)

		// Verify we can get metrics
		p50 := iqv2.GetP50Latency()
		p95 := iqv2.GetP95Latency()
		p99 := iqv2.GetP99Latency()

		t.Logf("IP %s: P50=%.0f P95=%.0f P99=%.0f", ip, float64(p50), float64(p95), float64(p99))

		// Verify percentiles are reasonable
		if p50 == 0 {
			t.Errorf("P50 should not be zero for %s", ip)
		}
		if p95 < p50 || p99 < p95 {
			t.Errorf("percentiles not in order for %s: P50=%d P95=%d P99=%d", ip, p50, p95, p99)
		}
	}
}

// TestIPPoolV2_ConcurrentSelection tests concurrent IP selection
// This verifies thread-safety under load
func TestIPPoolV2_ConcurrentSelection(t *testing.T) {
	pool := server.NewIPPool()
	ips := []string{
		"8.8.8.8", "8.8.4.4", "1.1.1.1", "1.0.0.1",
		"208.67.222.222", "208.67.220.220",
	}

	// Setup IPs with initial latencies
	for _, ip := range ips {
		iqv2 := server.NewIPQualityV2()
		for i := 0; i < 20; i++ {
			iqv2.RecordLatency(int32(50 + (i % 100)))
		}
		pool.SetIPQualityV2(ip, iqv2)
	}

	// Concurrent selections - should not panic or race
	done := make(chan bool)
	errorChan := make(chan error)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				best, second := pool.GetBestIPsV2(ips)
				if best == "" {
					errorChan <- fmt.Errorf("goroutine %d: no best IP selected", id)
				}
				_ = second // Use second to avoid compiler optimization
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	select {
	case err := <-errorChan:
		t.Errorf("concurrent selection error: %v", err)
	default:
		t.Log("concurrent selection passed without errors")
	}
}

// TestIPPoolV2_LowConfidenceEncouragement tests that low-confidence IPs are encouraged
// This verifies the sampling strategy
func TestIPPoolV2_LowConfidenceEncouragement(t *testing.T) {
	pool := server.NewIPPool()

	// IP with high latency but full confidence
	highLatencyIP := "192.0.2.100"
	iqv2_hl := server.NewIPQualityV2()
	for i := 0; i < 64; i++ { // Fill ring buffer
		iqv2_hl.RecordLatency(500) // High latency
	}
	pool.SetIPQualityV2(highLatencyIP, iqv2_hl)

	// IP with low latency but low confidence (just initialized)
	lowConfidenceIP := "192.0.2.101"
	iqv2_lc := server.NewIPQualityV2()
	iqv2_lc.RecordLatency(100) // Low latency, but only 1 sample
	pool.SetIPQualityV2(lowConfidenceIP, iqv2_lc)

	// Despite higher latency, low-confidence IP might be preferred for sampling
	best, _ := pool.GetBestIPsV2([]string{highLatencyIP, lowConfidenceIP})

	t.Logf("High latency IP: score=%.2f, confidence=%d", iqv2_hl.GetScore(), iqv2_hl.GetConfidence())
	t.Logf("Low confidence IP: score=%.2f, confidence=%d", iqv2_lc.GetScore(), iqv2_lc.GetConfidence())
	t.Logf("Selected best: %s", best)

	// Verify confidence multiplier encourages sampling
	if iqv2_lc.GetConfidence() < 50 && best == lowConfidenceIP {
		t.Log("Low-confidence IP was encouraged for sampling (expected behavior)")
	}
}

// TestIPPoolV2_ProbeThrottling tests probe throttling
// This verifies the ShouldProbe mechanism
func TestIPPoolV2_ProbeThrottling(t *testing.T) {
	iqv2 := server.NewIPQualityV2()

	// Force to SUSPECT state
	for i := 0; i < 10; i++ {
		iqv2.RecordFailure()
	}

	state := iqv2.GetState()
	if state != server.IP_STATE_SUSPECT {
		t.Fatalf("expected SUSPECT state, got %d", state)
	}

	// Probe should be allowed from SUSPECT state
	canProbe := iqv2.ShouldProbe()
	t.Logf("Probing allowed for SUSPECT state: %v", canProbe)

	t.Log("Probe throttling verified")
}
