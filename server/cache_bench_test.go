package server

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	monitor.Rec53Log = zap.NewNop().Sugar()
	monitor.InitMetricForTest()
}

// BenchmarkCacheKey measures the cost of building a cache key string.
func BenchmarkCacheKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getCacheKey("example.com.", dns.TypeA)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 10000 {
			b.Errorf("regression: %.2f ns/op > 10000 ns threshold", avgNs)
		}
	}
}

// BenchmarkCacheGetHit measures getCacheCopyByType on a cache hit (includes msg.Copy).
func BenchmarkCacheGetHit(b *testing.B) {
	FlushCacheForTest()
	msg := &dns.Msg{}
	msg.SetQuestion("example.com.", dns.TypeA)
	rr, _ := dns.NewRR("example.com. 60 IN A 1.2.3.4")
	msg.Answer = append(msg.Answer, rr)
	setCacheCopyByType("example.com.", dns.TypeA, msg, 60)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = getCacheCopyByType("example.com.", dns.TypeA)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 10000 {
			b.Errorf("regression: %.2f ns/op > 10000 ns threshold", avgNs)
		}
	}
}

// BenchmarkCacheGetMiss measures getCacheCopyByType when the key does not exist.
func BenchmarkCacheGetMiss(b *testing.B) {
	FlushCacheForTest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = getCacheCopyByType("nonexistent.example.com.", dns.TypeA)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 10000 {
			b.Errorf("regression: %.2f ns/op > 10000 ns threshold", avgNs)
		}
	}
}

// BenchmarkCacheSet measures setCacheCopyByType (includes msg.Copy and lock).
// Uses a unique key per iteration to avoid go-cache overwrite short-circuiting.
func BenchmarkCacheSet(b *testing.B) {
	msg := &dns.Msg{}
	msg.SetQuestion("example.com.", dns.TypeA)
	rr, _ := dns.NewRR("example.com. 60 IN A 1.2.3.4")
	msg.Answer = append(msg.Answer, rr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench%d.example.com.", i)
		setCacheCopyByType(key, dns.TypeA, msg, 60)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 30000 {
			b.Errorf("regression: %.2f ns/op > 30000 ns threshold", avgNs)
		}
	}
}

// BenchmarkCacheConcurrent exercises concurrent mixed reads and writes
// to verify RWMutex correctness. Run with -race.
func BenchmarkCacheConcurrent(b *testing.B) {
	FlushCacheForTest()
	msg := &dns.Msg{}
	msg.SetQuestion("concurrent.example.com.", dns.TypeA)
	rr, _ := dns.NewRR("concurrent.example.com. 60 IN A 1.2.3.4")
	msg.Answer = append(msg.Answer, rr)
	setCacheCopyByType("concurrent.example.com.", dns.TypeA, msg, 60)

	var counter atomic.Int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n := counter.Add(1)
			if n%3 == 0 {
				// write
				setCacheCopyByType("concurrent.example.com.", dns.TypeA, msg, 60)
			} else {
				// read
				_, _ = getCacheCopyByType("concurrent.example.com.", dns.TypeA)
			}
		}
	})
}

// BenchmarkCacheKeyWithContext is a convenience benchmark run via Change() to
// verify the key allocation path end-to-end at the state-machine level.
// It uses a pre-warmed cache to avoid network I/O.
func BenchmarkCacheKeyWithContext(b *testing.B) {
	FlushCacheForTest()
	req := &dns.Msg{}
	req.SetQuestion("keyctx.example.com.", dns.TypeA)
	resp := &dns.Msg{}
	resp.SetReply(req)
	rr, _ := dns.NewRR("keyctx.example.com. 60 IN A 5.6.7.8")
	resp.Answer = append(resp.Answer, rr)
	setCacheCopyByType("keyctx.example.com.", dns.TypeA, resp, 60)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req2 := &dns.Msg{}
		req2.SetQuestion("keyctx.example.com.", dns.TypeA)
		resp2 := &dns.Msg{}
		_, _ = Change(newStateInitState(req2, resp2, ctx))
	}
}
