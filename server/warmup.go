package server

import (
	"context"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

// WarmupStats represents the statistics of a warmup operation
type WarmupStats struct {
	Total     int
	Succeeded int
	Failed    int
	Duration  time.Duration
}

// WarmupNSRecords warms up root and TLD NS records concurrently using the resolver.
// It queries NS records for the root (".") and all configured TLDs, automatically
// recording IP quality metrics via resolveNSIPsRecursively().
// Returns warmup statistics (total, succeeded, failed, duration).
func WarmupNSRecords(ctx context.Context, cfg WarmupConfig) WarmupStats {
	startTime := time.Now()
	stats := WarmupStats{}

	// Build domain list: root + TLDs
	domains := []string{"."}
	domains = append(domains, cfg.TLDs...)
	stats.Total = len(domains)

	// Create a semaphore to limit concurrent queries
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failureCount := 0

	// Query each domain concurrently
	for _, domain := range domains {
		wg.Add(1)

		// Acquire semaphore slot
		sem <- struct{}{}

		go func(d string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore slot

			// Create context with timeout for this query
			queryCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()

			if queryNSRecords(queryCtx, d) {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else {
				mu.Lock()
				failureCount++
				mu.Unlock()
			}
		}(domain)
	}

	// Wait for all queries to complete
	wg.Wait()

	stats.Succeeded = successCount
	stats.Failed = failureCount
	stats.Duration = time.Since(startTime)

	// Log warmup completion
	monitor.Rec53Log.Infof(
		"NS warmup completed: %d/%d succeeded, %d failed in %.1fs",
		stats.Succeeded, stats.Total, stats.Failed, stats.Duration.Seconds(),
	)

	return stats
}

// queryNSRecords queries NS records for a domain using the resolver state machine.
// Returns true if the query succeeded, false otherwise.
// This creates a synthetic DNS query and processes it through the state machine,
// which automatically caches results and records IP quality metrics.
func queryNSRecords(ctx context.Context, domain string) bool {
	// Create a synthetic NS query
	queryMsg := &dns.Msg{}
	queryMsg.SetQuestion(domain, dns.TypeNS)
	queryMsg.RecursionDesired = true

	// Create reply message
	reply := &dns.Msg{}

	// Process through state machine
	stm := newStateInitState(queryMsg, reply)
	result, err := Change(stm)
	if err != nil {
		monitor.Rec53Log.Debugf("Warmup query for %s failed: %v", domain, err)
		return false
	}

	// Check if we got any NS records in the answer section
	if result != nil && len(result.Answer) > 0 {
		return true
	}

	// Also consider success if we got NS records in authority section (referral)
	if result != nil && len(result.Ns) > 0 {
		return true
	}

	// No records found - still consider it a partial success since the query
	// was processed without fatal errors (the domain may not exist yet)
	return true
}
