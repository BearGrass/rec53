package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rec53/monitor"

	"github.com/cilium/ebpf"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

func init() {
	if monitor.Rec53Log == nil {
		monitor.Rec53Log = zap.NewNop().Sugar()
	}
	if monitor.Rec53Metric == nil {
		monitor.InitMetricForTest()
	}
}

type badStateMachine struct{}

func (badStateMachine) getCurrentState() int                   { return 999 }
func (badStateMachine) getRequest() *dns.Msg                   { return new(dns.Msg) }
func (badStateMachine) getResponse() *dns.Msg                  { return new(dns.Msg) }
func (badStateMachine) handle(*dns.Msg, *dns.Msg) (int, error) { return 0, nil }
func (badStateMachine) getContext() context.Context            { return context.Background() }

func TestCacheObservabilityMetrics(t *testing.T) {
	monitor.InitMetricForTest()
	deleteAllCache()

	posBefore := testutil.ToFloat64(monitor.CacheLookupTotal.WithLabelValues("positive_hit"))
	negBefore := testutil.ToFloat64(monitor.CacheLookupTotal.WithLabelValues("negative_hit"))
	missBefore := testutil.ToFloat64(monitor.CacheLookupTotal.WithLabelValues("miss"))
	writeBefore := testutil.ToFloat64(monitor.CacheLifecycleTotal.WithLabelValues("write"))
	expireBefore := testutil.ToFloat64(monitor.CacheLifecycleTotal.WithLabelValues("delete_expired"))

	pos := new(dns.Msg)
	pos.SetReply(&dns.Msg{})
	pos.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "cache-positive.example.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("192.0.2.10"),
		},
	}
	setCacheCopyByType("cache-positive.example.", dns.TypeA, pos, 60)
	if _, found := getCacheCopyByType("cache-positive.example.", dns.TypeA); !found {
		t.Fatal("expected positive cache hit")
	}

	neg := new(dns.Msg)
	neg.Rcode = dns.RcodeNameError
	neg.Ns = []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{Name: "example.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 60},
			Ns:  "ns1.example.",
		},
	}
	setCacheCopyByType("cache-negative.example.", dns.TypeA, neg, 60)
	if _, found := getCacheCopyByType("cache-negative.example.", dns.TypeA); !found {
		t.Fatal("expected negative cache hit")
	}

	if _, found := getCacheCopyByType("cache-miss.example.", dns.TypeA); found {
		t.Fatal("expected cache miss")
	}

	expiring := new(dns.Msg)
	expiring.SetReply(&dns.Msg{})
	expiring.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "cache-expiring.example.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 1},
			A:   net.ParseIP("192.0.2.11"),
		},
	}
	setCacheCopyByType("cache-expiring.example.", dns.TypeA, expiring, 1)
	time.Sleep(1100 * time.Millisecond)
	deleteExpiredCache()

	if got := testutil.ToFloat64(monitor.CacheLookupTotal.WithLabelValues("positive_hit")) - posBefore; got != 1 {
		t.Fatalf("positive_hit delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.CacheLookupTotal.WithLabelValues("negative_hit")) - negBefore; got != 1 {
		t.Fatalf("negative_hit delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.CacheLookupTotal.WithLabelValues("miss")) - missBefore; got != 1 {
		t.Fatalf("miss delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.CacheLifecycleTotal.WithLabelValues("write")) - writeBefore; got < 3 {
		t.Fatalf("write delta = %f, want >= 3", got)
	}
	if got := testutil.ToFloat64(monitor.CacheLifecycleTotal.WithLabelValues("delete_expired")) - expireBefore; got < 1 {
		t.Fatalf("delete_expired delta = %f, want >= 1", got)
	}
	if got := testutil.ToFloat64(monitor.CacheEntries); got < 2 {
		t.Fatalf("CacheEntries = %f, want >= 2 after cleanup", got)
	}
}

func TestSnapshotObservabilityMetrics(t *testing.T) {
	monitor.InitMetricForTest()
	deleteAllCache()

	saveBefore := testutil.ToFloat64(monitor.SnapshotOperationsTotal.WithLabelValues("save", "success"))
	loadBefore := testutil.ToFloat64(monitor.SnapshotOperationsTotal.WithLabelValues("load", "success"))
	importedBefore := testutil.ToFloat64(monitor.SnapshotEntriesTotal.WithLabelValues("load", "imported"))
	expiredBefore := testutil.ToFloat64(monitor.SnapshotEntriesTotal.WithLabelValues("load", "skipped_expired"))
	corruptBefore := testutil.ToFloat64(monitor.SnapshotEntriesTotal.WithLabelValues("load", "skipped_corrupt"))

	msg := makeAMsg("snapshot-observability.example.", 300)
	setCacheCopy("snapshot-observability.example.:1", msg, 300)

	dir := t.TempDir()
	saveCfg := SnapshotConfig{Enabled: true, File: filepath.Join(dir, "saved.json")}
	if err := SaveSnapshot(saveCfg); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	fresh := makeNSMsg("fresh-ob.example.", "ns1.fresh-ob.example.", 3600)
	freshWire, _ := fresh.Pack()
	stale := makeNSMsg("stale-ob.example.", "ns1.stale-ob.example.", 1)
	staleWire, _ := stale.Pack()
	now := time.Now().Unix()
	sf := snapshotFile{Entries: []snapshotEntry{
		{Key: "fresh-ob.example.:2", MsgB64: base64.StdEncoding.EncodeToString(freshWire), SavedAt: now},
		{Key: "stale-ob.example.:2", MsgB64: base64.StdEncoding.EncodeToString(staleWire), SavedAt: now - 5},
		{Key: "corrupt-ob.example.:2", MsgB64: "not-base64", SavedAt: now},
	}}
	data, _ := json.Marshal(sf)
	loadFile := filepath.Join(dir, "load.json")
	if err := os.WriteFile(loadFile, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	n, err := LoadSnapshot(SnapshotConfig{Enabled: true, File: loadFile})
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if n != 1 {
		t.Fatalf("LoadSnapshot imported %d, want 1", n)
	}

	if got := testutil.ToFloat64(monitor.SnapshotOperationsTotal.WithLabelValues("save", "success")) - saveBefore; got != 1 {
		t.Fatalf("save success delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.SnapshotOperationsTotal.WithLabelValues("load", "success")) - loadBefore; got != 1 {
		t.Fatalf("load success delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.SnapshotEntriesTotal.WithLabelValues("load", "imported")) - importedBefore; got != 1 {
		t.Fatalf("imported delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.SnapshotEntriesTotal.WithLabelValues("load", "skipped_expired")) - expiredBefore; got != 1 {
		t.Fatalf("skipped_expired delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.SnapshotEntriesTotal.WithLabelValues("load", "skipped_corrupt")) - corruptBefore; got != 1 {
		t.Fatalf("skipped_corrupt delta = %f, want 1", got)
	}
}

func TestUpstreamObservabilityMetrics(t *testing.T) {
	monitor.InitMetricForTest()

	singleBefore := testutil.ToFloat64(monitor.UpstreamWinnerTotal.WithLabelValues("single"))
	timeoutBefore := testutil.ToFloat64(monitor.UpstreamFailuresTotal.WithLabelValues("timeout", "NONE"))

	handler := &MockDNSHandler{response: new(dns.Msg)}
	server, err := NewMockDNSServer("udp", handler)
	if err != nil {
		t.Fatalf("NewMockDNSServer: %v", err)
	}
	defer server.Stop()

	port := server.Server.PacketConn.LocalAddr().(*net.UDPAddr).Port
	query := new(dns.Msg)
	query.SetQuestion("example.com.", dns.TypeA)
	if _, err := queryHappyEyeballs(context.Background(), query, server.GetIP(), "", fmt.Sprintf("%d", port)); err != nil {
		t.Fatalf("queryHappyEyeballs single path: %v", err)
	}

	timeoutConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer timeoutConn.Close()

	oldTimeout := GetUpstreamTimeout()
	SetUpstreamTimeout(100 * time.Millisecond)
	defer SetUpstreamTimeout(oldTimeout)

	timeoutPort := timeoutConn.LocalAddr().(*net.UDPAddr).Port
	_, _ = queryHappyEyeballs(context.Background(), query, "127.0.0.1", "", fmt.Sprintf("%d", timeoutPort))

	if got := testutil.ToFloat64(monitor.UpstreamWinnerTotal.WithLabelValues("single")) - singleBefore; got != 1 {
		t.Fatalf("single winner delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.UpstreamFailuresTotal.WithLabelValues("timeout", "NONE")) - timeoutBefore; got != 1 {
		t.Fatalf("timeout delta = %f, want 1", got)
	}
}

func TestXDPSyncObservabilityMetrics(t *testing.T) {
	monitor.InitMetricForTest()

	keyBefore := testutil.ToFloat64(monitor.XDPSyncErrorsTotal.WithLabelValues("key_build"))
	valueBefore := testutil.ToFloat64(monitor.XDPSyncErrorsTotal.WithLabelValues("value_build"))
	updateBefore := testutil.ToFloat64(monitor.XDPSyncErrorsTotal.WithLabelValues("update"))

	origMap := globalXDPCacheMap.Load()
	defer func() { globalXDPCacheMap.Store(origMap) }()
	globalXDPCacheMap.Store(new(ebpf.Map))

	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{
		Question: []dns.Question{{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}},
	})
	msg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("192.0.2.30"),
		},
	}

	syncToBPFMap(strings.Repeat("a.", 128), dns.TypeA, msg, 60)

	oversized := new(dns.Msg)
	oversized.SetReply(&dns.Msg{
		Question: []dns.Question{{Name: "example.com.", Qtype: dns.TypeTXT, Qclass: dns.ClassINET}},
	})
	for i := 0; i < 20; i++ {
		oversized.Answer = append(oversized.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
			Txt: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		})
	}
	syncToBPFMap("example.com.", dns.TypeTXT, oversized, 60)
	syncToBPFMap("example.com.", dns.TypeA, msg, 60)

	if got := testutil.ToFloat64(monitor.XDPSyncErrorsTotal.WithLabelValues("key_build")) - keyBefore; got != 1 {
		t.Fatalf("key_build delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.XDPSyncErrorsTotal.WithLabelValues("value_build")) - valueBefore; got != 1 {
		t.Fatalf("value_build delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.XDPSyncErrorsTotal.WithLabelValues("update")) - updateBefore; got != 1 {
		t.Fatalf("update delta = %f, want 1", got)
	}
}

func TestStateMachineObservabilityMetrics(t *testing.T) {
	monitor.InitMetricForTest()

	stageBefore := testutil.ToFloat64(monitor.StateMachineStageTotal.WithLabelValues("unknown"))
	failBefore := testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("unknown_state"))

	if _, err := Change(badStateMachine{}); err == nil {
		t.Fatal("expected Change to fail for unknown state")
	}

	if got := testutil.ToFloat64(monitor.StateMachineStageTotal.WithLabelValues("unknown")) - stageBefore; got != 1 {
		t.Fatalf("unknown stage delta = %f, want 1", got)
	}
	if got := testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("unknown_state")) - failBefore; got != 1 {
		t.Fatalf("unknown_state delta = %f, want 1", got)
	}
}
