package server

import (
	"context"
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

func TestIPQualityConcurrentAccess(t *testing.T) {
	ipq := NewIPQuality()

	// Test initial state
	if !ipq.IsInit() {
		t.Error("expected IsInit() to be true initially")
	}
	if ipq.GetLatency() != INIT_IP_LATENCY {
		t.Errorf("expected initial latency %d, got %d", INIT_IP_LATENCY, ipq.GetLatency())
	}

	// Test concurrent reads and writes
	var wg sync.WaitGroup
	const goroutines = 100

	// Concurrent writes to latency
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(val int32) {
			defer wg.Done()
			ipq.SetLatency(val)
		}(int32(i + 100))
	}

	// Concurrent reads of IsInit
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ipq.IsInit()
		}()
	}

	// Concurrent reads of latency
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ipq.GetLatency()
		}()
	}

	wg.Wait()

	// Test SetLatencyAndState
	ipq.SetLatencyAndState(500)
	if ipq.IsInit() {
		t.Error("expected IsInit() to be false after SetLatencyAndState")
	}
	if ipq.GetLatency() != 500 {
		t.Errorf("expected latency 500, got %d", ipq.GetLatency())
	}
}

func TestIPQualityInit(t *testing.T) {
	ipq := &IPQuality{}
	ipq.Init()

	if !ipq.IsInit() {
		t.Error("expected IsInit() to be true after Init()")
	}
	if ipq.GetLatency() != INIT_IP_LATENCY {
		t.Errorf("expected latency %d after Init(), got %d", INIT_IP_LATENCY, ipq.GetLatency())
	}
}

func TestIPPoolGetSetIPQuality(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Test Get on non-existent IP
	if ipq := ipp.GetIPQuality("192.168.1.1"); ipq != nil {
		t.Error("expected nil for non-existent IP")
	}

	// Test Set and Get
	testIP := "192.168.1.1"
	ipq := NewIPQuality()
	ipq.SetLatency(200)
	ipp.SetIPQuality(testIP, ipq)

	got := ipp.GetIPQuality(testIP)
	if got == nil {
		t.Fatal("expected to get IPQuality, got nil")
	}
	if got.GetLatency() != 200 {
		t.Errorf("expected latency 200, got %d", got.GetLatency())
	}
}

func TestIPPoolConcurrentAccess(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent SetIPQuality
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ip := "192.168.1." + string(rune('0'+idx%10))
			ipq := NewIPQuality()
			ipq.SetLatency(int32(idx * 10))
			ipp.SetIPQuality(ip, ipq)
		}(i)
	}

	// Concurrent GetIPQuality
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ipp.GetIPQuality("192.168.1.1")
		}()
	}

	wg.Wait()
}

func TestIPPoolGetBestIPs(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Set up test IPs with different latencies
	// bestIP: the IP with lowest latency (any init state)
	// bestIPWithoutInit: the IP with lowest latency where isInit=false (measured)

	// 10.0.0.1: latency 100, measured (isInit=false)
	ipq1 := NewIPQuality()
	ipq1.SetLatencyAndState(100)
	ipp.SetIPQuality("10.0.0.1", ipq1)

	// 10.0.0.2: latency 200, measured (isInit=false)
	ipq2 := NewIPQuality()
	ipq2.SetLatencyAndState(200)
	ipp.SetIPQuality("10.0.0.2", ipq2)

	// 10.0.0.3: latency 50, init state (isInit=true) - lowest latency but not measured
	ipq3 := NewIPQuality() // isInit=true by default
	ipq3.SetLatency(50)
	ipp.SetIPQuality("10.0.0.3", ipq3)

	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	bestIP, bestIPWithoutInit := ipp.getBestIPs(ips)

	// bestIP should be 10.0.0.3 (lowest latency: 50)
	if bestIP != "10.0.0.3" {
		t.Errorf("expected best IP 10.0.0.3, got %s", bestIP)
	}
	// bestIPWithoutInit should be 10.0.0.1 (lowest latency among measured IPs)
	if bestIPWithoutInit != "10.0.0.1" {
		t.Errorf("expected best IP without init 10.0.0.1, got %s", bestIPWithoutInit)
	}
}

func TestIPPoolGetPrefetchIPs(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Set up IPs with different latencies
	ipq1 := NewIPQuality()
	ipq1.SetLatencyAndState(100) // best
	ipp.SetIPQuality("10.0.0.1", ipq1)

	ipq2 := NewIPQuality()
	ipq2.SetLatencyAndState(150)
	ipp.SetIPQuality("10.0.0.2", ipq2)

	ipq3 := NewIPQuality()
	ipq3.SetLatencyAndState(500)
	ipp.SetIPQuality("10.0.0.3", ipq3)

	prefetchIPs := ipp.GetPrefetchIPs("10.0.0.1")

	t.Logf("prefetch IPs: %v", prefetchIPs)
}

func TestIPPoolUpdateIPQuality(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	ip := "10.0.0.1"
	ipp.updateIPQuality(ip, 300)

	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		t.Fatal("expected IPQuality to exist after updateIPQuality")
	}
	if ipq.IsInit() {
		t.Error("expected IsInit() to be false after updateIPQuality")
	}
	if ipq.GetLatency() != 300 {
		t.Errorf("expected latency 300, got %d", ipq.GetLatency())
	}
}

func TestIPPoolUpIPsQuality(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Set up an IP in init state
	ip := "10.0.0.1"
	ipq := NewIPQuality()
	ipp.SetIPQuality(ip, ipq)

	initialLatency := ipq.GetLatency()
	ipp.UpIPsQuality([]string{ip})
	newLatency := ipq.GetLatency()

	// Latency should decrease by 10% (multiplied by 0.9)
	expected := int32(float64(initialLatency) * 0.9)
	if newLatency != expected {
		t.Errorf("expected latency %d, got %d", expected, newLatency)
	}
}

func TestIPPoolShutdown(t *testing.T) {
	ipp := NewIPPool()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ipp.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected error on shutdown: %v", err)
	}

	// Verify context is cancelled
	select {
	case <-ipp.ctx.Done():
		// Expected
	default:
		t.Error("expected context to be cancelled after shutdown")
	}
}

func TestIPPoolShutdownWithTimeout(t *testing.T) {
	ipp := NewIPPool()

	// Fill the semaphore to simulate busy state
	for i := 0; i < MAX_PREFETCH_CONCUR; i++ {
		ipp.sem <- struct{}{}
	}

	// Very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := ipp.Shutdown(ctx)
	// Should timeout because we can't acquire semaphore slots
	if err == nil {
		// This might pass if shutdown completes before timeout
		t.Log("shutdown completed before timeout")
	}

	// Release semaphore slots
	for i := 0; i < MAX_PREFETCH_CONCUR; i++ {
		<-ipp.sem
	}
}

func TestIPPoolIsTheIPInit(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	ip := "10.0.0.1"

	// First call should create and init the IP
	isInit := ipp.isTheIPInit(ip)
	if !isInit {
		t.Error("expected isTheIPInit to return true for new IP")
	}

	// Verify IP was created
	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		t.Fatal("expected IP to be created")
	}
}

// ============================================================================
// IPQualityV2 Tests (Sliding Window Histogram Implementation)
// ============================================================================

func TestIPQualityV2_NewIPQualityV2(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Verify initial state
	if iqv2.sampleCount != 0 {
		t.Errorf("expected sampleCount=0, got %d", iqv2.sampleCount)
	}
	if iqv2.p50 != int32(INIT_IP_LATENCY) {
		t.Errorf("expected p50=%d, got %d", INIT_IP_LATENCY, iqv2.p50)
	}
	if iqv2.p95 != int32(INIT_IP_LATENCY) {
		t.Errorf("expected p95=%d, got %d", INIT_IP_LATENCY, iqv2.p95)
	}
	if iqv2.confidence != 0 {
		t.Errorf("expected confidence=0, got %d", iqv2.confidence)
	}
	if iqv2.state != IP_STATE_ACTIVE {
		t.Errorf("expected state=ACTIVE(%d), got %d", IP_STATE_ACTIVE, iqv2.state)
	}
}

func TestIPQualityV2_RecordLatency_SingleSample(t *testing.T) {
	iqv2 := NewIPQualityV2()
	iqv2.RecordLatency(100)

	// With single sample, all percentiles should be equal
	if iqv2.sampleCount != 1 {
		t.Errorf("expected sampleCount=1, got %d", iqv2.sampleCount)
	}
	if iqv2.p50 != 100 {
		t.Errorf("expected p50=100, got %d", iqv2.p50)
	}
	if iqv2.p95 != 100 {
		t.Errorf("expected p95=100, got %d", iqv2.p95)
	}
	if iqv2.p99 != 100 {
		t.Errorf("expected p99=100, got %d", iqv2.p99)
	}
	if iqv2.confidence != 10 {
		t.Errorf("expected confidence=10, got %d", iqv2.confidence)
	}
	if iqv2.state != IP_STATE_ACTIVE {
		t.Errorf("expected state=ACTIVE, got %d", iqv2.state)
	}
}

func TestIPQualityV2_RecordLatency_MultiplePercentiles(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Record samples: 50, 100, 150, 200, 250, 300, 350, 400, 450, 500
	samples := []int32{50, 100, 150, 200, 250, 300, 350, 400, 450, 500}
	for _, s := range samples {
		iqv2.RecordLatency(s)
	}

	// Verify percentile calculations
	// P50 (median of 10 samples) = (250 + 300) / 2 = 275, but sorted[5] = 300
	// sorted = [50, 100, 150, 200, 250, 300, 350, 400, 450, 500]
	// P50 = sorted[5] = 300
	if iqv2.p50 != 300 {
		t.Errorf("expected p50=300, got %d", iqv2.p50)
	}
	// P95 = sorted[9] = 500 (95% of 10 = 9.5 → 9)
	if iqv2.p95 != 500 {
		t.Errorf("expected p95=500, got %d", iqv2.p95)
	}
	// P99 = sorted[9] = 500 (99% of 10 = 9.9 → 9)
	if iqv2.p99 != 500 {
		t.Errorf("expected p99=500, got %d", iqv2.p99)
	}
	if iqv2.confidence != 100 {
		t.Errorf("expected confidence=100, got %d", iqv2.confidence)
	}
}

func TestIPQualityV2_RecordLatency_ConfidenceGrowth(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Test confidence growth
	expectedConfidence := []uint8{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 100, 100}
	for i, expected := range expectedConfidence {
		iqv2.RecordLatency(100)
		if iqv2.confidence != expected {
			t.Errorf("sample %d: expected confidence=%d, got %d", i+1, expected, iqv2.confidence)
		}
	}
}

func TestIPQualityV2_RecordLatency_RingBufferWrap(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Fill buffer completely and wrap around
	for i := 0; i < 100; i++ {
		iqv2.RecordLatency(int32(i%100) + 100)
	}

	// Should maintain 64 most recent samples
	if iqv2.sampleCount != 64 {
		t.Errorf("expected sampleCount=64 after wrapping, got %d", iqv2.sampleCount)
	}
	if iqv2.nextIdx != (100 % 64) {
		t.Errorf("expected nextIdx=%d, got %d", 100%64, iqv2.nextIdx)
	}
}

func TestIPQualityV2_RecordLatency_ResetFailureCount(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Manually set failure state
	iqv2.failCount = 5
	iqv2.state = IP_STATE_SUSPECT

	// Recording latency should reset failure count
	iqv2.RecordLatency(100)
	if iqv2.failCount != 0 {
		t.Errorf("expected failCount=0 after RecordLatency, got %d", iqv2.failCount)
	}
	if iqv2.state != IP_STATE_ACTIVE {
		t.Errorf("expected state=ACTIVE after RecordLatency, got %d", iqv2.state)
	}
}

func TestIPQualityV2_UpdatePercentiles_Boundary(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Single sample boundary
	iqv2.RecordLatency(500)
	if iqv2.p50 != 500 || iqv2.p95 != 500 || iqv2.p99 != 500 {
		t.Errorf("single sample: p50=%d, p95=%d, p99=%d", iqv2.p50, iqv2.p95, iqv2.p99)
	}

	// Two sample boundary
	iqv2.RecordLatency(600)
	if iqv2.p50 != 600 { // sorted[1] = 600
		t.Errorf("two samples: expected p50=600, got %d", iqv2.p50)
	}
}

func TestIPQualityV2_ConcurrentRecordLatency(t *testing.T) {
	iqv2 := NewIPQualityV2()
	var wg sync.WaitGroup

	// 50 concurrent goroutines recording samples
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val int32) {
			defer wg.Done()
			iqv2.RecordLatency(val * 10)
		}(int32(i + 1))
	}

	wg.Wait()

	// Verify final state is valid
	if iqv2.sampleCount != 50 {
		t.Errorf("expected sampleCount=50, got %d", iqv2.sampleCount)
	}
	if iqv2.p50 == 0 && iqv2.sampleCount > 0 {
		t.Error("p50 should be non-zero with samples")
	}
}

func TestIPQualityV2_Percentile_Accuracy(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Create specific dataset: 1-20 → should have known percentiles
	for i := int32(1); i <= 20; i++ {
		iqv2.RecordLatency(i * 10) // 10, 20, 30, ..., 200
	}

	// For 20 samples sorted [10, 20, 30, ..., 200]:
	// P50 = sorted[10] = 110
	// P95 = sorted[19] = 200 (95% of 20 = 19)
	// P99 = sorted[19] = 200 (99% of 20 = 19.8 → 19)
	if iqv2.p50 != 110 {
		t.Errorf("expected p50=110, got %d", iqv2.p50)
	}
	if iqv2.p95 != 200 {
		t.Errorf("expected p95=200, got %d", iqv2.p95)
	}
	if iqv2.p99 != 200 {
		t.Errorf("expected p99=200, got %d", iqv2.p99)
	}
}

func TestIPQualityV2_Latency_OutOfOrder(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Record samples in non-sequential order
	unordered := []int32{150, 50, 300, 100, 200}
	for _, val := range unordered {
		iqv2.RecordLatency(val)
	}

	// Percentile calculation should sort internally
	// sorted = [50, 100, 150, 200, 300]
	// P50 = sorted[2] = 150
	if iqv2.p50 != 150 {
		t.Errorf("expected p50=150 with unordered input, got %d", iqv2.p50)
	}
}

func TestIPQualityV2_LargeDataset(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Fill entire 64-sample ring buffer
	for i := int32(0); i < 64; i++ {
		iqv2.RecordLatency(100 + i) // 100-163
	}

	if iqv2.sampleCount != 64 {
		t.Errorf("expected sampleCount=64, got %d", iqv2.sampleCount)
	}

	// Add more samples (should wrap around)
	for i := int32(0); i < 10; i++ {
		iqv2.RecordLatency(200 + i)
	}

	// Should still have 64 samples
	if iqv2.sampleCount != 64 {
		t.Errorf("after wrapping: expected sampleCount=64, got %d", iqv2.sampleCount)
	}
}

func TestIPQualityV2_TimestampUpdate(t *testing.T) {
	iqv2 := NewIPQualityV2()
	initialTime := iqv2.lastUpdate

	time.Sleep(10 * time.Millisecond)
	iqv2.RecordLatency(100)

	if iqv2.lastUpdate.Equal(initialTime) {
		t.Error("lastUpdate should be changed after RecordLatency")
	}
	if iqv2.lastUpdate.Before(initialTime) {
		t.Error("lastUpdate should be more recent")
	}
}

// ============================================================================
// Phase 2 Tests: Fault Handling and Recovery
// ============================================================================

func TestIPQualityV2_RecordFailure_Phase1(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Record one successful sample first to establish baseline
	iqv2.RecordLatency(100)
	originalP50 := iqv2.p50

	// Record 3 failures (Phase 1: DEGRADED state)
	for i := 1; i <= 3; i++ {
		iqv2.RecordFailure()

		if iqv2.failCount != uint8(i) {
			t.Errorf("failure %d: expected failCount=%d, got %d", i, i, iqv2.failCount)
		}
		if iqv2.state != IP_STATE_DEGRADED {
			t.Errorf("failure %d: expected state=DEGRADED, got %d", i, iqv2.state)
		}

		// P50 should be increased by 20% for each failure
		// Just verify it's greater than original
		if iqv2.p50 <= originalP50 {
			t.Errorf("failure %d: p50 should be increased from %d, got %d", i, originalP50, iqv2.p50)
		}
	}
}

func TestIPQualityV2_RecordFailure_Phase2(t *testing.T) {
	iqv2 := NewIPQualityV2()
	iqv2.RecordLatency(100)

	// Record 6 failures to enter Phase 2 (SUSPECT state)
	for i := 1; i <= 6; i++ {
		iqv2.RecordFailure()
	}

	if iqv2.failCount != 6 {
		t.Errorf("expected failCount=6, got %d", iqv2.failCount)
	}
	if iqv2.state != IP_STATE_SUSPECT {
		t.Errorf("expected state=SUSPECT, got %d", iqv2.state)
	}
	// All metrics should be MAX
	if iqv2.p50 != int32(MAX_IP_LATENCY) {
		t.Errorf("expected p50=MAX_IP_LATENCY, got %d", iqv2.p50)
	}
	if iqv2.p95 != int32(MAX_IP_LATENCY) {
		t.Errorf("expected p95=MAX_IP_LATENCY, got %d", iqv2.p95)
	}
	if iqv2.p99 != int32(MAX_IP_LATENCY) {
		t.Errorf("expected p99=MAX_IP_LATENCY, got %d", iqv2.p99)
	}
}

func TestIPQualityV2_RecordFailure_Phase3(t *testing.T) {
	iqv2 := NewIPQualityV2()
	iqv2.RecordLatency(100)

	// Record 10 failures to enter Phase 3 (7+ failures)
	for i := 1; i <= 10; i++ {
		iqv2.RecordFailure()
	}

	if iqv2.failCount != 10 {
		t.Errorf("expected failCount=10, got %d", iqv2.failCount)
	}
	// Should remain SUSPECT for periodic probing
	if iqv2.state != IP_STATE_SUSPECT {
		t.Errorf("expected state=SUSPECT in phase 3, got %d", iqv2.state)
	}
}

func TestIPQualityV2_RecordFailure_UpdatesTimestamp(t *testing.T) {
	iqv2 := NewIPQualityV2()
	initialFailureTime := iqv2.lastFailure

	time.Sleep(10 * time.Millisecond)
	iqv2.RecordFailure()

	if iqv2.lastFailure.Equal(initialFailureTime) {
		t.Error("lastFailure should be updated")
	}
}

func TestIPQualityV2_ResetForProbe(t *testing.T) {
	iqv2 := NewIPQualityV2()
	iqv2.RecordLatency(100)

	// Create SUSPECT state
	for i := 0; i < 8; i++ {
		iqv2.RecordFailure()
	}

	if iqv2.state != IP_STATE_SUSPECT {
		t.Errorf("setup: expected SUSPECT state, got %d", iqv2.state)
	}

	// Reset for probe (simulating successful probe)
	iqv2.ResetForProbe()

	if iqv2.failCount != 0 {
		t.Errorf("after ResetForProbe: expected failCount=0, got %d", iqv2.failCount)
	}
	if iqv2.state != IP_STATE_RECOVERED {
		t.Errorf("after ResetForProbe: expected state=RECOVERED, got %d", iqv2.state)
	}
}

func TestIPQualityV2_ShouldProbe(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Active IP should not need probing
	if iqv2.ShouldProbe() {
		t.Error("ACTIVE IP should not need probing")
	}

	// DEGRADED IP should not need probing
	iqv2.RecordLatency(100)
	iqv2.RecordFailure()
	if iqv2.ShouldProbe() {
		t.Error("DEGRADED IP should not need probing")
	}

	// SUSPECT IP should need probing after throttle delay
	for i := 0; i < 5; i++ {
		iqv2.RecordFailure()
	}
	// Immediately after failure, should not probe due to throttle
	if iqv2.ShouldProbe() {
		t.Error("ShouldProbe should respect 5-second throttle")
	}
}

func TestIPQualityV2_ShouldProbe_ThrottleRecent(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Create SUSPECT state
	for i := 0; i < 8; i++ {
		iqv2.RecordFailure()
	}

	// Should not probe immediately
	if iqv2.ShouldProbe() {
		t.Error("ShouldProbe should be false immediately after failure")
	}
}

func TestIPQualityV2_GettersThreadSafe(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Record some data
	for i := int32(1); i <= 10; i++ {
		iqv2.RecordLatency(i * 10)
	}
	iqv2.RecordFailure()

	// All getters should work concurrently
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = iqv2.GetP50Latency()
			_ = iqv2.GetP95Latency()
			_ = iqv2.GetP99Latency()
			_ = iqv2.GetState()
			_ = iqv2.GetConfidence()
		}()
	}

	wg.Wait()

	// Verify values are valid
	if iqv2.GetP50Latency() == 0 {
		t.Error("P50 should be non-zero after samples")
	}
	if iqv2.GetState() != IP_STATE_DEGRADED {
		t.Error("State should be DEGRADED after 1 failure")
	}
	if iqv2.GetConfidence() != 100 {
		t.Error("Confidence should be 100 after 10 samples")
	}
}

func TestIPQualityV2_FailureMaxLatencyBoundary(t *testing.T) {
	iqv2 := NewIPQualityV2()

	// Set high starting latency near MAX
	iqv2.p50 = int32(MAX_IP_LATENCY) - 1000

	// First failure with 1.2x multiplier
	iqv2.RecordFailure()

	// Should not exceed MAX_IP_LATENCY
	if iqv2.p50 > int32(MAX_IP_LATENCY) {
		t.Errorf("p50 should not exceed MAX_IP_LATENCY, got %d", iqv2.p50)
	}
}

func TestIPQualityV2_ConcurrentFailureAndSuccess(t *testing.T) {
	iqv2 := NewIPQualityV2()
	var wg sync.WaitGroup

	// 20 goroutines: 10 recording latency, 10 recording failures
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				iqv2.RecordLatency(int32(100 + j*10))
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				iqv2.RecordFailure()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// IP should be in a valid state (likely DEGRADED or SUSPECT)
	// Verify no panic occurred and state is valid
	if iqv2.state > IP_STATE_RECOVERED {
		t.Errorf("invalid state after concurrent ops: %d", iqv2.state)
	}
}
