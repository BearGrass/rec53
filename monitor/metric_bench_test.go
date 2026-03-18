package monitor

import (
	"testing"
)

func init() {
	InitMetricForTest()
}

// BenchmarkInCounterAdd measures the cost of incrementing the InCounter.
func BenchmarkInCounterAdd(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rec53Metric.InCounterAdd("iter", "A")
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 10000 {
			b.Errorf("regression: %.2f ns/op > 10000 ns threshold", avgNs)
		}
	}
}

// BenchmarkOutCounterAdd measures the cost of incrementing the OutCounter (4 labels).
func BenchmarkOutCounterAdd(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rec53Metric.OutCounterAdd("iter", "A", "NOERROR")
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 10000 {
			b.Errorf("regression: %.2f ns/op > 10000 ns threshold", avgNs)
		}
	}
}

// BenchmarkLatencyHistogram measures the cost of recording a latency observation.
func BenchmarkLatencyHistogram(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rec53Metric.LatencyHistogramObserve("iter", "A", "NOERROR", 1.5)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 15000 {
			b.Errorf("regression: %.2f ns/op > 15000 ns threshold", avgNs)
		}
	}
}

// BenchmarkIPQualityV2GaugeSet measures the cost of setting three gauges (P50/P95/P99).
func BenchmarkIPQualityV2GaugeSet(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rec53Metric.IPQualityV2GaugeSet("1.2.3.4", 10, 20, 30)
	}
	b.StopTimer()

	if b.N >= 1000 {
		avgNs := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		if avgNs > 15000 {
			b.Errorf("regression: %.2f ns/op > 15000 ns threshold", avgNs)
		}
	}
}
