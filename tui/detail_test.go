package tui

import (
	"strings"
	"testing"
)

func TestBuildTotalBreakdownBoundsAndSorts(t *testing.T) {
	items := buildTotalBreakdown(map[string]float64{
		"NOERROR":  120,
		"NXDOMAIN": 15,
		"SERVFAIL": 5,
		"REFUSED":  3,
		"FORMERR":  1,
	}, 4)

	if len(items) != 4 {
		t.Fatalf("len(items) = %d, want 4", len(items))
	}
	if items[0].Label != "NOERROR" || items[0].Total != 120 {
		t.Fatalf("top item = %+v, want NOERROR 120", items[0])
	}
	for _, item := range items {
		if item.Label == "FORMERR" {
			t.Fatalf("unexpected unbounded item in top-N output: %+v", items)
		}
	}
}

func TestBuildTrafficDetailModelDegraded(t *testing.T) {
	model := buildTrafficDetailModel(Dashboard{
		Traffic: TrafficPanel{
			Status:        statusDegraded,
			QPS:           120,
			P99MS:         1250,
			ServfailRatio: 0.12,
			NXDomainRatio: 0.08,
			NoErrorRatio:  0.80,
			ResponseCodes: []BreakdownItem{
				{Label: "SERVFAIL", Rate: 14, Ratio: 0.12},
				{Label: "NOERROR", Rate: 96, Ratio: 0.80},
			},
		},
	})

	if !strings.Contains(model.Standout, "SERVFAIL is elevated") {
		t.Fatalf("standout = %q, want SERVFAIL explanation", model.Standout)
	}
	if len(model.NextChecks) == 0 {
		t.Fatal("expected next checks for degraded traffic")
	}
	if !strings.Contains(strings.Join(model.NextChecks, "\n"), "4 Upstream") {
		t.Fatalf("next checks = %v, want upstream guidance", model.NextChecks)
	}
}

func TestBuildXDPDetailModelDisabled(t *testing.T) {
	model := buildXDPDetailModel(Dashboard{
		XDP: XDPPanel{
			Status: statusDisabled,
			Mode:   "disabled",
		},
	})

	if !strings.Contains(model.Standout, "intentionally disabled") {
		t.Fatalf("standout = %q, want disabled explanation", model.Standout)
	}
	if len(model.NextChecks) == 0 {
		t.Fatal("expected next checks for disabled xdp")
	}
}

func TestRenderDetailAddsDiagnosticSections(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailUpstream
	snapshot := mustParseMetricsForDetailTest(t, `
# TYPE rec53_upstream_failures_total counter
rec53_upstream_failures_total{reason="timeout",rcode=""} 7
rec53_upstream_failures_total{reason="bad_rcode",rcode="SERVFAIL"} 2
# TYPE rec53_upstream_fallback_total counter
rec53_upstream_fallback_total{result="success"} 5
# TYPE rec53_upstream_winner_total counter
rec53_upstream_winner_total{path="primary"} 11
rec53_upstream_winner_total{path="secondary"} 4
`)

	text := ui.renderDetail(Dashboard{
		CurrentSnapshot: snapshot,
		Upstream: UpstreamPanel{
			Status:         statusDegraded,
			TimeoutRate:    2,
			BadRcodeRate:   0.5,
			FallbackRate:   1,
			Winner:         "primary",
			WinnerRate:     3,
			DominantReason: "timeout",
			FailureReasons: []BreakdownItem{
				{Label: "timeout", Rate: 2, Ratio: 0.8},
				{Label: "bad_rcode", Rate: 0.5, Ratio: 0.2},
			},
			Winners: []BreakdownItem{
				{Label: "primary", Rate: 3, Ratio: 0.75},
				{Label: "secondary", Rate: 1, Ratio: 0.25},
			},
		},
	})

	for _, want := range []string{
		"What stands out now:",
		"Current window:",
		"Since start counters:",
		"Failure reasons:",
		"Winner mix:",
		"Winner paths:",
		"Next checks:",
		"timeout is the dominant recent upstream failure reason",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("detail view missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Reading guide:") {
		t.Fatalf("detail view should not use static Reading guide section anymore\n%s", text)
	}
}

func TestRenderDetailExplainsStaleState(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailState

	text := ui.renderDetail(Dashboard{
		LastError: "dial tcp 127.0.0.1:9999: connect: connection refused",
		StateMachine: StateMachinePanel{
			Status: statusStale,
		},
	})

	for _, want := range []string{
		"What stands out now:",
		"stale data because the latest scrape failed",
		"Next checks:",
		"Check scrape connectivity first",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("stale detail missing %q\n%s", want, text)
		}
	}
}

func TestRenderDetailTrafficAndCacheAreNotOverviewCopies(t *testing.T) {
	snapshot := mustParseMetricsForDetailTest(t, `
# TYPE rec53_query_counter counter
rec53_query_counter{stage="all",type="A"} 143
# TYPE rec53_response_counter counter
rec53_response_counter{stage="all",type="A",code="NOERROR"} 120
rec53_response_counter{stage="all",type="A",code="NXDOMAIN"} 15
rec53_response_counter{stage="all",type="A",code="SERVFAIL"} 5
# TYPE rec53_cache_lookup_total counter
rec53_cache_lookup_total{result="positive_hit"} 100
rec53_cache_lookup_total{result="negative_hit"} 18
rec53_cache_lookup_total{result="miss"} 32
# TYPE rec53_cache_lifecycle_total counter
rec53_cache_lifecycle_total{event="write"} 52
`)

	cases := []struct {
		name       string
		panel      detailPanel
		dashboard  Dashboard
		wantSubstr string
	}{
		{
			name:  "traffic",
			panel: detailTraffic,
			dashboard: Dashboard{
				CurrentSnapshot: snapshot,
				Traffic: TrafficPanel{
					Status:        statusOK,
					QPS:           44,
					P99MS:         42,
					ServfailRatio: 0.01,
					NoErrorRatio:  0.94,
					ResponseCodes: []BreakdownItem{{Label: "NOERROR", Rate: 40, Ratio: 0.94}},
				},
			},
			wantSubstr: "dominant recent response bucket",
		},
		{
			name:  "cache",
			panel: detailCache,
			dashboard: Dashboard{
				CurrentSnapshot: snapshot,
				Cache: CachePanel{
					Status:          statusDegraded,
					HitRatio:        0.35,
					PositiveHitRate: 20,
					MissRate:        40,
					Lifecycle:       "write 8.0/s",
					Results: []BreakdownItem{
						{Label: "miss", Rate: 40, Ratio: 0.57},
						{Label: "positive_hit", Rate: 20, Ratio: 0.29},
					},
				},
			},
			wantSubstr: "misses are currently outrunning",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := newDashboardUI()
			ui.detailPanel = tc.panel
			text := ui.renderDetail(tc.dashboard)
			for _, want := range []string{"Current window:", "Since start counters:"} {
				if !strings.Contains(text, want) {
					t.Fatalf("detail view missing %q\n%s", want, text)
				}
			}
			if !strings.Contains(strings.ToLower(text), tc.wantSubstr) {
				t.Fatalf("detail view missing diagnostic text %q\n%s", tc.wantSubstr, text)
			}
		})
	}
}

func TestRenderDetailShowsBoundedCumulativeTrafficCounters(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailTraffic

	text := ui.renderDetail(Dashboard{
		CurrentSnapshot: mustParseMetricsForDetailTest(t, `
# TYPE rec53_query_counter counter
rec53_query_counter{stage="all",type="A"} 144
# TYPE rec53_response_counter counter
rec53_response_counter{stage="all",type="A",code="NOERROR"} 120
rec53_response_counter{stage="all",type="A",code="NXDOMAIN"} 15
rec53_response_counter{stage="all",type="A",code="SERVFAIL"} 5
rec53_response_counter{stage="all",type="A",code="REFUSED"} 3
rec53_response_counter{stage="all",type="A",code="FORMERR"} 1
`),
		Traffic: TrafficPanel{
			Status:        statusDegraded,
			QPS:           120,
			P99MS:         1250,
			ServfailRatio: 0.12,
			NXDomainRatio: 0.08,
			NoErrorRatio:  0.80,
			ResponseCodes: []BreakdownItem{
				{Label: "SERVFAIL", Rate: 14, Ratio: 0.12},
				{Label: "NOERROR", Rate: 96, Ratio: 0.80},
			},
		},
	})

	for _, want := range []string{
		"Current window:",
		"Since start counters:",
		"queries total",
		"responses total",
		"Response codes:",
		"NOERROR",
		"REFUSED",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("detail view missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "FORMERR") {
		t.Fatalf("detail view should keep cumulative response codes bounded\n%s", text)
	}
	if !strings.Contains(text, "SERVFAIL is elevated") {
		t.Fatalf("detail view lost current-window standout\n%s", text)
	}
}

func mustParseMetricsForDetailTest(t *testing.T, text string) *MetricsSnapshot {
	t.Helper()
	snapshot, err := parseMetrics(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}
	return snapshot
}
