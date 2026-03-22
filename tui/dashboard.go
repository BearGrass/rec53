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
	TopTerminal       string
	TopTerminalRate   float64
	TopFailure        string
	TopFailureRate    float64
	SecondFailure     string
	SecondFailureRate float64
	Terminals         []BreakdownItem
	TerminalTotals    []BreakdownItem
	DominantPath      StateMachinePath
	PathSummary       string
	Stages            []BreakdownItem
	Failures          []BreakdownItem
	LiveEdges         []TransitionBreakdownItem
	LiveTerminals     []TransitionBreakdownItem
	SinceStartEdges   []TransitionBreakdownItem
	FailureContexts   []StateMachineFailureContext
}

type BreakdownItem struct {
	Label string
	Rate  float64
	Ratio float64
	Total float64
}

type TransitionBreakdownItem struct {
	From  string
	To    string
	Rate  float64
	Ratio float64
	Total float64
}

type StateMachinePath struct {
	Summary       string
	MainEdges     []TransitionBreakdownItem
	BranchPoint   string
	BranchOptions []TransitionBreakdownItem
	Ambiguous     bool
}

type StateMachineFailureContext struct {
	Reason string
	Exit   string
	Rate   float64
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
	if !curr.hasMetric("rec53_state_machine_stage_total") || !curr.hasMetric("rec53_state_machine_transition_total") {
		return StateMachinePanel{Status: statusUnavailable}
	}
	panel := StateMachinePanel{Status: statusWarming}
	if transitionsCurr, ok := curr.sumByLabelPair("rec53_state_machine_transition_total", "from", "to"); ok {
		panel.TerminalTotals = buildTerminalTotalBreakdown(transitionsCurr, 4)
		panel.SinceStartEdges = buildTransitionTotalBreakdown(transitionsCurr, 5)
	}
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

	if transitionsCurr, ok := curr.sumByLabelPair("rec53_state_machine_transition_total", "from", "to"); ok {
		transitionsPrev, _ := prev.sumByLabelPair("rec53_state_machine_transition_total", "from", "to")
		transitionDelta := deltaTransitionMap(transitionsCurr, transitionsPrev)
		panel.LiveEdges = buildTransitionBreakdown(transitionDelta, dt, 6)
		panel.Terminals = buildTerminalBreakdown(transitionDelta, dt, 4)
		panel.LiveTerminals = buildFilteredTransitionBreakdown(transitionDelta, dt, 3, func(key transitionKey) bool {
			return isStateMachineTerminalNode(key.To)
		})
		panel.TerminalTotals = buildTerminalTotalBreakdown(transitionsCurr, 4)
		panel.SinceStartEdges = buildTransitionTotalBreakdown(transitionsCurr, 5)
		panel.DominantPath = buildStateMachinePath(transitionDelta, dt)
		panel.PathSummary = panel.DominantPath.Summary
		if len(panel.Terminals) > 0 {
			panel.TopTerminal = panel.Terminals[0].Label
			panel.TopTerminalRate = panel.Terminals[0].Rate
		}
	}

	panel.FailureContexts = buildStateMachineFailureContexts(panel.Failures)
	panel.Status = statusOK
	if panel.TopFailureRate > 0 || (panel.TopTerminalRate > 0 && panel.TopTerminal != "" && panel.TopTerminal != "success_exit") {
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
			model.Standout = fmt.Sprintf("SERVFAIL %s; answer quality is degraded.", percent(panel.ServfailRatio))
		case panel.P99MS >= 1000:
			model.Standout = fmt.Sprintf("p99 %s; tail latency is elevated.", latency(panel.P99MS))
		case topLabel != "":
			model.Standout = fmt.Sprintf("%s leads the response mix; traffic needs a quality check.", topLabel)
		default:
			model.Standout = "Traffic is degraded, but the bucket mix is still split."
		}
		model.NextChecks = []string{
			"4 Upstream: check timeout/bad-rcode.",
			"6 State: check failure clustering.",
			"Logs: inspect SERVFAIL if mix stays split.",
		}
	default:
		if topLabel != "" {
			model.Standout = fmt.Sprintf("%s leads the response mix.", topLabel)
		} else {
			model.Standout = "Traffic looks healthy, but the window is quiet."
		}
		model.NextChecks = []string{
			"Watch SERVFAIL/p99 before flagging traffic.",
			"2 Cache: compare if latency rises.",
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
			model.Standout = fmt.Sprintf("Miss %s is ahead of cache-served traffic; hit ratio %s.", rate(panel.MissRate), percent(panel.HitRatio))
		} else {
			model.Standout = fmt.Sprintf("Hit ratio %s; miss pressure is visible.", percent(panel.HitRatio))
		}
		model.NextChecks = []string{
			"1 Traffic: compare latency/quality.",
			"4 Upstream: misses may force iter.",
			"6 State: check miss-linked failures.",
		}
	default:
		if topLabel != "" {
			model.Standout = fmt.Sprintf("%s leads the cache mix; lifecycle %s.", topLabel, panel.Lifecycle)
		} else {
			model.Standout = "Cache looks healthy, but the window is quiet."
		}
		model.NextChecks = []string{
			"Watch miss bursts during traffic shifts.",
			"1 Traffic: compare if latency rises.",
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
		"Summary: return to cache verdict.",
		"Lifecycle: open if churn stays high.",
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
		"Lookup Mix: compare if misses rise without writes.",
		"Summary: return to cache verdict.",
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
		model.Standout = "Snapshot failures seen; do not trust restore yet."
		model.NextChecks = []string{
			"Logs: check load/save failures.",
			"Compare imported vs skipped counts.",
		}
		return model
	}

	if panel.Imported > 0 {
		model.Standout = fmt.Sprintf("Snapshot calm; p99 %s.", latency(panel.DurationP99MS))
	} else {
		model.Standout = "Snapshot metrics are available, but recent activity is light."
	}
	model.NextChecks = []string{
		"Check around restart/shutdown events.",
		"Logs: inspect small import counts.",
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
			model.Standout = fmt.Sprintf("%s leads recent upstream failures.", panel.DominantReason)
		case panel.FallbackRate > 0:
			model.Standout = fmt.Sprintf("Fallback %s; the first upstream path is not stable.", rate(panel.FallbackRate))
		default:
			model.Standout = "Upstream is degraded, but no single failure dominates yet."
		}
		model.NextChecks = []string{
			"Network: check reachability and RTT.",
			"1 Traffic: compare SERVFAIL/p99.",
			"6 State: check query_upstream failures.",
		}
	default:
		if panel.Winner != "" {
			model.Standout = fmt.Sprintf("%s wins the upstream race most often.", panel.Winner)
		} else {
			model.Standout = "Upstream looks healthy, but the window is quiet."
		}
		model.NextChecks = []string{
			"Watch timeout/fallback growth.",
			"Winner mix: track path shifts.",
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
		"Winners: open if no failure dominates.",
		"Summary: confirm upstream is root cause.",
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
		"Failures: open if churn follows errors.",
		"Summary: compare winner mix with verdict.",
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
		model.Standout = "XDP is active, but sync or packet-path errors are non-zero."
		model.NextChecks = []string{
			"2 Cache: compare fast-path hit ratio.",
			"Check sync-error/error counters.",
		}
		return model
	}

	model.Standout = fmt.Sprintf("XDP stable; hit %s, pass %s.", percent(panel.HitRatio), rate(panel.PassRate))
	model.NextChecks = []string{
		"Watch sync-error growth.",
		"2 Cache: compare if hit ratio looks low.",
	}
	return model
}

func buildStateMachineDetailModel(d Dashboard) detailModel {
	panel := d.StateMachine
	model := detailModel{
		Status: panel.Status,
		CurrentWindowMetrics: []string{
			detailMetricLine("top stage", fmt.Sprintf("%s %s", fallbackText(panel.TopStage), rate(panel.TopStageRate))),
			detailMetricLine("top terminal", fmt.Sprintf("%s %s", fallbackText(panel.TopTerminal), rate(panel.TopTerminalRate))),
			detailMetricLine("top failure", fmt.Sprintf("%s %s", fallbackText(panel.TopFailure), rate(panel.TopFailureRate))),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Stage mix:", panel.Stages),
			detailRateBreakdownSection("Terminal exits:", panel.Terminals),
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
		if panel.TopFailure != "" && panel.TopTerminal != "" {
			model.Standout = fmt.Sprintf("%s is the top recent failure reason and %s is the exit currently accumulating fastest.", panel.TopFailure, panel.TopTerminal)
		} else if panel.TopFailure != "" {
			model.Standout = fmt.Sprintf("%s is the top recent state-machine failure while %s remains the busiest stage.", panel.TopFailure, fallbackText(panel.TopStage))
		} else if panel.TopTerminal != "" {
			model.Standout = fmt.Sprintf("%s is the terminal exit currently growing fastest, so the resolver is ending too many flows there.", panel.TopTerminal)
		} else {
			model.Standout = fmt.Sprintf("%s is the busiest stage right now, but the failure mix still needs more samples before one reason clearly dominates.", fallbackText(panel.TopStage))
		}
		model.NextChecks = stateMachineNextChecks(panel.TopStage, panel.TopTerminal, panel.TopFailure)
	default:
		if panel.TopTerminal == "success_exit" {
			model.Standout = fmt.Sprintf("%s is the leading terminal exit and %s is the hottest recent stage, so the aggregate resolver flow still looks healthy.", panel.TopTerminal, fallbackText(panel.TopStage))
		} else if panel.TopStage != "" {
			model.Standout = fmt.Sprintf("%s is the hottest recent stage, and no failure reason is currently strong enough to define this window.", panel.TopStage)
		} else {
			model.Standout = "State-machine metrics are healthy, but the current sample window is too quiet to elevate one stage or terminal exit as the standout signal."
		}
		model.NextChecks = []string{
			"Watch non-success exits before degrading.",
			"Trace: `rec53 --config ./config.yaml --trace-domain example.com --trace-type A`.",
		}
	}

	return model
}

func buildStateMachinePathGraphSubviewModel(d Dashboard) detailModel {
	panel := d.StateMachine
	model := detailModel{
		Status:   panel.Status,
		Standout: "Path Graph shows the real resolver edges taken in the current window, then keeps cumulative transition totals bounded for baseline comparison.",
		CurrentWindowMetrics: []string{
			detailMetricLine("top stage", fmt.Sprintf("%s %s", fallbackText(panel.TopStage), rate(panel.TopStageRate))),
			detailMetricLine("top terminal", fmt.Sprintf("%s %s", fallbackText(panel.TopTerminal), rate(panel.TopTerminalRate))),
		},
		CurrentSections: []detailSection{
			detailTextSection("Live path graph:", stateMachinePathGraphLines(panel.DominantPath)),
			detailTransitionRateSection("Branch hotspots:", panel.DominantPath.BranchOptions),
			detailTransitionRateSection("Terminal exits:", panel.LiveTerminals),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_state_machine_transition_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("transitions total", count(total)))
		}
	}
	model.SinceStartSections = []detailSection{
		detailTransitionTotalSection("Transition totals:", panel.SinceStartEdges),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required state-machine transition metrics are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}
	model.NextChecks = []string{
		"Failures: open if path ends badly.",
		"Summary: return to state verdict.",
	}
	return model
}

func buildStateMachineFailuresSubviewModel(d Dashboard) detailModel {
	panel := d.StateMachine
	model := detailModel{
		Status:   panel.Status,
		Standout: "Failures reconciles bounded failure reasons with terminal exits so the path view and the error view tell the same story.",
		CurrentWindowMetrics: []string{
			detailMetricLine("top failure", fmt.Sprintf("%s %s", fallbackText(panel.TopFailure), rate(panel.TopFailureRate))),
			detailMetricLine("top terminal", fmt.Sprintf("%s %s", fallbackText(panel.TopTerminal), rate(panel.TopTerminalRate))),
			detailMetricLine("dominant path", fallbackText(panel.PathSummary)),
		},
		CurrentSections: []detailSection{
			detailRateBreakdownSection("Failure reasons:", panel.Failures),
			detailTextSection("Failure context:", buildStateMachineFailureContextLines(panel.FailureContexts, panel.DominantPath)),
		},
		SinceStartMetrics: []string{},
	}
	if d.CurrentSnapshot != nil {
		if total, ok := d.CurrentSnapshot.sum("rec53_state_machine_failures_total"); ok {
			model.SinceStartMetrics = append(model.SinceStartMetrics, detailMetricLine("failures total", count(total)))
		}
	}
	model.SinceStartSections = []detailSection{
		detailTotalBreakdownSection("Failure totals:", buildStateMachineFailureTotals(d.CurrentSnapshot)),
	}
	if standout, nextChecks, handled := detailStateOverride(panel.Status, d.LastError, "", "Required state-machine failure or transition metrics are missing from the target scrape."); handled {
		model.Standout = standout
		model.NextChecks = nextChecks
		return model
	}
	model.NextChecks = []string{
		"Summary: compare failures with top-line exits.",
		"Logs: pivot if failures keep rising.",
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
		"Sync/Cleanup: open if correctness drifts.",
		"Summary: compare with cache verdict.",
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
		"Packet Paths: open if hit ratio stays low.",
		"Summary: return to XDP verdict.",
	}
	return model
}

func detailStateOverride(status panelStatus, lastError, disabledMsg, unavailableMsg string) (string, []string, bool) {
	switch status {
	case statusWarming:
		return "Only one successful scrape is available, so short-window rates and ratios are not stable yet.", []string{
			"Wait one more refresh or press r.",
			"Header time: confirm the live window.",
		}, true
	case statusUnavailable:
		if unavailableMsg == "" {
			unavailableMsg = "Required metric families for this panel are missing from the current scrape."
		}
		return unavailableMsg, []string{
			"Verify this target exposes the metric family.",
			"Check raw /metric or -plain output.",
		}, true
	case statusDisabled:
		if disabledMsg == "" {
			disabledMsg = "This panel is intentionally disabled for the current deployment, so there is no live signal to interpret here."
		}
		return disabledMsg, []string{
			"No action unless this feature should be on.",
			"Check config before treating it as a fault.",
		}, true
	case statusDisconnected:
		standout := "The target has not produced a successful scrape yet, so this panel is not showing live rec53 behavior."
		if lastError != "" {
			standout = "The target is disconnected from rec53top, so there is no fresh live-state interpretation. Last error: " + lastError
		}
		return standout, []string{
			"Verify target address and metrics reachability.",
			"Press r or use -plain/curl after recovery.",
		}, true
	case statusStale:
		standout := "This panel is showing stale data because the latest scrape failed, so live interpretation is temporarily frozen."
		if lastError != "" {
			standout += " Last error: " + lastError
		}
		return standout, []string{
			"Treat numbers as old until next scrape.",
			"Check scrape connectivity first.",
		}, true
	default:
		return "", nil, false
	}
}

func stateMachineNextChecks(topStage, topTerminal, topFailure string) []string {
	checks := []string{
		"Trace: `rec53 --config ./config.yaml --trace-domain example.com --trace-type A`.",
	}
	switch {
	case strings.Contains(topFailure, "upstream"), topStage == "query_upstream", topTerminal == "servfail_exit":
		checks = append([]string{
			"4 Upstream: dominant failure is upstream.",
		}, checks...)
	case strings.Contains(topFailure, "cache"), strings.Contains(topFailure, "glue"), strings.Contains(topStage, "cache"), strings.Contains(topStage, "glue"):
		checks = append([]string{
			"2 Cache: dominant failure is cache/referral.",
		}, checks...)
	default:
		checks = append([]string{
			"1 Traffic + 4 Upstream: correlate first.",
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

func detailTextSection(title string, lines []string) detailSection {
	if len(lines) == 0 {
		lines = []string{"  no recent samples"}
	}
	return detailSection{Title: title, Lines: lines}
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

func detailTransitionRateSection(title string, items []TransitionBreakdownItem) detailSection {
	lines := []string{"  no recent transition samples"}
	if len(items) > 0 {
		lines = make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("  %-30s %8s  %6s", transitionEdgeLabel(item.From, item.To), rate(item.Rate), percent(item.Ratio)))
		}
	}
	return detailSection{Title: title, Lines: lines}
}

func detailTransitionTotalSection(title string, items []TransitionBreakdownItem) detailSection {
	lines := []string{"  no cumulative transition samples"}
	if len(items) > 0 {
		lines = make([]string, 0, len(items))
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("  %-30s %8s  %6s", transitionEdgeLabel(item.From, item.To), count(item.Total), percent(item.Ratio)))
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

func buildTransitionBreakdown(delta map[transitionKey]float64, dt float64, limit int) []TransitionBreakdownItem {
	return buildFilteredTransitionBreakdown(delta, dt, limit, func(transitionKey) bool { return true })
}

func buildFilteredTransitionBreakdown(delta map[transitionKey]float64, dt float64, limit int, keep func(transitionKey) bool) []TransitionBreakdownItem {
	if len(delta) == 0 || dt <= 0 || limit <= 0 {
		return nil
	}
	type entry struct {
		key   transitionKey
		value float64
	}
	entries := make([]entry, 0, len(delta))
	total := 0.0
	for key, value := range delta {
		if value <= 0 || !keep(key) {
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
			return transitionEdgeLabel(entries[i].key.From, entries[i].key.To) < transitionEdgeLabel(entries[j].key.From, entries[j].key.To)
		}
		return entries[i].value > entries[j].value
	})
	if limit > len(entries) {
		limit = len(entries)
	}
	items := make([]TransitionBreakdownItem, 0, limit)
	for _, entry := range entries[:limit] {
		item := TransitionBreakdownItem{
			From: entry.key.From,
			To:   entry.key.To,
			Rate: entry.value / dt,
		}
		if total > 0 {
			item.Ratio = entry.value / total
		}
		items = append(items, item)
	}
	return items
}

func buildTransitionTotalBreakdown(values map[transitionKey]float64, limit int) []TransitionBreakdownItem {
	if len(values) == 0 || limit <= 0 {
		return nil
	}
	type entry struct {
		key   transitionKey
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
			return transitionEdgeLabel(entries[i].key.From, entries[i].key.To) < transitionEdgeLabel(entries[j].key.From, entries[j].key.To)
		}
		return entries[i].value > entries[j].value
	})
	if limit > len(entries) {
		limit = len(entries)
	}
	items := make([]TransitionBreakdownItem, 0, limit)
	for _, entry := range entries[:limit] {
		item := TransitionBreakdownItem{
			From:  entry.key.From,
			To:    entry.key.To,
			Total: entry.value,
		}
		if total > 0 {
			item.Ratio = entry.value / total
		}
		items = append(items, item)
	}
	return items
}

func buildTerminalBreakdown(delta map[transitionKey]float64, dt float64, limit int) []BreakdownItem {
	terminals := make(map[string]float64)
	for key, value := range delta {
		if value <= 0 || !isStateMachineTerminalNode(key.To) {
			continue
		}
		terminals[key.To] += value
	}
	return buildBreakdown(terminals, dt, limit)
}

func buildTerminalTotalBreakdown(values map[transitionKey]float64, limit int) []BreakdownItem {
	terminals := make(map[string]float64)
	for key, value := range values {
		if value <= 0 || !isStateMachineTerminalNode(key.To) {
			continue
		}
		terminals[key.To] += value
	}
	return buildTotalBreakdown(terminals, limit)
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
	lines := make([]string, 0, 3)
	if total, ok := snapshot.sum("rec53_state_machine_stage_total"); ok {
		lines = append(lines, detailMetricLine("stages total", count(total)))
	}
	if values, ok := snapshot.sumByLabelPair("rec53_state_machine_transition_total", "from", "to"); ok {
		terminalTotals := buildTerminalTotalBreakdown(values, 4)
		total := 0.0
		for _, item := range terminalTotals {
			total += item.Total
		}
		if total > 0 {
			lines = append(lines, detailMetricLine("terminal total", count(total)))
		}
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
	sections := make([]detailSection, 0, 3)
	if values, ok := snapshot.sumByLabel("rec53_state_machine_stage_total", "stage"); ok {
		sections = append(sections, detailTotalBreakdownSection("Stage totals:", buildTotalBreakdown(values, 5)))
	}
	if values, ok := snapshot.sumByLabelPair("rec53_state_machine_transition_total", "from", "to"); ok {
		sections = append(sections, detailTotalBreakdownSection("Terminal totals:", buildTerminalTotalBreakdown(values, 4)))
	}
	if values, ok := snapshot.sumByLabel("rec53_state_machine_failures_total", "reason"); ok {
		sections = append(sections, detailTotalBreakdownSection("Failure totals:", buildTotalBreakdown(values, 5)))
	}
	return sections
}

func buildStateMachineFailureTotals(snapshot *MetricsSnapshot) []BreakdownItem {
	if snapshot == nil {
		return nil
	}
	values, ok := snapshot.sumByLabel("rec53_state_machine_failures_total", "reason")
	if !ok {
		return nil
	}
	return buildTotalBreakdown(values, 5)
}

func transitionEdgeLabel(from, to string) string {
	return from + " -> " + to
}

func isStateMachineTerminalNode(name string) bool {
	return strings.HasSuffix(name, "_exit")
}

func buildStateMachinePath(delta map[transitionKey]float64, dt float64) StateMachinePath {
	path := StateMachinePath{}
	current := "state_init"
	visited := map[string]bool{current: true}
	for steps := 0; steps < 12; steps++ {
		outgoing := buildFilteredTransitionBreakdown(delta, dt, 4, func(key transitionKey) bool {
			return key.From == current
		})
		if len(outgoing) == 0 {
			break
		}
		best := outgoing[0]
		path.MainEdges = append(path.MainEdges, best)
		if len(outgoing) > 1 && !dominantStateMachineEdge(best.Rate, outgoing[1].Rate) {
			path.MainEdges = path.MainEdges[:len(path.MainEdges)-1]
			path.BranchPoint = current
			path.BranchOptions = outgoing
			path.Ambiguous = true
			break
		}
		if isStateMachineTerminalNode(best.To) {
			break
		}
		if visited[best.To] {
			break
		}
		visited[best.To] = true
		current = best.To
	}
	path.Summary = summarizeStateMachinePath(path)
	return path
}

func dominantStateMachineEdge(best, second float64) bool {
	if second <= 0 {
		return best > 0
	}
	return best >= second*1.2 && (best-second) >= 0.5
}

func summarizeStateMachinePath(path StateMachinePath) string {
	if len(path.MainEdges) == 0 && path.BranchPoint == "" {
		return ""
	}
	parts := make([]string, 0, len(path.MainEdges)+2)
	if len(path.MainEdges) > 0 {
		parts = append(parts, path.MainEdges[0].From)
		for _, edge := range path.MainEdges {
			parts = append(parts, edge.To)
		}
	}
	if path.Ambiguous {
		if len(parts) == 0 {
			return "branching live path"
		}
		return strings.Join(parts, " -> ") + " (branching at " + path.BranchPoint + ")"
	}
	return strings.Join(parts, " -> ")
}

func stateMachinePathSummaryLines(path StateMachinePath) []string {
	if path.Summary == "" {
		return []string{"  no dominant live path yet"}
	}
	lines := []string{"  " + path.Summary}
	if path.Ambiguous && len(path.BranchOptions) > 0 {
		lines = append(lines, fmt.Sprintf("  branch point: %s", path.BranchPoint))
	}
	return lines
}

func stateMachinePathGraphLines(path StateMachinePath) []string {
	if len(path.MainEdges) == 0 && path.BranchPoint == "" {
		return []string{"  no recent live transition samples"}
	}
	lines := make([]string, 0, len(path.MainEdges)+2)
	for i, edge := range path.MainEdges {
		prefix := "  "
		if i > 0 {
			prefix = "    "
		}
		lines = append(lines, prefix+edge.From)
		lines = append(lines, prefix+"  -> "+edge.To+fmt.Sprintf("  (%s)", rate(edge.Rate)))
	}
	if len(path.MainEdges) == 0 && path.BranchPoint != "" {
		lines = append(lines, "  "+path.BranchPoint)
	}
	if path.Ambiguous {
		lines = append(lines, fmt.Sprintf("  branch at %s", path.BranchPoint))
	}
	return lines
}

func buildStateMachineFailureContexts(failures []BreakdownItem) []StateMachineFailureContext {
	contexts := make([]StateMachineFailureContext, 0, len(failures))
	for _, item := range failures {
		contexts = append(contexts, StateMachineFailureContext{
			Reason: item.Label,
			Exit:   stateMachineFailureExit(item.Label),
			Rate:   item.Rate,
		})
	}
	return contexts
}

func stateMachineFailureExit(reason string) string {
	switch {
	case strings.Contains(reason, "formerr"):
		return "formerr_exit"
	case strings.Contains(reason, "servfail"):
		return "servfail_exit"
	case strings.Contains(reason, "max_iterations"):
		return "max_iterations_exit"
	case strings.Contains(reason, "query_upstream_error"):
		return "servfail_exit"
	default:
		return "error_exit"
	}
}

func buildStateMachineFailureContextLines(contexts []StateMachineFailureContext, path StateMachinePath) []string {
	if len(contexts) == 0 {
		return []string{"  no current failure reason is standing out"}
	}
	lines := make([]string, 0, len(contexts))
	pathText := fallbackText(path.Summary)
	for _, context := range contexts {
		lines = append(lines, fmt.Sprintf("  %-18s -> %-20s %s", context.Reason, context.Exit, rate(context.Rate)))
		if pathText != "--" && strings.Contains(pathText, context.Exit) {
			lines = append(lines, fmt.Sprintf("  path context: %s", pathText))
		}
	}
	return lines
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
