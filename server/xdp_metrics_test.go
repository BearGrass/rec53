package server

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestCleanupExpiredBPFEntries_NoMap verifies that cleanupExpiredBPFEntries
// is a no-op when called with a nil map (defensive — startXDPCleanupLoop
// guards this too).
func TestCleanupExpiredBPFEntries_NoMap(t *testing.T) {
	deleted := cleanupExpiredBPFEntries(nil)
	if deleted != 0 {
		t.Errorf("expected 0 deleted with nil map, got %d", deleted)
	}
}

// TestCollectXDPStats_NilMap verifies that collectXDPStats does not panic
// with a nil stats map.
func TestCollectXDPStats_NilMap(t *testing.T) {
	// Should be a no-op, not panic.
	collectXDPStats(nil)
}

// TestStartXDPMetricsLoop_CancelledContext verifies that the metrics loop
// exits promptly when its context is cancelled.
func TestStartXDPMetricsLoop_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		startXDPMetricsLoop(ctx, nil) // nil map → returns immediately
		close(done)
	}()

	select {
	case <-done:
		// ok — exited promptly
	case <-time.After(2 * time.Second):
		t.Fatal("startXDPMetricsLoop did not exit after context cancellation")
	}
}

// TestStartXDPCleanupLoop_CancelledContext verifies that the cleanup loop
// exits promptly when its context is cancelled.
func TestStartXDPCleanupLoop_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		startXDPCleanupLoop(ctx, nil) // nil map → returns immediately
		close(done)
	}()

	select {
	case <-done:
		// ok — exited promptly
	case <-time.After(2 * time.Second):
		t.Fatal("startXDPCleanupLoop did not exit after context cancellation")
	}
}

// TestStatConstants verifies that the BPF stat indices match dns_cache.h.
func TestStatConstants(t *testing.T) {
	if STAT_HIT != 0 {
		t.Errorf("STAT_HIT = %d, want 0", STAT_HIT)
	}
	if STAT_MISS != 1 {
		t.Errorf("STAT_MISS = %d, want 1", STAT_MISS)
	}
	if STAT_PASS != 2 {
		t.Errorf("STAT_PASS = %d, want 2", STAT_PASS)
	}
	if STAT_ERROR != 3 {
		t.Errorf("STAT_ERROR = %d, want 3", STAT_ERROR)
	}
}

// TestGetMonotonicSeconds_Reasonable verifies that getMonotonicSeconds returns
// a plausible monotonic time (used by both sync and cleanup).
func TestGetMonotonicSeconds_Reasonable(t *testing.T) {
	sec, err := getMonotonicSeconds()
	if err != nil {
		t.Fatalf("getMonotonicSeconds() failed: %v", err)
	}
	// Monotonic clock should be > 0 on any running system.
	if sec == 0 {
		t.Error("expected non-zero monotonic seconds")
	}
	// Sanity: monotonic clock shouldn't be wall-clock time (which would be ~1.7B+).
	// But on a system with long uptime it could be large. Just check it's < wall time.
	var ts unix.Timespec
	_ = unix.ClockGettime(unix.CLOCK_REALTIME, &ts)
	wallSec := uint64(ts.Sec)
	if sec > wallSec {
		t.Errorf("monotonic %d > wall-clock %d — unexpected", sec, wallSec)
	}
}
