package server

import (
	"context"
	"testing"
	"time"
)

// TestWarmupNSRecordsWithCuratedTLDs verifies warmup runs against all 30 curated TLDs.
// This is an integration test that exercises the warmup process end-to-end.
// It runs in short mode using a very short timeout so queries fail fast (no real DNS needed).
func TestWarmupNSRecordsWithCuratedTLDs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping warmup integration test in short mode")
	}

	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     100 * time.Millisecond,
		Duration:    2 * time.Second,
		Concurrency: 4,
		TLDs:        LoadTLDList(nil), // Use curated defaults
	}

	// Verify TLD count matches curated list
	if len(cfg.TLDs) != 30 {
		t.Errorf("expected 30 TLDs, got %d", len(cfg.TLDs))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	stats := WarmupNSRecords(ctx, cfg)

	// Total should include root "." + all TLDs
	expectedTotal := 1 + len(cfg.TLDs) // root + 30 TLDs = 31
	if stats.Total != expectedTotal {
		t.Errorf("expected total=%d, got %d", expectedTotal, stats.Total)
	}

	// Warmup should complete without hanging
	if stats.Duration > cfg.Duration+500*time.Millisecond {
		t.Errorf("warmup took too long: %v (max expected %v)", stats.Duration, cfg.Duration)
	}
}

// TestWarmupNSRecordsSingleTLDFailureDoesNotBlock verifies that a single TLD failure
// does not prevent other TLDs from being probed.
func TestWarmupNSRecordsSingleTLDFailureDoesNotBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping warmup integration test in short mode")
	}
	// Use a mix of real and invalid TLDs
	// Invalid TLDs will fail, but valid ones should still be probed
	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     200 * time.Millisecond,
		Duration:    3 * time.Second,
		Concurrency: 4,
		TLDs:        []string{"com", "invalid-tld-that-does-not-exist-xyz123"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	stats := WarmupNSRecords(ctx, cfg)

	// Both TLDs + root should be attempted (total=3)
	if stats.Total != 3 {
		t.Errorf("expected total=3 (root + 2 TLDs), got %d", stats.Total)
	}

	// Warmup should complete even with one invalid TLD
	if stats.Succeeded+stats.Failed != stats.Total {
		t.Errorf("succeeded(%d) + failed(%d) != total(%d)", stats.Succeeded, stats.Failed, stats.Total)
	}
}

// TestWarmupNSRecordsCustomTLDList verifies warmup uses a custom TLD list when provided via config.
func TestWarmupNSRecordsCustomTLDList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	customTLDs := []string{"com", "net", "org"}
	cfg := WarmupConfig{
		Enabled:     true,
		Timeout:     100 * time.Millisecond,
		Duration:    2 * time.Second,
		Concurrency: 2,
		TLDs:        LoadTLDList(customTLDs),
	}

	// Verify custom list was used
	if len(cfg.TLDs) != len(customTLDs) {
		t.Errorf("expected %d custom TLDs, got %d", len(customTLDs), len(cfg.TLDs))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	stats := WarmupNSRecords(ctx, cfg)

	// Total = root + 3 custom TLDs
	if stats.Total != 4 {
		t.Errorf("expected total=4 (root + 3 TLDs), got %d", stats.Total)
	}
}

// TestWarmupNSRecordsNoopOnEmptyContext verifies warmup respects context cancellation.
func TestWarmupNSRecordsContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping warmup integration test in short mode")
	}
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
