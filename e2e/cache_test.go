package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"rec53/monitor"
	"rec53/server"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// TestCacheBehavior tests caching functionality.
func TestCacheBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		// Shutdown will clear the cache
		// We need a way to clear cache before tests
		// For now, we use separate server instances
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	client := &dns.Client{
		Net:     "udp",
		Timeout: 10 * time.Second,
	}

	domain := "cache-test.example.com."

	// First query (cache miss)
	msg1 := new(dns.Msg)
	msg1.SetQuestion(domain, dns.TypeA)
	msg1.RecursionDesired = true

	start1 := time.Now()
	resp1, _, err := client.Exchange(msg1, s.UDPAddr())
	latency1 := time.Since(start1)

	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}

	t.Logf("First query latency: %v", latency1)

	// Second query (should hit cache)
	msg2 := new(dns.Msg)
	msg2.SetQuestion(domain, dns.TypeA)
	msg2.RecursionDesired = true

	start2 := time.Now()
	resp2, _, err := client.Exchange(msg2, s.UDPAddr())
	latency2 := time.Since(start2)

	if err != nil {
		t.Fatalf("Second query failed: %v", err)
	}

	t.Logf("Second query latency: %v", latency2)

	// Compare responses
	if resp1.Rcode != resp2.Rcode {
		t.Errorf("Response codes differ: %d vs %d", resp1.Rcode, resp2.Rcode)
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestCacheConcurrentAccess tests cache under concurrent load.
func TestCacheConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	const numWorkers = 10
	const numQueries = 20

	var wg sync.WaitGroup
	results := make(chan time.Duration, numWorkers*numQueries)

	domain := "concurrent-cache-test.google.com."

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			client := &dns.Client{
				Net:     "udp",
				Timeout: 10 * time.Second,
			}

			for i := 0; i < numQueries; i++ {
				msg := new(dns.Msg)
				msg.SetQuestion(domain, dns.TypeA)

				start := time.Now()
				_, _, err := client.Exchange(msg, s.UDPAddr())
				latency := time.Since(start)

				if err == nil {
					results <- latency
				}
			}
		}()
	}

	wg.Wait()
	close(results)

	// Analyze latencies
	var total time.Duration
	var count int
	var min, max time.Duration = time.Hour, 0

	for lat := range results {
		total += lat
		count++
		if lat < min {
			min = lat
		}
		if lat > max {
			max = lat
		}
	}

	avg := total / time.Duration(count)

	t.Logf("Cache performance: count=%d, min=%v, max=%v, avg=%v", count, min, max, avg)

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestCacheDifferentTypes tests caching of different record types.
func TestCacheDifferentTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name  string
		domain string
		qtype uint16
	}{
		{"A record", "google.com.", dns.TypeA},
		{"AAAA record", "google.com.", dns.TypeAAAA},
		{"MX record", "gmail.com.", dns.TypeMX},
		{"TXT record", "google.com.", dns.TypeTXT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &dns.Client{
				Net:     "udp",
				Timeout: 10 * time.Second,
			}

			// First query
			msg1 := new(dns.Msg)
			msg1.SetQuestion(tt.domain, tt.qtype)

			start1 := time.Now()
			_, _, err := client.Exchange(msg1, s.UDPAddr())
			latency1 := time.Since(start1)

			if err != nil {
				t.Skipf("Query failed: %v", err)
			}

			// Second query (should be cached)
			msg2 := new(dns.Msg)
			msg2.SetQuestion(tt.domain, tt.qtype)

			start2 := time.Now()
			_, _, err = client.Exchange(msg2, s.UDPAddr())
			latency2 := time.Since(start2)

			if err != nil {
				t.Fatalf("Second query failed: %v", err)
			}

			t.Logf("%s: first=%v, second=%v", tt.name, latency1, latency2)
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestCacheHitRate tests cache hit rate under repeated queries.
func TestCacheHitRate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	domains := []string{
		"google.com.",
		"github.com.",
		"cloudflare.com.",
	}

	client := &dns.Client{
		Net:     "udp",
		Timeout: 10 * time.Second,
	}

	// Query each domain twice
	latencies := make(map[string][]time.Duration)

	for _, domain := range domains {
		for i := 0; i < 2; i++ {
			msg := new(dns.Msg)
			msg.SetQuestion(domain, dns.TypeA)

			start := time.Now()
			_, _, err := client.Exchange(msg, s.UDPAddr())
			latency := time.Since(start)

			if err == nil {
				latencies[domain] = append(latencies[domain], latency)
			}
		}
	}

	// Analyze
	for domain, lats := range latencies {
		if len(lats) >= 2 {
			t.Logf("%s: first=%v, second=%v, speedup=%.2fx",
				domain, lats[0], lats[1], float64(lats[0])/float64(lats[1]))
		}
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}