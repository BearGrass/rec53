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

	Rec53Metric *Metric
)
