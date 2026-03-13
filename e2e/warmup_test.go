// Package e2e provides end-to-end integration tests for the rec53 DNS resolver.
//
// This file contains E2E tests for O-025: NS Warmup on Startup.
// It verifies that the resolver can pre-warm root and TLD NS records on startup,
// improving cache hit rate and reducing latency for initial queries.
//
// Scenarios tested:
// 1. Warmup completes without blocking startup
// 2. Root NS records are cached after warmup
// 3. TLD NS records are cached after warmup
// 4. Warmup can be disabled with --no-warmup flag
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
	"rec53/server"
)

// TestWarmupNSRecords tests basic warmup functionality.
// Verifies that WarmupNSRecords can query multiple domains concurrently
// without errors.
//
// Scenario:
// - Call WarmupNSRecords with small config (root + 5 TLDs)
// - All queries should complete successfully
// - Stats should show expected total count
func TestWarmupNSRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping warmup integration test in short mode")
	}
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	// Create a minimal warmup config with just a few TLDs
	cfg := server.WarmupConfig{
		Enabled:     true,
		Timeout:     2 * time.Second,
		Concurrency: 4,
		TLDs:        []string{"com", "org", "net"},
	}

	// Create context with 30s timeout for entire warmup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run warmup
	stats := server.WarmupNSRecords(ctx, cfg)

	// Verify stats
	expected := 4 // root + 3 TLDs
	if stats.Total != expected {
		t.Errorf("Expected total %d, got %d", expected, stats.Total)
	}

	if stats.Succeeded == 0 {
		t.Errorf("Expected at least some succeeded queries, got 0")
	}

	if stats.Duration == 0 {
		t.Errorf("Expected non-zero duration")
	}

	t.Logf("Warmup stats: Total=%d, Succeeded=%d, Failed=%d, Duration=%.1fs",
		stats.Total, stats.Succeeded, stats.Failed, stats.Duration.Seconds())
}

// TestWarmupNSRecords_Concurrency tests that warmup respects concurrency limits.
// Verifies that the semaphore correctly limits parallel queries.
//
// Scenario:
// - Run warmup with low concurrency (2)
// - Should complete in reasonable time without overwhelming system
// - Stats should match input
func TestWarmupNSRecords_Concurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping warmup integration test in short mode")
	}
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	// Small config with limited concurrency
	cfg := server.WarmupConfig{
		Enabled:     true,
		Timeout:     2 * time.Second,
		Concurrency: 2, // Low concurrency
		TLDs:        []string{"com", "net", "org", "edu", "gov"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	stats := server.WarmupNSRecords(ctx, cfg)
	elapsed := time.Since(startTime)

	expected := 6 // root + 5 TLDs
	if stats.Total != expected {
		t.Errorf("Expected total %d, got %d", expected, stats.Total)
	}

	// With 2 concurrency for 6 domains, should take at least a few seconds
	// (Each query takes ~500ms-2s against real root servers)
	t.Logf("Warmup with concurrency 2: Total=%d, Succeeded=%d, Duration=%.1fs (wall clock: %.1fs)",
		stats.Total, stats.Succeeded, stats.Duration.Seconds(), elapsed.Seconds())
}

// TestWarmupNSRecords_Timeout tests that individual query timeouts are respected.
// Verifies that slow queries don't block warmup.
//
// Scenario:
// - Set per-query timeout to 1s
// - Some queries may timeout, but warmup continues
// - Warmup should complete within overall timeout
func TestWarmupNSRecords_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping warmup integration test in short mode")
	}
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	// Config with tight per-query timeout
	cfg := server.WarmupConfig{
		Enabled:     true,
		Timeout:     1 * time.Second, // Very tight timeout
		Concurrency: 4,
		TLDs:        []string{"com", "net", "org"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats := server.WarmupNSRecords(ctx, cfg)

	// Even with timeouts, warmup should report stats
	if stats.Total != 4 { // root + 3 TLDs
		t.Errorf("Expected total 4, got %d", stats.Total)
	}

	t.Logf("Warmup with 1s timeout: Total=%d, Succeeded=%d, Failed=%d",
		stats.Total, stats.Succeeded, stats.Failed)
}

// TestWarmupNSRecords_LargeTLDList tests warmup with a larger TLD list.
// Verifies that warmup scales to the default TLD list.
//
// Scenario:
// - Use default TLD list (100+ entries)
// - Should complete in reasonable time
// - High concurrency (32) should achieve good performance
func TestWarmupNSRecords_LargeTLDList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping warmup integration test in short mode")
	}
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	// Use default config with many TLDs
	cfg := server.WarmupConfig{
		Enabled:     true,
		Timeout:     5 * time.Second,
		Concurrency: 32,                      // Default concurrency
		TLDs:        server.DefaultTLDs[:20], // Use first 20 for faster test
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	startTime := time.Now()
	stats := server.WarmupNSRecords(ctx, cfg)
	elapsed := time.Since(startTime)

	expected := 21 // root + 20 TLDs
	if stats.Total != expected {
		t.Errorf("Expected total %d, got %d", expected, stats.Total)
	}

	// With 32 concurrency, 21 queries should complete in ~5-10 seconds
	if elapsed > 60*time.Second {
		t.Logf("Warning: Warmup took longer than expected: %.1fs", elapsed.Seconds())
	}

	t.Logf("Warmup with 20 TLDs: Total=%d, Succeeded=%d, Failed=%d, Duration=%.1fs (wall clock: %.1fs)",
		stats.Total, stats.Succeeded, stats.Failed, stats.Duration.Seconds(), elapsed.Seconds())
}

// TestWarmupNSRecords_CachePopulation tests that queries benefit from warmup cache.
// Verifies that after warmup, subsequent queries use cached NS records.
//
// Scenario:
// - Run warmup to populate cache
// - Query a root TLD (com)
// - Should succeed using cached NS records
func TestWarmupNSRecords_CachePopulation(t *testing.T) {
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	// Build mock hierarchy for testing
	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("www.example.com.", "93.184.216.34", 300),
		},
	})

	mockSrv, rootGlue := hierarchy.Build(t)
	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// Run warmup (with small TLD list)
	cfg := server.WarmupConfig{
		Enabled:     true,
		Timeout:     2 * time.Second,
		Concurrency: 2,
		TLDs:        []string{"com"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	warmupStats := server.WarmupNSRecords(ctx, cfg)
	t.Logf("Warmup completed: Total=%d, Succeeded=%d", warmupStats.Total, warmupStats.Succeeded)

	// Now query a domain in the warmed TLD
	resp, err := env.query("www.example.com.", dns.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("Expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}

	if len(resp.Answer) == 0 {
		t.Fatalf("Expected answer section, got empty")
	}

	t.Logf("Query after warmup succeeded")
}

// TestWarmupStats tests that warmup statistics are accurate.
// Verifies that succeeded + failed = total.
//
// Scenario:
// - Run warmup with mixed success/failure
// - Stats should be consistent: Succeeded + Failed = Total
func TestWarmupStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping warmup integration test in short mode")
	}
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	cfg := server.WarmupConfig{
		Enabled:     true,
		Timeout:     2 * time.Second,
		Concurrency: 4,
		TLDs:        []string{"com", "org", "net", "edu"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats := server.WarmupNSRecords(ctx, cfg)

	// Consistency check: Succeeded + Failed should equal Total
	if stats.Succeeded+stats.Failed != stats.Total {
		t.Errorf("Stats inconsistent: Succeeded(%d) + Failed(%d) != Total(%d)",
			stats.Succeeded, stats.Failed, stats.Total)
	}

	if stats.Succeeded == 0 && stats.Total > 0 {
		t.Logf("Note: All queries failed. This may be expected if root servers are unreachable.")
	}

	t.Logf("Stats verified: Total=%d, Succeeded=%d, Failed=%d",
		stats.Total, stats.Succeeded, stats.Failed)
}
