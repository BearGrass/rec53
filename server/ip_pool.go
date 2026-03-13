package server

import (
	"context"
	"sort"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

// IPPool manages IP quality tracking and selection for upstream DNS servers.
// It maintains a pool of IPQualityV2 entries keyed by IP address and provides
// best-IP selection, periodic health probing, and graceful shutdown.
type IPPool struct {
	poolV2        map[string]*IPQualityV2 // V2 pool with sliding window histogram
	l             sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	probeLoopOnce sync.Once // ensures StartProbeLoop starts exactly one goroutine
}

var globalIPPool = NewIPPool()

func NewIPPool() *IPPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &IPPool{
		poolV2: make(map[string]*IPQualityV2),
		l:      sync.RWMutex{},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Shutdown gracefully stops all background goroutines
func (ipp *IPPool) Shutdown(ctx context.Context) error {
	ipp.cancel()
	done := make(chan struct{})
	go func() {
		ipp.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StartProbeLoop initializes and launches the background probe goroutine
// for periodic SUSPECT IP recovery attempts.
// Safe to call multiple times — only one goroutine is ever started (sync.Once).
func (ipp *IPPool) StartProbeLoop() {
	ipp.probeLoopOnce.Do(func() {
		ipp.wg.Add(1)
		go ipp.periodicProbeLoop()
		monitor.Rec53Log.Debugf("IP pool probe loop started")
	})
}

// periodicProbeLoop runs periodically every 30 seconds to probe SUSPECT IPs
func (ipp *IPPool) periodicProbeLoop() {
	defer ipp.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	monitor.Rec53Log.Debugf("probe loop: entering periodic check loop")

	for {
		select {
		case <-ipp.ctx.Done():
			monitor.Rec53Log.Debugf("probe loop: context cancelled, exiting")
			return
		case <-ticker.C:
			ipp.probeAllSuspiciousIPs()
		}
	}
}

// probeAllSuspiciousIPs queries all SUSPECT IPs to detect recovery
func (ipp *IPPool) probeAllSuspiciousIPs() {
	// Find SUSPECT IP candidates (under read lock)
	ipp.l.RLock()
	candidates := make([]string, 0)
	for ip, iqv2 := range ipp.poolV2 {
		if iqv2.ShouldProbe() {
			candidates = append(candidates, ip)
		}
	}
	ipp.l.RUnlock()

	if len(candidates) == 0 {
		return
	}

	monitor.Rec53Log.Debugf("probe loop: probing %d SUSPECT IPs", len(candidates))

	// Probe each candidate (no lock during probing to avoid blocking queries)
	for _, ip := range candidates {
		// Create a simple DNS query for root zone
		req := new(dns.Msg)
		req.SetQuestion(".", dns.TypeA)

		// Probe with 3-second timeout, but respect pool context cancellation
		client := &dns.Client{
			Timeout: 3 * time.Second,
			Net:     "udp",
		}
		probeCtx, probeCancel := context.WithTimeout(ipp.ctx, 3*time.Second)
		_, _, err := client.ExchangeContext(probeCtx, req, ip+":53")
		probeCancel()

		// Check IP quality tracker
		iqv2 := ipp.GetIPQualityV2(ip)
		if iqv2 == nil {
			continue
		}

		if err == nil {
			// Probe succeeded - mark IP as recovered
			iqv2.ResetForProbe()
			monitor.Rec53Log.Debugf("probe loop: IP %s recovered from SUSPECT state", ip)
		} else {
			// Probe failed - IP stays in SUSPECT state, retry in 30s
			monitor.Rec53Log.Debugf("probe loop: IP %s probe failed (will retry in 30s): %v", ip, err)
		}
	}
}

// GetBestIPsV2 selects the best and second-best IPs using V2 scoring algorithm
// Lower composite score is better (p50 * confidenceMult * stateWeight)
// Creates or retrieves IPQualityV2 for each IP
// Thread-safe via internal RWMutex protection
func (ipp *IPPool) GetBestIPsV2(ips []string) (string, string) {
	type scoreEntry struct {
		ip    string
		score float64
	}

	scores := make([]scoreEntry, 0, len(ips))

	ipp.l.RLock()
	for _, ip := range ips {
		// Get or create IPQualityV2 for this IP
		iqv2, ok := ipp.poolV2[ip]
		if !ok {
			// Release read lock before potentially creating new entries
			ipp.l.RUnlock()
			ipp.l.Lock()
			// Check again in case another goroutine created it
			if iqv2, ok := ipp.poolV2[ip]; ok {
				ipp.l.Unlock()
				ipp.l.RLock()
				scores = append(scores, scoreEntry{
					ip:    ip,
					score: iqv2.GetScore(),
				})
			} else {
				// Create new IPQualityV2
				iqv2 = NewIPQualityV2()
				ipp.poolV2[ip] = iqv2
				ipp.l.Unlock()
				ipp.l.RLock()
				scores = append(scores, scoreEntry{
					ip:    ip,
					score: iqv2.GetScore(),
				})
			}
			continue
		}
		scores = append(scores, scoreEntry{
			ip:    ip,
			score: iqv2.GetScore(),
		})
	}
	ipp.l.RUnlock()

	// Sort by score (lower is better)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	bestIP := ""
	secondIP := ""
	if len(scores) > 0 {
		bestIP = scores[0].ip
	}
	if len(scores) > 1 {
		secondIP = scores[1].ip
	}

	return bestIP, secondIP
}

// GetIPQualityV2 retrieves an IPQualityV2 object for a given IP
func (ipp *IPPool) GetIPQualityV2(ip string) *IPQualityV2 {
	ipp.l.RLock()
	defer ipp.l.RUnlock()
	return ipp.poolV2[ip]
}

// SetIPQualityV2 stores an IPQualityV2 object for a given IP
func (ipp *IPPool) SetIPQualityV2(ip string, iqv2 *IPQualityV2) {
	ipp.l.Lock()
	defer ipp.l.Unlock()
	ipp.poolV2[ip] = iqv2
}

// ResetIPPoolForTest replaces the global IP pool with a fresh instance.
// Exported for use by E2E tests to ensure a clean state.
func ResetIPPoolForTest() {
	// Shutdown existing pool's goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	globalIPPool.Shutdown(ctx)

	globalIPPool = NewIPPool()
}
