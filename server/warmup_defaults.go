package server

import (
	"runtime"
	"time"
)

// WarmupConfig represents the configuration for NS record warmup on startup
type WarmupConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Timeout     time.Duration `yaml:"timeout"`  // Per-query timeout for individual NS resolutions
	Duration    time.Duration `yaml:"duration"` // Overall warmup process hard deadline
	Concurrency int           `yaml:"concurrency"`
	TLDs        []string      `yaml:"tlds"`
}

// DefaultTLDs is the default list of TLDs for NS record warmup.
// NOTE: This is now deprecated in favor of the curated list in DefaultCuratedTLDs.
// It's kept for backward compatibility, but new configurations should use LoadTLDList()
// from tld_config.go to get the optimized curated list.
var DefaultTLDs = DefaultCuratedTLDs

// calcOptimalConcurrency calculates the optimal warmup concurrency based on available CPU cores.
// Formula: min(NumCPU() * 2, 8)
// - 2x multiplier because DNS queries are I/O-bound (network latency dominates)
// - 8 is the hard upper limit to prevent excessive goroutine overhead on large machines
// On 4-core: min(8, 8) = 8; on 2-core: min(4, 8) = 4; on 16-core: min(32, 8) = 8
func calcOptimalConcurrency() int {
	numCPU := runtime.NumCPU()
	concurrency := numCPU * 2
	if concurrency > 8 {
		concurrency = 8
	}
	return concurrency
}

// DefaultWarmupConfig is the default warmup configuration
var DefaultWarmupConfig = WarmupConfig{
	Enabled:     true,
	Timeout:     5 * time.Second,
	Duration:    5 * time.Second,
	Concurrency: calcOptimalConcurrency(),
	TLDs:        DefaultTLDs,
}
