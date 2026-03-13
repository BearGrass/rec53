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

func (m *Metric) InCounterAdd(stage string, name string, qtype string) {
	InCounter.With(prometheus.Labels{"stage": stage, "name": name, "type": qtype}).Inc()
}

func (m *Metric) OutCounterAdd(stage string, name string, qtype string, code string) {
	OutCounter.With(prometheus.Labels{"stage": stage, "name": name, "type": qtype, "code": code}).Inc()
}

func (m *Metric) LatencyHistogramObserve(stage string, name string, qtype string, code string, latency float64) {
	LatencyHistogramObserver.With(prometheus.Labels{"stage": stage, "name": name, "type": qtype, "code": code}).Observe(latency)
}

func (m *Metric) IPQualityGaugeSet(ip string, quality float64) {
	IPQuality.With(prometheus.Labels{"ip": ip}).Set(quality)
}

// IPQualityV2GaugeSet sets the P50, P95, P99 latency gauges for an IP
func (m *Metric) IPQualityV2GaugeSet(ip string, p50, p95, p99 float64) {
	IPQualityV2_P50.With(prometheus.Labels{"ip": ip}).Set(p50)
	IPQualityV2_P95.With(prometheus.Labels{"ip": ip}).Set(p95)
	IPQualityV2_P99.With(prometheus.Labels{"ip": ip}).Set(p99)
}

// register metric
func (m *Metric) Register() {
	m.reg.MustRegister(InCounter)
	m.reg.MustRegister(OutCounter)
	m.reg.MustRegister(LatencyHistogramObserver)
	m.reg.MustRegister(IPQuality)
	m.reg.MustRegister(IPQualityV2_P50)
	m.reg.MustRegister(IPQualityV2_P95)
	m.reg.MustRegister(IPQualityV2_P99)
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
func InitMetricForTest() {
	Rec53Metric = &Metric{}
}
