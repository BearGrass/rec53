package monitor

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metric struct {
	reg prometheus.Registerer
}

func (m *Metric) InCounterAdd(stage string, qtype string) {
	InCounter.WithLabelValues(stage, qtype).Inc()
}

func (m *Metric) OutCounterAdd(stage string, qtype string, code string) {
	OutCounter.WithLabelValues(stage, qtype, code).Inc()
}

func (m *Metric) LatencyHistogramObserve(stage string, qtype string, code string, latency float64) {
	LatencyHistogramObserver.WithLabelValues(stage, qtype, code).Observe(latency)
}

// IPQualityV2GaugeSet sets the P50, P95, P99 latency gauges for an IP
func (m *Metric) IPQualityV2GaugeSet(ip string, p50, p95, p99 float64) {
	IPQualityV2_P50.WithLabelValues(ip).Set(p50)
	IPQualityV2_P95.WithLabelValues(ip).Set(p95)
	IPQualityV2_P99.WithLabelValues(ip).Set(p99)
}

// register metric
func (m *Metric) Register() {
	m.reg.MustRegister(InCounter)
	m.reg.MustRegister(OutCounter)
	m.reg.MustRegister(LatencyHistogramObserver)
	m.reg.MustRegister(IPQualityV2_P50)
	m.reg.MustRegister(IPQualityV2_P95)
	m.reg.MustRegister(IPQualityV2_P99)
	m.reg.MustRegister(XDPStatus)
	m.reg.MustRegister(XDPCacheHitsTotal)
	m.reg.MustRegister(XDPCacheMissesTotal)
	m.reg.MustRegister(XDPPassTotal)
	m.reg.MustRegister(XDPErrorsTotal)
}

// MetricServer holds the HTTP server for metrics
var MetricServer *http.Server

// InitMetric initializes metrics with default address ":9999"
func InitMetric() {
	InitMetricWithAddr(":9999")
}

// InitMetricWithAddr initializes metrics with a custom address
func InitMetricWithAddr(addr string) {
	Rec53Metric = &Metric{
		reg: prometheus.DefaultRegisterer,
	}
	Rec53Metric.Register()

	http.Handle("/metric", promhttp.Handler())
	MetricServer = &http.Server{Addr: addr}
	go func() {
		if err := MetricServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Rec53Log.Errorf("Metrics server error: %s", err.Error())
		}
	}()
}

// ShutdownMetric gracefully shuts down the metrics server
func ShutdownMetric(ctx context.Context) error {
	if MetricServer != nil {
		return MetricServer.Shutdown(ctx)
	}
	return nil
}

// InitMetricForTest initializes a minimal Metric instance suitable for use in
// tests. It does not register any Prometheus collectors or bind an HTTP
// listener, so it is safe to call from TestMain or individual test functions
// without causing duplicate-registration panics.
// Only use in tests.
func InitMetricForTest() {
	Rec53Metric = &Metric{}
}
