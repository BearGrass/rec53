package server

import (
	"context"
	"testing"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	monitor.Rec53Log = zap.NewNop().Sugar()
	monitor.InitMetricForTest()
}

// BenchmarkStateMachineCacheHit measures the full Change() call on the
// cache-hit path: STATE_INIT → CACHE_LOOKUP (hit) → RETURN_RESP.
// No network I/O is involved.
func BenchmarkStateMachineCacheHit(b *testing.B) {
	b.ReportAllocs()
	FlushCacheForTest()

	// Pre-warm cache with a valid A response.
	cached := &dns.Msg{}
	cached.SetQuestion("bench.example.com.", dns.TypeA)
	cached.Response = true
	rr, _ := dns.NewRR("bench.example.com. 60 IN A 1.2.3.4")
	cached.Answer = append(cached.Answer, rr)
	setCacheCopyByType("bench.example.com.", dns.TypeA, cached, 60)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &dns.Msg{}
		req.SetQuestion("bench.example.com.", dns.TypeA)
		resp := &dns.Msg{}
		_, _ = Change(newStateInitState(req, resp, ctx))
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 50000 {
			b.Errorf("regression: %.2f ns/op > 50000 ns threshold", avgNs)
		}
	}
}

// BenchmarkStateInitHandle measures the stateInitState.handle FORMERR fast path
// (request with no question sections).
func BenchmarkStateInitHandle(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()

	// Request with zero questions triggers FORMERR immediately.
	req := &dns.Msg{}
	resp := &dns.Msg{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := newStateInitState(req, resp, ctx)
		_, _ = s.handle(req, resp)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 5000 {
			b.Errorf("regression: %.2f ns/op > 5000 ns threshold", avgNs)
		}
	}
}

// BenchmarkCacheLookupHit measures cacheLookupState.handle on a cache hit.
func BenchmarkCacheLookupHit(b *testing.B) {
	b.ReportAllocs()
	FlushCacheForTest()

	req := &dns.Msg{}
	req.SetQuestion("lookup.example.com.", dns.TypeA)
	cached := &dns.Msg{}
	cached.SetReply(req)
	rr, _ := dns.NewRR("lookup.example.com. 60 IN A 2.3.4.5")
	cached.Answer = append(cached.Answer, rr)
	setCacheCopyByType("lookup.example.com.", dns.TypeA, cached, 60)

	ctx := context.Background()
	resp := &dns.Msg{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := newCacheLookupState(req, resp, ctx)
		_, _ = s.handle(req, resp)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 10000 {
			b.Errorf("regression: %.2f ns/op > 10000 ns threshold", avgNs)
		}
	}
}

// BenchmarkCacheLookupMiss measures cacheLookupState.handle on a cache miss.
func BenchmarkCacheLookupMiss(b *testing.B) {
	b.ReportAllocs()
	FlushCacheForTest()

	req := &dns.Msg{}
	req.SetQuestion("miss.example.com.", dns.TypeA)
	ctx := context.Background()
	resp := &dns.Msg{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := newCacheLookupState(req, resp, ctx)
		_, _ = s.handle(req, resp)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 5000 {
			b.Errorf("regression: %.2f ns/op > 5000 ns threshold", avgNs)
		}
	}
}
