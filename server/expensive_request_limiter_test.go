package server

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"rec53/monitor"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

func TestExpensiveRequestLimiter_TryAcquireAndRelease(t *testing.T) {
	limiter := newExpensiveRequestLimiter(ExpensiveRequestLimitConfig{
		Mode:  ExpensiveRequestLimitModeEnabled,
		Limit: 1,
	})
	clientIP := netip.MustParseAddr("192.0.2.10")
	holder := newExpensiveRequestHolder(clientIP)
	ctx := withExpensiveRequestHolder(withExpensiveRequestLimiter(context.Background(), limiter), holder)

	if !tryAcquireExpensiveRequest(ctx, expensivePathForward) {
		t.Fatal("first acquire should succeed")
	}
	if !tryAcquireExpensiveRequest(ctx, expensivePathIterative) {
		t.Fatal("same request should not reacquire a second slot")
	}

	otherHolder := newExpensiveRequestHolder(clientIP)
	otherCtx := withExpensiveRequestHolder(withExpensiveRequestLimiter(context.Background(), limiter), otherHolder)
	if tryAcquireExpensiveRequest(otherCtx, expensivePathIterative) {
		t.Fatal("second request should be refused while slot is held")
	}

	holder.ReleaseIfHeld(limiter)
	if !tryAcquireExpensiveRequest(otherCtx, expensivePathIterative) {
		t.Fatal("acquire should succeed after release")
	}
}

func TestExpensiveRequestLimiter_ExtractClientIP(t *testing.T) {
	udpAddr := &net.UDPAddr{IP: net.ParseIP("::ffff:192.0.2.1"), Port: 53}
	if got := extractClientIP(udpAddr); got != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("extractClientIP() = %v, want 192.0.2.1", got)
	}

	tcpAddr := &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 53}
	if got := extractClientIP(tcpAddr); got != netip.MustParseAddr("2001:db8::1") {
		t.Fatalf("extractClientIP() = %v, want 2001:db8::1", got)
	}
}

func TestForwardLookupState_RefusedWhenOverLimit(t *testing.T) {
	saved := globalHostsForward.Load()
	defer setSnapshotForTest(saved)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("203.0.113.10"),
		})
		_ = w.WriteMsg(m)
	})
	srv := &dns.Server{PacketConn: pc, Handler: handler}
	go srv.ActivateAndServe()
	defer srv.Shutdown()

	setSnapshotForTest(&hostsForwardSnapshot{forwardZones: sortForwardZones([]ForwardZone{{Zone: "corp.test.", Upstreams: []string{pc.LocalAddr().String()}}})})

	limiter := newExpensiveRequestLimiter(ExpensiveRequestLimitConfig{Mode: ExpensiveRequestLimitModeEnabled, Limit: 1})
	clientIP := netip.MustParseAddr("192.0.2.99")
	blockingHolder := newExpensiveRequestHolder(clientIP)
	blockingCtx := withExpensiveRequestHolder(withExpensiveRequestLimiter(context.Background(), limiter), blockingHolder)
	if !tryAcquireExpensiveRequest(blockingCtx, expensivePathForward) {
		t.Fatal("pre-acquire should succeed")
	}
	defer blockingHolder.ReleaseIfHeld(limiter)

	req := new(dns.Msg)
	req.SetQuestion("a.corp.test.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)
	state := newForwardLookupState(req, resp, withExpensiveRequestHolder(withExpensiveRequestLimiter(context.Background(), limiter), newExpensiveRequestHolder(clientIP)))
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != FORWARD_LOOKUP_REFUSED {
		t.Fatalf("ret = %d, want %d", ret, FORWARD_LOOKUP_REFUSED)
	}
}

func TestChange_ReturnsRefusedAfterCacheMissOverLimit(t *testing.T) {
	monitor.InitMetricForTest()
	monitor.Rec53Log = zap.NewNop().Sugar()
	deleteAllCache()
	ResetHostsAndForwardForTest()
	before := testutil.ToFloat64(monitor.ExpensiveRequestLimitTotal.WithLabelValues("refused", "iterative"))

	msg := makeNSMsg("iter-limit.test.", "ns1.iter-limit.test.", 60)
	msg.Extra = append(msg.Extra, &dns.A{
		Hdr: dns.RR_Header{Name: "ns1.iter-limit.test.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.ParseIP("127.0.0.1").To4(),
	})
	setCacheCopy("iter-limit.test.", msg, 60)

	limiter := newExpensiveRequestLimiter(ExpensiveRequestLimitConfig{Mode: ExpensiveRequestLimitModeEnabled, Limit: 1})
	clientIP := netip.MustParseAddr("198.51.100.3")
	holder := newExpensiveRequestHolder(clientIP)
	ctx := withExpensiveRequestHolder(withExpensiveRequestLimiter(context.Background(), limiter), holder)
	if !tryAcquireExpensiveRequest(ctx, expensivePathForward) {
		t.Fatal("pre-acquire should succeed")
	}
	defer holder.ReleaseIfHeld(limiter)

	req := new(dns.Msg)
	req.SetQuestion("www.iter-limit.test.", dns.TypeA)
	resp, err := Change(newStateInitState(req, new(dns.Msg), withExpensiveRequestHolder(withExpensiveRequestLimiter(context.Background(), limiter), newExpensiveRequestHolder(clientIP))))
	if err != nil {
		t.Fatalf("Change() error = %v", err)
	}
	if resp == nil || resp.Rcode != dns.RcodeRefused {
		t.Fatalf("rcode = %v, want REFUSED", resp)
	}
	if got := testutil.ToFloat64(monitor.ExpensiveRequestLimitTotal.WithLabelValues("refused", "iterative")) - before; got != 1 {
		t.Fatalf("refused iterative metric delta = %f, want 1", got)
	}
}
