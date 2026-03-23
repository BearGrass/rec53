package server

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/miekg/dns"
)

func BenchmarkExpensiveRequestProtectionCacheHit(b *testing.B) {
	benchExpensiveRequestScenario(b, "cache_hit", func(limitCfg ExpensiveRequestLimitConfig) func(*testing.B) {
		FlushCacheForTest()
		ResetHostsAndForwardForTest()

		cached := &dns.Msg{}
		cached.SetQuestion("bench-cache.example.", dns.TypeA)
		cached.Response = true
		rr, _ := dns.NewRR("bench-cache.example. 60 IN A 1.2.3.4")
		cached.Answer = append(cached.Answer, rr)
		setCacheCopyByType("bench-cache.example.", dns.TypeA, cached, 60)

		s := NewServerWithFullConfig("127.0.0.1:0", 1, WarmupConfig{Enabled: false}, SnapshotConfig{}, nil, nil, "", limitCfg)
		writer := &mockResponseWriterWithCapture{addr: &net.UDPAddr{IP: net.ParseIP("192.0.2.20"), Port: 53530}}

		return func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := new(dns.Msg)
				req.SetQuestion("bench-cache.example.", dns.TypeA)
				writer.written = nil
				s.ServeDNS(writer, req)
			}
		}
	})
}

func BenchmarkExpensiveRequestProtectionForwardMiss(b *testing.B) {
	benchExpensiveRequestScenario(b, "forward_miss", func(limitCfg ExpensiveRequestLimitConfig) func(*testing.B) {
		FlushCacheForTest()
		ResetHostsAndForwardForTest()

		forwardResp := &dns.Msg{}
		forwardResp.SetQuestion("www.forward-bench.test.", dns.TypeA)
		rr, _ := dns.NewRR("www.forward-bench.test. 60 IN A 192.0.2.44")
		forwardResp.Answer = append(forwardResp.Answer, rr)
		forwardServer, err := NewMockDNSServer("udp", &MockDNSHandler{response: forwardResp})
		if err != nil {
			b.Fatalf("NewMockDNSServer: %v", err)
		}
		b.Cleanup(func() { forwardServer.Stop() })

		s := NewServerWithFullConfig(
			"127.0.0.1:0",
			1,
			WarmupConfig{Enabled: false},
			SnapshotConfig{},
			nil,
			[]ForwardZone{{Zone: "forward-bench.test.", Upstreams: []string{forwardServer.Addr}}},
			"",
			limitCfg,
		)
		writer := &mockResponseWriterWithCapture{addr: &net.UDPAddr{IP: net.ParseIP("192.0.2.21"), Port: 53531}}

		return func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := new(dns.Msg)
				req.SetQuestion("www.forward-bench.test.", dns.TypeA)
				writer.written = nil
				s.ServeDNS(writer, req)
			}
		}
	})
}

func BenchmarkExpensiveRequestProtectionIterativeMiss(b *testing.B) {
	benchExpensiveRequestScenario(b, "iterative_miss", func(limitCfg ExpensiveRequestLimitConfig) func(*testing.B) {
		FlushCacheForTest()
		ResetHostsAndForwardForTest()
		ResetIPPoolForTest()

		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			b.Fatalf("ListenPacket: %v", err)
		}
		iterServer := &dns.Server{PacketConn: pc, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			m := new(dns.Msg)
			m.SetReply(r)
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP("198.51.100.9"),
			})
			_ = w.WriteMsg(m)
		})}
		go iterServer.ActivateAndServe()
		b.Cleanup(func() { _ = iterServer.Shutdown() })

		setCachedDelegation("iter-bench.test.", "ns1.iter-bench.test.", "127.0.0.1")
		SetIterPort(portOfAddr(pc.LocalAddr().String()))
		b.Cleanup(ResetIterPort)

		s := NewServerWithFullConfig("127.0.0.1:0", 1, WarmupConfig{Enabled: false}, SnapshotConfig{}, nil, nil, "", limitCfg)
		writer := &mockResponseWriterWithCapture{addr: &net.UDPAddr{IP: net.ParseIP("192.0.2.22"), Port: 53532}}

		return func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				qname := fmt.Sprintf("bench-%d.iter-bench.test.", i)
				req := new(dns.Msg)
				req.SetQuestion(qname, dns.TypeA)
				writer.written = nil
				s.ServeDNS(writer, req)
			}
		}
	})
}

func benchExpensiveRequestScenario(b *testing.B, name string, setup func(limitCfg ExpensiveRequestLimitConfig) func(*testing.B)) {
	b.Run(name+"/disabled", func(b *testing.B) {
		bench := setup(ExpensiveRequestLimitConfig{})
		bench(b)
	})
	b.Run(name+"/observe_would_refuse", func(b *testing.B) {
		bench := setup(ExpensiveRequestLimitConfig{
			Mode:               ExpensiveRequestLimitModeEnabled,
			Limit:              1024,
			observeWouldRefuse: true,
		})
		bench(b)
	})
}

func BenchmarkExpensiveRequestProtectionAcquireRelease(b *testing.B) {
	limiter := newExpensiveRequestLimiter(ExpensiveRequestLimitConfig{
		Mode:               ExpensiveRequestLimitModeEnabled,
		Limit:              1024,
		observeWouldRefuse: true,
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := withExpensiveRequestHolder(
			withExpensiveRequestLimiter(context.Background(), limiter),
			newExpensiveRequestHolder(extractClientIP(&net.UDPAddr{IP: net.ParseIP("192.0.2.99"), Port: 5300})),
		)
		if !tryAcquireExpensiveRequest(ctx, expensivePathForward) {
			b.Fatal("unexpected acquire refusal")
		}
		expensiveRequestHolderFromContext(ctx).ReleaseIfHeld(limiter)
	}
}
