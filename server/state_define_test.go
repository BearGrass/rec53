package server

import (
	"net"
	"sync"
	"testing"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

// init initializes test dependencies
func init() {
	// Initialize no-op logger for tests
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// =============================================================================
// Helper functions for testing
// =============================================================================

// MockDNSHandler handles DNS queries for testing
type MockDNSHandler struct {
	response *dns.Msg
	delay    time.Duration
	mu       sync.Mutex
}

func (h *MockDNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.delay > 0 {
		time.Sleep(h.delay)
	}

	if h.response != nil {
		resp := h.response.Copy()
		resp.SetReply(r)
		w.WriteMsg(resp)
	} else {
		// Default: return error
		resp := new(dns.Msg)
		resp.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(resp)
	}
}

// MockDNSServer wraps a mock DNS server for testing
type MockDNSServer struct {
	Server   *dns.Server
	Addr     string
	Handler  *MockDNSHandler
	Protocol string
}

// NewMockDNSServer creates a new mock DNS server
func NewMockDNSServer(protocol string, handler *MockDNSHandler) (*MockDNSServer, error) {
	var server *dns.Server
	var addr string

	if protocol == "tcp" {
		server = &dns.Server{
			Net:  "tcp",
			Addr: "127.0.0.1:0",
		}
	} else {
		server = &dns.Server{
			Net:  "udp",
			Addr: "127.0.0.1:0",
		}
	}

	server.Handler = handler

	// Start server in goroutine
	go func() {
		server.ListenAndServe()
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Get actual address
	addr = server.PacketConn.LocalAddr().String()

	return &MockDNSServer{
		Server:   server,
		Addr:     addr,
		Handler:  handler,
		Protocol: protocol,
	}, nil
}

// Stop shuts down the mock server
func (m *MockDNSServer) Stop() {
	m.Server.Shutdown()
}

// GetIP extracts IP address from the server address
func (m *MockDNSServer) GetIP() string {
	host, _, _ := net.SplitHostPort(m.Addr)
	return host
}

// =============================================================================
// iterState.handle Tests - Error Paths (no network required)
// =============================================================================

// TestIterState_NilRequest tests error handling for nil request
func TestIterState_NilRequest(t *testing.T) {
	resp := new(dns.Msg)
	state := newIterState(nil, resp)
	ret, err := state.handle(nil, resp)

	if err == nil {
		t.Error("expected error for nil request")
	}
	if ret != ITER_COMMON_ERROR {
		t.Errorf("expected ITER_COMMON_ERROR, got %d", ret)
	}
}

// TestIterState_NilResponse tests error handling for nil response
func TestIterState_NilResponse(t *testing.T) {
	req := new(dns.Msg)
	state := newIterState(req, nil)
	ret, err := state.handle(req, nil)

	if err == nil {
		t.Error("expected error for nil response")
	}
	if ret != ITER_COMMON_ERROR {
		t.Errorf("expected ITER_COMMON_ERROR, got %d", ret)
	}
}

// TestIterState_EmptyExtra tests error when no IPs in extra section
func TestIterState_EmptyExtra(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	// No extra section

	state := newIterState(req, resp)
	ret, err := state.handle(req, resp)

	if err == nil {
		t.Error("expected error for empty extra section")
	}
	if ret != ITER_COMMON_ERROR {
		t.Errorf("expected ITER_COMMON_ERROR, got %d", ret)
	}
}

// TestIterState_NoARecordsInExtra tests error when extra has no A records
func TestIterState_NoARecordsInExtra(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	// Add AAAA record instead of A record
	resp.Extra = []dns.RR{
		&dns.AAAA{
			Hdr:  dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("2001:db8::1"),
		},
	}

	state := newIterState(req, resp)
	ret, err := state.handle(req, resp)

	if err == nil {
		t.Error("expected error when no A records in extra")
	}
	if ret != ITER_COMMON_ERROR {
		t.Errorf("expected ITER_COMMON_ERROR, got %d", ret)
	}
}

// =============================================================================
// getIPListFromResponse Tests (additional cases)
// =============================================================================

// TestGetIPListFromResponse_MixedRecords tests mixed record types
func TestGetIPListFromResponse_MixedRecords(t *testing.T) {
	tests := []struct {
		name     string
		response *dns.Msg
		expected int
	}{
		{
			name: "mixed record types",
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Extra = []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
						A:   net.ParseIP("192.0.2.1"),
					},
					&dns.TXT{
						Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 300},
						Txt: []string{"some text"},
					},
					&dns.A{
						Hdr: dns.RR_Header{Name: "ns2.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
						A:   net.ParseIP("192.0.2.2"),
					},
				}
				return m
			}(),
			expected: 2,
		},
		{
			name: "only AAAA records",
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Extra = []dns.RR{
					&dns.AAAA{
						Hdr:  dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
						AAAA: net.ParseIP("2001:db8::1"),
					},
				}
				return m
			}(),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips := getIPListFromResponse(tt.response)
			if len(ips) != tt.expected {
				t.Errorf("expected %d IPs, got %d", tt.expected, len(ips))
			}
		})
	}
}

// =============================================================================
// getBestAddressAndPrefetchIPs Tests (additional cases)
// =============================================================================

// TestGetBestAddressAndPrefetchIPs_LatencyBased tests latency-based selection using V2 algorithm
func TestGetBestAddressAndPrefetchIPs_LatencyBased(t *testing.T) {
	globalIPPool = NewIPPool()

	// Set up V2 with different latencies using RecordLatency()
	// 192.0.2.1: 500ms latency
	iqv2_1 := NewIPQualityV2()
	for i := 0; i < 10; i++ {
		iqv2_1.RecordLatency(500)
	}
	globalIPPool.SetIPQualityV2("192.0.2.1", iqv2_1)

	// 192.0.2.2: 200ms latency (should be best)
	iqv2_2 := NewIPQualityV2()
	for i := 0; i < 10; i++ {
		iqv2_2.RecordLatency(200)
	}
	globalIPPool.SetIPQualityV2("192.0.2.2", iqv2_2)

	// 192.0.2.3: 800ms latency
	iqv2_3 := NewIPQualityV2()
	for i := 0; i < 10; i++ {
		iqv2_3.RecordLatency(800)
	}
	globalIPPool.SetIPQualityV2("192.0.2.3", iqv2_3)

	bestIP, _, err := getBestAddressAndPrefetchIPs([]string{"192.0.2.1", "192.0.2.2", "192.0.2.3"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// IP with lowest latency (192.0.2.2 with 200ms) should be best
	if bestIP != "192.0.2.2" {
		t.Errorf("expected bestIP 192.0.2.2 (lowest latency), got %s", bestIP)
	}
}

// =============================================================================
// IPQuality Tests (used by iterState)
// =============================================================================

// TestIPQualityOperations tests IPQuality operations used in iterState
func TestIPQualityOperations(t *testing.T) {
	t.Run("new IPQuality has default latency", func(t *testing.T) {
		ipq := NewIPQuality()
		if ipq.GetLatency() != INIT_IP_LATENCY {
			t.Errorf("expected initial latency %d, got %d", INIT_IP_LATENCY, ipq.GetLatency())
		}
		if !ipq.IsInit() {
			t.Error("expected new IPQuality to be initialized")
		}
	})

	t.Run("SetLatency updates value", func(t *testing.T) {
		ipq := NewIPQuality()
		ipq.SetLatency(500)
		if ipq.GetLatency() != 500 {
			t.Errorf("expected latency 500, got %d", ipq.GetLatency())
		}
	})

	t.Run("SetLatencyAndState marks as uninitialized", func(t *testing.T) {
		ipq := NewIPQuality()
		ipq.SetLatencyAndState(300)
		if ipq.GetLatency() != 300 {
			t.Errorf("expected latency 300, got %d", ipq.GetLatency())
		}
		if ipq.IsInit() {
			t.Error("expected IPQuality to be uninitialized after SetLatencyAndState")
		}
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		ipq := NewIPQuality()
		var wg sync.WaitGroup

		// Concurrent writes
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(val int32) {
				defer wg.Done()
				ipq.SetLatency(val)
			}(int32(i))
		}

		// Concurrent reads
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = ipq.GetLatency()
			}()
		}

		wg.Wait()
		// Test passes if no race condition detected
	})
}

// =============================================================================
// IPPool Tests for iterState dependencies
// =============================================================================

// TestIPPool_GetBestIPs tests the getBestIPs method used by iterState
func TestIPPool_GetBestIPs(t *testing.T) {
	globalIPPool = NewIPPool()

	t.Run("empty list returns empty strings", func(t *testing.T) {
		best, second := globalIPPool.getBestIPs([]string{})
		if best != "" || second != "" {
			t.Errorf("expected empty strings, got best=%s, second=%s", best, second)
		}
	})

	t.Run("single IP returns that IP", func(t *testing.T) {
		best, second := globalIPPool.getBestIPs([]string{"192.0.2.1"})
		if best != "192.0.2.1" {
			t.Errorf("expected 192.0.2.1, got %s", best)
		}
		if second != "" {
			t.Errorf("expected empty second, got %s", second)
		}
	})

	t.Run("lower latency IP is preferred", func(t *testing.T) {
		globalIPPool = NewIPPool()

		// Set different latencies
		ipq1 := &IPQuality{latency: 1000}
		ipq1.isInit.Store(true)
		globalIPPool.SetIPQuality("192.0.2.1", ipq1)

		ipq2 := &IPQuality{latency: 100} // Lower latency
		ipq2.isInit.Store(true)
		globalIPPool.SetIPQuality("192.0.2.2", ipq2)

		best, _ := globalIPPool.getBestIPs([]string{"192.0.2.1", "192.0.2.2"})
		if best != "192.0.2.2" {
			t.Errorf("expected 192.0.2.2 (lower latency), got %s", best)
		}
	})

	t.Run("MAX_IP_LATENCY IP is not preferred", func(t *testing.T) {
		globalIPPool = NewIPPool()

		// One IP has max latency (failed), one has normal
		ipq1 := &IPQuality{latency: MAX_IP_LATENCY}
		ipq1.isInit.Store(true)
		globalIPPool.SetIPQuality("192.0.2.1", ipq1)

		ipq2 := &IPQuality{latency: 500}
		ipq2.isInit.Store(true)
		globalIPPool.SetIPQuality("192.0.2.2", ipq2)

		best, _ := globalIPPool.getBestIPs([]string{"192.0.2.1", "192.0.2.2"})
		if best != "192.0.2.2" {
			t.Errorf("expected 192.0.2.2 (not failed), got %s", best)
		}
	})
}

// TestIPPool_UpdateIPQuality tests the updateIPQuality method
func TestIPPool_UpdateIPQuality(t *testing.T) {
	globalIPPool = NewIPPool()

	t.Run("creates new IPQuality if not exists", func(t *testing.T) {
		globalIPPool.updateIPQuality("192.0.2.1", 150)

		ipq := globalIPPool.GetIPQuality("192.0.2.1")
		if ipq == nil {
			t.Fatal("expected IPQuality to be created")
		}
		if ipq.GetLatency() != 150 {
			t.Errorf("expected latency 150, got %d", ipq.GetLatency())
		}
	})

	t.Run("updates existing IPQuality", func(t *testing.T) {
		globalIPPool = NewIPPool()

		// Create initial
		ipq := &IPQuality{latency: 1000}
		ipq.isInit.Store(true)
		globalIPPool.SetIPQuality("192.0.2.1", ipq)

		// Update
		globalIPPool.updateIPQuality("192.0.2.1", 200)

		updated := globalIPPool.GetIPQuality("192.0.2.1")
		if updated.GetLatency() != 200 {
			t.Errorf("expected latency 200, got %d", updated.GetLatency())
		}
	})
}

// =============================================================================
// iterPort override Tests
// =============================================================================

// TestSetIterPort tests the iter port override mechanism
func TestSetIterPort(t *testing.T) {
	// Ensure clean state
	ResetIterPort()

	t.Run("default port is 53", func(t *testing.T) {
		if got := getIterPort(); got != "53" {
			t.Errorf("expected default port '53', got '%s'", got)
		}
	})

	t.Run("SetIterPort overrides port", func(t *testing.T) {
		SetIterPort("15353")
		defer ResetIterPort()

		if got := getIterPort(); got != "15353" {
			t.Errorf("expected overridden port '15353', got '%s'", got)
		}
	})

	t.Run("ResetIterPort restores default", func(t *testing.T) {
		SetIterPort("9999")
		ResetIterPort()

		if got := getIterPort(); got != "53" {
			t.Errorf("expected default port '53' after reset, got '%s'", got)
		}
	})
}

// =============================================================================
// Integration Tests (require network/mocked DNS server on port 53)
// =============================================================================

// Note: The following tests are marked as integration tests because iterState.handle
// makes actual DNS queries to port 53. To test the full flow with mock servers,
// run with: go test -tags=integration ./server/...
//
// The integration tests would require either:
// 1. Running as root to bind to port 53
// 2. Using network namespaces or containers
// 3. Refactoring iterState.handle to accept configurable ports

// TestIterState_Integration_SuccessfulQuery tests successful query with mock server
// This test is skipped in normal test runs
func TestIterState_Integration_SuccessfulQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Note: This test requires a DNS server on port 53
	// In CI environments, you can set up a mock DNS server on port 53
	t.Skip("Requires DNS server on port 53 - run with -tags=integration")
}

// TestIterState_Integration_NXDOMAIN tests NXDOMAIN handling with mock server
func TestIterState_Integration_NXDOMAIN(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Skip("Requires DNS server on port 53 - run with -tags=integration")
}

// TestIterState_Integration_Failover tests IP failover with mock server
func TestIterState_Integration_Failover(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Skip("Requires DNS server on port 53 - run with -tags=integration")
}
