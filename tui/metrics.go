package tui

import (
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type metricSample struct {
	Labels map[string]string
	Value  float64
}

type histogramSample struct {
	Labels  map[string]string
	Count   float64
	Sum     float64
	Buckets map[float64]float64
}

type transitionKey struct {
	From string
	To   string
}

type MetricsSnapshot struct {
	At         time.Time
	Gauges     map[string][]metricSample
	Counters   map[string][]metricSample
	Histograms map[string][]histogramSample
}

type ScrapeResult struct {
	Snapshot *MetricsSnapshot
	Duration time.Duration
}

type Scraper struct {
	client *http.Client
}

func NewScraper(timeout time.Duration) *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: timeout},
	}
}

func (s *Scraper) Scrape(target string) (*ScrapeResult, error) {
	start := time.Now()
	resp, err := s.client.Get(target)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	snapshot, err := parseMetrics(resp.Body)
	if err != nil {
		return nil, err
	}
	snapshot.At = time.Now()

	return &ScrapeResult{
		Snapshot: snapshot,
		Duration: time.Since(start),
	}, nil
}

func parseMetrics(r io.Reader) (*MetricsSnapshot, error) {
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return nil, fmt.Errorf("parse metrics: %w", err)
	}

	snapshot := &MetricsSnapshot{
		At:         time.Now(),
		Gauges:     make(map[string][]metricSample),
		Counters:   make(map[string][]metricSample),
		Histograms: make(map[string][]histogramSample),
	}

	for name, family := range families {
		switch family.GetType() {
		case dto.MetricType_GAUGE:
			snapshot.Gauges[name] = append(snapshot.Gauges[name], collectScalarSamples(family, true)...)
		case dto.MetricType_COUNTER:
			snapshot.Counters[name] = append(snapshot.Counters[name], collectScalarSamples(family, false)...)
		case dto.MetricType_HISTOGRAM:
			snapshot.Histograms[name] = append(snapshot.Histograms[name], collectHistogramSamples(family)...)
		}
	}

	return snapshot, nil
}

func collectScalarSamples(family *dto.MetricFamily, gauge bool) []metricSample {
	samples := make([]metricSample, 0, len(family.GetMetric()))
	for _, metric := range family.GetMetric() {
		value := 0.0
		if gauge {
			value = metric.GetGauge().GetValue()
		} else {
			value = metric.GetCounter().GetValue()
		}
		samples = append(samples, metricSample{
			Labels: collectLabels(metric.GetLabel()),
			Value:  value,
		})
	}
	return samples
}

func collectHistogramSamples(family *dto.MetricFamily) []histogramSample {
	samples := make([]histogramSample, 0, len(family.GetMetric()))
	for _, metric := range family.GetMetric() {
		buckets := make(map[float64]float64)
		for _, bucket := range metric.GetHistogram().GetBucket() {
			buckets[bucket.GetUpperBound()] = float64(bucket.GetCumulativeCount())
		}
		buckets[math.Inf(1)] = float64(metric.GetHistogram().GetSampleCount())

		samples = append(samples, histogramSample{
			Labels:  collectLabels(metric.GetLabel()),
			Count:   float64(metric.GetHistogram().GetSampleCount()),
			Sum:     metric.GetHistogram().GetSampleSum(),
			Buckets: buckets,
		})
	}
	return samples
}

func collectLabels(labels []*dto.LabelPair) map[string]string {
	result := make(map[string]string, len(labels))
	for _, label := range labels {
		result[label.GetName()] = label.GetValue()
	}
	return result
}

func (s *MetricsSnapshot) aggregateSamples(name string) ([]metricSample, bool) {
	if samples, ok := s.Counters[name]; ok {
		return samples, true
	}
	if samples, ok := s.Gauges[name]; ok {
		return samples, true
	}
	return nil, false
}

func (s *MetricsSnapshot) sum(name string) (float64, bool) {
	samples, ok := s.aggregateSamples(name)
	if !ok {
		return 0, false
	}
	total := 0.0
	for _, sample := range samples {
		total += sample.Value
	}
	return total, true
}

func (s *MetricsSnapshot) sumByLabel(name, label string) (map[string]float64, bool) {
	samples, ok := s.aggregateSamples(name)
	if !ok {
		return nil, false
	}
	values := make(map[string]float64)
	for _, sample := range samples {
		key := sample.Labels[label]
		if key == "" {
			key = "unknown"
		}
		values[key] += sample.Value
	}
	return values, true
}

func (s *MetricsSnapshot) sumByLabelPair(name, left, right string) (map[transitionKey]float64, bool) {
	samples, ok := s.aggregateSamples(name)
	if !ok {
		return nil, false
	}
	values := make(map[transitionKey]float64)
	for _, sample := range samples {
		key := transitionKey{
			From: sample.Labels[left],
			To:   sample.Labels[right],
		}
		if key.From == "" {
			key.From = "unknown"
		}
		if key.To == "" {
			key.To = "unknown"
		}
		values[key] += sample.Value
	}
	return values, true
}

func (s *MetricsSnapshot) histogramBuckets(name string) (map[float64]float64, bool) {
	histograms, ok := s.Histograms[name]
	if !ok {
		return nil, false
	}
	buckets := make(map[float64]float64)
	for _, histogram := range histograms {
		for upper, count := range histogram.Buckets {
			buckets[upper] += count
		}
	}
	return buckets, true
}

func (s *MetricsSnapshot) gaugeValue(name string) (float64, bool) {
	samples, ok := s.Gauges[name]
	if !ok || len(samples) == 0 {
		return 0, false
	}
	return samples[0].Value, true
}

func (s *MetricsSnapshot) hasMetric(name string) bool {
	if _, ok := s.aggregateSamples(name); ok {
		return true
	}
	_, ok := s.Histograms[name]
	return ok
}

func deltaFloat(curr, prev float64) float64 {
	if curr < prev {
		return 0
	}
	return curr - prev
}

func deltaMap(curr, prev map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(curr))
	for key, value := range curr {
		result[key] = deltaFloat(value, prev[key])
	}
	return result
}

func deltaTransitionMap(curr, prev map[transitionKey]float64) map[transitionKey]float64 {
	result := make(map[transitionKey]float64, len(curr))
	for key, value := range curr {
		result[key] = deltaFloat(value, prev[key])
	}
	return result
}

func deltaBuckets(curr, prev map[float64]float64) map[float64]float64 {
	result := make(map[float64]float64, len(curr))
	for key, value := range curr {
		result[key] = deltaFloat(value, prev[key])
	}
	return result
}

func sortFloatKeys(values map[float64]float64) []float64 {
	keys := make([]float64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Float64s(keys)
	return keys
}

func histogramQuantile(q float64, buckets map[float64]float64) (float64, bool) {
	if len(buckets) == 0 {
		return 0, false
	}
	keys := sortFloatKeys(buckets)
	total := buckets[keys[len(keys)-1]]
	if total <= 0 {
		return 0, false
	}

	target := q * total
	prevUpper := 0.0
	prevCount := 0.0
	for _, upper := range keys {
		count := buckets[upper]
		if count >= target {
			if math.IsInf(upper, 1) {
				return prevUpper, true
			}
			bucketCount := count - prevCount
			if bucketCount <= 0 {
				return upper, true
			}
			position := (target - prevCount) / bucketCount
			return prevUpper + (upper-prevUpper)*position, true
		}
		prevUpper = upper
		prevCount = count
	}
	return keys[len(keys)-1], true
}

func pickTopLabel(values map[string]float64) (string, float64) {
	if len(values) == 0 {
		return "", 0
	}
	type entry struct {
		key   string
		value float64
	}
	entries := make([]entry, 0, len(values))
	for key, value := range values {
		entries = append(entries, entry{key: key, value: value})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value == entries[j].value {
			return entries[i].key < entries[j].key
		}
		return entries[i].value > entries[j].value
	})
	return entries[0].key, entries[0].value
}

func cloneLabels(labels map[string]string) map[string]string {
	return maps.Clone(labels)
}

func labelSignature(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ",")
}

func parseBucketUpper(value string) (float64, error) {
	if value == "+Inf" {
		return math.Inf(1), nil
	}
	return strconv.ParseFloat(value, 64)
}
