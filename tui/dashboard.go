package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type panelStatus string

const (
	statusOK           panelStatus = "OK"
	statusDegraded     panelStatus = "DEGRADED"
	statusDisabled     panelStatus = "DISABLED"
	statusUnavailable  panelStatus = "UNAVAILABLE"
	statusDisconnected panelStatus = "DISCONNECTED"
	statusWarming      panelStatus = "WARMING"
	statusStale        panelStatus = "STALE"
)

type TrafficPanel struct {
	Status        panelStatus
	QPS           float64
	P99MS         float64
	ServfailRatio float64
	NXDomainRatio float64
	NoErrorRatio  float64
	ResponseCodes []BreakdownItem
}

type CachePanel struct {
	Status          panelStatus
	HitRatio        float64
	PositiveHitRate float64
	NegativeHitRate float64
	DelegationRate  float64
	MissRate        float64
	Entries         float64
	Lifecycle       string
	Results         []BreakdownItem
}

type SnapshotPanel struct {
	Status         panelStatus
	LoadSuccess    float64
	LoadFailure    float64
	Imported       float64
	SkippedExpired float64
	SkippedCorrupt float64
	SaveSuccess    float64
	DurationP99MS  float64
}

type UpstreamPanel struct {
	Status         panelStatus
	TimeoutRate    float64
	BadRcodeRate   float64
	FallbackRate   float64
	Winner         string
	WinnerRate     float64
	DominantReason string
	FailureReasons []BreakdownItem
	Winners        []BreakdownItem
}

type XDPPanel struct {
	Status        panelStatus
	Mode          string
	HitRatio      float64
	SyncErrorRate float64
	CleanupRate   float64
	Entries       float64
	PassRate      float64
	ErrorRate     float64
}

type StateMachinePanel struct {
	Status            panelStatus
	TopStage          string
	TopStageRate      float64
	TopFailure        string
	TopFailureRate    float64
	SecondFailure     string
	SecondFailureRate float64
	Stages            []BreakdownItem
	Failures          []BreakdownItem
}

type BreakdownItem struct {
	Label string
	Rate  float64
	Ratio float64
	Total float64
}

type Dashboard struct {
	Target          string
	Mode            panelStatus
	LastUpdate      time.Time
	LastSuccess     time.Time
	ScrapeDuration  time.Duration
	LastError       string
	OverallSummary  string
	CurrentSnapshot *MetricsSnapshot
	Traffic         TrafficPanel
	Cache           CachePanel
	Snapshot        SnapshotPanel
	Upstream        UpstreamPanel
	XDP             XDPPanel
	StateMachine    StateMachinePanel
	HelpExpanded    bool
}

type detailSection struct {
	Title string
	Lines []string
}

type detailModel struct {
	Status               panelStatus
	Standout             string
	CurrentWindowMetrics []string
	CurrentSections      []detailSection
	SinceStartMetrics    []string
	SinceStartSections   []detailSection
	NextChecks           []string
}

func deriveDashboard(target string, prev, curr *MetricsSnapshot, scrapeDuration time.Duration) Dashboard {
	dashboard := Dashboard{
		Target:          target,
		Mode:            statusOK,
		LastUpdate:      time.Now(),
		LastSuccess:     time.Now(),
		ScrapeDuration:  scrapeDuration,
		CurrentSnapshot: curr,
	}

	if curr == nil {
		dashboard.Mode = statusDisconnected
		dashboard.OverallSummary = "target unreachable"
		dashboard.Traffic.Status = statusDisconnected
		dashboard.Cache.Status = statusDisconnected
		dashboard.Snapshot.Status = statusDisconnected
		dashboard.Upstream.Status = statusDisconnected
		dashboard.XDP.Status = statusDisconnected
		dashboard.StateMachine.Status = statusDisconnected
		return dashboard
	}

	dt := 0.0
	if prev != nil {
		dt = curr.At.Sub(prev.At).Seconds()
		if dt <= 0 {
			dt = 0
		}
	}

	dashboard.LastSuccess = curr.At
	dashboard.Traffic = buildTrafficPanel(prev, curr, dt)
	dashboard.Cache = buildCachePanel(prev, curr, dt)
	dashboard.Snapshot = buildSnapshotPanel(prev, curr, dt)
	dashboard.Upstream = buildUpstreamPanel(prev, curr, dt)
	dashboard.XDP = buildXDPPanel(prev, curr, dt)
	dashboard.StateMachine = buildStateMachinePanel(prev, curr, dt)
	dashboard.OverallSummary = buildOverallSummary(dashboard)
	if prev == nil {
		dashboard.Mode = statusWarming
	}
	return dashboard
}

func withScrapeError(prev Dashboard, err error) Dashboard {
	if prev.LastSuccess.IsZero() {
		return Dashboard{
			Target:         prev.Target,
			Mode:           statusDisconnected,
			LastUpdate:     time.Now(),
			LastError:      err.Error(),
			OverallSummary: "DISCONNECTED: " + err.Error(),
			Traffic:        TrafficPanel{Status: statusDisconnected},
			Cache:          CachePanel{Status: statusDisconnected},
			Snapshot:       SnapshotPanel{Status: statusDisconnected},
			Upstream:       UpstreamPanel{Status: statusDisconnected},
			XDP:            XDPPanel{Status: statusDisconnected},
			StateMachine:   StateMachinePanel{Status: statusDisconnected},
		}
	}
	prev.Mode = statusStale
	prev.LastUpdate = time.Now()
	prev.LastError = err.Error()
	prev.OverallSummary = "STALE: " + err.Error()
	return prev
}

func buildTrafficPanel(prev, curr *MetricsSnapshot, dt float64) TrafficPanel {
	if !curr.hasMetric("rec53_query_counter") || !curr.hasMetric("rec53_response_counter") || !curr.hasMetric("rec53_latency") {
		return TrafficPanel{Status: statusUnavailable}
	}
	panel := TrafficPanel{Status: statusWarming}
	if prev == nil || dt == 0 {
		return panel
	}

	queryCurr, ok := curr.sum("rec53_query_counter")
	if queryPrev, okPrev := prev.sum("rec53_query_counter"); ok && okPrev {
		panel.QPS = deltaFloat(queryCurr, queryPrev) / dt
	}

	respCurr, ok := curr.sumByLabel("rec53_response_counter", "code")
	if ok {
		respPrev, _ := prev.sumByLabel("rec53_response_counter", "code")
		respDelta := deltaMap(respCurr, respPrev)
		panel.ResponseCodes = buildBreakdown(respDelta, dt, 4)
		totalResponses := 0.0
		for _, value := range respDelta {
			totalResponses += value
		}
		if totalResponses > 0 {
			panel.ServfailRatio = respDelta["SERVFAIL"] / totalResponses
			panel.NXDomainRatio = respDelta["NXDOMAIN"] / totalResponses
			panel.NoErrorRatio = respDelta["NOERROR"] / totalResponses
		}
	}

	if currBuckets, ok := curr.histogramBuckets("rec53_latency"); ok {
		prevBuckets, _ := prev.histogramBuckets("rec53_latency")
		if p99, ok := histogramQuantile(0.99, deltaBuckets(currBuckets, prevBuckets)); ok {
			panel.P99MS = p99
		}
	}

	panel.Status = statusOK
	if panel.ServfailRatio >= 0.05 || panel.P99MS >= 1000 {
		panel.Status = statusDegraded
	}
	return panel
}

func buildCachePanel(prev, curr *MetricsSnapshot, dt float64) CachePanel {
	if !curr.hasMetric("rec53_cache_lookup_total") || !curr.hasMetric("rec53_cache_entries") {
		return CachePanel{Status: statusUnavailable}
	}
	panel := CachePanel{Status: statusWarming}
	if entries, ok := curr.gaugeValue("rec53_cache_entries"); ok {
		panel.Entries = entries
	}
	if prev == nil || dt == 0 {
		return panel
	}

	resultsCurr, ok := curr.sumByLabel("rec53_cache_lookup_total", "result")
	if ok {
		resultsPrev, _ := prev.sumByLabel("rec53_cache_lookup_total", "result")
		delta := deltaMap(resultsCurr, resultsPrev)
		panel.Results = buildBreakdown(delta, dt, 4)
		panel.PositiveHitRate = delta["positive_hit"] / dt
		panel.NegativeHitRate = delta["negative_hit"] / dt
		panel.DelegationRate = delta["delegation_hit"] / dt
		panel.MissRate = delta["miss"] / dt
		total := 0.0
		for _, value := range delta {
			total += value
		}
		if total > 0 {
			hits := delta["positive_hit"] + delta["negative_hit"] + delta["delegation_hit"]
			panel.HitRatio = hits / total
		}
	}

	if eventsCurr, ok := curr.sumByLabel("rec53_cache_lifecycle_total", "event"); ok {
		eventsPrev, _ := prev.sumByLabel("rec53_cache_lifecycle_total", "event")
		top, value := pickTopLabel(deltaMap(eventsCurr, eventsPrev))
		if top != "" && value > 0 {
			panel.Lifecycle = fmt.Sprintf("%s %.1f/s", top, value/dt)
		}
	}
	if panel.Lifecycle == "" {
		panel.Lifecycle = "steady"
	}

	panel.Status = statusOK
	if panel.MissRate > 0 && panel.HitRatio < 0.50 {
		panel.Status = statusDegraded
	}
	return panel
}

func buildSnapshotPanel(prev, curr *MetricsSnapshot, dt float64) SnapshotPanel {
	if !curr.hasMetric("rec53_snapshot_operations_total") || !curr.hasMetric("rec53_snapshot_entries_total") {
		return SnapshotPanel{Status: statusUnavailable}
	}
	panel := SnapshotPanel{Status: statusWarming}
	if operations, ok := curr.Counters["rec53_snapshot_operations_total"]; ok {
		for _, sample := range operations {
			switch sample.Labels["operation"] {
			case "load":
				switch sample.Labels["result"] {
				case "success":
					panel.LoadSuccess += sample.Value
				case "failure":
					panel.LoadFailure += sample.Value
				}
			case "save":
				if sample.Labels["result"] == "success" {
					panel.SaveSuccess += sample.Value
				}
			}
		}
	}
	if entries, ok := curr.sumByLabel("rec53_snapshot_entries_total", "result"); ok {
		panel.Imported = entries["imported"]
		panel.SkippedExpired = entries["skipped_expired"]
		panel.SkippedCorrupt = entries["skipped_corrupt"]
		if panel.SaveSuccess == 0 {
			panel.SaveSuccess = entries["saved"]
		}
	}
	if buckets, ok := curr.histogramBuckets("rec53_snapshot_duration_ms"); ok {
		if p99, ok := histogramQuantile(0.99, buckets); ok {
			panel.DurationP99MS = p99
		}
	}
	if prev == nil || dt == 0 {
		return panel
	}

	panel.Status = statusOK
	if operationsCurr, ok := curr.Counters["rec53_snapshot_operations_total"]; ok {
		failures := 0.0
		previousBySignature := make(map[string]float64)
		for _, sample := range prev.Counters["rec53_snapshot_operations_total"] {
			previousBySignature[labelSignature(sample.Labels)] = sample.Value
		}
		for _, sample := range operationsCurr {
			if sample.Labels["result"] != "failure" {
				continue
			}
			failures += deltaFloat(sample.Value, previousBySignature[labelSignature(sample.Labels)])
		}
		if failures > 0 {
			panel.Status = statusDegraded
		}
	}
	return panel
}

func buildUpstreamPanel(prev, curr *MetricsSnapshot, dt float64) UpstreamPanel {
	if !curr.hasMetric("rec53_upstream_failures_total") || !curr.hasMetric("rec53_upstream_fallback_total") || !curr.hasMetric("rec53_upstream_winner_total") {
		return UpstreamPanel{Status: statusUnavailable}
	}
	panel := UpstreamPanel{Status: statusWarming}
	if prev == nil || dt == 0 {
		return panel
	}

	failuresCurr, ok := curr.sumByLabel("rec53_upstream_failures_total", "reason")
	if ok {
		failuresPrev, _ := prev.sumByLabel("rec53_upstream_failures_total", "reason")
		delta := deltaMap(failuresCurr, failuresPrev)
		panel.FailureReasons = buildBreakdown(delta, dt, 4)
		panel.TimeoutRate = delta["timeout"] / dt
		panel.BadRcodeRate = delta["bad_rcode"] / dt
		if len(panel.FailureReasons) > 0 {
			panel.DominantReason = panel.FailureReasons[0].Label
		}
	}

	fallbackCurr, ok := curr.sum("rec53_upstream_fallback_total")
	if ok {
		fallbackPrev, _ := prev.sum("rec53_upstream_fallback_total")
		panel.FallbackRate = deltaFloat(fallbackCurr, fallbackPrev) / dt
	}

	if winnersCurr, ok := curr.sumByLabel("rec53_upstream_winner_total", "path"); ok {
		winnersPrev, _ := prev.sumByLabel("rec53_upstream_winner_total", "path")
		delta := deltaMap(winnersCurr, winnersPrev)
		panel.Winners = buildBreakdown(delta, dt, 3)
		if len(panel.Winners) > 0 {
			panel.Winner = panel.Winners[0].Label
			panel.WinnerRate = panel.Winners[0].Rate
		}
	}

	panel.Status = statusOK
	if panel.TimeoutRate > 0 || panel.BadRcodeRate > 0 || panel.FallbackRate > 0 {
		panel.Status = statusDegraded
	}
	return panel
}

func buildXDPPanel(prev, curr *MetricsSnapshot, dt float64) XDPPanel {
	panel := XDPPanel{Status: statusUnavailable, Mode: "unavailable"}
	status, ok := curr.gaugeValue("rec53_xdp_status")
	if !ok {
		return panel
	}

	if status < 1 {
		panel.Status = statusDisabled
		panel.Mode = "disabled"
		return panel
	}

	panel.Status = statusWarming
	panel.Mode = "active"
	panel.Entries, _ = curr.gaugeValue("rec53_xdp_entries")

	if prev == nil || dt == 0 {
		return panel
	}

	hitsCurr, hitsOK := curr.gaugeValue("rec53_xdp_cache_hits_total")
	hitsPrev, _ := prev.gaugeValue("rec53_xdp_cache_hits_total")
	missesCurr, missesOK := curr.gaugeValue("rec53_xdp_cache_misses_total")
	missesPrev, _ := prev.gaugeValue("rec53_xdp_cache_misses_total")
	passCurr, passOK := curr.gaugeValue("rec53_xdp_pass_total")
	passPrev, _ := prev.gaugeValue("rec53_xdp_pass_total")
	errorCurr, errorOK := curr.gaugeValue("rec53_xdp_errors_total")
	errorPrev, _ := prev.gaugeValue("rec53_xdp_errors_total")
	if hitsOK && missesOK {
		hitDelta := deltaFloat(hitsCurr, hitsPrev)
		missDelta := deltaFloat(missesCurr, missesPrev)
		total := hitDelta + missDelta
		if total > 0 {
			panel.HitRatio = hitDelta / total
		}
	}
	if passOK {
		panel.PassRate = deltaFloat(passCurr, passPrev) / dt
	}
	if errorOK {
		panel.ErrorRate = deltaFloat(errorCurr, errorPrev) / dt
	}
	if syncCurr, ok := curr.sum("rec53_xdp_cache_sync_errors_total"); ok {
		syncPrev, _ := prev.sum("rec53_xdp_cache_sync_errors_total")
		panel.SyncErrorRate = deltaFloat(syncCurr, syncPrev) / dt
	}
	if cleanupCurr, ok := curr.sum("rec53_xdp_cleanup_deleted_total"); ok {
		cleanupPrev, _ := prev.sum("rec53_xdp_cleanup_deleted_total")
		panel.CleanupRate = deltaFloat(cleanupCurr, cleanupPrev) / dt
	}

	panel.Status = statusOK
	if panel.SyncErrorRate > 0 || panel.ErrorRate > 0 {
		panel.Status = statusDegraded
	}
	return panel
}

func buildStateMachinePanel(prev, curr *MetricsSnapshot, dt float64) StateMachinePanel {
	if !curr.hasMetric("rec53_state_machine_stage_total") || !curr.hasMetric("rec53_state_machine_failures_total") {
		return StateMachinePanel{Status: statusUnavailable}
	}
	panel := StateMachinePanel{Status: statusWarming}
	if prev == nil || dt == 0 {
		return panel
	}

	stagesCurr, ok := curr.sumByLabel("rec53_state_machine_stage_total", "stage")
	if ok {
		stagesPrev, _ := prev.sumByLabel("rec53_state_machine_stage_total", "stage")
		delta := deltaMap(stagesCurr, stagesPrev)
		panel.Stages = buildBreakdown(delta, dt, 5)
		if len(panel.Stages) > 0 {
			panel.TopStage = panel.Stages[0].Label
			panel.TopStageRate = panel.Stages[0].Rate
		}
	}

	failuresCurr, ok := curr.sumByLabel("rec53_state_machine_failures_total", "reason")
	if ok {
		failuresPrev, _ := prev.sumByLabel("rec53_state_machine_failures_total", "reason")
		panel.Failures = buildBreakdown(deltaMap(failuresCurr, failuresPrev), dt, 5)
		if len(panel.Failures) > 0 {
			panel.TopFailure = panel.Failures[0].Label
			panel.TopFailureRate = panel.Failures[0].Rate
		}
		if len(panel.Failures) > 1 {
			panel.SecondFailure = panel.Failures[1].Label
			panel.SecondFailureRate = panel.Failures[1].Rate
		}
	}

	panel.Status = statusOK
	if panel.TopFailureRate > 0 {
		panel.Status = statusDegraded
	}
	return panel
}

func buildOverallSummary(d Dashboard) string {
	parts := make([]string, 0, 6)
	appendPart := func(name string, status panelStatus) {
		parts = append(parts, fmt.Sprintf("%s %s", name, status))
	}
	appendPart("TRAFFIC", d.Traffic.Status)
	appendPart("CACHE", d.Cache.Status)
	appendPart("SNAPSHOT", d.Snapshot.Status)
	appendPart("UPSTREAM", d.Upstream.Status)
	appendPart("XDP", d.XDP.Status)
	appendPart("SM", d.StateMachine.Status)
	return strings.Join(parts, " | ")
}

func buildTrafficDetailModel(d Dashboard) detailModel {
	panel := d.Traffic
	model := detailModel{
		Status: panel.Status,
		CurrentWindowMetrics: []string{
			detailMetricLine("qps", number(panel.QPS)),
			detailMetricLine("p99 latency", latency(panel.P99MS)),
			detailMetricLine("servfail ratio", percent(panel.ServfailRatio)),
			detailMetricLine("nxdomain ratio", percent(panel.NXDomainRatio)),
			detailMetricLine("noerror ratio", percent(panel.NoErrorRatio)),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Response mix:", panel.ResponseCodes),
		},
		SinceStartMetrics:  buildTrafficSinceStartMetrics(d.CurrentSnapshot),
		SinceStartSections: buildTrafficSinceStartSections(d.CurrentSnapshot),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required traffic metric families are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}

	topLabel := topBreakdownLabel(panel.ResponseCodes)
	switch panel.Status {
	case statusDegraded:
		switch {
		case panel.ServfailRatio >= 0.05:
			model.Standout = fmt.Sprintf("SERVFAIL is elevated at %s, so recent request quality is currently defined by failed answers more than by throughput.", percent(panel.ServfailRatio))
		case panel.P99MS >= 1000:
			model.Standout = fmt.Sprintf("Tail latency is high at %s while traffic is still flowing, so users are likely feeling slow recursive paths before they see hard failures.", latency(panel.P99MS))
		case topLabel != "":
			model.Standout = fmt.Sprintf("%s currently leads the response mix, and the traffic panel needs a closer look for answer quality rather than raw volume.", topLabel)
		default:
			model.Standout = "Traffic is degraded even though the top response bucket is not yet dominant enough to explain the whole picture."
		}
		model.NextChecks = []string{
			"See 4 Upstream for timeout or bad-rcode growth.",
			"See 6 State Machine for concentrated failure reasons.",
			"Use rec53 logs if SERVFAIL keeps rising and the breakdown stays mixed.",
		}
	default:
		if topLabel != "" {
			model.Standout = fmt.Sprintf("%s is the dominant recent response bucket, so the current traffic shape still looks readable from a response-quality perspective.", topLabel)
		} else {
			model.Standout = "Traffic is currently healthy, but there is not enough recent response mix to elevate one code as the clear leader."
		}
		model.NextChecks = []string{
			"Watch for SERVFAIL or P99 growth before treating traffic as degraded.",
			"See 2 Cache if latency rises while traffic volume stays steady.",
		}
	}

	return model
}

func buildCacheDetailModel(d Dashboard) detailModel {
	panel := d.Cache
	model := detailModel{
		Status: panel.Status,
		CurrentWindowMetrics: []string{
			detailMetricLine("hit ratio", percent(panel.HitRatio)),
			detailMetricLine("positive hit", rate(panel.PositiveHitRate)),
			detailMetricLine("negative hit", rate(panel.NegativeHitRate)),
			detailMetricLine("delegation hit", rate(panel.DelegationRate)),
			detailMetricLine("miss", rate(panel.MissRate)),
			detailMetricLine("entries", number(panel.Entries)),
			detailMetricLine("lifecycle", panel.Lifecycle),
		},
		SinceStartMetrics: buildCacheSinceStartMetrics(d.CurrentSnapshot),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required cache metric families are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}

	totalHitRate := panel.PositiveHitRate + panel.NegativeHitRate + panel.DelegationRate
	topLabel := topBreakdownLabel(panel.Results)
	switch panel.Status {
	case statusDegraded:
		if panel.MissRate > totalHitRate {
			model.Standout = fmt.Sprintf("Cache misses are currently outrunning cache-served answers at %s, and the overall hit ratio has fallen to %s.", rate(panel.MissRate), percent(panel.HitRatio))
		} else {
			model.Standout = fmt.Sprintf("Cache effectiveness is slipping: hit ratio is %s and miss pressure is visible even though hits still exist.", percent(panel.HitRatio))
		}
		model.NextChecks = []string{
			"See 1 Traffic if latency or response quality dropped with miss growth.",
			"See 4 Upstream when misses are forcing more iterative work.",
			"See 6 State Machine if one failure reason is clustering around misses.",
		}
	default:
		if topLabel != "" {
			model.Standout = fmt.Sprintf("%s is the leading recent cache outcome, and lifecycle activity currently looks %s.", topLabel, panel.Lifecycle)
		} else {
			model.Standout = "Cache is currently healthy, but there are not enough recent lookups to promote one result class as the standout signal."
		}
		model.NextChecks = []string{
			"Watch miss rate during traffic shifts or cold-name bursts.",
			"Compare with 1 Traffic if latency rises without obvious cache regression.",
		}
	}

	return model
}

func buildCacheLookupSubviewModel(d Dashboard) detailModel {
	panel := d.Cache
	model := detailModel{
		Status:   panel.Status,
		Standout: "Cache lookup mix isolates which result classes are currently dominating and how that compares with cumulative lookup totals.",
		CurrentWindowMetrics: []string{
			detailMetricLine("hit ratio", percent(panel.HitRatio)),
			detailMetricLine("positive hit", rate(panel.PositiveHitRate)),
			detailMetricLine("negative hit", rate(panel.NegativeHitRate)),
			detailMetricLine("delegation hit", rate(panel.DelegationRate)),
			detailMetricLine("miss", rate(panel.MissRate)),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Lookup mix:", panel.Results),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_cache_lookup_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("lookups total", count(total)))
		}
		if values, ok := d.CurrentSnapshot.sumByLabel("rec53_cache_lookup_total", "result"); ok {
			model.SinceStartSections = []detailSection{detailTotalBreakdownSection("Lookup results:", buildTotalBreakdown(values, 4))}
		}
	}
	model.NextChecks = []string{
		"Return to Summary when you want the overall cache verdict and next checks.",
		"Switch to Lifecycle if lookup pressure looks normal but cache contents are still churning.",
	}
	return model
}

func buildCacheLifecycleSubviewModel(d Dashboard) detailModel {
	panel := d.Cache
	model := detailModel{
		Status:   panel.Status,
		Standout: "Cache lifecycle activity shows whether writes, refreshes, or evictions are driving churn behind the summary hit ratio.",
		CurrentWindowMetrics: []string{
			detailMetricLine("entries", number(panel.Entries)),
			detailMetricLine("lifecycle", panel.Lifecycle),
		},
	}
	if d.CurrentSnapshot != nil {
		if values, ok := d.CurrentSnapshot.sumByLabel("rec53_cache_lifecycle_total", "event"); ok {
			model.SinceStartSections = []detailSection{detailTotalBreakdownSection("Lifecycle events:", buildTotalBreakdown(values, 4))}
		}
	}
	model.NextChecks = []string{
		"Compare with Lookup Mix if misses are climbing but lifecycle writes are not.",
		"Use Summary to return to the higher-level cache diagnosis before pivoting to another panel.",
	}
	return model
}

func buildSnapshotDetailModel(d Dashboard) detailModel {
	panel := d.Snapshot
	model := detailModel{
		Status: panel.Status,
		SinceStartMetrics: []string{
			detailMetricLine("load success", number(panel.LoadSuccess)),
			detailMetricLine("load failure", number(panel.LoadFailure)),
			detailMetricLine("imported", number(panel.Imported)),
			detailMetricLine("skipped expired", number(panel.SkippedExpired)),
			detailMetricLine("skipped corrupt", number(panel.SkippedCorrupt)),
			detailMetricLine("saved", number(panel.SaveSuccess)),
			detailMetricLine("duration p99", latency(panel.DurationP99MS)),
		},
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required snapshot metric families are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}

	if panel.Status == statusDegraded || panel.LoadFailure > 0 {
		model.Standout = fmt.Sprintf("Snapshot activity has seen failures, so restart and shutdown paths need attention before treating snapshot restore as trustworthy.")
		model.NextChecks = []string{
			"Check snapshot file integrity and related log lines around restart or shutdown.",
			"Compare imported, skipped-expired, and skipped-corrupt counts before relying on restore quality.",
		}
		return model
	}

	if panel.Imported > 0 {
		model.Standout = fmt.Sprintf("Recent snapshot history looks calm: imported entries are visible and duration remains at %s p99.", latency(panel.DurationP99MS))
	} else {
		model.Standout = "Snapshot metrics are available, but there is little recent restore or save activity to diagnose beyond current health."
	}
	model.NextChecks = []string{
		"Revisit this panel around restart and shutdown events rather than steady-state traffic.",
		"Use logs if imported counts look unexpectedly small for a known snapshot file.",
	}
	return model
}

func buildUpstreamDetailModel(d Dashboard) detailModel {
	panel := d.Upstream
	model := detailModel{
		Status: panel.Status,
		CurrentWindowMetrics: []string{
			detailMetricLine("timeout", rate(panel.TimeoutRate)),
			detailMetricLine("bad rcode", rate(panel.BadRcodeRate)),
			detailMetricLine("fallback", rate(panel.FallbackRate)),
			detailMetricLine("winner", fmt.Sprintf("%s %s", fallbackText(panel.Winner), rate(panel.WinnerRate))),
			detailMetricLine("dominant reason", fallbackText(panel.DominantReason)),
		},
		SinceStartMetrics: buildUpstreamSinceStartMetrics(d.CurrentSnapshot),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required upstream metric families are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}

	switch panel.Status {
	case statusDegraded:
		switch {
		case panel.DominantReason != "":
			model.Standout = fmt.Sprintf("%s is the dominant recent upstream failure reason, so transport or answer-quality instability is currently upstream-led.", panel.DominantReason)
		case panel.FallbackRate > 0:
			model.Standout = fmt.Sprintf("Fallbacks are active at %s, so the first upstream path is not consistently winning on its own.", rate(panel.FallbackRate))
		default:
			model.Standout = "Upstream is degraded even though no single failure reason cleanly dominates yet."
		}
		model.NextChecks = []string{
			"Check network reachability and upstream responsiveness first.",
			"Compare with 1 Traffic for SERVFAIL or latency growth.",
			"See 6 State Machine if query-upstream failures are clustering.",
		}
	default:
		if panel.Winner != "" {
			model.Standout = fmt.Sprintf("%s is currently winning the upstream race most often, and no failure path is standing out as a dominant risk.", panel.Winner)
		} else {
			model.Standout = "Upstream looks healthy, but there are not enough recent winner or failure samples to elevate one path as the clear leader."
		}
		model.NextChecks = []string{
			"Watch timeout and fallback growth before treating upstream as the root cause.",
			"Use winner mix to judge whether primary or secondary paths are shifting over time.",
		}
	}

	return model
}

func buildUpstreamFailuresSubviewModel(d Dashboard) detailModel {
	panel := d.Upstream
	model := detailModel{
		Status:   panel.Status,
		Standout: "Upstream failures isolates the recent error mix so timeout, bad-rcode, and fallback pressure are not hidden behind the overall summary.",
		CurrentWindowMetrics: []string{
			detailMetricLine("timeout", rate(panel.TimeoutRate)),
			detailMetricLine("bad rcode", rate(panel.BadRcodeRate)),
			detailMetricLine("fallback", rate(panel.FallbackRate)),
			detailMetricLine("dominant reason", fallbackText(panel.DominantReason)),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Failure reasons:", panel.FailureReasons),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_upstream_failures_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("failures total", count(total)))
		}
		if total, ok := d.CurrentSnapshot.sum("rec53_upstream_fallback_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("fallback total", count(total)))
		}
		if values, ok := d.CurrentSnapshot.sumByLabel("rec53_upstream_failures_total", "reason"); ok {
			model.SinceStartSections = []detailSection{detailTotalBreakdownSection("Failure reasons:", buildTotalBreakdown(values, 4))}
		}
	}
	model.NextChecks = []string{
		"Switch to Winners if path selection changed without a single dominant failure reason.",
		"Return to Summary before deciding whether upstream is the root cause or only a symptom.",
	}
	return model
}

func buildUpstreamWinnersSubviewModel(d Dashboard) detailModel {
	panel := d.Upstream
	model := detailModel{
		Status:   panel.Status,
		Standout: "Winner paths isolates which upstream path is actually winning, which is useful when failures exist but one route still dominates overall behavior.",
		CurrentWindowMetrics: []string{
			detailMetricLine("winner", fmt.Sprintf("%s %s", fallbackText(panel.Winner), rate(panel.WinnerRate))),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Winner mix:", panel.Winners),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_upstream_winner_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("winner total", count(total)))
		}
		if values, ok := d.CurrentSnapshot.sumByLabel("rec53_upstream_winner_total", "path"); ok {
			model.SinceStartSections = []detailSection{detailTotalBreakdownSection("Winner paths:", buildTotalBreakdown(values, 3))}
		}
	}
	model.NextChecks = []string{
		"Switch to Failures if path churn seems to be driven by timeout or bad-rcode pressure.",
		"Use Summary to compare the winner picture with the overall upstream verdict.",
	}
	return model
}

func buildXDPDetailModel(d Dashboard) detailModel {
	panel := d.XDP
	model := detailModel{
		Status: panel.Status,
		CurrentWindowMetrics: []string{
			detailMetricLine("mode", panel.Mode),
			detailMetricLine("hit ratio", percent(panel.HitRatio)),
			detailMetricLine("sync errors", rate(panel.SyncErrorRate)),
			detailMetricLine("cleanup", rate(panel.CleanupRate)),
			detailMetricLine("entries", number(panel.Entries)),
			detailMetricLine("pass", rate(panel.PassRate)),
			detailMetricLine("errors", rate(panel.ErrorRate)),
		},
		SinceStartMetrics: buildXDPSinceStartMetrics(d.CurrentSnapshot),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "XDP is intentionally disabled or unsupported for this deployment, so there is no fast-path activity to diagnose here.", "Required XDP metrics are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}

	if panel.Status == statusDegraded {
		model.Standout = fmt.Sprintf("The XDP fast path is active, but sync or packet-path errors are non-zero, so correctness pressure matters more than raw hit ratio right now.")
		model.NextChecks = []string{
			"Compare XDP hit ratio with 2 Cache to see whether fast-path behavior matches the Go-path cache.",
			"Check sync-error and error counters before trusting fast-path wins.",
		}
		return model
	}

	model.Standout = fmt.Sprintf("The XDP fast path is active and currently stable, with hit ratio %s and pass rate %s.", percent(panel.HitRatio), rate(panel.PassRate))
	model.NextChecks = []string{
		"Watch sync-error growth before treating XDP as clean under load.",
		"Compare with 2 Cache if fast-path hit ratio looks unexpectedly low.",
	}
	return model
}

func buildStateMachineDetailModel(d Dashboard) detailModel {
	panel := d.StateMachine
	model := detailModel{
		Status: panel.Status,
		CurrentWindowMetrics: []string{
			detailMetricLine("top stage", fmt.Sprintf("%s %s", fallbackText(panel.TopStage), rate(panel.TopStageRate))),
			detailMetricLine("fail top 1", fmt.Sprintf("%s %s", fallbackText(panel.TopFailure), rate(panel.TopFailureRate))),
			detailMetricLine("fail top 2", fmt.Sprintf("%s %s", fallbackText(panel.SecondFailure), rate(panel.SecondFailureRate))),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Stage mix:", panel.Stages),
			detailRateBreakdownSection("Failure reasons:", panel.Failures),
		},
		SinceStartMetrics:  buildStateMachineSinceStartMetrics(d.CurrentSnapshot),
		SinceStartSections: buildStateMachineSinceStartSections(d.CurrentSnapshot),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required state-machine metric families are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}

	switch panel.Status {
	case statusDegraded:
		if panel.TopFailure != "" {
			model.Standout = fmt.Sprintf("%s is the top recent state-machine failure, while %s remains the busiest stage. This is where the resolver flow is currently concentrating its pain.", panel.TopFailure, fallbackText(panel.TopStage))
		} else {
			model.Standout = fmt.Sprintf("%s is the busiest stage, but the current failure mix still needs more samples to identify a single dominant reason.", fallbackText(panel.TopStage))
		}
		model.NextChecks = stateMachineNextChecks(panel.TopFailure)
	default:
		if panel.TopStage != "" {
			model.Standout = fmt.Sprintf("%s is currently the busiest state-machine stage, and there is no active failure leader competing with it.", panel.TopStage)
		} else {
			model.Standout = "State-machine metrics are healthy, but the current sample window is too quiet to elevate one stage as the clear standout."
		}
		model.NextChecks = []string{
			"Watch this panel when traffic is failing but neither cache nor upstream alone explains it.",
			"Use failure reasons as a bounded summary, then pivot to logs for request-level detail.",
		}
	}

	return model
}

func buildXDPPacketPathsSubviewModel(d Dashboard) detailModel {
	panel := d.XDP
	model := detailModel{
		Status:   panel.Status,
		Standout: "Packet paths isolates fast-path wins versus pass-through and error pressure, which helps judge whether XDP is paying for itself right now.",
		CurrentWindowMetrics: []string{
			detailMetricLine("hit ratio", percent(panel.HitRatio)),
			detailMetricLine("pass", rate(panel.PassRate)),
			detailMetricLine("errors", rate(panel.ErrorRate)),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_xdp_cache_hits_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("hits total", count(total)))
		}
		if total, ok := d.CurrentSnapshot.sum("rec53_xdp_cache_misses_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("misses total", count(total)))
		}
		if total, ok := d.CurrentSnapshot.sum("rec53_xdp_pass_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("pass total", count(total)))
		}
		if total, ok := d.CurrentSnapshot.sum("rec53_xdp_errors_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("error total", count(total)))
		}
	}
	model.NextChecks = []string{
		"Switch to Sync/Cleanup if packet-path ratios look fine but correctness counters are still growing.",
		"Return to Summary before comparing XDP behavior with the Go-path cache.",
	}
	return model
}

func buildXDPSyncCleanupSubviewModel(d Dashboard) detailModel {
	panel := d.XDP
	model := detailModel{
		Status:   panel.Status,
		Standout: "Sync and cleanup isolates whether XDP maintenance overhead is staying quiet or quietly eroding fast-path confidence.",
		CurrentWindowMetrics: []string{
			detailMetricLine("mode", panel.Mode),
			detailMetricLine("sync errors", rate(panel.SyncErrorRate)),
			detailMetricLine("cleanup", rate(panel.CleanupRate)),
			detailMetricLine("entries", number(panel.Entries)),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_xdp_cache_sync_errors_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("sync error total", count(total)))
		}
		if total, ok := d.CurrentSnapshot.sum("rec53_xdp_cleanup_deleted_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("cleanup total", count(total)))
		}
	}
	model.NextChecks = []string{
		"Switch to Packet Paths if maintenance looks calm but fast-path hit ratio is still lower than expected.",
		"Use Summary when you want the top-line XDP verdict before pivoting away.",
	}
	return model
}

func detailStateOverride(status panelStatus, lastError, disabledMsg, unavailableMsg string) (string, []string, bool) {
	switch status {
	case statusWarming:
		return "Only one successful scrape is available, so short-window rates and ratios are not stable yet.", []string{
			"Wait for the next refresh or press r to collect another successful sample.",
			"Use the header timestamps to confirm when live short-window interpretation becomes meaningful.",
		}, true
	case statusUnavailable:
		if unavailableMsg == "" {
			unavailableMsg = "Required metric families for this panel are missing from the current scrape."
		}
		return unavailableMsg, []string{
			"Verify that the target rec53 build exposes the expected metric families.",
			"Inspect the raw /metric output if this capability should exist on this node.",
		}, true
	case statusDisabled:
		if disabledMsg == "" {
			disabledMsg = "This panel is intentionally disabled for the current deployment, so there is no live signal to interpret here."
		}
		return disabledMsg, []string{
			"No action is required unless this feature is expected to be enabled.",
			"Compare with the current configuration before treating this state as a fault.",
		}, true
	case statusDisconnected:
		standout := "The target has not produced a successful scrape yet, so this panel is not showing live rec53 behavior."
		if lastError != "" {
			standout = "The target is disconnected from rec53top, so there is no fresh live-state interpretation. Last error: " + lastError
		}
		return standout, []string{
			"Verify the metrics endpoint is reachable and the target address is correct.",
			"Retry with r after connectivity returns or use -plain/curl to confirm scrape health.",
		}, true
	case statusStale:
		standout := "This panel is showing stale data because the latest scrape failed, so live interpretation is temporarily frozen."
		if lastError != "" {
			standout += " Last error: " + lastError
		}
		return standout, []string{
			"Treat current numbers as old until the next successful scrape lands.",
			"Check scrape connectivity first before following normal panel guidance.",
		}, true
	default:
		return "", nil, false
	}
}

func stateMachineNextChecks(topFailure string) []string {
	checks := []string{
		"Use rec53 logs for exact per-request state transitions once a failure reason stands out.",
	}
	switch {
	case strings.Contains(topFailure, "upstream"):
		checks = append([]string{
			"See 4 Upstream because the dominant state-machine failure is upstream-related.",
		}, checks...)
	case strings.Contains(topFailure, "cache"), strings.Contains(topFailure, "glue"):
		checks = append([]string{
			"See 2 Cache because the dominant state-machine failure is cache or referral related.",
		}, checks...)
	default:
		checks = append([]string{
			"Correlate this failure mix with 1 Traffic and 4 Upstream before assuming a single root cause.",
		}, checks...)
	}
	return checks
}

func topBreakdownLabel(items []BreakdownItem) string {
	if len(items) == 0 {
		return ""
	}
	return items[0].Label
}

func detailMetricLine(label, value string) string {
	return fmt.Sprintf("  %-16s %s", label, value)
}

func detailRateBreakdownSection(title string, items []BreakdownItem) detailSection {
	lines := []string{"  no recent samples"}
	if len(items) > 0 {
		lines = make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("  %-16s %8s  %6s", item.Label, rate(item.Rate), percent(item.Ratio)))
		}
	}
	return detailSection{Title: title, Lines: lines}
}

func detailTotalBreakdownSection(title string, items []BreakdownItem) detailSection {
	lines := []string{"  no cumulative samples"}
	if len(items) > 0 {
		lines = make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("  %-16s %8s  %6s", item.Label, count(item.Total), percent(item.Ratio)))
		}
	}
	return detailSection{Title: title, Lines: lines}
}

func buildBreakdown(delta map[string]float64, dt float64, limit int) []BreakdownItem {
	if len(delta) == 0 || dt <= 0 || limit <= 0 {
		return nil
	}
	type entry struct {
		key   string
		value float64
	}
	entries := make([]entry, 0, len(delta))
	total := 0.0
	for key, value := range delta {
		if value <= 0 {
			continue
		}
		entries = append(entries, entry{key: key, value: value})
		total += value
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value == entries[j].value {
			return entries[i].key < entries[j].key
		}
		return entries[i].value > entries[j].value
	})
	if limit > len(entries) {
		limit = len(entries)
	}
	items := make([]BreakdownItem, 0, limit)
	for _, entry := range entries[:limit] {
		item := BreakdownItem{
			Label: entry.key,
			Rate:  entry.value / dt,
		}
		if total > 0 {
			item.Ratio = entry.value / total
		}
		items = append(items, item)
	}
	return items
}

func buildTotalBreakdown(values map[string]float64, limit int) []BreakdownItem {
	if len(values) == 0 || limit <= 0 {
		return nil
	}
	type entry struct {
		key   string
		value float64
	}
	entries := make([]entry, 0, len(values))
	total := 0.0
	for key, value := range values {
		if value <= 0 {
			continue
		}
		entries = append(entries, entry{key: key, value: value})
		total += value
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value == entries[j].value {
			return entries[i].key < entries[j].key
		}
		return entries[i].value > entries[j].value
	})
	if limit > len(entries) {
		limit = len(entries)
	}
	items := make([]BreakdownItem, 0, limit)
	for _, entry := range entries[:limit] {
		item := BreakdownItem{
			Label: entry.key,
			Total: entry.value,
		}
		if total > 0 {
			item.Ratio = entry.value / total
		}
		items = append(items, item)
	}
	return items
}

func buildTrafficSinceStartMetrics(snapshot *MetricsSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	lines := make([]string, 0, 2)
	if total, ok := snapshot.sum("rec53_query_counter"); ok {
		lines = append(lines, detailMetricLine("queries total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_response_counter"); ok {
		lines = append(lines, detailMetricLine("responses total", count(total)))
	}
	return lines
}

func buildTrafficSinceStartSections(snapshot *MetricsSnapshot) []detailSection {
	if snapshot == nil {
		return nil
	}
	if values, ok := snapshot.sumByLabel("rec53_response_counter", "code"); ok {
		return []detailSection{detailTotalBreakdownSection("Response codes:", buildTotalBreakdown(values, 4))}
	}
	return nil
}

func buildCacheSinceStartMetrics(snapshot *MetricsSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	lines := make([]string, 0, 1)
	if total, ok := snapshot.sum("rec53_cache_lookup_total"); ok {
		lines = append(lines, detailMetricLine("lookups total", count(total)))
	}
	return lines
}

func buildCacheSinceStartSections(snapshot *MetricsSnapshot) []detailSection {
	if snapshot == nil {
		return nil
	}
	sections := make([]detailSection, 0, 2)
	if values, ok := snapshot.sumByLabel("rec53_cache_lookup_total", "result"); ok {
		sections = append(sections, detailTotalBreakdownSection("Lookup results:", buildTotalBreakdown(values, 4)))
	}
	if values, ok := snapshot.sumByLabel("rec53_cache_lifecycle_total", "event"); ok {
		sections = append(sections, detailTotalBreakdownSection("Lifecycle events:", buildTotalBreakdown(values, 4)))
	}
	return sections
}

func buildUpstreamSinceStartMetrics(snapshot *MetricsSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	lines := make([]string, 0, 3)
	if total, ok := snapshot.sum("rec53_upstream_failures_total"); ok {
		lines = append(lines, detailMetricLine("failures total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_upstream_fallback_total"); ok {
		lines = append(lines, detailMetricLine("fallback total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_upstream_winner_total"); ok {
		lines = append(lines, detailMetricLine("winner total", count(total)))
	}
	return lines
}

func buildUpstreamSinceStartSections(snapshot *MetricsSnapshot) []detailSection {
	if snapshot == nil {
		return nil
	}
	sections := make([]detailSection, 0, 2)
	if values, ok := snapshot.sumByLabel("rec53_upstream_failures_total", "reason"); ok {
		sections = append(sections, detailTotalBreakdownSection("Failure reasons:", buildTotalBreakdown(values, 4)))
	}
	if values, ok := snapshot.sumByLabel("rec53_upstream_winner_total", "path"); ok {
		sections = append(sections, detailTotalBreakdownSection("Winner paths:", buildTotalBreakdown(values, 3)))
	}
	return sections
}

func buildXDPSinceStartMetrics(snapshot *MetricsSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	lines := make([]string, 0, 6)
	if total, ok := snapshot.sum("rec53_xdp_cache_hits_total"); ok {
		lines = append(lines, detailMetricLine("hits total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_xdp_cache_misses_total"); ok {
		lines = append(lines, detailMetricLine("misses total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_xdp_pass_total"); ok {
		lines = append(lines, detailMetricLine("pass total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_xdp_errors_total"); ok {
		lines = append(lines, detailMetricLine("error total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_xdp_cache_sync_errors_total"); ok {
		lines = append(lines, detailMetricLine("sync error total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_xdp_cleanup_deleted_total"); ok {
		lines = append(lines, detailMetricLine("cleanup total", count(total)))
	}
	return lines
}

func buildStateMachineSinceStartMetrics(snapshot *MetricsSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	lines := make([]string, 0, 2)
	if total, ok := snapshot.sum("rec53_state_machine_stage_total"); ok {
		lines = append(lines, detailMetricLine("stages total", count(total)))
	}
	if total, ok := snapshot.sum("rec53_state_machine_failures_total"); ok {
		lines = append(lines, detailMetricLine("failures total", count(total)))
	}
	return lines
}

func buildStateMachineSinceStartSections(snapshot *MetricsSnapshot) []detailSection {
	if snapshot == nil {
		return nil
	}
	sections := make([]detailSection, 0, 2)
	if values, ok := snapshot.sumByLabel("rec53_state_machine_stage_total", "stage"); ok {
		sections = append(sections, detailTotalBreakdownSection("Stage totals:", buildTotalBreakdown(values, 5)))
	}
	if values, ok := snapshot.sumByLabel("rec53_state_machine_failures_total", "reason"); ok {
		sections = append(sections, detailTotalBreakdownSection("Failure totals:", buildTotalBreakdown(values, 5)))
	}
	return sections
}

func percent(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "--"
	}
	return fmt.Sprintf("%.1f%%", value*100)
}

func rate(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "--"
	}
	return fmt.Sprintf("%.1f/s", value)
}

func number(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "--"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func count(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "--"
	}
	if math.Abs(value-math.Round(value)) < 0.000001 {
		return fmt.Sprintf("%.0f", math.Round(value))
	}
	return fmt.Sprintf("%.1f", value)
}

func latency(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "--"
	}
	return fmt.Sprintf("%.1fms", value)
}
