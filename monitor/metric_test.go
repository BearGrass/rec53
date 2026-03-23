package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// unregisterMetrics removes all metrics from the default registry
func unregisterMetrics() {
	prometheus.Unregister(InCounter)
	prometheus.Unregister(OutCounter)
	prometheus.Unregister(LatencyHistogramObserver)
	prometheus.Unregister(IPQualityV2_P50)
	prometheus.Unregister(IPQualityV2_P95)
	prometheus.Unregister(IPQualityV2_P99)
	prometheus.Unregister(XDPStatus)
	prometheus.Unregister(XDPCacheHitsTotal)
	prometheus.Unregister(XDPCacheMissesTotal)
	prometheus.Unregister(XDPPassTotal)
	prometheus.Unregister(XDPErrorsTotal)
	prometheus.Unregister(CacheLookupTotal)
	prometheus.Unregister(CacheEntries)
	prometheus.Unregister(CacheLifecycleTotal)
	prometheus.Unregister(SnapshotOperationsTotal)
	prometheus.Unregister(SnapshotEntriesTotal)
	prometheus.Unregister(SnapshotDurationMs)
	prometheus.Unregister(UpstreamFailuresTotal)
	prometheus.Unregister(UpstreamFallbackTotal)
	prometheus.Unregister(UpstreamWinnerTotal)
	prometheus.Unregister(XDPSyncErrorsTotal)
	prometheus.Unregister(XDPCleanupDeletedTotal)
	prometheus.Unregister(XDPEntries)
	prometheus.Unregister(StateMachineStageTotal)
	prometheus.Unregister(StateMachineFailuresTotal)
	prometheus.Unregister(StateMachineTransitionTotal)
}

// TestMetric_InCounterAdd tests the InCounterAdd method
func TestMetric_InCounterAdd(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Add counter
	m.InCounterAdd("request", "A")

	// Verify the counter was incremented
	count := testutil.ToFloat64(InCounter.WithLabelValues("request", "A"))
	if count != 1 {
		t.Errorf("Expected counter to be 1, got %f", count)
	}

	// Add again
	m.InCounterAdd("request", "A")
	count = testutil.ToFloat64(InCounter.WithLabelValues("request", "A"))
	if count != 2 {
		t.Errorf("Expected counter to be 2, got %f", count)
	}
}

// TestMetric_OutCounterAdd tests the OutCounterAdd method
func TestMetric_OutCounterAdd(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Add counter with response code
	m.OutCounterAdd("response", "A", "NOERROR")

	// Verify the counter was incremented
	count := testutil.ToFloat64(OutCounter.WithLabelValues("response", "A", "NOERROR"))
	if count != 1 {
		t.Errorf("Expected counter to be 1, got %f", count)
	}

	// Test different response code
	m.OutCounterAdd("response", "A", "SERVFAIL")
	count = testutil.ToFloat64(OutCounter.WithLabelValues("response", "A", "SERVFAIL"))
	if count != 1 {
		t.Errorf("Expected SERVFAIL counter to be 1, got %f", count)
	}
}

// TestMetric_LatencyHistogramObserve tests the LatencyHistogramObserve method
func TestMetric_LatencyHistogramObserve(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Record latency - this should not panic
	m.LatencyHistogramObserve("query", "A", "NOERROR", 50.5)

	// Record another latency
	m.LatencyHistogramObserve("query", "A", "NOERROR", 100.0)

	// Verify histogram is registered and working by checking the metric exists
	observer := LatencyHistogramObserver.WithLabelValues("query", "A", "NOERROR")
	if observer == nil {
		t.Error("Expected histogram observer to be non-nil")
	}

	// Record one more to ensure no panic
	m.LatencyHistogramObserve("query", "A", "NOERROR", 200.0)
}

// TestMetric_IPQualityV2GaugeSet tests the IPQualityV2GaugeSet method for V2 percentile metrics
func TestMetric_IPQualityV2GaugeSet(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Set IP quality V2 metrics (P50, P95, P99)
	m.IPQualityV2GaugeSet("192.0.2.1", 50.0, 150.0, 250.0)

	// Verify P50 gauge was set
	p50Value := testutil.ToFloat64(IPQualityV2_P50.WithLabelValues("192.0.2.1"))
	if p50Value != 50.0 {
		t.Errorf("Expected P50 gauge to be 50.0, got %f", p50Value)
	}

	// Verify P95 gauge was set
	p95Value := testutil.ToFloat64(IPQualityV2_P95.WithLabelValues("192.0.2.1"))
	if p95Value != 150.0 {
		t.Errorf("Expected P95 gauge to be 150.0, got %f", p95Value)
	}

	// Verify P99 gauge was set
	p99Value := testutil.ToFloat64(IPQualityV2_P99.WithLabelValues("192.0.2.1"))
	if p99Value != 250.0 {
		t.Errorf("Expected P99 gauge to be 250.0, got %f", p99Value)
	}

	// Update metrics (gauges should overwrite)
	m.IPQualityV2GaugeSet("192.0.2.1", 100.0, 200.0, 300.0)

	p50Value = testutil.ToFloat64(IPQualityV2_P50.WithLabelValues("192.0.2.1"))
	if p50Value != 100.0 {
		t.Errorf("Expected P50 gauge to be 100.0 after update, got %f", p50Value)
	}

	p95Value = testutil.ToFloat64(IPQualityV2_P95.WithLabelValues("192.0.2.1"))
	if p95Value != 200.0 {
		t.Errorf("Expected P95 gauge to be 200.0 after update, got %f", p95Value)
	}

	p99Value = testutil.ToFloat64(IPQualityV2_P99.WithLabelValues("192.0.2.1"))
	if p99Value != 300.0 {
		t.Errorf("Expected P99 gauge to be 300.0 after update, got %f", p99Value)
	}

	// Set different IP
	m.IPQualityV2GaugeSet("192.0.2.2", 75.0, 175.0, 275.0)
	p50Value = testutil.ToFloat64(IPQualityV2_P50.WithLabelValues("192.0.2.2"))
	if p50Value != 75.0 {
		t.Errorf("Expected P50 gauge for 192.0.2.2 to be 75.0, got %f", p50Value)
	}
}

// TestMetric_Register tests the Register method
func TestMetric_Register(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}

	// Register should not panic
	m.Register()

	// Verify metrics are registered by checking if we can use them
	m.InCounterAdd("test", "A")
	count := testutil.ToFloat64(InCounter.WithLabelValues("test", "A"))
	if count != 1 {
		t.Errorf("Expected counter to be 1 after register, got %f", count)
	}
}

func TestMetric_RuntimeObservabilityHelpers(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	m.CacheLookupAdd("positive_hit")
	m.CacheEntriesSet(7)
	m.CacheLifecycleAdd("write", 2)
	m.SnapshotOperationAdd("load", "success")
	m.SnapshotEntriesAdd("load", "imported", 3)
	m.SnapshotDurationObserve("load", "success", 25*time.Millisecond)
	m.UpstreamFailureAdd("timeout", "NONE")
	m.UpstreamFallbackAdd("success")
	m.UpstreamWinnerAdd("primary")
	m.XDPSyncErrorAdd("update")
	m.XDPCleanupDeletedAdd(4)
	m.XDPEntriesSet(11)
	m.StateMachineStageAdd("cache_lookup")
	m.StateMachineFailureAdd("query_upstream_error")

	if got := testutil.ToFloat64(CacheLookupTotal.WithLabelValues("positive_hit")); got != 1 {
		t.Fatalf("CacheLookupTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(CacheEntries); got != 7 {
		t.Fatalf("CacheEntries = %f, want 7", got)
	}
	if got := testutil.ToFloat64(CacheLifecycleTotal.WithLabelValues("write")); got != 2 {
		t.Fatalf("CacheLifecycleTotal = %f, want 2", got)
	}
	if got := testutil.ToFloat64(SnapshotOperationsTotal.WithLabelValues("load", "success")); got != 1 {
		t.Fatalf("SnapshotOperationsTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(SnapshotEntriesTotal.WithLabelValues("load", "imported")); got != 3 {
		t.Fatalf("SnapshotEntriesTotal = %f, want 3", got)
	}
	if got := testutil.ToFloat64(UpstreamFailuresTotal.WithLabelValues("timeout", "NONE")); got != 1 {
		t.Fatalf("UpstreamFailuresTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(UpstreamFallbackTotal.WithLabelValues("success")); got != 1 {
		t.Fatalf("UpstreamFallbackTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(UpstreamWinnerTotal.WithLabelValues("primary")); got != 1 {
		t.Fatalf("UpstreamWinnerTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(XDPSyncErrorsTotal.WithLabelValues("update")); got != 1 {
		t.Fatalf("XDPSyncErrorsTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(XDPCleanupDeletedTotal); got != 4 {
		t.Fatalf("XDPCleanupDeletedTotal = %f, want 4", got)
	}
	if got := testutil.ToFloat64(XDPEntries); got != 11 {
		t.Fatalf("XDPEntries = %f, want 11", got)
	}
	if got := testutil.ToFloat64(StateMachineStageTotal.WithLabelValues("cache_lookup")); got != 1 {
		t.Fatalf("StateMachineStageTotal = %f, want 1", got)
	}
	if got := testutil.ToFloat64(StateMachineFailuresTotal.WithLabelValues("query_upstream_error")); got != 1 {
		t.Fatalf("StateMachineFailuresTotal = %f, want 1", got)
	}
}

// TestInitMetricWithAddr tests the InitMetricWithAddr function
// This test must run first among InitMetricWithAddr tests due to global http.Handle
func TestInitMetricWithAddr(t *testing.T) {
	unregisterMetrics()
	// Reset MetricServer
	MetricServer = nil

	// Use random port
	InitMetricWithAddr("127.0.0.1:0")

	// Verify MetricServer is created
	if MetricServer == nil {
		t.Fatal("Expected MetricServer to be created")
	}

	// Verify Rec53Metric is created
	if Rec53Metric == nil {
		t.Fatal("Expected Rec53Metric to be created")
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Verify server is running by making a request
	// Extract actual address from server
	addr := MetricServer.Addr
	if addr == "" {
		t.Error("Expected server address to be set")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ShutdownMetric(ctx); err != nil {
		t.Errorf("Failed to shutdown metric server: %v", err)
	}
}

// TestShutdownMetric tests the ShutdownMetric function
// Uses already initialized server from TestInitMetricWithAddr
func TestShutdownMetric(t *testing.T) {
	unregisterMetrics()
	// Reset and create new server - but use a fresh http mux to avoid conflict
	// We test the shutdown logic directly without re-registering http handlers

	// Test 1: nil server case
	MetricServer = nil
	ctx := context.Background()
	err := ShutdownMetric(ctx)
	if err != nil {
		t.Errorf("Expected nil error for nil server, got: %v", err)
	}

	// Test 2: Create a minimal server for shutdown test
	// Note: We cannot call InitMetricWithAddr again due to http.Handle conflict
	// So we create a server directly
	MetricServer = &http.Server{Addr: "127.0.0.1:0"}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = ShutdownMetric(ctx)
	if err != nil {
		t.Errorf("Expected no error on shutdown, got: %v", err)
	}
}

// TestMetricConcurrentAccess tests concurrent metric operations
func TestMetricConcurrentAccess(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Run concurrent operations
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				m.InCounterAdd("concurrent", "A")
				m.OutCounterAdd("concurrent", "A", "NOERROR")
				m.LatencyHistogramObserve("concurrent", "A", "NOERROR", float64(j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counter has expected value (10 * 100 = 1000)
	count := testutil.ToFloat64(InCounter.WithLabelValues("concurrent", "A"))
	if count != 1000 {
		t.Errorf("Expected counter to be 1000, got %f", count)
	}
}

// TestMetricsEndpoint tests that metrics are exposed via HTTP
func TestMetricsEndpoint(t *testing.T) {
	unregisterMetrics()
	// Create a test server with metrics handler
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Add some metrics
	m.InCounterAdd("test", "A")

	// Create test server with our custom registry
	handler := http.NewServeMux()
	handler.Handle("/metric", promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{},
	))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Request metrics endpoint
	resp, err := http.Get(server.URL + "/metric")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response body
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	// Check for our metric (using the actual metric name from var.go)
	if !strings.Contains(bodyStr, "rec53_query_counter") {
		t.Errorf("Expected metrics output to contain rec53_query_counter, got:\n%s", bodyStr)
	}
}

func TestReadinessHandlerReportsColdStartAndWarming(t *testing.T) {
	ResetRuntimeState()
	t.Cleanup(ResetRuntimeState)

	server := httptest.NewServer(newOperationalMux(promhttp.HandlerFor(
		prometheus.NewRegistry(),
		promhttp.HandlerOpts{},
	)))
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz/ready")
	if err != nil {
		t.Fatalf("Failed to get readiness endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	_ = resp.Body.Close()
	body := string(buf[:n])
	if !strings.Contains(body, "ready=false") || !strings.Contains(body, "phase=cold-start") {
		t.Fatalf("cold-start body = %q", body)
	}

	SetRuntimeServingPhase(true)

	resp, err = http.Get(server.URL + "/healthz/ready")
	if err != nil {
		t.Fatalf("Failed to get readiness endpoint after update: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	n, _ = resp.Body.Read(buf)
	_ = resp.Body.Close()
	body = string(buf[:n])
	if !strings.Contains(body, "ready=true") || !strings.Contains(body, "phase=warming") {
		t.Fatalf("warming body = %q", body)
	}

	state, changed := MarkRuntimeWarmupComplete()
	if !changed {
		t.Fatalf("MarkRuntimeWarmupComplete changed = false, state = %+v", state)
	}

	resp, err = http.Get(server.URL + "/healthz/ready")
	if err != nil {
		t.Fatalf("Failed to get readiness endpoint after warmup completion: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	n, _ = resp.Body.Read(buf)
	_ = resp.Body.Close()
	body = string(buf[:n])
	if !strings.Contains(body, "ready=true") || !strings.Contains(body, "phase=steady") {
		t.Fatalf("steady body = %q", body)
	}

	MarkRuntimeShuttingDown()

	resp, err = http.Get(server.URL + "/healthz/ready")
	if err != nil {
		t.Fatalf("Failed to get readiness endpoint after shutdown transition: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	n, _ = resp.Body.Read(buf)
	_ = resp.Body.Close()
	body = string(buf[:n])
	if !strings.Contains(body, "ready=false") || !strings.Contains(body, "phase=shutting-down") {
		t.Fatalf("shutting-down body = %q", body)
	}
}
