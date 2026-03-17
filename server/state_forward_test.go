package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestForwardLookupState_NilInput(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		resp := new(dns.Msg)
		resp.SetQuestion("example.com.", dns.TypeA)
		state := newForwardLookupState(nil, resp, context.Background())
		ret, err := state.handle(nil, resp)
		if err == nil {
			t.Error("expected error for nil request")
		}
		if ret != FORWARD_LOOKUP_COMMON_ERROR {
			t.Errorf("expected FORWARD_LOOKUP_COMMON_ERROR (%d), got %d", FORWARD_LOOKUP_COMMON_ERROR, ret)
		}
	})

	t.Run("nil response", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		state := newForwardLookupState(req, nil, context.Background())
		ret, err := state.handle(req, nil)
		if err == nil {
			t.Error("expected error for nil response")
		}
		if ret != FORWARD_LOOKUP_COMMON_ERROR {
			t.Errorf("expected FORWARD_LOOKUP_COMMON_ERROR (%d), got %d", FORWARD_LOOKUP_COMMON_ERROR, ret)
		}
	})
}

func TestForwardLookupState_EmptyZones(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	setSnapshotForTest(&hostsForwardSnapshot{})

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_MISS {
		t.Errorf("expected FORWARD_LOOKUP_MISS (%d), got %d", FORWARD_LOOKUP_MISS, ret)
	}
}

func TestForwardLookupState_NoMatchingZone(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	zones := sortForwardZones([]ForwardZone{
		{Zone: "corp.internal", Upstreams: []string{"10.0.0.1:53"}},
	})
	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: zones})

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_MISS {
		t.Errorf("expected FORWARD_LOOKUP_MISS (%d), got %d", FORWARD_LOOKUP_MISS, ret)
	}
}

func TestForwardLookupState_LongestSuffixMatch(t *testing.T) {
	// startMockUpstream returns a mock DNS server that responds to all queries
	// with the given IP. Returns the server and its address.
	startMock := func(t *testing.T, ip string) (*dns.Server, string) {
		t.Helper()
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			m := new(dns.Msg)
			m.SetReply(r)
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(ip),
			})
			w.WriteMsg(m)
		})
		srv := &dns.Server{PacketConn: pc, Handler: handler}
		go srv.ActivateAndServe()
		t.Cleanup(func() { srv.Shutdown() })
		return srv, pc.LocalAddr().String()
	}

	// Two mock upstreams: short zone returns 1.1.1.1, long zone returns 2.2.2.2
	_, shortAddr := startMock(t, "1.1.1.1")
	_, longAddr := startMock(t, "2.2.2.2")

	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	zones := sortForwardZones([]ForwardZone{
		{Zone: "internal", Upstreams: []string{shortAddr}},
		{Zone: "deep.internal", Upstreams: []string{longAddr}},
	})
	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: zones})

	req := new(dns.Msg)
	req.SetQuestion("app.deep.internal.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_HIT {
		t.Fatalf("expected FORWARD_LOOKUP_HIT, got %d", ret)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected *dns.A, got %T", resp.Answer[0])
	}
	// longest match "deep.internal" should be used → 2.2.2.2
	if !a.A.Equal(net.ParseIP("2.2.2.2")) {
		t.Errorf("expected 2.2.2.2 (longest suffix), got %s", a.A)
	}
}

func TestForwardLookupState_UpstreamSuccess(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("192.168.1.100"),
		})
		w.WriteMsg(m)
	})
	srv := &dns.Server{PacketConn: pc, Handler: handler}
	go srv.ActivateAndServe()
	defer srv.Shutdown()

	addr := pc.LocalAddr().String()
	zones := sortForwardZones([]ForwardZone{
		{Zone: "corp.example.com", Upstreams: []string{addr}},
	})
	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: zones})

	req := new(dns.Msg)
	req.SetQuestion("app.corp.example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_HIT {
		t.Fatalf("expected FORWARD_LOOKUP_HIT, got %d", ret)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	a := resp.Answer[0].(*dns.A)
	if !a.A.Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("expected 192.168.1.100, got %s", a.A)
	}
}

func TestForwardLookupState_AllUpstreamsFail(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	// Use addresses that will not respond (unreachable)
	zones := sortForwardZones([]ForwardZone{
		{Zone: "fail.example.com", Upstreams: []string{"192.0.2.1:53", "192.0.2.2:53"}},
	})
	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: zones})

	// Use a short timeout so the test doesn't take forever
	savedTimeout := globalUpstreamTimeout
	globalUpstreamTimeout = 200 * time.Millisecond
	defer func() { globalUpstreamTimeout = savedTimeout }()

	req := new(dns.Msg)
	req.SetQuestion("host.fail.example.com.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error from handle: %v", err)
	}
	if ret != FORWARD_LOOKUP_SERVFAIL {
		t.Errorf("expected FORWARD_LOOKUP_SERVFAIL (%d), got %d", FORWARD_LOOKUP_SERVFAIL, ret)
	}
}

func TestForwardLookupState_UpstreamFailover(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	// First upstream: unreachable. Second upstream: real mock.
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("10.10.10.10"),
		})
		w.WriteMsg(m)
	})
	srv := &dns.Server{PacketConn: pc, Handler: handler}
	go srv.ActivateAndServe()
	defer srv.Shutdown()

	goodAddr := pc.LocalAddr().String()

	savedTimeout := globalUpstreamTimeout
	globalUpstreamTimeout = 200 * time.Millisecond
	defer func() { globalUpstreamTimeout = savedTimeout }()

	zones := sortForwardZones([]ForwardZone{
		{Zone: "failover.test", Upstreams: []string{"192.0.2.1:53", goodAddr}},
	})
	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: zones})

	req := new(dns.Msg)
	req.SetQuestion("app.failover.test.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_HIT {
		t.Fatalf("expected FORWARD_LOOKUP_HIT, got %d", ret)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	a := resp.Answer[0].(*dns.A)
	if !a.A.Equal(net.ParseIP("10.10.10.10")) {
		t.Errorf("expected 10.10.10.10, got %s", a.A)
	}
}

func TestForwardLookupState_GetCurrentState(t *testing.T) {
	state := newForwardLookupState(new(dns.Msg), new(dns.Msg), context.Background())
	if state.getCurrentState() != FORWARD_LOOKUP {
		t.Errorf("expected FORWARD_LOOKUP (%d), got %d", FORWARD_LOOKUP, state.getCurrentState())
	}
}

func TestForwardLookupState_NilContext(t *testing.T) {
	state := newForwardLookupState(new(dns.Msg), new(dns.Msg), nil)
	if state.getContext() == nil {
		t.Error("expected non-nil context when nil is passed")
	}
}

// TestForwardLookupState_TCFlagPreserved verifies that a TC=1 (truncated)
// response from an upstream is forwarded to the client with TC still set,
// so the client can retry over TCP.
func TestForwardLookupState_TCFlagPreserved(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Truncated = true // upstream says "retry over TCP"
		m.RecursionAvailable = true
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("10.0.0.1"),
		})
		w.WriteMsg(m)
	})
	srv := &dns.Server{PacketConn: pc, Handler: handler}
	go srv.ActivateAndServe()
	defer srv.Shutdown()

	addr := pc.LocalAddr().String()
	zones := sortForwardZones([]ForwardZone{
		{Zone: "tc.example.", Upstreams: []string{addr}},
	})
	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: zones})

	req := new(dns.Msg)
	req.SetQuestion("big.tc.example.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)

	state := newForwardLookupState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_HIT {
		t.Fatalf("expected FORWARD_LOOKUP_HIT, got %d", ret)
	}
	if !resp.Truncated {
		t.Error("expected TC=1 to be preserved from upstream response")
	}
	if !resp.RecursionAvailable {
		t.Error("expected RA=1 to be preserved from upstream response")
	}
	// Response id must match the original request, not the forwarded query id.
	if resp.Id != req.Id {
		t.Errorf("expected response Id=%d to match request Id=%d", resp.Id, req.Id)
	}
}
