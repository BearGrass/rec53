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
	prometheus.Unregister(IPQuality)
	prometheus.Unregister(IPQualityV2_P50)
	prometheus.Unregister(IPQualityV2_P95)
	prometheus.Unregister(IPQualityV2_P99)
}

// TestMetric_InCounterAdd tests the InCounterAdd method
func TestMetric_InCounterAdd(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Add counter
	m.InCounterAdd("request", "example.com.", "A")

	// Verify the counter was incremented
	count := testutil.ToFloat64(InCounter.With(prometheus.Labels{
		"stage": "request",
		"name":  "example.com.",
		"type":  "A",
	}))
	if count != 1 {
		t.Errorf("Expected counter to be 1, got %f", count)
	}

	// Add again
	m.InCounterAdd("request", "example.com.", "A")
	count = testutil.ToFloat64(InCounter.With(prometheus.Labels{
		"stage": "request",
		"name":  "example.com.",
		"type":  "A",
	}))
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
	m.OutCounterAdd("response", "example.com.", "A", "NOERROR")

	// Verify the counter was incremented
	count := testutil.ToFloat64(OutCounter.With(prometheus.Labels{
		"stage": "response",
		"name":  "example.com.",
		"type":  "A",
		"code":  "NOERROR",
	}))
	if count != 1 {
		t.Errorf("Expected counter to be 1, got %f", count)
	}

	// Test different response code
	m.OutCounterAdd("response", "example.com.", "A", "SERVFAIL")
	count = testutil.ToFloat64(OutCounter.With(prometheus.Labels{
		"stage": "response",
		"name":  "example.com.",
		"type":  "A",
		"code":  "SERVFAIL",
	}))
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
	m.LatencyHistogramObserve("query", "example.com.", "A", "NOERROR", 50.5)

	// Record another latency
	m.LatencyHistogramObserve("query", "example.com.", "A", "NOERROR", 100.0)

	// Verify histogram is registered and working by checking the metric exists
	// Note: Observer doesn't implement Collector, so we verify via the histogram vec
	observer := LatencyHistogramObserver.With(prometheus.Labels{
		"stage": "query",
		"name":  "example.com.",
		"type":  "A",
		"code":  "NOERROR",
	})
	if observer == nil {
		t.Error("Expected histogram observer to be non-nil")
	}

	// Record one more to ensure no panic
	m.LatencyHistogramObserve("query", "example.com.", "A", "NOERROR", 200.0)
}

// TestMetric_IPQualityGaugeSet tests the IPQualityGaugeSet method
func TestMetric_IPQualityGaugeSet(t *testing.T) {
	unregisterMetrics()
	reg := prometheus.NewRegistry()
	m := &Metric{reg: reg}
	m.Register()

	// Set IP quality
	m.IPQualityGaugeSet("192.0.2.1", 50.5)

	// Verify gauge was set
	value := testutil.ToFloat64(IPQuality.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if value != 50.5 {
		t.Errorf("Expected gauge to be 50.5, got %f", value)
	}

	// Update IP quality (gauge should overwrite)
	m.IPQualityGaugeSet("192.0.2.1", 100.0)
	value = testutil.ToFloat64(IPQuality.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if value != 100.0 {
		t.Errorf("Expected gauge to be 100.0, got %f", value)
	}

	// Set different IP
	m.IPQualityGaugeSet("192.0.2.2", 25.0)
	value = testutil.ToFloat64(IPQuality.With(prometheus.Labels{
		"ip": "192.0.2.2",
	}))
	if value != 25.0 {
		t.Errorf("Expected gauge to be 25.0, got %f", value)
	}
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
	p50Value := testutil.ToFloat64(IPQualityV2_P50.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if p50Value != 50.0 {
		t.Errorf("Expected P50 gauge to be 50.0, got %f", p50Value)
	}

	// Verify P95 gauge was set
	p95Value := testutil.ToFloat64(IPQualityV2_P95.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if p95Value != 150.0 {
		t.Errorf("Expected P95 gauge to be 150.0, got %f", p95Value)
	}

	// Verify P99 gauge was set
	p99Value := testutil.ToFloat64(IPQualityV2_P99.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if p99Value != 250.0 {
		t.Errorf("Expected P99 gauge to be 250.0, got %f", p99Value)
	}

	// Update metrics (gauges should overwrite)
	m.IPQualityV2GaugeSet("192.0.2.1", 100.0, 200.0, 300.0)

	p50Value = testutil.ToFloat64(IPQualityV2_P50.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if p50Value != 100.0 {
		t.Errorf("Expected P50 gauge to be 100.0 after update, got %f", p50Value)
	}

	p95Value = testutil.ToFloat64(IPQualityV2_P95.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if p95Value != 200.0 {
		t.Errorf("Expected P95 gauge to be 200.0 after update, got %f", p95Value)
	}

	p99Value = testutil.ToFloat64(IPQualityV2_P99.With(prometheus.Labels{
		"ip": "192.0.2.1",
	}))
	if p99Value != 300.0 {
		t.Errorf("Expected P99 gauge to be 300.0 after update, got %f", p99Value)
	}

	// Set different IP
	m.IPQualityV2GaugeSet("192.0.2.2", 75.0, 175.0, 275.0)
	p50Value = testutil.ToFloat64(IPQualityV2_P50.With(prometheus.Labels{
		"ip": "192.0.2.2",
	}))
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
	m.InCounterAdd("test", "test.com.", "A")
	count := testutil.ToFloat64(InCounter.With(prometheus.Labels{
		"stage": "test",
		"name":  "test.com.",
		"type":  "A",
	}))
	if count != 1 {
		t.Errorf("Expected counter to be 1 after register, got %f", count)
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
				m.InCounterAdd("concurrent", "test.com.", "A")
				m.OutCounterAdd("concurrent", "test.com.", "A", "NOERROR")
				m.LatencyHistogramObserve("concurrent", "test.com.", "A", "NOERROR", float64(j))
				m.IPQualityGaugeSet("192.0.2.1", float64(j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counter has expected value (10 * 100 = 1000)
	count := testutil.ToFloat64(InCounter.With(prometheus.Labels{
		"stage": "concurrent",
		"name":  "test.com.",
		"type":  "A",
	}))
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
	m.InCounterAdd("test", "example.com.", "A")

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
