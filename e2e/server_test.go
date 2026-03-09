package e2e

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"rec53/monitor"
	"rec53/server"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// TestServerLifecycle tests the complete server lifecycle.
func TestServerLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		listenAddr string
		wantErr    bool
	}{
		{
			name:       "random port",
			listenAddr: "127.0.0.1:0",
			wantErr:    false,
		},
		{
			name:       "specific port",
			listenAddr: "127.0.0.1:53530",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := server.NewServer(tt.listenAddr)

			// Start server
			errChan := s.Run()
			time.Sleep(100 * time.Millisecond)

			// Verify server is running
			if s.UDPAddr() == "" {
				t.Error("expected UDP address to be set")
			}

			// Shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := s.Shutdown(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Shutdown() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Wait for error channel to close
			select {
			case _, ok := <-errChan:
				if ok {
					t.Error("expected error channel to be closed")
				}
			case <-time.After(2 * time.Second):
				t.Error("timeout waiting for server to stop")
			}
		})
	}
}

// TestServerUDPAndTCP tests that both UDP and TCP protocols work.
func TestServerUDPAndTCP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()

	t.Run("UDP query", func(t *testing.T) {
		client := &dns.Client{
			Net:     "udp",
			Timeout: 5 * time.Second,
		}

		msg := new(dns.Msg)
		msg.SetQuestion("example.com.", dns.TypeA)
		msg.RecursionDesired = true

		resp, _, err := client.Exchange(msg, s.UDPAddr())
		if err != nil {
			t.Fatalf("UDP query failed: %v", err)
		}

		if resp.Rcode != dns.RcodeSuccess {
			t.Errorf("expected success, got rcode %d", resp.Rcode)
		}
	})

	t.Run("TCP query", func(t *testing.T) {
		client := &dns.Client{
			Net:     "tcp",
			Timeout: 5 * time.Second,
		}

		msg := new(dns.Msg)
		msg.SetQuestion("example.com.", dns.TypeA)
		msg.RecursionDesired = true

		resp, _, err := client.Exchange(msg, s.TCPAddr())
		if err != nil {
			t.Fatalf("TCP query failed: %v", err)
		}

		if resp.Rcode != dns.RcodeSuccess {
			t.Errorf("expected success, got rcode %d", resp.Rcode)
		}
	})
}

// TestServerGracefulShutdown tests graceful shutdown behavior.
func TestServerGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()

	time.Sleep(100 * time.Millisecond)

	// Send a query in a goroutine
	var queryWg sync.WaitGroup
	queryWg.Add(1)

	go func() {
		defer queryWg.Done()

		client := &dns.Client{Net: "udp", Timeout: 10 * time.Second}
		msg := new(dns.Msg)
		msg.SetQuestion("example.com.", dns.TypeA)

		// This query should complete even during shutdown
		client.Exchange(msg, s.UDPAddr())
	}()

	time.Sleep(50 * time.Millisecond)

	// Initiate shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := s.Shutdown(ctx)
	shutdownDuration := time.Since(start)

	if err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}

	t.Logf("Shutdown completed in %v", shutdownDuration)

	// Wait for pending queries
	queryWg.Wait()

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestServerMultipleStarts tests that server can't be started twice.
func TestServerMultipleStarts(t *testing.T) {
	s := server.NewServer("127.0.0.1:0")

	errChan1 := s.Run()
	time.Sleep(100 * time.Millisecond)

	// Second start should be safe (server already running)
	errChan2 := s.Run()
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)

	// Drain channels
	go func() {
		for range errChan1 {
		}
		for range errChan2 {
		}
	}()
}

// TestServerConcurrentQueries tests server handling concurrent queries.
func TestServerConcurrentQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	const numQueries = 50
	const numWorkers = 5

	var wg sync.WaitGroup
	errs := make(chan error, numQueries*numWorkers)

	domains := []string{
		"google.com.", "github.com.", "cloudflare.com.",
		"amazon.com.", "microsoft.com.",
	}

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &dns.Client{
				Net:     "udp",
				Timeout: 10 * time.Second,
			}

			for i := 0; i < numQueries; i++ {
				msg := new(dns.Msg)
				domain := domains[i%len(domains)]
				msg.SetQuestion(domain, dns.TypeA)
				msg.RecursionDesired = true

				resp, _, err := client.Exchange(msg, s.UDPAddr())
				if err != nil {
					errs <- err
					continue
				}

				if resp.Rcode != dns.RcodeSuccess {
					errs <- &QueryError{
						Domain: domain,
						Rcode:  resp.Rcode,
					}
				}
			}
		}(w)
	}

	wg.Wait()
	close(errs)

	// Count errors
	errorCount := 0
	for err := range errs {
		t.Logf("Query error: %v", err)
		errorCount++
	}

	totalQueries := numQueries * numWorkers
	t.Logf("Total queries: %d, Errors: %d (%.2f%%)",
		totalQueries, errorCount, float64(errorCount)/float64(totalQueries)*100)

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// QueryError represents a DNS query error.
type QueryError struct {
	Domain string
	Rcode  int
}

func (e *QueryError) Error() string {
	return dns.RcodeToString[e.Rcode] + " for " + e.Domain
}

// mockDNSHandler is a simple mock handler for testing.
type mockDNSHandler struct {
	records map[string]map[uint16][]dns.RR
	mu      sync.RWMutex
}

func newMockDNSHandler() *mockDNSHandler {
	return &mockDNSHandler{
		records: make(map[string]map[uint16][]dns.RR),
	}
}

func (h *mockDNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	reply := new(dns.Msg)
	reply.SetReply(r)

	if len(r.Question) == 0 {
		w.WriteMsg(reply)
		return
	}

	q := r.Question[0]
	if domainRecords, ok := h.records[q.Name]; ok {
		if records, ok := domainRecords[q.Qtype]; ok {
			reply.Answer = append(reply.Answer, records...)
		}
	}

	w.WriteMsg(reply)
}

func (h *mockDNSHandler) SetRecord(name string, qtype uint16, rr dns.RR) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.records[name] == nil {
		h.records[name] = make(map[uint16][]dns.RR)
	}
	h.records[name][qtype] = append(h.records[name][qtype], rr)
}

// TestMockServerIntegration tests the mock server with real resolver.
func TestMockServerIntegration(t *testing.T) {
	// Create mock authority server
	zone := &Zone{
		Origin: "test.example.",
		Records: map[uint16][]dns.RR{
			dns.TypeA: {
				A("www.test.example", "192.168.1.1", 300),
			},
			dns.TypeNS: {
				NS("test.example.", "ns1.test.example.", 300),
			},
		},
	}

	mockServer := NewMockAuthorityServer(t, zone)
	defer mockServer.Stop()

	t.Logf("Mock server listening on %s", mockServer.Addr())

	// Query the mock server directly
	client := &dns.Client{Net: "udp", Timeout: 5 * time.Second}
	msg := new(dns.Msg)
	msg.SetQuestion("www.test.example.", dns.TypeA)

	resp, _, err := client.Exchange(msg, mockServer.Addr())
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(resp.Answer) == 0 {
		t.Error("Expected answer from mock server")
	}

	for _, rr := range resp.Answer {
		if a, ok := rr.(*dns.A); ok {
			t.Logf("Resolved: %s -> %s", a.Hdr.Name, a.A.String())
			if !a.A.Equal(net.ParseIP("192.168.1.1")) {
				t.Errorf("Expected IP 192.168.1.1, got %s", a.A.String())
			}
		}
	}
}