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

//register metric
func (m *Metric) Register() {
	m.reg.MustRegister(InCounter)
	m.reg.MustRegister(OutCounter)
	m.reg.MustRegister(LatencyHistogramObserver)
	m.reg.MustRegister(IPQuality)
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
