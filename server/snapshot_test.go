package server

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	// Ensure logger is initialised for snapshot tests (mirrors e2e/main_test.go pattern).
	if monitor.Rec53Log == nil {
		monitor.Rec53Log = zap.NewNop().Sugar()
	}
}

// makeNSMsg builds a minimal DNS response whose Ns section contains one NS RR.
func makeNSMsg(zone, ns string, ttl uint32) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(&dns.Msg{})
	m.Ns = append(m.Ns, &dns.NS{
		Hdr: dns.RR_Header{
			Name:   zone,
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns: ns,
	})
	return m
}

// makeAMsg builds a minimal DNS response whose Answer section contains one A RR.
func makeAMsg(name string, ttl uint32) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(&dns.Msg{})
	m.Answer = append(m.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		A: []byte{1, 2, 3, 4},
	})
	return m
}

// makeCNAMEMsg builds a minimal DNS response whose Answer section contains one CNAME RR.
func makeCNAMEMsg(name, target string, ttl uint32) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(&dns.Msg{})
	m.Answer = append(m.Answer, &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: target,
	})
	return m
}

// TestSaveSnapshotAllEntryTypes verifies that SaveSnapshot persists all cache
// entry types, not only NS-delegation entries.
func TestSaveSnapshotAllEntryTypes(t *testing.T) {
	deleteAllCache()

	// Populate cache with NS, A, and CNAME entries.
	nsMsg := makeNSMsg("example.com.", "ns1.example.com.", 3600)
	aMsg := makeAMsg("www.example.com.", 300)
	cnameMsg := makeCNAMEMsg("cdn.example.com.", "cdn.provider.net.", 600)
	setCacheCopy("example.com.:2", nsMsg, 3600)
	setCacheCopy("www.example.com.:1", aMsg, 300)
	setCacheCopy("cdn.example.com.:5", cnameMsg, 600)

	dir := t.TempDir()
	cfg := SnapshotConfig{Enabled: true, File: filepath.Join(dir, "snap.json")}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	data, _ := os.ReadFile(cfg.File)
	var sf snapshotFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(sf.Entries) != 3 {
		t.Fatalf("expected 3 entries (NS+A+CNAME), got %d", len(sf.Entries))
	}

	keys := make(map[string]bool)
	for _, e := range sf.Entries {
		keys[e.Key] = true
	}
	for _, want := range []string{"example.com.:2", "www.example.com.:1", "cdn.example.com.:5"} {
		if !keys[want] {
			t.Errorf("expected key %s in snapshot, not found", want)
		}
	}
}

// TestLoadSnapshotSkipsExpired verifies that expired entries are skipped and
// unexpired entries are written into the cache.
func TestLoadSnapshotSkipsExpired(t *testing.T) {
	deleteAllCache()

	dir := t.TempDir()
	file := filepath.Join(dir, "snap.json")

	// Build a snapshot file manually: one fresh entry (TTL 3600), one expired (TTL 1, savedAt 2s ago).
	fresh := makeNSMsg("fresh.com.", "ns1.fresh.com.", 3600)
	freshWire, _ := fresh.Pack()

	stale := makeNSMsg("stale.com.", "ns1.stale.com.", 1)
	staleWire, _ := stale.Pack()

	import64 := func(b []byte) string {
		return base64.StdEncoding.EncodeToString(b)
	}

	now := time.Now().Unix()
	sf := snapshotFile{Entries: []snapshotEntry{
		{Key: "fresh.com.:2", MsgB64: import64(freshWire), SavedAt: now},
		{Key: "stale.com.:2", MsgB64: import64(staleWire), SavedAt: now - 5}, // saved 5s ago, TTL=1 → expired
	}}
	data, _ := json.Marshal(sf)
	os.WriteFile(file, data, 0o644)

	cfg := SnapshotConfig{Enabled: true, File: file}
	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 imported entry, got %d", n)
	}

	if _, found := getCacheCopyByType("fresh.com.", dns.TypeNS); !found {
		t.Error("expected fresh.com. NS entry in cache, not found")
	}
	if _, found := getCacheCopyByType("stale.com.", dns.TypeNS); found {
		t.Error("stale.com. NS entry should not be in cache")
	}
}

// TestSnapshotRoundTrip verifies SaveSnapshot → LoadSnapshot restores the same entries.
func TestSnapshotRoundTrip(t *testing.T) {
	deleteAllCache()

	nsMsg := makeNSMsg("roundtrip.net.", "ns1.roundtrip.net.", 7200)
	setCacheCopy("roundtrip.net.:2", nsMsg, 7200)

	dir := t.TempDir()
	cfg := SnapshotConfig{Enabled: true, File: filepath.Join(dir, "snap.json")}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	// Clear cache, then restore.
	deleteAllCache()

	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 restored entry, got %d", n)
	}
	if _, found := getCacheCopyByType("roundtrip.net.", dns.TypeNS); !found {
		t.Error("expected roundtrip.net. NS entry restored to cache")
	}
}

// TestLoadSnapshotMissingFile verifies that a missing snapshot file is not an error.
func TestLoadSnapshotMissingFile(t *testing.T) {
	cfg := SnapshotConfig{Enabled: true, File: "/tmp/rec53-nonexistent-snapshot-99999.json"}
	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 entries, got %d", n)
	}
}

// TestLoadSnapshotCorruptJSON verifies that a corrupt snapshot returns an error (caller can degrade).
func TestLoadSnapshotCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "corrupt.json")
	os.WriteFile(file, []byte("not valid json {{{"), 0o644)

	cfg := SnapshotConfig{Enabled: true, File: file}
	n, err := LoadSnapshot(cfg)
	if err == nil {
		t.Error("expected error for corrupt JSON, got nil")
	}
	if n != 0 {
		t.Errorf("expected 0 entries on error, got %d", n)
	}
}

// TestSnapshotDisabledNoOp verifies that Enabled=false is a complete no-op for both Save and Load.
func TestSnapshotDisabledNoOp(t *testing.T) {
	deleteAllCache()

	nsMsg := makeNSMsg("disabled.com.", "ns1.disabled.com.", 3600)
	setCacheCopy("disabled.com.:2", nsMsg, 3600)

	dir := t.TempDir()
	file := filepath.Join(dir, "should-not-exist.json")
	cfg := SnapshotConfig{Enabled: false, File: file}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot with Enabled=false: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("SaveSnapshot with Enabled=false should not create a file")
	}

	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Errorf("LoadSnapshot with Enabled=false: %v", err)
	}
	if n != 0 {
		t.Errorf("LoadSnapshot with Enabled=false should return 0, got %d", n)
	}
}

// TestSnapshotRoundTripARecord verifies that A answer records survive a save → load cycle.
func TestSnapshotRoundTripARecord(t *testing.T) {
	deleteAllCache()

	aMsg := makeAMsg("api.example.com.", 600)
	setCacheCopy("api.example.com.:1", aMsg, 600)

	dir := t.TempDir()
	cfg := SnapshotConfig{Enabled: true, File: filepath.Join(dir, "snap.json")}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	deleteAllCache()

	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 restored entry, got %d", n)
	}
	if _, found := getCacheCopyByType("api.example.com.", dns.TypeA); !found {
		t.Error("expected api.example.com. A entry restored to cache")
	}
}

// TestSnapshotRoundTripCNAME verifies that CNAME records survive a save → load cycle.
func TestSnapshotRoundTripCNAME(t *testing.T) {
	deleteAllCache()

	cnameMsg := makeCNAMEMsg("cdn.example.com.", "cdn.provider.net.", 1200)
	setCacheCopy("cdn.example.com.:5", cnameMsg, 1200)

	dir := t.TempDir()
	cfg := SnapshotConfig{Enabled: true, File: filepath.Join(dir, "snap.json")}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	deleteAllCache()

	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 restored entry, got %d", n)
	}
	if _, found := getCacheCopyByType("cdn.example.com.", dns.TypeCNAME); !found {
		t.Error("expected cdn.example.com. CNAME entry restored to cache")
	}
}

// TestRemainingTTLAnswerOnly verifies that remainingTTL works correctly when
// only msg.Answer is populated (Ns and Extra are empty).
func TestRemainingTTLAnswerOnly(t *testing.T) {
	msg := makeAMsg("pure.example.com.", 300)
	// msg.Ns and msg.Extra are nil — only Answer has RRs.

	savedAt := int64(1000)
	now := int64(1120) // 120 seconds elapsed

	got := remainingTTL(msg, savedAt, now)
	want := uint32(180) // 300 - 120
	if got != want {
		t.Errorf("remainingTTL = %d, want %d", got, want)
	}
}

// TestRemainingTTLMixedSections verifies that remainingTTL returns the minimum
// across Answer, Ns, and Extra sections.
func TestRemainingTTLMixedSections(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{})

	// Answer: TTL 300
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "mix.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   []byte{1, 2, 3, 4},
	})
	// Ns: TTL 600
	msg.Ns = append(msg.Ns, &dns.NS{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 600},
		Ns:  "ns1.example.com.",
	})
	// Extra: TTL 200
	msg.Extra = append(msg.Extra, &dns.A{
		Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 200},
		A:   []byte{5, 6, 7, 8},
	})

	savedAt := int64(1000)
	now := int64(1100) // 100 seconds elapsed

	got := remainingTTL(msg, savedAt, now)
	// Extra has TTL 200 → remaining 100; Answer has 300 → 200; Ns has 600 → 500.
	// Minimum is 100.
	want := uint32(100)
	if got != want {
		t.Errorf("remainingTTL = %d, want %d", got, want)
	}
}

// TestRemainingTTLAllExpired verifies that remainingTTL returns 0 when all RRs
// across all sections have expired.
func TestRemainingTTLAllExpired(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "old.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   []byte{1, 2, 3, 4},
	})
	msg.Ns = append(msg.Ns, &dns.NS{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 30},
		Ns:  "ns1.example.com.",
	})

	savedAt := int64(1000)
	now := int64(1100) // 100s elapsed, both TTLs (60, 30) exceeded

	got := remainingTTL(msg, savedAt, now)
	if got != 0 {
		t.Errorf("remainingTTL = %d, want 0 (all expired)", got)
	}
}

// TestSnapshotEmptyFileNoOp verifies that File="" is a complete no-op even when Enabled=true.
func TestSnapshotEmptyFileNoOp(t *testing.T) {
	deleteAllCache()

	nsMsg := makeNSMsg("nofile.com.", "ns1.nofile.com.", 3600)
	setCacheCopy("nofile.com.:2", nsMsg, 3600)

	cfg := SnapshotConfig{Enabled: true, File: ""}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot with File='': %v", err)
	}

	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Errorf("LoadSnapshot with File='': %v", err)
	}
	if n != 0 {
		t.Errorf("LoadSnapshot with File='' should return 0, got %d", n)
	}
}

// TestLoadSnapshotCorruptMsgB64 verifies that entries with invalid base64 or
// unparseable wire format are silently skipped rather than causing a fatal error.
func TestLoadSnapshotCorruptMsgB64(t *testing.T) {
	deleteAllCache()

	dir := t.TempDir()
	file := filepath.Join(dir, "corrupt-b64.json")

	// Build a snapshot with one valid entry and one with bad base64.
	nsMsg := makeNSMsg("good.com.", "ns1.good.com.", 3600)
	wire, _ := nsMsg.Pack()
	now := time.Now().Unix()

	sf := snapshotFile{Entries: []snapshotEntry{
		{Key: "good.com.:2", MsgB64: base64.StdEncoding.EncodeToString(wire), SavedAt: now},
		{Key: "bad-b64.com.:2", MsgB64: "!!!not-valid-base64!!!", SavedAt: now},
		{Key: "bad-wire.com.:2", MsgB64: base64.StdEncoding.EncodeToString([]byte("not dns wire")), SavedAt: now},
	}}
	data, _ := json.Marshal(sf)
	os.WriteFile(file, data, 0o644)

	cfg := SnapshotConfig{Enabled: true, File: file}
	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot should not return error for corrupt entries: %v", err)
	}
	// Only the valid entry should be imported.
	if n != 1 {
		t.Errorf("expected 1 imported entry (skipping 2 corrupt), got %d", n)
	}
	if _, found := getCacheCopyByType("good.com.", dns.TypeNS); !found {
		t.Error("expected good.com. NS entry in cache")
	}
}

// TestSnapshotFileDirAutoCreated verifies that SaveSnapshot creates intermediate
// directories automatically when the target path does not yet exist.
func TestSnapshotFileDirAutoCreated(t *testing.T) {
	deleteAllCache()

	nsMsg := makeNSMsg("autodir.com.", "ns1.autodir.com.", 3600)
	setCacheCopy("autodir.com.:2", nsMsg, 3600)

	// Use a nested path that does not exist yet.
	dir := t.TempDir()
	file := filepath.Join(dir, "sub", "nested", "snap.json")
	cfg := SnapshotConfig{Enabled: true, File: file}

	if err := SaveSnapshot(cfg); err != nil {
		t.Fatalf("SaveSnapshot should create intermediate dirs: %v", err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("expected snapshot file at %s: %v", file, err)
	}

	// Verify it can be loaded back.
	deleteAllCache()
	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot from auto-created dir: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 restored entry, got %d", n)
	}
}

// TestRemainingTTLEmptyMsg verifies that remainingTTL returns 0 when all three
// sections (Answer, Ns, Extra) are empty.
func TestRemainingTTLEmptyMsg(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{})
	// All sections empty.

	got := remainingTTL(msg, 1000, 1100)
	if got != 0 {
		t.Errorf("remainingTTL = %d, want 0 for empty msg", got)
	}
}

// TestSnapshotBackwardCompatibility verifies that a snapshot file written by
// the old NS-only version (containing only NS entries) is correctly loaded
// by the new version.
func TestSnapshotBackwardCompatibility(t *testing.T) {
	deleteAllCache()

	dir := t.TempDir()
	file := filepath.Join(dir, "old-format.json")

	// Simulate an old-format snapshot: only NS entries, same JSON structure.
	nsMsg := makeNSMsg("legacy.com.", "ns1.legacy.com.", 7200)
	wire, _ := nsMsg.Pack()
	now := time.Now().Unix()

	sf := snapshotFile{Entries: []snapshotEntry{
		{Key: "legacy.com.:2", MsgB64: base64.StdEncoding.EncodeToString(wire), SavedAt: now},
	}}
	data, _ := json.Marshal(sf)
	os.WriteFile(file, data, 0o644)

	cfg := SnapshotConfig{Enabled: true, File: file}
	n, err := LoadSnapshot(cfg)
	if err != nil {
		t.Fatalf("LoadSnapshot old-format: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 imported entry from old-format snapshot, got %d", n)
	}
	if _, found := getCacheCopyByType("legacy.com.", dns.TypeNS); !found {
		t.Error("expected legacy.com. NS entry restored from old-format snapshot")
	}
}
