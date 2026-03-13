package e2e

// BenchmarkFirstPacket measures real-world first-packet DNS resolution latency
// for popular Chinese domains under three scenarios:
//
//  1. NoWarmup  — cold start: IP pool is empty, no prior latency data for any NS.
//  2. WithWarmup — IP pool is pre-seeded with .com TLD nameserver latencies via the
//     default warmup process; only the domain-level cache is flushed so that each
//     query still requires a full iterative resolution but benefits from informed NS
//     selection.
//  3. CacheHit  — the domain was already resolved on a prior query; the result is
//     served entirely from the in-memory cache (baseline comparison).
//
// # Default domain list
//
// The built-in domains are a representative set of popular Chinese sites:
//
//	www.qq.com, www.baidu.com, www.taobao.com
//
// # Custom domain list
//
// Set the REC53_BENCH_DOMAINS environment variable to override the built-in list
// and measure latency for domains that matter in your own environment:
//
//	REC53_BENCH_DOMAINS="www.example.com,api.myservice.net,cdn.corp.io" \
//	    go test -v -run='^$' -bench='BenchmarkFirstPacket' \
//	    -benchtime=5x -timeout=300s ./e2e/...
//
// Each value must be a valid hostname; the trailing dot is added automatically.
// Separate multiple domains with commas (no spaces around the comma).
//
// # Recommended invocation
//
//	go test -v -run='^$' -bench='BenchmarkFirstPacket' \
//	    -benchtime=5x -timeout=300s ./e2e/...

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"rec53/server"

	"github.com/miekg/dns"
)

// defaultFirstPacketDomains is the built-in domain list used when
// REC53_BENCH_DOMAINS is not set.
var defaultFirstPacketDomains = []string{
	"www.qq.com",
	"www.baidu.com",
	"www.taobao.com",
}

// firstPacketDomains returns the domain list for the benchmark.
// If the environment variable REC53_BENCH_DOMAINS is set, its comma-separated
// values override the built-in defaults. This lets users test with domains
// that are representative of their own infrastructure without modifying code.
//
// Example:
//
//	export REC53_BENCH_DOMAINS="www.example.com,api.internal.net"
func firstPacketDomains() []string {
	raw := os.Getenv("REC53_BENCH_DOMAINS")
	if raw == "" {
		return defaultFirstPacketDomains
	}
	var out []string
	for _, d := range strings.Split(raw, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return defaultFirstPacketDomains
	}
	return out
}

// newFirstPacketClient returns a dns.Client configured for these benchmarks.
func newFirstPacketClient() *dns.Client {
	return &dns.Client{
		Net:     "udp",
		Timeout: 10 * time.Second,
		UDPSize: 4096,
	}
}

// fpQueryOnce sends a single A query to addr and returns the RTT.
// It calls b.Fatalf immediately on network error or non-NOERROR rcode.
func fpQueryOnce(b *testing.B, client *dns.Client, addr, domain string) time.Duration {
	b.Helper()
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	msg.RecursionDesired = true
	msg.SetEdns0(4096, false)

	resp, rtt, err := client.Exchange(msg, addr)
	if err != nil {
		b.Fatalf("query %s: %v", domain, err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		b.Fatalf("query %s: rcode %s", domain, dns.RcodeToString[resp.Rcode])
	}
	return rtt
}

// BenchmarkFirstPacketNoWarmup measures iterative resolution latency from a
// completely cold state. Both the domain cache and the IP pool are reset before
// each b.N iteration, so every measurement is a genuine first-packet lookup
// with no prior NS latency knowledge.
//
// This represents the worst-case first-packet scenario.
func BenchmarkFirstPacketNoWarmup(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping network benchmark in short mode")
	}

	noWarmup := server.DefaultWarmupConfig
	noWarmup.Enabled = false

	client := newFirstPacketClient()

	for _, domain := range firstPacketDomains() {
		domain := domain
		b.Run(domain, func(b *testing.B) {
			var totalRTT time.Duration

			for i := 0; i < b.N; i++ {
				// Full cold-start: flush domain cache and IP pool each iteration.
				server.FlushCacheForTest()
				server.ResetIPPoolForTest()

				s := server.NewServerWithConfig("127.0.0.1:0", noWarmup)
				s.Run()
				addr := s.UDPAddr()

				b.ResetTimer()
				rtt := fpQueryOnce(b, client, addr, domain)
				b.StopTimer()

				totalRTT += rtt

				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.Shutdown(shutdownCtx) //nolint:errcheck
				cancel()
			}

			b.ReportMetric(msPerQuery(totalRTT, b.N), "ms/query")
			b.ReportMetric(0, "ns/op") // suppress meaningless default
		})
	}
}

// BenchmarkFirstPacketWithWarmup measures iterative resolution latency after
// the default warmup has completed. The IP pool contains real RTT measurements
// for .com TLD nameservers so NS selection is informed, but the domain cache is
// flushed before each query so a full iterative lookup is still performed.
//
// This represents the typical steady-state first-packet scenario in production:
// the server has been running long enough for warmup to finish but the queried
// domain has not been seen before.
func BenchmarkFirstPacketWithWarmup(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping network benchmark in short mode")
	}

	client := newFirstPacketClient()

	for _, domain := range firstPacketDomains() {
		domain := domain
		b.Run(domain, func(b *testing.B) {
			var totalRTT time.Duration

			for i := 0; i < b.N; i++ {
				// Reset IP pool so warmup effect is measured fresh each iteration.
				server.ResetIPPoolForTest()

				warmupCfg := server.DefaultWarmupConfig // Enabled=true
				s := server.NewServerWithConfig("127.0.0.1:0", warmupCfg)
				s.Run()

				// Warmup runs as a background goroutine bounded by Duration (5 s).
				// Wait Duration + 2 s to ensure it has finished before measuring.
				time.Sleep(warmupCfg.Duration + 2*time.Second)

				// Flush only domain cache; retain IP pool data built by warmup.
				server.FlushCacheForTest()

				addr := s.UDPAddr()

				b.ResetTimer()
				rtt := fpQueryOnce(b, client, addr, domain)
				b.StopTimer()

				totalRTT += rtt

				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.Shutdown(shutdownCtx) //nolint:errcheck
				cancel()
			}

			b.ReportMetric(msPerQuery(totalRTT, b.N), "ms/query")
			b.ReportMetric(0, "ns/op")
		})
	}
}

// BenchmarkFirstPacketCacheHit measures resolution latency when the response is
// already present in the in-memory cache. A single priming query is issued
// before the timed loop so every b.N iteration hits the cache.
//
// Use this as a baseline to quantify how much slower first-packet resolution
// is compared to cached resolution.
func BenchmarkFirstPacketCacheHit(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping network benchmark in short mode")
	}

	noWarmup := server.DefaultWarmupConfig
	noWarmup.Enabled = false

	client := newFirstPacketClient()

	for _, domain := range firstPacketDomains() {
		domain := domain
		b.Run(domain, func(b *testing.B) {
			server.FlushCacheForTest()
			server.ResetIPPoolForTest()

			s := server.NewServerWithConfig("127.0.0.1:0", noWarmup)
			s.Run()
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				s.Shutdown(shutdownCtx) //nolint:errcheck
			}()

			addr := s.UDPAddr()

			// Prime the cache with one real iterative lookup (not timed).
			fpQueryOnce(b, client, addr, domain)

			var totalRTT time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rtt := fpQueryOnce(b, client, addr, domain)
				totalRTT += rtt
			}
			b.StopTimer()

			b.ReportMetric(msPerQuery(totalRTT, b.N), "ms/query")
			b.ReportMetric(0, "ns/op")
		})
	}
}

// BenchmarkFirstPacketComparison runs all three scenarios sequentially for
// each domain and emits a human-readable comparison table via b.Log.
// Intended for quick one-shot reporting on a specific machine.
//
//	go test -v -run='^$' -bench=BenchmarkFirstPacketComparison \
//	    -benchtime=1x -timeout=120s ./e2e/...
func BenchmarkFirstPacketComparison(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping network benchmark in short mode")
	}

	noWarmup := server.DefaultWarmupConfig
	noWarmup.Enabled = false

	client := newFirstPacketClient()

	domains := firstPacketDomains()
	results := make([]string, 0, len(domains)*3+1)
	results = append(results, fmt.Sprintf("\n%-30s  %-20s  %-20s  %-20s",
		"domain", "cold (no warmup)", "first-pkt (warmup)", "cache hit"))

	for _, domain := range domains {
		// Scenario 1: cold start
		server.FlushCacheForTest()
		server.ResetIPPoolForTest()
		s1 := server.NewServerWithConfig("127.0.0.1:0", noWarmup)
		s1.Run()
		cold := fpQueryOnce(b, client, s1.UDPAddr(), domain)
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s1.Shutdown(ctx) //nolint:errcheck
		}()

		// Scenario 2: after warmup
		server.ResetIPPoolForTest()
		warmupCfg := server.DefaultWarmupConfig
		s2 := server.NewServerWithConfig("127.0.0.1:0", warmupCfg)
		s2.Run()
		time.Sleep(warmupCfg.Duration + 2*time.Second)
		server.FlushCacheForTest()
		withWarmup := fpQueryOnce(b, client, s2.UDPAddr(), domain)
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s2.Shutdown(ctx) //nolint:errcheck
		}()

		// Scenario 3: cache hit (reuse s2 state — IP pool still warm, prime cache)
		server.FlushCacheForTest()
		server.ResetIPPoolForTest()
		s3 := server.NewServerWithConfig("127.0.0.1:0", noWarmup)
		s3.Run()
		fpQueryOnce(b, client, s3.UDPAddr(), domain) // prime
		cacheHit := fpQueryOnce(b, client, s3.UDPAddr(), domain)
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s3.Shutdown(ctx) //nolint:errcheck
		}()

		results = append(results, fmt.Sprintf("%-30s  %-20s  %-20s  %-20s",
			domain,
			cold.Round(time.Millisecond).String(),
			withWarmup.Round(time.Millisecond).String(),
			cacheHit.Round(time.Microsecond).String(),
		))
	}

	b.Log(strings.Join(results, "\n"))
}

// msPerQuery converts a total duration and iteration count to ms/query float64.
func msPerQuery(total time.Duration, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(total.Microseconds()) / float64(n) / 1000.0
}
