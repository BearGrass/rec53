package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"rec53/server"

	"github.com/miekg/dns"
)

// TestXDPCacheFastPath is an end-to-end integration test for the XDP/eBPF
// DNS cache fast path.  It requires root (CAP_BPF + CAP_NET_ADMIN) and
// exclusive access to port 53 on loopback.
//
// Test flow:
//  1. Start a DNS server with XDP enabled on loopback, listening on port 53.
//  2. Send a DNS query → cache miss → XDP_PASS → Go resolver handles it.
//  3. Go cache + BPF cache are populated (syncToBPFMap).
//  4. Send the same query again → XDP cache hit → XDP_TX (response from kernel).
//  5. Verify XDP stats show at least one HIT.
func TestXDPCacheFastPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping XDP integration test in short mode")
	}

	// Check if we can bind to port 53 (requires root).
	ln, err := net.ListenPacket("udp", "127.0.0.1:53")
	if err != nil {
		t.Skipf("cannot bind to port 53 (need root): %v", err)
	}
	ln.Close()

	// Create server with XDP on loopback.
	s := server.NewServerWithFullConfig(
		"127.0.0.1:53", 1,
		server.WarmupConfig{Enabled: false},
		server.SnapshotConfig{},
		nil, nil,
		"lo", // XDP on loopback (generic mode)
	)

	errChan := s.Run()

	// Check if XDP actually attached (needs CAP_BPF).
	loader := s.XDPLoaderForTest()
	if loader == nil {
		// XDP failed to attach — Run() logged a warning and set loader to nil.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
		t.Skip("XDP failed to attach (likely needs CAP_BPF)")
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			t.Errorf("shutdown error: %v", err)
		}
		// Drain errChan.
		for range errChan {
		}
	}()

	client := &dns.Client{Net: "udp", Timeout: 10 * time.Second}
	domain := "example.com."

	// -----------------------------------------------------------------------
	// Step 1: First query — cache miss, resolved by Go, populates BPF cache.
	// -----------------------------------------------------------------------
	msg := new(dns.Msg)
	msg.SetQuestion(domain, dns.TypeA)
	msg.RecursionDesired = true

	resp, _, err := client.Exchange(msg, "127.0.0.1:53")
	if err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("first query returned rcode %s, want NOERROR", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("first query returned no answers")
	}
	t.Logf("first query: %d answers, rcode=%s", len(resp.Answer), dns.RcodeToString[resp.Rcode])

	// Read stats after first query.
	hit1, miss1, pass1, err1, sErr := loader.XDPStatsForTest()
	if sErr != nil {
		t.Fatalf("failed to read XDP stats: %v", sErr)
	}
	t.Logf("after 1st query: hit=%d miss=%d pass=%d error=%d", hit1, miss1, pass1, err1)

	// First query should show a miss (cache was empty).
	if miss1 == 0 {
		t.Log("warning: expected at least 1 MISS after first query (may have been served from pre-existing cache)")
	}

	// -----------------------------------------------------------------------
	// Step 2: Second query — should hit XDP cache (XDP_TX).
	// -----------------------------------------------------------------------
	msg2 := new(dns.Msg)
	msg2.SetQuestion(domain, dns.TypeA)
	msg2.RecursionDesired = true

	resp2, _, err := client.Exchange(msg2, "127.0.0.1:53")
	if err != nil {
		t.Fatalf("second query failed: %v", err)
	}
	if resp2.Rcode != dns.RcodeSuccess {
		t.Fatalf("second query returned rcode %s, want NOERROR", dns.RcodeToString[resp2.Rcode])
	}
	if len(resp2.Answer) == 0 {
		t.Fatal("second query returned no answers")
	}
	t.Logf("second query: %d answers, rcode=%s", len(resp2.Answer), dns.RcodeToString[resp2.Rcode])

	// Read stats after second query.
	hit2, miss2, pass2, err2, sErr := loader.XDPStatsForTest()
	if sErr != nil {
		t.Fatalf("failed to read XDP stats: %v", sErr)
	}
	t.Logf("after 2nd query: hit=%d miss=%d pass=%d error=%d", hit2, miss2, pass2, err2)

	// Verify cache HIT was incremented.
	if hit2 <= hit1 {
		t.Errorf("expected XDP HIT to increase after second query: before=%d after=%d", hit1, hit2)
	}

	// Verify the response contains the same domain.
	if len(resp2.Question) == 0 || resp2.Question[0].Name != domain {
		t.Errorf("response question mismatch: got %v", resp2.Question)
	}
}

// TestXDPShutdownCleanup verifies that the XDP program is properly detached
// and all BPF objects are cleaned up during server shutdown.
func TestXDPShutdownCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping XDP integration test in short mode")
	}

	ln, err := net.ListenPacket("udp", "127.0.0.1:53")
	if err != nil {
		t.Skipf("cannot bind to port 53 (need root): %v", err)
	}
	ln.Close()

	s := server.NewServerWithFullConfig(
		"127.0.0.1:53", 1,
		server.WarmupConfig{Enabled: false},
		server.SnapshotConfig{},
		nil, nil,
		"lo",
	)

	errChan := s.Run()

	loader := s.XDPLoaderForTest()
	if loader == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
		t.Skip("XDP failed to attach")
	}

	// Verify XDP is active.
	if loader.CacheMap() == nil {
		t.Error("expected CacheMap to be non-nil while XDP is active")
	}

	// Shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("shutdown error: %v", err)
	}

	// After shutdown, the loader on the server should be nil.
	if s.XDPLoaderForTest() != nil {
		t.Error("expected xdpLoader to be nil after shutdown")
	}

	// Drain errChan.
	for range errChan {
	}
}
