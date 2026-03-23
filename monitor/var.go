package monitor

import "github.com/prometheus/client_golang/prometheus"

var (
	InCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_query_counter",
			Help: "rec53 query counter",
		},
		[]string{"stage", "type"},
	)
	OutCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_response_counter",
			Help: "rec53 response counter",
		},
		[]string{"stage", "type", "code"},
	)

	LatencyHistogramObserver = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rec53_latency",
			Help:    "rec53 latency",
			Buckets: []float64{10, 50, 200, 1000, 3000}, // ms
		},
		[]string{"stage", "type", "code"},
	)
	IPQualityV2_P50 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rec53_ipv2_p50_latency_ms",
			Help: "rec53 IP quality V2 P50 latency in milliseconds",
		},
		[]string{"ip"},
	)

	IPQualityV2_P95 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rec53_ipv2_p95_latency_ms",
			Help: "rec53 IP quality V2 P95 latency in milliseconds",
		},
		[]string{"ip"},
	)

	IPQualityV2_P99 = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rec53_ipv2_p99_latency_ms",
			Help: "rec53 IP quality V2 P99 latency in milliseconds",
		},
		[]string{"ip"},
	)

	// XDPStatus reports the XDP fast path status:
	//   0 = disabled or unavailable (degraded to Go-only cache)
	//   1 = active (eBPF attached, serving cache hits via XDP_TX)
	XDPStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_xdp_status",
			Help: "XDP fast path status: 0=disabled/unavailable, 1=active",
		},
	)

	// XDP BPF per-CPU counters exported as Prometheus gauges.
	// These are absolute counters read periodically from the BPF xdp_stats
	// per-CPU array map (summed across all CPUs). Using Gauge instead of
	// Counter because the values are set from BPF-side totals, not incremented.
	XDPCacheHitsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_xdp_cache_hits_total",
			Help: "Total DNS cache hits served via XDP_TX (from BPF per-CPU counters)",
		},
	)
	XDPCacheMissesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_xdp_cache_misses_total",
			Help: "Total DNS cache misses passed to Go resolver via XDP_PASS (from BPF per-CPU counters)",
		},
	)
	XDPPassTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_xdp_pass_total",
			Help: "Total packets passed to Go (non-DNS, non-UDP, TC bit, etc.) via XDP_PASS (from BPF per-CPU counters)",
		},
	)
	XDPErrorsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_xdp_errors_total",
			Help: "Total XDP processing errors (from BPF per-CPU counters)",
		},
	)

	CacheLookupTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_cache_lookup_total",
			Help: "DNS cache lookup results by bounded outcome",
		},
		[]string{"result"},
	)
	CacheEntries = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_cache_entries",
			Help: "Current number of entries in the Go DNS cache",
		},
	)
	CacheLifecycleTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_cache_lifecycle_total",
			Help: "DNS cache lifecycle events by bounded event type",
		},
		[]string{"event"},
	)

	SnapshotOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_snapshot_operations_total",
			Help: "Snapshot save/load attempts by operation and result",
		},
		[]string{"operation", "result"},
	)
	SnapshotEntriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_snapshot_entries_total",
			Help: "Snapshot entry counts by operation and bounded result",
		},
		[]string{"operation", "result"},
	)
	SnapshotDurationMs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rec53_snapshot_duration_ms",
			Help:    "Snapshot save/load duration in milliseconds",
			Buckets: []float64{1, 5, 10, 50, 100, 250, 500, 1000, 5000},
		},
		[]string{"operation", "result"},
	)

	UpstreamFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_upstream_failures_total",
			Help: "Upstream query failures by bounded reason and rcode",
		},
		[]string{"reason", "rcode"},
	)
	UpstreamFallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_upstream_fallback_total",
			Help: "Alternate upstream fallback attempts by bounded result",
		},
		[]string{"result"},
	)
	UpstreamWinnerTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_upstream_winner_total",
			Help: "Happy Eyeballs winner path by bounded path name",
		},
		[]string{"path"},
	)

	XDPSyncErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_xdp_cache_sync_errors_total",
			Help: "XDP cache sync failures by bounded reason",
		},
		[]string{"reason"},
	)
	XDPCleanupDeletedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rec53_xdp_cleanup_deleted_total",
			Help: "Total expired XDP cache entries deleted during cleanup",
		},
	)
	XDPEntries = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "rec53_xdp_entries",
			Help: "Current active XDP cache entry count after cleanup reconciliation",
		},
	)

	StateMachineStageTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_state_machine_stage_total",
			Help: "State machine stage transitions by bounded stage name",
		},
		[]string{"stage"},
	)
	StateMachineFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_state_machine_failures_total",
			Help: "State machine terminal failures by bounded reason",
		},
		[]string{"reason"},
	)
	StateMachineTransitionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_state_machine_transition_total",
			Help: "State machine transitions by bounded from/to edge",
		},
		[]string{"from", "to"},
	)
	ExpensiveRequestLimitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_expensive_request_limit_total",
			Help: "Per-client expensive request limit events by bounded action and path",
		},
		[]string{"action", "path"},
	)

	Rec53Metric *Metric
)
