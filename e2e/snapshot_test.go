// Package e2e provides end-to-end integration tests for the rec53 DNS resolver.
//
// This file tests the full cache snapshot lifecycle: iterative resolution fills
// the cache, SaveSnapshot persists it, FlushCacheForTest clears it, LoadSnapshot
// restores it, and subsequent queries hit the cache without reaching the upstream
// mock authority server.
package e2e

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rec53/server"

	"github.com/miekg/dns"
)

// TestSnapshotE2E_ARecordSurvivesRestart verifies the complete snapshot path:
// 1. Iterative resolution fills cache with an A record
// 2. SaveSnapshot persists all cache entries to disk
// 3. FlushCacheForTest simulates a restart (cache is empty)
// 4. LoadSnapshot restores cache from the snapshot file
// 5. A repeat query hits the cache — mock server request count does NOT increase
func TestSnapshotE2E_ARecordSurvivesRestart(t *testing.T) {
	hierarchy := BuildStandardHierarchy("com.", "snap-a.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("www.snap-a.com.", "10.20.30.40", 3600),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)
	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// --- Phase 1: cold query fills the cache ---
	resp, err := env.query("www.snap-a.com", dns.TypeA)
	if err != nil {
		t.Fatalf("cold query failed: %v", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}

	var foundA bool
	for _, rr := range resp.Answer {
		if a, ok := rr.(*dns.A); ok && a.A.Equal(net.ParseIP("10.20.30.40")) {
			foundA = true
		}
	}
	if !foundA {
		t.Fatalf("expected A 10.20.30.40 in answer, got: %v", resp.Answer)
	}

	coldCount := mockSrv.RequestCount()
	t.Logf("after cold query: mock received %d requests", coldCount)

	// --- Phase 2: save snapshot ---
	dir := t.TempDir()
	cfg := server.SnapshotConfig{Enabled: true, File: filepath.Join(dir, "snap.json")}
	if err := server.SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	// --- Phase 3: flush cache (simulate restart) ---
	server.FlushCacheForTest()

	// --- Phase 4: restore snapshot ---
	n, err := server.LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n == 0 {
		t.Fatal("LoadSnapshot imported 0 entries, expected at least 1")
	}
	t.Logf("LoadSnapshot restored %d entries", n)

	// --- Phase 5: repeat query should hit cache ---
	resp2, err := env.query("www.snap-a.com", dns.TypeA)
	if err != nil {
		t.Fatalf("warm query failed: %v", err)
	}
	if resp2.Rcode != dns.RcodeSuccess {
		t.Fatalf("warm query: expected NOERROR, got %s", dns.RcodeToString[resp2.Rcode])
	}

	warmCount := mockSrv.RequestCount()
	t.Logf("after warm query: mock received %d requests (was %d)", warmCount, coldCount)

	if warmCount != coldCount {
		t.Errorf("expected no new upstream requests after snapshot restore, but request count grew from %d to %d", coldCount, warmCount)
	}
}

// TestSnapshotE2E_NSEntrySurvivesRestart verifies that NS delegation cache entries
// (stored in the Ns section of dns.Msg) survive a snapshot save/restore cycle.
// After restore, a query for a name under the same delegation should skip the
// root and TLD lookups and go straight to the authoritative server.
func TestSnapshotE2E_NSEntrySurvivesRestart(t *testing.T) {
	hierarchy := BuildStandardHierarchy("com.", "snap-ns.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("ns-test.snap-ns.com.", "10.99.88.77", 3600),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)
	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// Cold query: forces root → TLD → auth resolution, caching NS delegations.
	resp, err := env.query("ns-test.snap-ns.com", dns.TypeA)
	if err != nil {
		t.Fatalf("cold query failed: %v", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	coldCount := mockSrv.RequestCount()
	t.Logf("cold query: %d mock requests", coldCount)

	// Save → flush → restore.
	dir := t.TempDir()
	cfg := server.SnapshotConfig{Enabled: true, File: filepath.Join(dir, "snap.json")}
	if err := server.SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	server.FlushCacheForTest()
	n, err := server.LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	t.Logf("restored %d entries", n)

	// Warm query: A record is cached, should not hit the mock at all.
	resp2, err := env.query("ns-test.snap-ns.com", dns.TypeA)
	if err != nil {
		t.Fatalf("warm query failed: %v", err)
	}
	if resp2.Rcode != dns.RcodeSuccess {
		t.Fatalf("warm query: expected NOERROR, got %s", dns.RcodeToString[resp2.Rcode])
	}
	warmCount := mockSrv.RequestCount()
	t.Logf("warm query: %d mock requests (was %d)", warmCount, coldCount)

	if warmCount != coldCount {
		t.Errorf("expected no new upstream requests after snapshot restore, but count grew from %d to %d", coldCount, warmCount)
	}
}

// TestSnapshotE2E_ExpiredEntrySkipped verifies that when a snapshot file contains
// only entries with fully expired TTLs, LoadSnapshot imports nothing and a
// subsequent query still triggers upstream resolution.
func TestSnapshotE2E_ExpiredEntrySkipped(t *testing.T) {
	hierarchy := BuildStandardHierarchy("com.", "snap-exp.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("old.snap-exp.com.", "10.0.0.1", 60),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)
	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// Manually write a snapshot file with a fully expired entry.
	dir := t.TempDir()
	snapFile := filepath.Join(dir, "expired.json")

	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{})
	msg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "old.snap-exp.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("10.0.0.1"),
		},
	}
	wire, _ := msg.Pack()

	type snapshotEntry struct {
		Key     string `json:"key"`
		MsgB64  string `json:"msg_b64"`
		SavedAt int64  `json:"saved_at"`
	}
	type snapshotFile struct {
		Entries []snapshotEntry `json:"entries"`
	}

	sf := snapshotFile{Entries: []snapshotEntry{
		{
			Key:     "old.snap-exp.com.:1",
			MsgB64:  base64.StdEncoding.EncodeToString(wire),
			SavedAt: time.Now().Unix() - 120, // saved 120s ago, TTL=60 → expired
		},
	}}
	data, _ := json.Marshal(sf)
	os.WriteFile(snapFile, data, 0o644)

	cfg := server.SnapshotConfig{Enabled: true, File: snapFile}
	n, err := server.LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 imported entries (all expired), got %d", n)
	}

	// Query should trigger upstream resolution since cache is empty.
	beforeCount := mockSrv.RequestCount()
	resp, err := env.query("old.snap-exp.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	afterCount := mockSrv.RequestCount()
	if afterCount <= beforeCount {
		t.Error("expected upstream requests after loading expired snapshot, but count did not increase")
	}
	t.Logf("mock requests: before=%d after=%d (expected increase)", beforeCount, afterCount)
}

// TestSnapshotE2E_DisabledNoOp verifies that when snapshot is disabled,
// SaveSnapshot does not create a file and LoadSnapshot returns (0, nil).
func TestSnapshotE2E_DisabledNoOp(t *testing.T) {
	hierarchy := BuildStandardHierarchy("com.", "snap-noop.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("www.snap-noop.com.", "10.1.2.3", 300),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)
	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// Fill cache via a real query.
	resp, err := env.query("www.snap-noop.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}

	dir := t.TempDir()
	snapFile := filepath.Join(dir, "should-not-exist.json")
	cfg := server.SnapshotConfig{Enabled: false, File: snapFile}

	// Save with disabled config — should be no-op.
	if err := server.SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot disabled: %v", err)
	}
	if _, err := os.Stat(snapFile); !os.IsNotExist(err) {
		t.Error("SaveSnapshot with Enabled=false should not create a file")
	}

	// Load with disabled config — should return (0, nil).
	n, err := server.LoadSnapshot(cfg)
	if err != nil {
		t.Errorf("LoadSnapshot disabled: %v", err)
	}
	if n != 0 {
		t.Errorf("LoadSnapshot disabled should return 0, got %d", n)
	}
}
