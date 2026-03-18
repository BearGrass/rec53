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

	Rec53Metric *Metric
)
