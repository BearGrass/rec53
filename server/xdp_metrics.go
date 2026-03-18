package server

import (
	"context"
	"time"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"

	"rec53/monitor"
)

// xdpMetricsInterval controls how often BPF per-CPU counters are read and
// exported to Prometheus. 5 seconds balances freshness with CPU overhead.
const xdpMetricsInterval = 5 * time.Second

// xdpCleanupInterval controls how often expired entries are removed from the
// BPF cache map. 100ms keeps stale-entry space usage bounded while allowing
// the eBPF inline expire_ts check to handle correctness (no stale responses
// are ever served regardless of cleanup frequency).
const xdpCleanupInterval = 100 * time.Millisecond

// startXDPMetricsLoop periodically reads the BPF xdp_stats per-CPU array map
// and updates the 4 Prometheus counters. The loop exits when ctx is cancelled.
func startXDPMetricsLoop(ctx context.Context, statsMap *ebpf.Map) {
	if statsMap == nil {
		return
	}
	ticker := time.NewTicker(xdpMetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectXDPStats(statsMap)
		}
	}
}

// collectXDPStats reads all 4 BPF per-CPU stat counters and sets the
// corresponding Prometheus counters. Each per-CPU value is summed across
// all CPUs.
func collectXDPStats(statsMap *ebpf.Map) {
	if statsMap == nil {
		return
	}

	type statEntry struct {
		index uint32
		gauge *float64
	}
	var hit, miss, pass, xerr float64

	entries := []statEntry{
		{STAT_HIT, &hit},
		{STAT_MISS, &miss},
		{STAT_PASS, &pass},
		{STAT_ERROR, &xerr},
	}

	for _, e := range entries {
		var perCPU []uint64
		if err := statsMap.Lookup(e.index, &perCPU); err != nil {
			monitor.Rec53Log.Debugf("[XDP] stats lookup key %d failed: %v", e.index, err)
			continue
		}
		var total uint64
		for _, v := range perCPU {
			total += v
		}
		*e.gauge = float64(total)
	}

	monitor.XDPCacheHitsTotal.Set(hit)
	monitor.XDPCacheMissesTotal.Set(miss)
	monitor.XDPPassTotal.Set(pass)
	monitor.XDPErrorsTotal.Set(xerr)
}

// BPF stats indices matching dns_cache.h STAT_* constants.
const (
	STAT_HIT   = 0
	STAT_MISS  = 1
	STAT_PASS  = 2
	STAT_ERROR = 3
)

// startXDPCleanupLoop periodically iterates the BPF cache_map and deletes
// entries whose expire_ts has passed. This reclaims map space for new entries.
//
// Correctness note: the eBPF program already checks expire_ts inline and
// treats expired entries as cache misses (XDP_PASS). This cleanup loop is
// purely for space reclamation — skipping a cleanup cycle never causes
// stale responses to be served.
func startXDPCleanupLoop(ctx context.Context, cacheMap *ebpf.Map) {
	if cacheMap == nil {
		return
	}
	ticker := time.NewTicker(xdpCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupExpiredBPFEntries(cacheMap)
		}
	}
}

// cleanupExpiredBPFEntries iterates the BPF cache map and deletes entries
// whose monotonic expire_ts has passed. Returns the number of entries deleted.
func cleanupExpiredBPFEntries(cacheMap *ebpf.Map) int {
	if cacheMap == nil {
		return 0
	}

	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		monitor.Rec53Log.Debugf("[XDP] cleanup: ClockGettime failed: %v", err)
		return 0
	}
	nowSec := uint64(ts.Sec)

	var (
		key      dnsCacheCacheKey
		val      dnsCacheCacheValue
		deleted  int
		toDelete []dnsCacheCacheKey
	)

	// Phase 1: iterate and collect expired keys.
	// We cannot delete during iteration (undefined BPF map behavior),
	// so collect keys first, then delete in a second pass.
	iter := cacheMap.Iterate()
	for iter.Next(&key, &val) {
		if val.ExpireTs <= nowSec {
			keyCopy := key
			toDelete = append(toDelete, keyCopy)
		}
	}
	// iter.Err() can return an error if the map was modified during iteration;
	// this is benign for cleanup — we'll catch remaining entries next cycle.

	// Phase 2: delete collected expired keys.
	for _, k := range toDelete {
		if err := cacheMap.Delete(k); err == nil {
			deleted++
		}
		// Ignore delete errors (entry may have been updated/deleted concurrently).
	}

	if deleted > 0 {
		monitor.Rec53Log.Debugf("[XDP] cleanup: deleted %d expired entries", deleted)
	}
	return deleted
}
