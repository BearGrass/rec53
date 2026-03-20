package tui

import (
	"strings"
	"testing"
	"time"
)

func TestParseMetricsAndDeriveDashboard(t *testing.T) {
	prevText := `
# TYPE rec53_query_counter counter
rec53_query_counter{stage="all",type="A"} 100
# TYPE rec53_response_counter counter
rec53_response_counter{stage="all",type="A",code="NOERROR"} 90
rec53_response_counter{stage="all",type="A",code="NXDOMAIN"} 8
rec53_response_counter{stage="all",type="A",code="SERVFAIL"} 2
# TYPE rec53_latency histogram
rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="10"} 50
rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="50"} 90
rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="200"} 98
rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="1000"} 100
rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="3000"} 100
rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="+Inf"} 100
rec53_latency_sum{stage="all",type="A",code="NOERROR"} 1200
rec53_latency_count{stage="all",type="A",code="NOERROR"} 100
# TYPE rec53_cache_lookup_total counter
rec53_cache_lookup_total{result="positive_hit"} 70
rec53_cache_lookup_total{result="negative_hit"} 10
rec53_cache_lookup_total{result="miss"} 20
# TYPE rec53_cache_entries gauge
rec53_cache_entries 300
# TYPE rec53_cache_lifecycle_total counter
rec53_cache_lifecycle_total{event="write"} 40
# TYPE rec53_snapshot_operations_total counter
rec53_snapshot_operations_total{operation="load",result="success"} 1
# TYPE rec53_snapshot_entries_total counter
rec53_snapshot_entries_total{operation="load",result="imported"} 200
rec53_snapshot_entries_total{operation="load",result="skipped_expired"} 5
# TYPE rec53_snapshot_duration_ms histogram
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="10"} 0
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="50"} 0
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="100"} 1
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="250"} 1
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="500"} 1
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="1000"} 1
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="5000"} 1
rec53_snapshot_duration_ms_bucket{operation="load",result="success",le="+Inf"} 1
rec53_snapshot_duration_ms_sum{operation="load",result="success"} 120
rec53_snapshot_duration_ms_count{operation="load",result="success"} 1
# TYPE rec53_upstream_failures_total counter
rec53_upstream_failures_total{reason="timeout",rcode=""} 3
rec53_upstream_failures_total{reason="bad_rcode",rcode="SERVFAIL"} 1
# TYPE rec53_upstream_fallback_total counter
rec53_upstream_fallback_total{result="success"} 2
# TYPE rec53_upstream_winner_total counter
rec53_upstream_winner_total{path="primary"} 5
# TYPE rec53_xdp_status gauge
rec53_xdp_status 1
# TYPE rec53_xdp_cache_hits_total gauge
rec53_xdp_cache_hits_total 100
# TYPE rec53_xdp_cache_misses_total gauge
rec53_xdp_cache_misses_total 20
# TYPE rec53_xdp_pass_total gauge
rec53_xdp_pass_total 4
# TYPE rec53_xdp_errors_total gauge
rec53_xdp_errors_total 0
# TYPE rec53_xdp_cache_sync_errors_total counter
rec53_xdp_cache_sync_errors_total{reason="update"} 0
# TYPE rec53_xdp_cleanup_deleted_total counter
rec53_xdp_cleanup_deleted_total 5
# TYPE rec53_xdp_entries gauge
rec53_xdp_entries 12
# TYPE rec53_state_machine_stage_total counter
rec53_state_machine_stage_total{stage="IN_CACHE"} 80
rec53_state_machine_stage_total{stage="ITER"} 20
# TYPE rec53_state_machine_failures_total counter
rec53_state_machine_failures_total{reason="query_upstream_error"} 1
`

	currText := strings.ReplaceAll(prevText,
		`rec53_query_counter{stage="all",type="A"} 100`,
		`rec53_query_counter{stage="all",type="A"} 140`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_response_counter{stage="all",type="A",code="NOERROR"} 90`,
		`rec53_response_counter{stage="all",type="A",code="NOERROR"} 120`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_response_counter{stage="all",type="A",code="NXDOMAIN"} 8`,
		`rec53_response_counter{stage="all",type="A",code="NXDOMAIN"} 15`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_response_counter{stage="all",type="A",code="SERVFAIL"} 2`,
		`rec53_response_counter{stage="all",type="A",code="SERVFAIL"} 5`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="10"} 50`,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="10"} 60`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="50"} 90`,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="50"} 125`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="200"} 98`,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="200"} 138`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="1000"} 100`,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="1000"} 140`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="3000"} 100`,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="3000"} 140`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="+Inf"} 100`,
		`rec53_latency_bucket{stage="all",type="A",code="NOERROR",le="+Inf"} 140`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_sum{stage="all",type="A",code="NOERROR"} 1200`,
		`rec53_latency_sum{stage="all",type="A",code="NOERROR"} 1800`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_latency_count{stage="all",type="A",code="NOERROR"} 100`,
		`rec53_latency_count{stage="all",type="A",code="NOERROR"} 140`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_cache_lookup_total{result="positive_hit"} 70`,
		`rec53_cache_lookup_total{result="positive_hit"} 100`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_cache_lookup_total{result="negative_hit"} 10`,
		`rec53_cache_lookup_total{result="negative_hit"} 18`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_cache_lookup_total{result="miss"} 20`,
		`rec53_cache_lookup_total{result="miss"} 32`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_cache_entries 300`,
		`rec53_cache_entries 320`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_cache_lifecycle_total{event="write"} 40`,
		`rec53_cache_lifecycle_total{event="write"} 52`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_upstream_failures_total{reason="timeout",rcode=""} 3`,
		`rec53_upstream_failures_total{reason="timeout",rcode=""} 7`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_upstream_failures_total{reason="bad_rcode",rcode="SERVFAIL"} 1`,
		`rec53_upstream_failures_total{reason="bad_rcode",rcode="SERVFAIL"} 2`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_upstream_fallback_total{result="success"} 2`,
		`rec53_upstream_fallback_total{result="success"} 5`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_upstream_winner_total{path="primary"} 5`,
		`rec53_upstream_winner_total{path="primary"} 11`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_xdp_cache_hits_total 100`,
		`rec53_xdp_cache_hits_total 140`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_xdp_cache_misses_total 20`,
		`rec53_xdp_cache_misses_total 32`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_xdp_pass_total 4`,
		`rec53_xdp_pass_total 6`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_xdp_cleanup_deleted_total 5`,
		`rec53_xdp_cleanup_deleted_total 9`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_state_machine_stage_total{stage="IN_CACHE"} 80`,
		`rec53_state_machine_stage_total{stage="IN_CACHE"} 110`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_state_machine_stage_total{stage="ITER"} 20`,
		`rec53_state_machine_stage_total{stage="ITER"} 30`,
	)
	currText = strings.ReplaceAll(currText,
		`rec53_state_machine_failures_total{reason="query_upstream_error"} 1`,
		`rec53_state_machine_failures_total{reason="query_upstream_error"} 4`,
	)

	prev, err := parseMetrics(strings.NewReader(prevText))
	if err != nil {
		t.Fatalf("parse prev: %v", err)
	}
	curr, err := parseMetrics(strings.NewReader(currText))
	if err != nil {
		t.Fatalf("parse curr: %v", err)
	}

	prev.At = time.Unix(100, 0)
	curr.At = time.Unix(102, 0)

	dashboard := deriveDashboard(DefaultTarget, prev, curr, 35*time.Millisecond)

	if dashboard.Traffic.Status != statusDegraded {
		t.Fatalf("traffic status = %s, want %s", dashboard.Traffic.Status, statusDegraded)
	}
	if dashboard.Cache.Status != statusOK {
		t.Fatalf("cache status = %s, want %s", dashboard.Cache.Status, statusOK)
	}
	if dashboard.Upstream.Status != statusDegraded {
		t.Fatalf("upstream status = %s, want %s", dashboard.Upstream.Status, statusDegraded)
	}
	if dashboard.XDP.Status != statusOK {
		t.Fatalf("xdp status = %s, want %s", dashboard.XDP.Status, statusOK)
	}
	if dashboard.StateMachine.Status != statusDegraded {
		t.Fatalf("state-machine status = %s, want %s", dashboard.StateMachine.Status, statusDegraded)
	}
	if dashboard.Traffic.QPS <= 0 {
		t.Fatalf("qps = %f, want > 0", dashboard.Traffic.QPS)
	}
	if dashboard.Cache.HitRatio <= 0 {
		t.Fatalf("cache hit ratio = %f, want > 0", dashboard.Cache.HitRatio)
	}
	if dashboard.XDP.HitRatio <= 0 {
		t.Fatalf("xdp hit ratio = %f, want > 0", dashboard.XDP.HitRatio)
	}
}

func TestDeriveDashboardMarksXDPDisabled(t *testing.T) {
	text := `
# TYPE rec53_xdp_status gauge
rec53_xdp_status 0
`
	snapshot, err := parseMetrics(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}

	dashboard := deriveDashboard(DefaultTarget, nil, snapshot, 10*time.Millisecond)
	if dashboard.XDP.Status != statusDisabled {
		t.Fatalf("xdp status = %s, want %s", dashboard.XDP.Status, statusDisabled)
	}
}

func TestDeriveDashboardMarksMissingFamiliesUnavailable(t *testing.T) {
	text := `
# TYPE rec53_query_counter counter
rec53_query_counter{stage="all",type="A"} 1
`
	snapshot, err := parseMetrics(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}

	dashboard := deriveDashboard(DefaultTarget, nil, snapshot, 10*time.Millisecond)
	if dashboard.Traffic.Status != statusUnavailable {
		t.Fatalf("traffic status = %s, want %s", dashboard.Traffic.Status, statusUnavailable)
	}
	if dashboard.Cache.Status != statusUnavailable {
		t.Fatalf("cache status = %s, want %s", dashboard.Cache.Status, statusUnavailable)
	}
	if dashboard.Upstream.Status != statusUnavailable {
		t.Fatalf("upstream status = %s, want %s", dashboard.Upstream.Status, statusUnavailable)
	}
}

func TestWithScrapeErrorPreservesStaleStateAfterSuccess(t *testing.T) {
	previous := Dashboard{
		Target:         DefaultTarget,
		Mode:           statusOK,
		LastSuccess:    time.Unix(100, 0),
		OverallSummary: "TRAFFIC OK",
	}

	next := withScrapeError(previous, errTestScrape)
	if next.Mode != statusStale {
		t.Fatalf("mode = %s, want %s", next.Mode, statusStale)
	}
	if next.LastError == "" {
		t.Fatal("expected last error to be recorded")
	}
}

func TestRenderPlainDashboard(t *testing.T) {
	dashboard := Dashboard{
		Target:         DefaultTarget,
		Mode:           statusOK,
		LastUpdate:     time.Unix(100, 0),
		ScrapeDuration: 25 * time.Millisecond,
		OverallSummary: "TRAFFIC OK | CACHE OK",
		Traffic:        TrafficPanel{Status: statusOK, QPS: 12.5},
		Cache:          CachePanel{Status: statusOK, HitRatio: 0.9},
		Upstream:       UpstreamPanel{Status: statusDisabled},
		XDP:            XDPPanel{Status: statusDisabled, Mode: "disabled"},
		StateMachine:   StateMachinePanel{Status: statusOK},
		Snapshot:       SnapshotPanel{Status: statusWarming},
	}

	out := renderPlainDashboard(dashboard, 2*time.Second)
	for _, want := range []string{
		"rec53top OK",
		"target=http://127.0.0.1:9999/metric",
		"Traffic",
		"qps=12.5",
		"Cache",
		"hit_ratio=90.0%",
		"XDP",
		"mode=disabled",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("plain dashboard missing %q\n%s", want, out)
		}
	}
}

var errTestScrape = testError("scrape failed")

type testError string

func (e testError) Error() string {
	return string(e)
}
