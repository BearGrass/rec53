package server

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestHostsLookupState_NilInput(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		resp := new(dns.Msg)
		resp.SetQuestion("example.com.", dns.TypeA)
		state := newHostsLookupState(nil, resp, context.Background())
		ret, err := state.handle(nil, resp)
		if err == nil {
			t.Error("expected error for nil request")
		}
		if ret != HOSTS_LOOKUP_COMMON_ERROR {
			t.Errorf("expected HOSTS_LOOKUP_COMMON_ERROR (%d), got %d", HOSTS_LOOKUP_COMMON_ERROR, ret)
		}
	})

	t.Run("nil response", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		state := newHostsLookupState(req, nil, context.Background())
		ret, err := state.handle(req, nil)
		if err == nil {
			t.Error("expected error for nil response")
		}
		if ret != HOSTS_LOOKUP_COMMON_ERROR {
			t.Errorf("expected HOSTS_LOOKUP_COMMON_ERROR (%d), got %d", HOSTS_LOOKUP_COMMON_ERROR, ret)
		}
	})
}

func TestHostsLookupState_EmptyHostsMap(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	setSnapshotForTest(&hostsForwardSnapshot{})

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_MISS {
		t.Errorf("expected HOSTS_LOOKUP_MISS (%d), got %d", HOSTS_LOOKUP_MISS, ret)
	}
}

func TestHostsLookupState_ARecordHit(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	entries := []HostEntry{
		{Name: "myhost.local", Type: "A", Value: "10.0.0.1", TTL: 120},
	}
	hostsMap, hostsNames := compileHostsEntries(entries)
	setSnapshotForTest(&hostsForwardSnapshot{hostsMap: hostsMap, hostsNames: hostsNames})

	req := new(dns.Msg)
	req.SetQuestion("myhost.local.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_HIT {
		t.Fatalf("expected HOSTS_LOOKUP_HIT (%d), got %d", HOSTS_LOOKUP_HIT, ret)
	}
	if !resp.Authoritative {
		t.Error("expected AA=true")
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Errorf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected *dns.A, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("10.0.0.1")) {
		t.Errorf("expected 10.0.0.1, got %s", a.A)
	}
}

func TestHostsLookupState_AAAARecordHit(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	entries := []HostEntry{
		{Name: "v6host.local", Type: "AAAA", Value: "::1", TTL: 60},
	}
	hostsMap, hostsNames := compileHostsEntries(entries)
	setSnapshotForTest(&hostsForwardSnapshot{hostsMap: hostsMap, hostsNames: hostsNames})

	req := new(dns.Msg)
	req.SetQuestion("v6host.local.", dns.TypeAAAA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_HIT {
		t.Fatalf("expected HOSTS_LOOKUP_HIT, got %d", ret)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	aaaa, ok := resp.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("expected *dns.AAAA, got %T", resp.Answer[0])
	}
	if !aaaa.AAAA.Equal(net.ParseIP("::1")) {
		t.Errorf("expected ::1, got %s", aaaa.AAAA)
	}
}

func TestHostsLookupState_CNAMERecordHit(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	entries := []HostEntry{
		{Name: "alias.local", Type: "CNAME", Value: "real.local", TTL: 30},
	}
	hostsMap, hostsNames := compileHostsEntries(entries)
	setSnapshotForTest(&hostsForwardSnapshot{hostsMap: hostsMap, hostsNames: hostsNames})

	req := new(dns.Msg)
	req.SetQuestion("alias.local.", dns.TypeCNAME)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_HIT {
		t.Fatalf("expected HOSTS_LOOKUP_HIT, got %d", ret)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	cname, ok := resp.Answer[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("expected *dns.CNAME, got %T", resp.Answer[0])
	}
	if cname.Target != "real.local." {
		t.Errorf("expected real.local., got %s", cname.Target)
	}
}

func TestHostsLookupState_TypeMismatchNODATA(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	entries := []HostEntry{
		{Name: "myhost.local", Type: "A", Value: "10.0.0.1"},
	}
	hostsMap, hostsNames := compileHostsEntries(entries)
	setSnapshotForTest(&hostsForwardSnapshot{hostsMap: hostsMap, hostsNames: hostsNames})

	req := new(dns.Msg)
	req.SetQuestion("myhost.local.", dns.TypeAAAA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_HIT {
		t.Fatalf("expected HOSTS_LOOKUP_HIT (NODATA), got %d", ret)
	}
	if !resp.Authoritative {
		t.Error("expected AA=true for NODATA")
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Errorf("expected NOERROR for NODATA, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) != 0 {
		t.Errorf("expected 0 answers for NODATA, got %d", len(resp.Answer))
	}
}

func TestHostsLookupState_Miss(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	entries := []HostEntry{
		{Name: "known.local", Type: "A", Value: "10.0.0.1"},
	}
	hostsMap, hostsNames := compileHostsEntries(entries)
	setSnapshotForTest(&hostsForwardSnapshot{hostsMap: hostsMap, hostsNames: hostsNames})

	req := new(dns.Msg)
	req.SetQuestion("unknown.example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_MISS {
		t.Errorf("expected HOSTS_LOOKUP_MISS (%d), got %d", HOSTS_LOOKUP_MISS, ret)
	}
}

func TestHostsLookupState_MultipleRecordsSameName(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	entries := []HostEntry{
		{Name: "multi.local", Type: "A", Value: "10.0.0.1", TTL: 60},
		{Name: "multi.local", Type: "A", Value: "10.0.0.2", TTL: 60},
	}
	hostsMap, hostsNames := compileHostsEntries(entries)
	setSnapshotForTest(&hostsForwardSnapshot{hostsMap: hostsMap, hostsNames: hostsNames})

	req := new(dns.Msg)
	req.SetQuestion("multi.local.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newHostsLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != HOSTS_LOOKUP_HIT {
		t.Fatalf("expected HOSTS_LOOKUP_HIT, got %d", ret)
	}
	if len(resp.Answer) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(resp.Answer))
	}
}

func TestHostsLookupState_GetCurrentState(t *testing.T) {
	state := newHostsLookupState(new(dns.Msg), new(dns.Msg), context.Background())
	if state.getCurrentState() != HOSTS_LOOKUP {
		t.Errorf("expected HOSTS_LOOKUP (%d), got %d", HOSTS_LOOKUP, state.getCurrentState())
	}
}

func TestHostsLookupState_NilContext(t *testing.T) {
	state := newHostsLookupState(new(dns.Msg), new(dns.Msg), nil)
	if state.getContext() == nil {
		t.Error("expected non-nil context when nil is passed")
	}
}
