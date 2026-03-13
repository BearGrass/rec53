package server

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"rec53/monitor"
	"rec53/utils"

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
	state := newQueryUpstreamState(nil, resp)
	ret, err := state.handle(nil, resp)

	if err == nil {
		t.Error("expected error for nil request")
	}
	if ret != QUERY_UPSTREAM_COMMON_ERROR {
		t.Errorf("expected QUERY_UPSTREAM_COMMON_ERROR, got %d", ret)
	}
}

// TestIterState_NilResponse tests error handling for nil response
func TestIterState_NilResponse(t *testing.T) {
	req := new(dns.Msg)
	state := newQueryUpstreamState(req, nil)
	ret, err := state.handle(req, nil)

	if err == nil {
		t.Error("expected error for nil response")
	}
	if ret != QUERY_UPSTREAM_COMMON_ERROR {
		t.Errorf("expected QUERY_UPSTREAM_COMMON_ERROR, got %d", ret)
	}
}

// TestIterState_EmptyExtra tests error when no IPs in extra section
func TestIterState_EmptyExtra(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	// No extra section

	state := newQueryUpstreamState(req, resp)
	ret, err := state.handle(req, resp)

	if err == nil {
		t.Error("expected error for empty extra section")
	}
	if ret != QUERY_UPSTREAM_COMMON_ERROR {
		t.Errorf("expected QUERY_UPSTREAM_COMMON_ERROR, got %d", ret)
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

	state := newQueryUpstreamState(req, resp)
	ret, err := state.handle(req, resp)

	if err == nil {
		t.Error("expected error when no A records in extra")
	}
	if ret != QUERY_UPSTREAM_COMMON_ERROR {
		t.Errorf("expected QUERY_UPSTREAM_COMMON_ERROR, got %d", ret)
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

// =============================================================================
// extractSOAFromAuthority Tests
// =============================================================================

// TestExtractSOAFromAuthority tests the extractSOAFromAuthority helper function
func TestExtractSOAFromAuthority(t *testing.T) {
	t.Run("returns SOA and its minttl", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns:      "ns1.example.com.",
				Mbox:    "admin.example.com.",
				Serial:  2021010101,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  600, // Negative cache TTL
			},
		}

		soa, ttl := extractSOAFromAuthority(response)
		if soa == nil {
			t.Fatal("expected SOA record, got nil")
		}
		if ttl != 600 {
			t.Errorf("expected TTL 600, got %d", ttl)
		}
	})

	t.Run("returns DefaultNegativeCacheTTL when SOA minttl is 0", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns:      "ns1.example.com.",
				Mbox:    "admin.example.com.",
				Serial:  2021010101,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  0, // Zero minttl
			},
		}

		soa, ttl := extractSOAFromAuthority(response)
		if soa == nil {
			t.Fatal("expected SOA record, got nil")
		}
		if ttl != DefaultNegativeCacheTTL {
			t.Errorf("expected TTL %d (default), got %d", DefaultNegativeCacheTTL, ttl)
		}
	})

	t.Run("returns nil when no SOA in authority", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns: "ns1.example.com.",
			},
		}

		soa, ttl := extractSOAFromAuthority(response)
		if soa != nil {
			t.Errorf("expected nil SOA, got %v", soa)
		}
		if ttl != 0 {
			t.Errorf("expected TTL 0, got %d", ttl)
		}
	})

	t.Run("returns nil when authority section is empty", func(t *testing.T) {
		response := new(dns.Msg)
		// Empty Ns section

		soa, ttl := extractSOAFromAuthority(response)
		if soa != nil {
			t.Errorf("expected nil SOA, got %v", soa)
		}
		if ttl != 0 {
			t.Errorf("expected TTL 0, got %d", ttl)
		}
	})

	t.Run("returns first SOA when multiple SOAs present", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns:      "ns1.example.com.",
				Mbox:    "admin.example.com.",
				Serial:  2021010101,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  100, // First SOA
			},
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns:      "ns2.example.com.",
				Mbox:    "admin2.example.com.",
				Serial:  2021010102,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  200, // Second SOA
			},
		}

		soa, ttl := extractSOAFromAuthority(response)
		if soa == nil {
			t.Fatal("expected SOA record, got nil")
		}
		if ttl != 100 {
			t.Errorf("expected TTL 100 (first SOA), got %d", ttl)
		}
		if soa.Ns != "ns1.example.com." {
			t.Errorf("expected first SOA's NS, got %s", soa.Ns)
		}
	})
}

// =============================================================================
// hasSOAInAuthority Tests
// =============================================================================

// TestHasSOAInAuthority tests the hasSOAInAuthority helper function
func TestHasSOAInAuthority(t *testing.T) {
	t.Run("returns true when SOA present", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns:      "ns1.example.com.",
				Mbox:    "admin.example.com.",
				Serial:  2021010101,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  600,
			},
		}

		if !hasSOAInAuthority(response) {
			t.Error("expected true when SOA present, got false")
		}
	})

	t.Run("returns false when no SOA", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns: "ns1.example.com.",
			},
		}

		if hasSOAInAuthority(response) {
			t.Error("expected false when no SOA, got true")
		}
	})

	t.Run("returns false when authority section empty", func(t *testing.T) {
		response := new(dns.Msg)
		// Empty Ns section

		if hasSOAInAuthority(response) {
			t.Error("expected false when authority empty, got true")
		}
	})

	t.Run("returns true when SOA mixed with other records", func(t *testing.T) {
		response := new(dns.Msg)
		response.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns: "ns1.example.com.",
			},
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   "example.com.",
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns:      "ns1.example.com.",
				Mbox:    "admin.example.com.",
				Serial:  2021010101,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  600,
			},
		}

		if !hasSOAInAuthority(response) {
			t.Error("expected true when SOA mixed with other records, got false")
		}
	})
}

// =============================================================================
// resolveNSIPsConcurrently bug-fix regression tests
// Covers: B1 (defer close(semaphore) panic) and B2 (dual-consumer deadlock).
// =============================================================================

// makeAResponse builds a minimal DNS A-record response for use in mock servers.
func makeAResponse(name, ip string) *dns.Msg {
	m := new(dns.Msg)
	m.Response = true
	m.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP(ip),
		},
	}
	return m
}

// TestResolveNSIPsConcurrentlyNoPanic verifies that calling resolveNSIPsConcurrently
// with many NS names does not panic due to a send-on-closed-channel (B1 fix).
// Run with -race to also catch data-race regressions.
func TestResolveNSIPsConcurrentlyNoPanic(t *testing.T) {
	// Redirect all DNS traffic to a local SERVFAIL mock so the state machine
	// fails fast (no real network needed, no 5-second DNS timeout).
	startWarmupTestMockDNS(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Use several NS names — enough to exercise the semaphore path.
	// The state machine will fail quickly (mock returns SERVFAIL) and return
	// nil, but must not panic.
	nsNames := []string{
		"ns1.example.com.", "ns2.example.com.", "ns3.example.com.",
		"ns4.example.com.", "ns5.example.com.",
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = resolveNSIPsConcurrently(ctx, nsNames)
	}()

	select {
	case <-done:
		// OK — no panic, no hang
	case <-time.After(2 * time.Second):
		t.Fatal("resolveNSIPsConcurrently hung — possible deadlock (B2)")
	}
}

// TestResolveNSIPsConcurrentlyContextCancelDoesNotHang verifies that a pre-cancelled
// context causes resolveNSIPsConcurrently to return promptly without blocking (B2 fix).
func TestResolveNSIPsConcurrentlyContextCancelDoesNotHang(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before calling

	nsNames := []string{"ns1.example.com.", "ns2.example.com.", "ns3.example.com."}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = resolveNSIPsConcurrently(ctx, nsNames)
	}()

	select {
	case <-done:
		// OK — returned promptly
	case <-time.After(3 * time.Second):
		t.Fatal("resolveNSIPsConcurrently did not return after context cancellation — possible deadlock (B2)")
	}
}

// TestInGlueStateNSRelevance verifies that inGlueState.handle validates NS zone
// relevance before accepting glue records. Stale NS from a prior CNAME hop must
// not be reused when they belong to a different delegation zone.
func TestInGlueStateNSRelevance(t *testing.T) {
	// Helper: build a dns.Msg with a question
	makeRequest := func(qname string) *dns.Msg {
		req := new(dns.Msg)
		req.SetQuestion(qname, dns.TypeA)
		return req
	}

	// Helper: build a dns.Msg with NS + Extra (glue)
	makeResponseWithNS := func(nsZone string) *dns.Msg {
		resp := new(dns.Msg)
		resp.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{
					Name:   nsZone,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				Ns: "ns1." + nsZone,
			},
		}
		resp.Extra = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ns1." + nsZone,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: net.ParseIP("1.2.3.4"),
			},
		}
		return resp
	}

	tests := []struct {
		name          string
		queryName     string
		nsZone        string // empty string means: use empty Ns/Extra
		wantCode      int
		wantNsCleared bool
	}{
		{
			name:          "NS zone is ancestor of query domain → EXTRACT_GLUE_EXIST",
			queryName:     "www.foo.akadns.net.",
			nsZone:        "akadns.net.",
			wantCode:      EXTRACT_GLUE_EXIST,
			wantNsCleared: false,
		},
		{
			name:          "NS zone equals query domain → EXTRACT_GLUE_EXIST",
			queryName:     "akadns.net.",
			nsZone:        "akadns.net.",
			wantCode:      EXTRACT_GLUE_EXIST,
			wantNsCleared: false,
		},
		{
			name:          "NS zone unrelated to query domain → EXTRACT_GLUE_NOT_EXIST, Ns cleared",
			queryName:     "www.huawei.com.c.cdnhwc1.com.",
			nsZone:        "akadns.net.",
			wantCode:      EXTRACT_GLUE_NOT_EXIST,
			wantNsCleared: true,
		},
		{
			name:          "NS zone is root → EXTRACT_GLUE_EXIST (universal ancestor)",
			queryName:     "www.example.com.",
			nsZone:        ".",
			wantCode:      EXTRACT_GLUE_EXIST,
			wantNsCleared: false,
		},
		{
			name:          "Empty Ns → EXTRACT_GLUE_NOT_EXIST",
			queryName:     "www.example.com.",
			nsZone:        "", // signals: build empty response
			wantCode:      EXTRACT_GLUE_NOT_EXIST,
			wantNsCleared: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeRequest(tt.queryName)

			var resp *dns.Msg
			if tt.nsZone == "" {
				resp = new(dns.Msg) // empty Ns and Extra
			} else {
				resp = makeResponseWithNS(tt.nsZone)
			}

			state := newExtractGlueState(req, resp)
			code, err := state.handle(req, resp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if code != tt.wantCode {
				t.Errorf("handle() = %d, want %d", code, tt.wantCode)
			}
			if tt.wantNsCleared {
				if len(resp.Ns) != 0 {
					t.Errorf("expected Ns to be cleared, got %d records", len(resp.Ns))
				}
				if len(resp.Extra) != 0 {
					t.Errorf("expected Extra to be cleared, got %d records", len(resp.Extra))
				}
			} else if tt.nsZone != "" {
				// Non-empty nsZone and not cleared: Ns should still be present
				if len(resp.Ns) == 0 {
					t.Errorf("expected Ns to be preserved but it was cleared")
				}
			}
		})
	}
}

// TestResolveNSIPsConcurrentlyEmptyInput verifies that an empty nsNames slice
// returns nil immediately without spawning any goroutines.
func TestResolveNSIPsConcurrentlyEmptyInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result := resolveNSIPsConcurrently(ctx, nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}

	result = resolveNSIPsConcurrently(ctx, []string{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}

// =============================================================================
// Cross-domain CNAME Integration Tests
// =============================================================================

// startCNAMEChainMockDNS starts a mock DNS server that responds to a three-hop
// cross-domain CNAME chain:
//
//	www.d1.test. → CNAME www.d2.test. → CNAME www.d3.test. → A 1.2.3.4
//
// The mock acts as both root and authoritative server for all test domains.
// Every response that delivers a delegation also includes an A glue record
// pointing back to the mock server so that iterState can reach it without
// needing a real network.
//
// The same helper also installs root-glue and iter-port overrides identical to
// startWarmupTestMockDNS so the full state machine is exercised in isolation.
//
// Returns the mock server port string (e.g. "54321").
func startCNAMEChainMockDNS(t *testing.T) (port string, mockIP string) {
	t.Helper()

	// We need to know the port before building glue records, so we start the
	// server on :0 and inject globals once the socket is ready.
	started := make(chan struct{})
	var portOnce sync.Once

	var srv *dns.Server
	srv = &dns.Server{
		Addr: "127.0.0.1:0",
		Net:  "udp",
		NotifyStartedFunc: func() {
			portOnce.Do(func() { close(started) })
		},
	}

	// Handler closed over srv so it can read the actual port after binding.
	srv.Handler = dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			resp := new(dns.Msg)
			resp.SetRcode(r, dns.RcodeServerFailure)
			w.WriteMsg(resp) //nolint:errcheck
			return
		}

		_, mockPort, _ := net.SplitHostPort(srv.PacketConn.LocalAddr().String())
		mockAddr := net.ParseIP("127.0.0.1")

		qname := r.Question[0].Name
		resp := new(dns.Msg)
		resp.SetReply(r)
		resp.RecursionAvailable = false
		resp.Authoritative = true

		makeNS := func(zone, nsName string) dns.RR {
			return &dns.NS{
				Hdr: dns.RR_Header{Name: zone, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60},
				Ns:  nsName,
			}
		}
		makeGlue := func(nsName string) dns.RR {
			return &dns.A{
				Hdr: dns.RR_Header{Name: nsName, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   mockAddr,
			}
		}
		_ = mockPort // port is encoded in the global iterPortOverride set below

		switch qname {
		case "www.d1.test.":
			// First hop: CNAME to a completely different domain.
			resp.Answer = []dns.RR{
				&dns.CNAME{
					Hdr:    dns.RR_Header{Name: "www.d1.test.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
					Target: "www.d2.test.",
				},
			}
			// Intentionally include NS for d1.test. (old domain) in the response.
			// After this CNAME, the state machine will switch the question to www.d2.test.
			// inGlueState MUST detect that d1.test. NS is unrelated to www.d2.test.
			// and return EXTRACT_GLUE_NOT_EXIST instead of EXTRACT_GLUE_EXIST.
			resp.Ns = []dns.RR{makeNS("d1.test.", "ns.d1.test.")}
			resp.Extra = []dns.RR{makeGlue("ns.d1.test.")}

		case "www.d2.test.":
			// Second hop: CNAME to yet another domain.
			resp.Answer = []dns.RR{
				&dns.CNAME{
					Hdr:    dns.RR_Header{Name: "www.d2.test.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
					Target: "www.d3.test.",
				},
			}
			// Include NS for d2.test. — again unrelated to www.d3.test.
			resp.Ns = []dns.RR{makeNS("d2.test.", "ns.d2.test.")}
			resp.Extra = []dns.RR{makeGlue("ns.d2.test.")}

		case "www.d3.test.":
			// Final hop: actual A record answer.
			resp.Answer = []dns.RR{
				&dns.A{
					Hdr: dns.RR_Header{Name: "www.d3.test.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP("1.2.3.4"),
				},
			}

		default:
			// For any other query (delegation, NS resolution, etc.) return a
			// referral to the mock server itself so the iterator can reach us.
			// This covers queries to d2.test., d3.test., test., etc.
			resp.Authoritative = false
			resp.Ns = []dns.RR{makeNS(".", "ns.mock-root.")}
			resp.Extra = []dns.RR{makeGlue("ns.mock-root.")}
		}

		w.WriteMsg(resp) //nolint:errcheck
	})

	go func() {
		srv.ListenAndServe() //nolint:errcheck
	}()
	<-started

	_, p, _ := net.SplitHostPort(srv.PacketConn.LocalAddr().String())

	// Build root glue pointing to our mock server.
	rootGlue := new(dns.Msg)
	rootGlue.SetUpdate(".")
	rootGlue.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 0},
			Ns:  "ns.mock-root.",
		},
	}
	rootGlue.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "ns.mock-root.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
			A:   net.ParseIP("127.0.0.1"),
		},
	}

	utils.SetRootGlue(rootGlue)
	SetIterPort(p)
	FlushCacheForTest()
	ResetIPPoolForTest()

	t.Cleanup(func() {
		srv.Shutdown()
		utils.ResetRootGlue()
		ResetIterPort()
		FlushCacheForTest()
		ResetIPPoolForTest()
	})

	return p, "127.0.0.1"
}

// TestCrossdomainCNAMEColdCacheResolves verifies that the resolver successfully
// follows a three-hop cross-domain CNAME chain on the very first query (cold
// cache), without returning SERVFAIL.
//
// The mock DNS server returns old-domain NS records alongside each CNAME
// response, deliberately triggering the scenario where inGlueState might
// incorrectly accept stale glue from a prior hop as valid for the new target
// domain. The fix in inGlueState.handle must detect this and return
// EXTRACT_GLUE_NOT_EXIST so the resolver fetches the correct delegation.
func TestCrossdomainCNAMEColdCacheResolves(t *testing.T) {
	startCNAMEChainMockDNS(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := new(dns.Msg)
	req.SetQuestion("www.d1.test.", dns.TypeA)
	resp := new(dns.Msg)

	stm := newStateInitStateWithContext(req, resp, ctx)
	result, err := Change(stm)

	if err != nil {
		t.Fatalf("Change() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Change() returned nil response")
	}
	if result.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Change() returned SERVFAIL — cold-cache cross-domain CNAME resolution failed")
	}

	// Find the final A record in the answer section.
	var gotA net.IP
	for _, rr := range result.Answer {
		if a, ok := rr.(*dns.A); ok {
			gotA = a.A
			break
		}
	}
	if gotA == nil {
		t.Fatalf("no A record in answer; got: %v", result.Answer)
	}
	if !gotA.Equal(net.ParseIP("1.2.3.4")) {
		t.Errorf("expected A=1.2.3.4, got %v", gotA)
	}

	// CNAME chain should be preserved in the answer per RFC1034.
	cnames := 0
	for _, rr := range result.Answer {
		if _, ok := rr.(*dns.CNAME); ok {
			cnames++
		}
	}
	if cnames < 2 {
		t.Errorf("expected at least 2 CNAME records in answer (RFC1034), got %d", cnames)
	}
}

// TestSameZoneCNAMEPreservesGlue verifies that when a CNAME target is within
// the same delegated zone (e.g. foo.d1.test → bar.d1.test), the existing NS
// glue for d1.test. is preserved by inGlueState (EXTRACT_GLUE_EXIST), avoiding an
// unnecessary re-delegation round-trip.
func TestSameZoneCNAMEPreservesGlue(t *testing.T) {
	_, mockIP := startCNAMEChainMockDNS(t)

	// Manually seed d1.test. NS in the response to simulate a warm-glue scenario
	// (as if a prior ITER for a different d1.test. name already filled response.Ns).
	nsZone := "d1.test."
	queryName := "bar.d1.test." // same zone as NS

	req := new(dns.Msg)
	req.SetQuestion(queryName, dns.TypeA)

	resp := new(dns.Msg)
	resp.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: nsZone, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60},
			Ns:  "ns.d1.test.",
		},
	}
	resp.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "ns.d1.test.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP(mockIP),
		},
	}

	state := newExtractGlueState(req, resp)
	code, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != EXTRACT_GLUE_EXIST {
		t.Errorf("same-zone CNAME: expected EXTRACT_GLUE_EXIST (%d), got %d", EXTRACT_GLUE_EXIST, code)
	}
	if len(resp.Ns) == 0 {
		t.Error("same-zone CNAME: NS glue was incorrectly cleared")
	}
}
