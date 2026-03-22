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
	ui.detailView[detailUpstream] = subviewUpstreamFailures
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
		"Subview:",
		"What stands out now:",
		"Current window:",
		"Since start counters:",
		"Failure reasons:",
		"Next checks:",
		"Upstream failures isolates the recent error mix",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("detail view missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Winner mix:") {
		t.Fatalf("failure drilldown should not flatten winner sections into the same page\n%s", text)
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

func TestRenderDetailStateSummaryShowsCounters(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailState

	text := ui.renderDetail(Dashboard{
		CurrentSnapshot: mustParseMetricsForDetailTest(t, `
# TYPE rec53_state_machine_stage_total counter
rec53_state_machine_stage_total{stage="cache_lookup"} 120
rec53_state_machine_stage_total{stage="query_upstream"} 40
# TYPE rec53_state_machine_transition_total counter
rec53_state_machine_transition_total{from="state_init",to="hosts_lookup"} 130
rec53_state_machine_transition_total{from="hosts_lookup",to="forward_lookup"} 130
rec53_state_machine_transition_total{from="forward_lookup",to="cache_lookup"} 100
rec53_state_machine_transition_total{from="cache_lookup",to="classify_resp"} 96
rec53_state_machine_transition_total{from="classify_resp",to="return_resp"} 90
rec53_state_machine_transition_total{from="return_resp",to="success_exit"} 90
# TYPE rec53_state_machine_failures_total counter
rec53_state_machine_failures_total{reason="query_upstream_error"} 3
`),
		StateMachine: StateMachinePanel{
			Status:          statusOK,
			TopStage:        "cache_lookup",
			TopStageRate:    15,
			TopTerminal:     "success_exit",
			TopTerminalRate: 10,
			TopFailure:      "",
			Terminals: []BreakdownItem{
				{Label: "success_exit", Rate: 10, Ratio: 1},
			},
			Stages: []BreakdownItem{
				{Label: "cache_lookup", Rate: 15, Ratio: 0.75},
				{Label: "query_upstream", Rate: 5, Ratio: 0.25},
			},
			Failures: []BreakdownItem{
				{Label: "query_upstream_error", Rate: 1.5, Ratio: 1},
			},
		},
	})

	for _, want := range []string{
		"What stands out now:",
		"Current window:",
		"Stage mix:",
		"Terminal exits:",
		"Failure reasons:",
		"success_exit",
		"cache_lookup",
		"trace-domain",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("state summary missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Subview:") {
		t.Fatalf("state summary should not render subview tabs\n%s", text)
	}
}

func TestRenderDetailStateDegradedHighlightsFailureAndTerminal(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailState

	text := ui.renderDetail(Dashboard{
		CurrentSnapshot: mustParseMetricsForDetailTest(t, `
# TYPE rec53_state_machine_stage_total counter
rec53_state_machine_stage_total{stage="query_upstream"} 7
# TYPE rec53_state_machine_failures_total counter
rec53_state_machine_failures_total{reason="query_upstream_error"} 7
rec53_state_machine_failures_total{reason="max_iterations"} 2
# TYPE rec53_state_machine_transition_total counter
rec53_state_machine_transition_total{from="query_upstream",to="servfail_exit"} 7
rec53_state_machine_transition_total{from="return_resp",to="success_exit"} 2
`),
		StateMachine: StateMachinePanel{
			Status:          statusDegraded,
			TopStage:        "query_upstream",
			TopStageRate:    2,
			TopFailure:      "query_upstream_error",
			TopFailureRate:  2,
			TopTerminal:     "servfail_exit",
			TopTerminalRate: 2,
			Terminals: []BreakdownItem{
				{Label: "servfail_exit", Rate: 2, Ratio: 0.8},
				{Label: "success_exit", Rate: 0.5, Ratio: 0.2},
			},
			Stages: []BreakdownItem{
				{Label: "query_upstream", Rate: 2, Ratio: 1},
			},
			Failures: []BreakdownItem{
				{Label: "query_upstream_error", Rate: 2, Ratio: 0.8},
				{Label: "max_iterations", Rate: 0.5, Ratio: 0.2},
			},
		},
	})

	for _, want := range []string{
		"query_upstream_error is the top recent failure reason",
		"Failure reasons:",
		"query_upstream_error",
		"servfail_exit",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("state degraded summary missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Subview:") {
		t.Fatalf("state degraded summary should not render subview tabs\n%s", text)
	}
}

func TestRenderDetailStateStaleDoesNotShowSubviewTabs(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailState

	text := ui.renderDetail(Dashboard{
		LastError: "scrape timeout",
		StateMachine: StateMachinePanel{
			Status: statusStale,
		},
	})

	if !strings.Contains(text, "stale data because the latest scrape failed") {
		t.Fatalf("state stale detail lost stale explanation\n%s", text)
	}
	if strings.Contains(text, "Subview:") {
		t.Fatalf("state stale detail should not show subview tabs\n%s", text)
	}
}

func TestRenderDetailStateOverrideStatusesStayReadable(t *testing.T) {
	tests := []struct {
		name   string
		status panelStatus
		want   string
	}{
		{name: "warming", status: statusWarming, want: "Only one successful scrape is available"},
		{name: "unavailable", status: statusUnavailable, want: "Required state-machine metric families are missing"},
		{name: "disconnected", status: statusDisconnected, want: "has not produced a successful scrape yet"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui := newDashboardUI()
			ui.detailPanel = detailState

			text := ui.renderDetail(Dashboard{
				StateMachine: StateMachinePanel{
					Status: tc.status,
				},
			})

			if !strings.Contains(text, tc.want) {
				t.Fatalf("detail missing %q\n%s", tc.want, text)
			}
			if strings.Contains(text, "Subview:") {
				t.Fatalf("override state should not show subview tabs\n%s", text)
			}
		})
	}
}

func TestRenderDetailCacheSubviewShowsLookupOnly(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailCache
	ui.detailView[detailCache] = subviewCacheLookup

	text := ui.renderDetail(Dashboard{
		CurrentSnapshot: mustParseMetricsForDetailTest(t, `
# TYPE rec53_cache_lookup_total counter
rec53_cache_lookup_total{result="positive_hit"} 100
rec53_cache_lookup_total{result="negative_hit"} 18
rec53_cache_lookup_total{result="miss"} 32
# TYPE rec53_cache_lifecycle_total counter
rec53_cache_lifecycle_total{event="write"} 52
`),
		Cache: CachePanel{
			Status:          statusOK,
			HitRatio:        0.68,
			PositiveHitRate: 20,
			NegativeHitRate: 5,
			DelegationRate:  1,
			MissRate:        8,
			Results: []BreakdownItem{
				{Label: "positive_hit", Rate: 20, Ratio: 0.59},
				{Label: "miss", Rate: 8, Ratio: 0.24},
			},
			Lifecycle: "write 4.0/s",
		},
	})

	for _, want := range []string{
		"Subview:",
		"Lookup mix:",
		"Lookup results:",
		"lookups total",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("cache lookup subview missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "Lifecycle events:") {
		t.Fatalf("cache lookup subview should not include lifecycle totals\n%s", text)
	}
}

func TestRenderDetailDisabledStateDoesNotShowSubviewTabs(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailXDP
	ui.detailView[detailXDP] = subviewXDPPacketPaths

	text := ui.renderDetail(Dashboard{
		XDP: XDPPanel{
			Status: statusDisabled,
			Mode:   "disabled",
		},
	})

	if strings.Contains(text, "Subview:") {
		t.Fatalf("disabled xdp detail should fall back to summary-only state explanation\n%s", text)
	}
	if !strings.Contains(text, "intentionally disabled") {
		t.Fatalf("disabled xdp detail lost state explanation\n%s", text)
	}
}

func TestRenderDetailAddsLightweightTrendCues(t *testing.T) {
	ui := newDashboardUI()
	ui.detailPanel = detailCache
	ui.history = []Dashboard{
		{Cache: CachePanel{HitRatio: 0.20, MissRate: 20}},
		{Cache: CachePanel{HitRatio: 0.35, MissRate: 16}},
		{Cache: CachePanel{HitRatio: 0.50, MissRate: 10}},
		{Cache: CachePanel{HitRatio: 0.65, MissRate: 6}},
	}

	text := ui.renderDetail(Dashboard{
		Cache: CachePanel{
			Status:          statusOK,
			HitRatio:        0.65,
			PositiveHitRate: 22,
			NegativeHitRate: 3,
			DelegationRate:  1,
			MissRate:        6,
			Entries:         128,
			Lifecycle:       "steady",
		},
	})

	for _, want := range []string{
		"Recent trend cues:",
		"recent in-process samples only; use Prometheus/Grafana for long-range history",
		"hit ratio",
		"miss rate",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("trend cues missing %q\n%s", want, text)
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
