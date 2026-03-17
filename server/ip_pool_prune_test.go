package server

import (
	"context"
	"testing"
	"time"
)

// ============================================================================
// IP Pool Prune Tests
// ============================================================================

// TestPruneStaleIPs_StaleIPRemoved verifies IPs exceeding the threshold are pruned.
func TestPruneStaleIPs_StaleIPRemoved(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Add an IP and manually backdate its lastSeen
	iq := NewIPQualityV2()
	iq.mu.Lock()
	iq.lastSeen = time.Now().Add(-25 * time.Hour) // 25h ago, exceeds 24h threshold
	iq.mu.Unlock()
	ipp.SetIPQualityV2("192.0.2.1", iq)

	ipp.exemptIPs = nil
	ipp.PruneStaleIPs(24 * time.Hour)

	if got := ipp.GetIPQualityV2("192.0.2.1"); got != nil {
		t.Errorf("expected stale IP to be pruned, but it still exists")
	}
}

// TestPruneStaleIPs_FreshIPRetained verifies IPs within the threshold are kept.
func TestPruneStaleIPs_FreshIPRetained(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Add an IP with recent lastSeen (default from NewIPQualityV2)
	iq := NewIPQualityV2()
	ipp.SetIPQualityV2("192.0.2.2", iq)

	ipp.exemptIPs = nil
	ipp.PruneStaleIPs(24 * time.Hour)

	if got := ipp.GetIPQualityV2("192.0.2.2"); got == nil {
		t.Errorf("expected fresh IP to be retained, but it was pruned")
	}
}

// TestPruneStaleIPs_ExemptIPNotPruned verifies exempt IPs survive even when stale.
func TestPruneStaleIPs_ExemptIPNotPruned(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	exemptIP := "198.41.0.4" // a.root-servers.net
	iq := NewIPQualityV2()
	iq.mu.Lock()
	iq.lastSeen = time.Now().Add(-48 * time.Hour) // 48h ago, very stale
	iq.mu.Unlock()
	ipp.SetIPQualityV2(exemptIP, iq)

	ipp.exemptIPs = map[string]struct{}{exemptIP: {}}
	ipp.PruneStaleIPs(24 * time.Hour)

	if got := ipp.GetIPQualityV2(exemptIP); got == nil {
		t.Errorf("expected exempt IP %s to be retained, but it was pruned", exemptIP)
	}
}

// TestPruneStaleIPs_EmptyPoolNoPanic verifies pruning an empty pool is safe.
func TestPruneStaleIPs_EmptyPoolNoPanic(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	ipp.exemptIPs = nil
	// Should not panic
	ipp.PruneStaleIPs(24 * time.Hour)
}

// TestNewIPQualityV2_LastSeenNotZero verifies lastSeen is initialized to time.Now().
func TestNewIPQualityV2_LastSeenNotZero(t *testing.T) {
	before := time.Now()
	iq := NewIPQualityV2()
	after := time.Now()

	lastSeen := iq.GetLastSeen()
	if lastSeen.IsZero() {
		t.Fatal("expected lastSeen to be non-zero after NewIPQualityV2()")
	}
	if lastSeen.Before(before) || lastSeen.After(after) {
		t.Errorf("lastSeen=%v not within [%v, %v]", lastSeen, before, after)
	}
}

// TestRecordLatencyUpdatesLastSeen verifies RecordLatency refreshes lastSeen.
func TestRecordLatencyUpdatesLastSeen(t *testing.T) {
	iq := NewIPQualityV2()

	// Backdate lastSeen
	iq.mu.Lock()
	iq.lastSeen = time.Now().Add(-1 * time.Hour)
	iq.mu.Unlock()
	oldLastSeen := iq.GetLastSeen()

	time.Sleep(1 * time.Millisecond)
	iq.RecordLatency(50)

	newLastSeen := iq.GetLastSeen()
	if !newLastSeen.After(oldLastSeen) {
		t.Errorf("RecordLatency did not update lastSeen: old=%v, new=%v", oldLastSeen, newLastSeen)
	}
}

// TestRecordFailureUpdatesLastSeen verifies RecordFailure refreshes lastSeen.
func TestRecordFailureUpdatesLastSeen(t *testing.T) {
	iq := NewIPQualityV2()

	// Backdate lastSeen
	iq.mu.Lock()
	iq.lastSeen = time.Now().Add(-1 * time.Hour)
	iq.mu.Unlock()
	oldLastSeen := iq.GetLastSeen()

	time.Sleep(1 * time.Millisecond)
	iq.RecordFailure()

	newLastSeen := iq.GetLastSeen()
	if !newLastSeen.After(oldLastSeen) {
		t.Errorf("RecordFailure did not update lastSeen: old=%v, new=%v", oldLastSeen, newLastSeen)
	}
}

// TestPruneStaleIPs_TriggeredByElapsedTime verifies the periodic prune is based on
// wall-clock elapsed time (PRUNE_INTERVAL) rather than tick count.
func TestPruneStaleIPs_TriggeredByElapsedTime(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Add a stale IP
	iq := NewIPQualityV2()
	iq.mu.Lock()
	iq.lastSeen = time.Now().Add(-25 * time.Hour)
	iq.mu.Unlock()
	ipp.SetIPQualityV2("192.0.2.10", iq)

	ipp.exemptIPs = nil

	// Simulate: lastPruneAt was recent — prune should NOT be triggered
	ipp.lastPruneAt = time.Now()
	if time.Since(ipp.lastPruneAt) >= PRUNE_INTERVAL {
		t.Fatal("precondition failed: lastPruneAt should be recent")
	}
	// IP should still exist (prune not called)
	if got := ipp.GetIPQualityV2("192.0.2.10"); got == nil {
		t.Fatal("IP should exist before prune interval elapses")
	}

	// Simulate: lastPruneAt was PRUNE_INTERVAL ago — prune should trigger
	ipp.lastPruneAt = time.Now().Add(-PRUNE_INTERVAL)
	if time.Since(ipp.lastPruneAt) >= PRUNE_INTERVAL {
		ipp.PruneStaleIPs(STALE_IP_THRESHOLD)
		ipp.lastPruneAt = time.Now()
	}

	if got := ipp.GetIPQualityV2("192.0.2.10"); got != nil {
		t.Errorf("expected stale IP to be pruned after PRUNE_INTERVAL elapsed")
	}
}

// TestPruneStaleIPs_MixedFreshAndStale verifies only stale IPs are removed in a mixed pool.
func TestPruneStaleIPs_MixedFreshAndStale(t *testing.T) {
	ipp := NewIPPool()
	defer ipp.Shutdown(context.Background())

	// Fresh IP
	freshIQ := NewIPQualityV2()
	ipp.SetIPQualityV2("192.0.2.100", freshIQ)

	// Stale IP
	staleIQ := NewIPQualityV2()
	staleIQ.mu.Lock()
	staleIQ.lastSeen = time.Now().Add(-48 * time.Hour)
	staleIQ.mu.Unlock()
	ipp.SetIPQualityV2("192.0.2.200", staleIQ)

	// Exempt stale IP
	exemptIQ := NewIPQualityV2()
	exemptIQ.mu.Lock()
	exemptIQ.lastSeen = time.Now().Add(-48 * time.Hour)
	exemptIQ.mu.Unlock()
	ipp.SetIPQualityV2("198.41.0.4", exemptIQ)

	ipp.exemptIPs = map[string]struct{}{"198.41.0.4": {}}
	ipp.PruneStaleIPs(24 * time.Hour)

	if ipp.GetIPQualityV2("192.0.2.100") == nil {
		t.Error("fresh IP should be retained")
	}
	if ipp.GetIPQualityV2("192.0.2.200") != nil {
		t.Error("stale non-exempt IP should be pruned")
	}
	if ipp.GetIPQualityV2("198.41.0.4") == nil {
		t.Error("stale exempt IP should be retained")
	}
}
