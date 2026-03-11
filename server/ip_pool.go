package server

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

const (
	INIT_IP_LATENCY     = 1000
	MAX_IP_LATENCY      = 10000
	MAX_PREFETCH_CONCUR = 10 // 最大并发 prefetch 数
	PREFETCH_TIMEOUT    = 3  // prefetch 超时秒数
)

type IPQuality struct {
	isInit  atomic.Bool
	latency int32
}

func NewIPQuality() *IPQuality {
	ipq := &IPQuality{
		latency: INIT_IP_LATENCY,
	}
	ipq.isInit.Store(true)
	return ipq
}

func (ipq *IPQuality) Init() {
	ipq.isInit.Store(true)
	atomic.StoreInt32(&ipq.latency, INIT_IP_LATENCY)
}

// IsInit returns whether the IP quality has been initialized
func (ipq *IPQuality) IsInit() bool {
	return ipq.isInit.Load()
}

func (ipq *IPQuality) GetLatency() int32 {
	return atomic.LoadInt32(&ipq.latency)
}

func (ipq *IPQuality) SetLatency(latency int32) {
	atomic.StoreInt32(&ipq.latency, latency)
}

func (ipq *IPQuality) SetLatencyAndState(latency int32) {
	atomic.StoreInt32(&ipq.latency, latency)
	ipq.isInit.Store(false)
}

// IP state constants for IPQualityV2
const (
	IP_STATE_ACTIVE    = 0 // Normal operation
	IP_STATE_DEGRADED  = 1 // Performance degraded (1-3 failures)
	IP_STATE_SUSPECT   = 2 // Suspected bad (4-6 failures)
	IP_STATE_RECOVERED = 3 // Recovering (probe successful)
)

// IPQualityV2 tracks IP quality using sliding window histogram with P50/P95/P99 metrics
// This replaces the simple IPQuality struct for improved fault recovery and confidence-based selection
type IPQualityV2 struct {
	// Sliding window samples (ring buffer)
	samples     [64]int32 // Last 64 RTT samples in milliseconds
	sampleCount uint8     // Number of samples currently in buffer (0-64)
	nextIdx     uint8     // Next write position in ring buffer

	// Statistical metrics
	p50        int32 // Median latency (P50) - used for selection
	p95        int32 // 95th percentile latency - for monitoring
	p99        int32 // 99th percentile latency - for monitoring
	confidence uint8 // Confidence level 0-100% (sampleCount * 10, capped at 100)

	// Failure tracking
	failCount   uint8     // Consecutive failure count
	state       uint8     // Current IP state (ACTIVE, DEGRADED, SUSPECT, RECOVERED)
	lastUpdate  time.Time // Last update timestamp
	lastFailure time.Time // Last failure timestamp

	// Concurrency protection
	mu sync.RWMutex
}

// NewIPQualityV2 creates a new IP quality tracker with initial state
func NewIPQualityV2() *IPQualityV2 {
	return &IPQualityV2{
		sampleCount: 0,
		nextIdx:     0,
		p50:         int32(INIT_IP_LATENCY),
		p95:         int32(INIT_IP_LATENCY),
		p99:         int32(INIT_IP_LATENCY),
		confidence:  0,
		failCount:   0,
		state:       IP_STATE_ACTIVE,
		lastUpdate:  time.Now(),
		lastFailure: time.Time{},
	}
}

// RecordLatency records a successful latency sample and updates percentiles
// Thread-safe via internal RWMutex
func (iq *IPQualityV2) RecordLatency(latency int32) {
	iq.mu.Lock()
	defer iq.mu.Unlock()

	// Add sample to ring buffer
	iq.samples[iq.nextIdx] = latency
	iq.nextIdx = (iq.nextIdx + 1) % 64
	if iq.sampleCount < 64 {
		iq.sampleCount++
	}

	// Update confidence (10 samples = 100%)
	iq.confidence = uint8(int(iq.sampleCount) * 10)
	if iq.confidence > 100 {
		iq.confidence = 100
	}

	// Reset failure counter on success (recovery sign)
	iq.failCount = 0
	iq.state = IP_STATE_ACTIVE

	// Recalculate percentiles
	iq.updatePercentiles()
	iq.lastUpdate = time.Now()
}

// updatePercentiles recalculates P50, P95, P99 from current samples
// Must be called with mutex held
func (iq *IPQualityV2) updatePercentiles() {
	if iq.sampleCount == 0 {
		return
	}

	// Copy samples for sorting (must sort to compute percentiles)
	samples := make([]int32, iq.sampleCount)
	for i := 0; i < int(iq.sampleCount); i++ {
		samples[i] = iq.samples[i]
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	// Calculate P50 (median)
	iq.p50 = samples[iq.sampleCount/2]

	// Calculate P95
	idx95 := int(float64(iq.sampleCount) * 0.95)
	if idx95 >= int(iq.sampleCount) {
		idx95 = int(iq.sampleCount) - 1
	}
	iq.p95 = samples[idx95]

	// Calculate P99
	idx99 := int(float64(iq.sampleCount) * 0.99)
	if idx99 >= int(iq.sampleCount) {
		idx99 = int(iq.sampleCount) - 1
	}
	iq.p99 = samples[idx99]
}

// RecordFailure records a failure and applies exponential backoff strategy
// Thread-safe via internal RWMutex
func (iq *IPQualityV2) RecordFailure() {
	iq.mu.Lock()
	defer iq.mu.Unlock()

	iq.failCount++
	iq.lastFailure = time.Now()

	// Exponential backoff strategy with 3 phases
	switch {
	case iq.failCount <= 3:
		// Phase 1 (1-3 failures): DEGRADED state with 20% latency penalty
		iq.state = IP_STATE_DEGRADED
		iq.p50 = int32(float64(iq.p50) * 1.2)
		if iq.p50 > int32(MAX_IP_LATENCY) {
			iq.p50 = int32(MAX_IP_LATENCY)
		}

	case iq.failCount <= 6:
		// Phase 2 (4-6 failures): SUSPECT state, all metrics set to MAX
		iq.state = IP_STATE_SUSPECT
		iq.p50 = int32(MAX_IP_LATENCY)
		iq.p95 = int32(MAX_IP_LATENCY)
		iq.p99 = int32(MAX_IP_LATENCY)

	default:
		// Phase 3 (7+ failures): Remains SUSPECT, will be periodically probed
		iq.state = IP_STATE_SUSPECT
		// Keep marked for periodic probe recovery in background task
	}
}

// ResetForProbe resets the IP state after a successful probe attempt
// Used by periodic probe loop to mark recovery
func (iq *IPQualityV2) ResetForProbe() {
	iq.mu.Lock()
	defer iq.mu.Unlock()

	iq.failCount = 0
	iq.state = IP_STATE_RECOVERED
	iq.lastUpdate = time.Now()
}

// ShouldProbe returns whether this IP is a candidate for periodic probing
// Returns true if IP is in SUSPECT state and has not been recently probed
func (iq *IPQualityV2) ShouldProbe() bool {
	iq.mu.RLock()
	defer iq.mu.RUnlock()

	// Only probe SUSPECT IPs
	if iq.state != IP_STATE_SUSPECT {
		return false
	}

	// Avoid probing too frequently - wait at least 5 seconds between probes
	if !iq.lastFailure.IsZero() && time.Since(iq.lastFailure) < 5*time.Second {
		return false
	}

	return true
}

// GetP50Latency returns the current P50 latency in a thread-safe manner
func (iq *IPQualityV2) GetP50Latency() int32 {
	iq.mu.RLock()
	defer iq.mu.RUnlock()
	return iq.p50
}

// GetP95Latency returns the current P95 latency in a thread-safe manner
func (iq *IPQualityV2) GetP95Latency() int32 {
	iq.mu.RLock()
	defer iq.mu.RUnlock()
	return iq.p95
}

// GetP99Latency returns the current P99 latency in a thread-safe manner
func (iq *IPQualityV2) GetP99Latency() int32 {
	iq.mu.RLock()
	defer iq.mu.RUnlock()
	return iq.p99
}

// GetState returns the current IP state in a thread-safe manner
func (iq *IPQualityV2) GetState() uint8 {
	iq.mu.RLock()
	defer iq.mu.RUnlock()
	return iq.state
}

// GetConfidence returns the current confidence level (0-100) in a thread-safe manner
func (iq *IPQualityV2) GetConfidence() uint8 {
	iq.mu.RLock()
	defer iq.mu.RUnlock()
	return iq.confidence
}

// GetScore returns a composite quality score for this IP
// Lower score is better (like latency)
// Formula: p50 × confidenceMult × stateWeight
// Thread-safe via internal RWMutex
func (iq *IPQualityV2) GetScore() float64 {
	iq.mu.RLock()
	defer iq.mu.RUnlock()

	// Base score: P50 latency
	score := float64(iq.p50)

	// Confidence multiplier: penalize low-confidence IPs to encourage sampling
	// confidence=0 → mult=2.0 (2x penalty, eager to try)
	// confidence=100 → mult=1.0 (no penalty, fully trusted)
	confidenceMult := 1.0 + float64(100-iq.confidence)*0.01
	score *= confidenceMult

	// State weight: apply penalty based on health
	stateWeights := []float64{
		1.0,   // ACTIVE: trusted IP, no penalty
		1.5,   // DEGRADED: underperforming, 50% penalty
		100.0, // SUSPECT: avoid at all costs (basically infinite)
		1.1,   // RECOVERED: just recovering, 10% penalty
	}

	// Clamp state index to valid range
	stateIdx := iq.state
	if stateIdx >= uint8(len(stateWeights)) {
		stateIdx = IP_STATE_ACTIVE
	}
	score *= stateWeights[stateIdx]

	return score
}

type IPPool struct {
	pool      map[string]*IPQuality
	poolV2    map[string]*IPQualityV2 // V2 pool with sliding window histogram
	l         sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	sem       chan struct{} // semaphore for concurrency limit
	dnsClient *dns.Client
}

var globalIPPool = NewIPPool()

func NewIPPool() *IPPool {
	ctx, cancel := context.WithCancel(context.Background())
	ipp := &IPPool{
		pool:   make(map[string]*IPQuality),
		poolV2: make(map[string]*IPQualityV2),
		l:      sync.RWMutex{},
		ctx:    ctx,
		cancel: cancel,
		sem:    make(chan struct{}, MAX_PREFETCH_CONCUR),
		dnsClient: &dns.Client{
			Net:     "udp",
			Timeout: PREFETCH_TIMEOUT * time.Second,
		},
	}
	return ipp
}

// Shutdown gracefully stops all prefetch goroutines
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
// for periodic SUSPECT IP recovery attempts
func (ipp *IPPool) StartProbeLoop() {
	ipp.wg.Add(1)
	go ipp.periodicProbeLoop()
	monitor.Rec53Log.Debugf("IP pool probe loop started")
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

		// Probe with 3-second timeout
		client := &dns.Client{
			Timeout: 3 * time.Second,
			Net:     "udp",
		}

		_, _, err := client.Exchange(req, ip+":53")

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

func (ipp *IPPool) isTheIPInit(ip string) bool {
	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		ipq = &IPQuality{}
		ipq.Init()
		ipp.SetIPQuality(ip, ipq)
	}
	return ipq.IsInit()
}

func (ipp *IPPool) GetIPQuality(ip string) *IPQuality {
	ipp.l.RLock()
	defer ipp.l.RUnlock()
	if ipq, ok := ipp.pool[ip]; ok {
		return ipq
	}
	return nil
}

func (ipp *IPPool) SetIPQuality(ip string, ipq *IPQuality) {
	ipp.l.Lock()
	defer ipp.l.Unlock()
	ipp.pool[ip] = ipq
}

func (ipp *IPPool) updateIPQuality(ip string, latency int32) {
	ipq := ipp.GetIPQuality(ip)
	if ipq == nil {
		ipq = &IPQuality{}
		ipq.Init()
		ipp.SetIPQuality(ip, ipq)
	}
	ipq.SetLatencyAndState(latency)
}

func (ipp *IPPool) UpIPsQuality(ips []string) {
	for _, ip := range ips {
		ipq := ipp.GetIPQuality(ip)
		if ipq == nil {
			ipq = &IPQuality{}
			ipq.Init()
			ipp.SetIPQuality(ip, ipq)
		}
		if !ipq.IsInit() {
			continue
		}
		currentLatency := ipq.GetLatency()
		nextLatency := int32(float64(currentLatency) * 0.9)
		ipq.SetLatency(nextLatency)
	}
}

func (ipp *IPPool) getBestIPs(ips []string) (string, string) {
	var (
		bestIP      string = ""
		bestLatency int32  = MAX_IP_LATENCY
		secondIP    string = ""
		secondDelay int32  = MAX_IP_LATENCY
	)

	for _, ip := range ips {
		ipq := ipp.GetIPQuality(ip)
		if ipq == nil {
			ipq = &IPQuality{}
			ipq.Init()
			ipp.SetIPQuality(ip, ipq)
		}
		currentLatency := ipq.GetLatency()
		monitor.Rec53Log.Debug(ip, ",", ipq.GetLatency())

		// Update best and second best IPs
		if currentLatency < bestLatency {
			// Current best becomes second best
			secondIP = bestIP
			secondDelay = bestLatency
			// New best
			bestIP = ip
			bestLatency = currentLatency
		} else if currentLatency < secondDelay && ip != bestIP {
			// Update second best if better than current second
			secondIP = ip
			secondDelay = currentLatency
		}
	}
	return bestIP, secondIP
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

func (ipp *IPPool) GetPrefetchIPs(bestIP string) []string {
	var prefetchIPs []string

	ipp.l.RLock()
	defer ipp.l.RUnlock()

	bestIPQuality, ok := ipp.pool[bestIP]
	if !ok {
		return prefetchIPs
	}
	theBestLatency := bestIPQuality.GetLatency()

	for ip, ipq := range ipp.pool {
		latency := ipq.GetLatency()
		if latency < theBestLatency && int32(float32(latency)/0.9) > theBestLatency && ip != bestIP {
			prefetchIPs = append(prefetchIPs, ip)
		}
	}
	return prefetchIPs
}

// PrefetchIPs prefetches IP quality for given IPs with concurrency control
func (ipp *IPPool) PrefetchIPs(ips []string) {
	for _, ip := range ips {
		select {
		case ipp.sem <- struct{}{}:
			ipp.wg.Add(1)
			go func(targetIP string) {
				defer ipp.wg.Done()
				defer func() { <-ipp.sem }()

				ipp.prefetchIPQuality(targetIP)
			}(ip)
		default:
			// Skip if semaphore is full (too many concurrent prefetches)
			monitor.Rec53Log.Debugf("skip prefetch for %s: too many concurrent prefetches", ip)
		}
	}
}

// prefetchIPQuality performs the actual IP quality check
func (ipp *IPPool) prefetchIPQuality(ip string) {
	select {
	case <-ipp.ctx.Done():
		return
	default:
	}

	_, rtt, err := ipp.dnsClient.Exchange(&dns.Msg{}, ip+":"+getIterPort())
	if err != nil {
		monitor.Rec53Log.Errorf("prefetch ip %s error: %s", ip, err.Error())
		return
	}

	ipq := NewIPQuality()
	ipq.SetLatencyAndState(int32(rtt / time.Millisecond))
	ipp.SetIPQuality(ip, ipq)
	monitor.Rec53Metric.IPQualityGaugeSet(ip, float64(rtt/time.Millisecond))
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
