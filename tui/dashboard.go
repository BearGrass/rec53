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
}

type Dashboard struct {
	Target         string
	Mode           panelStatus
	LastUpdate     time.Time
	LastSuccess    time.Time
	ScrapeDuration time.Duration
	LastError      string
	OverallSummary string
	Traffic        TrafficPanel
	Cache          CachePanel
	Snapshot       SnapshotPanel
	Upstream       UpstreamPanel
	XDP            XDPPanel
	StateMachine   StateMachinePanel
	HelpExpanded   bool
}

func deriveDashboard(target string, prev, curr *MetricsSnapshot, scrapeDuration time.Duration) Dashboard {
	dashboard := Dashboard{
		Target:         target,
		Mode:           statusOK,
		LastUpdate:     time.Now(),
		LastSuccess:    time.Now(),
		ScrapeDuration: scrapeDuration,
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
		panel.TimeoutRate = delta["timeout"] / dt
		panel.BadRcodeRate = delta["bad_rcode"] / dt
		panel.DominantReason, _ = pickTopLabel(delta)
	}

	fallbackCurr, ok := curr.sum("rec53_upstream_fallback_total")
	if ok {
		fallbackPrev, _ := prev.sum("rec53_upstream_fallback_total")
		panel.FallbackRate = deltaFloat(fallbackCurr, fallbackPrev) / dt
	}

	if winnersCurr, ok := curr.sumByLabel("rec53_upstream_winner_total", "path"); ok {
		winnersPrev, _ := prev.sumByLabel("rec53_upstream_winner_total", "path")
		delta := deltaMap(winnersCurr, winnersPrev)
		panel.Winner, panel.WinnerRate = pickTopLabel(delta)
		panel.WinnerRate = panel.WinnerRate / dt
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
		panel.TopStage, panel.TopStageRate = pickTopLabel(delta)
		panel.TopStageRate = panel.TopStageRate / dt
	}

	failuresCurr, ok := curr.sumByLabel("rec53_state_machine_failures_total", "reason")
	if ok {
		failuresPrev, _ := prev.sumByLabel("rec53_state_machine_failures_total", "reason")
		delta := deltaMap(failuresCurr, failuresPrev)
		type entry struct {
			key   string
			value float64
		}
		entries := make([]entry, 0, len(delta))
		for key, value := range delta {
			entries = append(entries, entry{key: key, value: value})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].value == entries[j].value {
				return entries[i].key < entries[j].key
			}
			return entries[i].value > entries[j].value
		})
		if len(entries) > 0 {
			panel.TopFailure = entries[0].key
			panel.TopFailureRate = entries[0].value / dt
		}
		if len(entries) > 1 {
			panel.SecondFailure = entries[1].key
			panel.SecondFailureRate = entries[1].value / dt
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

func latency(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "--"
	}
	return fmt.Sprintf("%.1fms", value)
}
