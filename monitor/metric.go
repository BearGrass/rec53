package monitor

import (
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

//register metric
func (m *Metric) Register() {
	m.reg.MustRegister(InCounter)
	m.reg.MustRegister(OutCounter)
	m.reg.MustRegister(LatencyHistogramObserver)
}

func InitMetric() {
	Rec53Metric = &Metric{
		reg: prometheus.DefaultRegisterer,
	}
	Rec53Metric.Register()

	http.Handle("/metric", promhttp.Handler())
	go http.ListenAndServe(":9999", nil)
}
