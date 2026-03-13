package server

import (
	"context"
	"net"
	"testing"
	"time"

	"rec53/utils"

	"github.com/miekg/dns"
)

// startWarmupTestMockDNS starts a local UDP DNS server that immediately returns
// SERVFAIL for every query. It injects a mock root glue (pointing to 127.0.0.1
// on the mock server's port) and overrides the iter port so that the state machine
// sends all upstream queries to the mock server instead of the real internet.
//
// t.Cleanup is registered to stop the server and restore global state.
// Returns the mock server's port string (e.g. "54321").
func startWarmupTestMockDNS(t *testing.T) string {
	t.Helper()

	// A handler that immediately replies SERVFAIL (fast, no network needed).
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		resp := new(dns.Msg)
		resp.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(resp) //nolint:errcheck
	})

	srv := &dns.Server{
		Addr:    "127.0.0.1:0",
		Net:     "udp",
		Handler: handler,
	}

	started := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(started) }

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			// Server was shut down — normal during test cleanup.
		}
	}()

	<-started // wait until socket is ready

	_, port, _ := net.SplitHostPort(srv.PacketConn.LocalAddr().String())

	// Build a minimal root glue message pointing to 127.0.0.1 at our mock port.
	// inGlueCacheState.handle() calls utils.GetRootGlue() to seed the first iteration,
	// so this ensures the very first DNS exchange goes to the mock server.
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
	SetIterPort(port)
	FlushCacheForTest()
	ResetIPPoolForTest()

	t.Cleanup(func() {
		srv.Shutdown()
		utils.ResetRootGlue()
		ResetIterPort()
		FlushCacheForTest()
		ResetIPPoolForTest()
	})

	return port
}

// TestWarmupNSRecordsWithCuratedTLDs verifies warmup runs against all 30 curated TLDs
// without hanging. A local mock DNS server is used so no real network is needed.
func TestWarmupNSRecordsWithCuratedTLDs(t *testing.T) {
	startWarmupTestMockDNS(t)

	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     500 * time.Millisecond,
		Duration:    5 * time.Second,
		Concurrency: 4,
		TLDs:        LoadTLDList(nil), // Use curated defaults
	}

	// Verify TLD count matches curated list
	if len(cfg.TLDs) != 30 {
		t.Errorf("expected 30 TLDs, got %d", len(cfg.TLDs))
	}

	done := make(chan WarmupStats, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
		defer cancel()
		done <- WarmupNSRecords(ctx, cfg)
	}()

	select {
	case stats := <-done:
		// Total should include root "." + all TLDs
		expectedTotal := 1 + len(cfg.TLDs) // root + 30 TLDs = 31
		if stats.Total != expectedTotal {
			t.Errorf("expected total=%d, got %d", expectedTotal, stats.Total)
		}
	case <-time.After(cfg.Duration + 2*time.Second):
		t.Fatal("TestWarmupNSRecordsWithCuratedTLDs hung — warmup did not complete in time")
	}
}

// TestWarmupNSRecordsSingleTLDFailureDoesNotBlock verifies that a single TLD failure
// does not prevent other TLDs from being probed, and that warmup does not hang.
func TestWarmupNSRecordsSingleTLDFailureDoesNotBlock(t *testing.T) {
	startWarmupTestMockDNS(t)

	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     500 * time.Millisecond,
		Duration:    5 * time.Second,
		Concurrency: 4,
		TLDs:        []string{"com", "invalid-tld-that-does-not-exist-xyz123"},
	}

	done := make(chan WarmupStats, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
		defer cancel()
		done <- WarmupNSRecords(ctx, cfg)
	}()

	select {
	case stats := <-done:
		// Both TLDs + root should be attempted (total=3)
		if stats.Total != 3 {
			t.Errorf("expected total=3 (root + 2 TLDs), got %d", stats.Total)
		}

		// Warmup should complete even with one invalid TLD
		if stats.Succeeded+stats.Failed != stats.Total {
			t.Errorf("succeeded(%d) + failed(%d) != total(%d)", stats.Succeeded, stats.Failed, stats.Total)
		}
	case <-time.After(cfg.Duration + 2*time.Second):
		t.Fatal("TestWarmupNSRecordsSingleTLDFailureDoesNotBlock hung — warmup did not complete in time")
	}
}

// TestWarmupNSRecordsCustomTLDList verifies warmup uses a custom TLD list when provided via config.
func TestWarmupNSRecordsCustomTLDList(t *testing.T) {
	startWarmupTestMockDNS(t)

	customTLDs := []string{"com", "net", "org"}
	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     500 * time.Millisecond,
		Duration:    5 * time.Second,
		Concurrency: 2,
		TLDs:        LoadTLDList(customTLDs),
	}

	// Verify custom list was used
	if len(cfg.TLDs) != len(customTLDs) {
		t.Errorf("expected %d custom TLDs, got %d", len(customTLDs), len(cfg.TLDs))
	}

	done := make(chan WarmupStats, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
		defer cancel()
		done <- WarmupNSRecords(ctx, cfg)
	}()

	select {
	case stats := <-done:
		// Total = root + 3 custom TLDs
		if stats.Total != 4 {
			t.Errorf("expected total=4 (root + 3 TLDs), got %d", stats.Total)
		}
	case <-time.After(cfg.Duration + 2*time.Second):
		t.Fatal("TestWarmupNSRecordsCustomTLDList hung — warmup did not complete in time")
	}
}

// TestWarmupNSRecordsContextCancellation verifies warmup respects context cancellation.
func TestWarmupNSRecordsContextCancellation(t *testing.T) {
	startWarmupTestMockDNS(t)

	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     5 * time.Second,
		Duration:    10 * time.Second,
		Concurrency: 4,
		TLDs:        []string{"com", "net"},
	}

	// Immediately cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Should not panic or hang
	done := make(chan WarmupStats, 1)
	go func() {
		done <- WarmupNSRecords(ctx, cfg)
	}()

	select {
	case <-done:
		// Good - completed without blocking
	case <-time.After(5 * time.Second):
		t.Error("warmup did not complete after context cancellation within 5 seconds")
	}
}

// TestWarmupConfigUsesDefaultTLDCount verifies DefaultWarmupConfig has correct TLD count.
func TestWarmupConfigUsesDefaultTLDCount(t *testing.T) {
	if len(DefaultWarmupConfig.TLDs) != 30 {
		t.Errorf("expected DefaultWarmupConfig.TLDs to have 30 entries, got %d", len(DefaultWarmupConfig.TLDs))
	}
}

// TestWarmupNSRecordsSemaphoreRespectsCtx is a regression test for B3:
// the semaphore acquire in WarmupNSRecords must not block when ctx is cancelled,
// even when concurrency=1 (only one slot) and many domains are queued.
// Run with -race to detect any data races introduced by the fix.
func TestWarmupNSRecordsSemaphoreRespectsCtx(t *testing.T) {
	startWarmupTestMockDNS(t)

	// concurrency=1 maximizes the chance of the loop blocking on sem<- before the fix.
	// Many TLDs means the loop cannot finish before ctx fires.
	tlds := make([]string, 50)
	for i := range tlds {
		tlds[i] = "tld-placeholder.example."
	}

	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     50 * time.Millisecond, // short per-query timeout so goroutines exit quickly
		Duration:    100 * time.Millisecond,
		Concurrency: 1, // single slot — old code would block the loop immediately
		TLDs:        tlds,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	done := make(chan WarmupStats, 1)
	go func() {
		done <- WarmupNSRecords(ctx, cfg)
	}()

	// Allow generous headroom; the real deadline is cfg.Duration (100ms).
	// Before the fix the loop would block on sem<- indefinitely after ctx fired.
	select {
	case stats := <-done:
		// Verify the total matches what was attempted (root + tlds).
		expected := 1 + len(tlds)
		if stats.Total != expected {
			t.Errorf("expected total=%d, got %d", expected, stats.Total)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("WarmupNSRecords blocked after context cancellation — semaphore not ctx-aware (B3)")
	}
}
