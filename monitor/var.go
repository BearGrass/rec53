package monitor

import "github.com/prometheus/client_golang/prometheus"

var (
	InCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_query_counter",
			Help: "rec53 query counter",
		},
		[]string{"stage", "name", "type"},
	)
	OutCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rec53_response_counter",
			Help: "rec53 response counter",
		},
		[]string{"stage", "name", "type", "code"},
	)
	QpsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rec53_qps",
			Help: "rec53 qps",
		},
		[]string{"action"},
	)

	Rec53Metric *Metric
)
