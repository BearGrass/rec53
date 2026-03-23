package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"

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

func (m *Metric) CacheLookupAdd(result string) {
	CacheLookupTotal.WithLabelValues(result).Inc()
}

func (m *Metric) CacheEntriesSet(entries int) {
	CacheEntries.Set(float64(entries))
}

func (m *Metric) CacheLifecycleAdd(event string, delta int) {
	CacheLifecycleTotal.WithLabelValues(event).Add(float64(delta))
}

// IPQualityV2GaugeSet sets the P50, P95, P99 latency gauges for an IP
func (m *Metric) IPQualityV2GaugeSet(ip string, p50, p95, p99 float64) {
	IPQualityV2_P50.WithLabelValues(ip).Set(p50)
	IPQualityV2_P95.WithLabelValues(ip).Set(p95)
	IPQualityV2_P99.WithLabelValues(ip).Set(p99)
}

func (m *Metric) SnapshotOperationAdd(operation, result string) {
	SnapshotOperationsTotal.WithLabelValues(operation, result).Inc()
}

func (m *Metric) SnapshotEntriesAdd(operation, result string, count int) {
	SnapshotEntriesTotal.WithLabelValues(operation, result).Add(float64(count))
}

func (m *Metric) SnapshotDurationObserve(operation, result string, duration time.Duration) {
	SnapshotDurationMs.WithLabelValues(operation, result).Observe(float64(duration.Milliseconds()))
}

func (m *Metric) UpstreamFailureAdd(reason, rcode string) {
	UpstreamFailuresTotal.WithLabelValues(reason, rcode).Inc()
}

func (m *Metric) UpstreamFallbackAdd(result string) {
	UpstreamFallbackTotal.WithLabelValues(result).Inc()
}

func (m *Metric) UpstreamWinnerAdd(path string) {
	UpstreamWinnerTotal.WithLabelValues(path).Inc()
}

func (m *Metric) XDPSyncErrorAdd(reason string) {
	XDPSyncErrorsTotal.WithLabelValues(reason).Inc()
}

func (m *Metric) XDPCleanupDeletedAdd(count int) {
	XDPCleanupDeletedTotal.Add(float64(count))
}

func (m *Metric) XDPEntriesSet(count int) {
	XDPEntries.Set(float64(count))
}

func (m *Metric) StateMachineStageAdd(stage string) {
	StateMachineStageTotal.WithLabelValues(stage).Inc()
}

func (m *Metric) StateMachineFailureAdd(reason string) {
	StateMachineFailuresTotal.WithLabelValues(reason).Inc()
}

func (m *Metric) StateMachineTransitionAdd(from, to string) {
	StateMachineTransitionTotal.WithLabelValues(from, to).Inc()
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
	m.reg.MustRegister(CacheLookupTotal)
	m.reg.MustRegister(CacheEntries)
	m.reg.MustRegister(CacheLifecycleTotal)
	m.reg.MustRegister(SnapshotOperationsTotal)
	m.reg.MustRegister(SnapshotEntriesTotal)
	m.reg.MustRegister(SnapshotDurationMs)
	m.reg.MustRegister(UpstreamFailuresTotal)
	m.reg.MustRegister(UpstreamFallbackTotal)
	m.reg.MustRegister(UpstreamWinnerTotal)
	m.reg.MustRegister(XDPSyncErrorsTotal)
	m.reg.MustRegister(XDPCleanupDeletedTotal)
	m.reg.MustRegister(XDPEntries)
	m.reg.MustRegister(StateMachineStageTotal)
	m.reg.MustRegister(StateMachineFailuresTotal)
	m.reg.MustRegister(StateMachineTransitionTotal)
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

	MetricServer = &http.Server{
		Addr:    addr,
		Handler: newOperationalMux(promhttp.Handler()),
	}
	go func() {
		if err := MetricServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Rec53Log.Errorf("Metrics server error: %s", err.Error())
		}
	}()
}

func newOperationalMux(metricHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metric", instrumentMetricsHandler(metricHandler))
	mux.HandleFunc("/healthz/ready", readinessHandler)
	return mux
}

func readinessHandler(w http.ResponseWriter, _ *http.Request) {
	state := RuntimeState()
	status := http.StatusOK
	if !state.Readiness {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "ready=%t\nphase=%s\n", state.Readiness, state.Phase)
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
