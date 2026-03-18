package server

import (
	"slices"
	"sync"
	"time"
)

// IP state constants for IPQualityV2
const (
	IP_STATE_ACTIVE    = 0 // Normal operation
	IP_STATE_DEGRADED  = 1 // Performance degraded (1-3 failures)
	IP_STATE_SUSPECT   = 2 // Suspected bad (4-6 failures)
	IP_STATE_RECOVERED = 3 // Recovering (probe successful)

	INIT_IP_LATENCY = 1000  // Initial assumed latency (ms) for new IPs
	MAX_IP_LATENCY  = 10000 // Maximum latency threshold (ms); IPs exceeding this are avoided
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
	lastSeen    time.Time // Last time this IP was referenced by a query path

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
		lastSeen:    time.Now(),
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
	iq.lastSeen = iq.lastUpdate
}

// updatePercentiles recalculates P50, P95, P99 from current samples
// Must be called with mutex held
func (iq *IPQualityV2) updatePercentiles() {
	if iq.sampleCount == 0 {
		return
	}

	// Copy samples for sorting (must sort to compute percentiles)
	var buf [64]int32
	sorted := buf[:iq.sampleCount]
	for i := 0; i < int(iq.sampleCount); i++ {
		sorted[i] = iq.samples[i]
	}
	slices.Sort(sorted)

	// Calculate P50 (median)
	iq.p50 = sorted[iq.sampleCount/2]

	// Calculate P95
	idx95 := int(float64(iq.sampleCount) * 0.95)
	if idx95 >= int(iq.sampleCount) {
		idx95 = int(iq.sampleCount) - 1
	}
	iq.p95 = sorted[idx95]

	// Calculate P99
	idx99 := int(float64(iq.sampleCount) * 0.99)
	if idx99 >= int(iq.sampleCount) {
		idx99 = int(iq.sampleCount) - 1
	}
	iq.p99 = sorted[idx99]
}

// RecordFailure records a failure and applies exponential backoff strategy
// Thread-safe via internal RWMutex
func (iq *IPQualityV2) RecordFailure() {
	iq.mu.Lock()
	defer iq.mu.Unlock()

	iq.failCount++
	iq.lastFailure = time.Now()
	iq.lastSeen = iq.lastFailure

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

// GetLastSeen returns the last time this IP was referenced by a query path
func (iq *IPQualityV2) GetLastSeen() time.Time {
	iq.mu.RLock()
	defer iq.mu.RUnlock()
	return iq.lastSeen
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
