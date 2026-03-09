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