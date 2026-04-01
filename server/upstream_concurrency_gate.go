package server

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"rec53/monitor"
)

const upstreamGateLogInterval = 30 * time.Second

var errUpstreamConcurrencyGateSaturated = errors.New("upstream concurrency gate saturated")

type upstreamGatePath uint8

const (
	upstreamGatePathForward upstreamGatePath = iota
	upstreamGatePathIterative
	upstreamGatePathHappyEyeballs
	upstreamGatePathNSResolution
	upstreamGatePathIterativeRetry
)

type upstreamGateEvent struct {
	action          string
	path            upstreamGatePath
	qname           string
	inflight        int
	limit           int
	shouldLog       bool
	suppressedCount int
}

type upstreamGateLogState struct {
	lastLogAt       time.Time
	suppressedCount int
}

type upstreamConcurrencyGate struct {
	mu        sync.Mutex
	limit     int
	inflight  int
	logStates map[string]*upstreamGateLogState
}

var globalUpstreamGate = newUpstreamConcurrencyGate(0)

func newUpstreamConcurrencyGate(limit int) *upstreamConcurrencyGate {
	g := &upstreamConcurrencyGate{logStates: make(map[string]*upstreamGateLogState)}
	g.setLimit(limit)
	return g
}

func normalizeUpstreamConcurrencyLimit(limit int) int {
	if limit <= 0 {
		limit = runtime.NumCPU()
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

func SetUpstreamConcurrencyLimit(limit int) {
	globalUpstreamGate.setLimit(limit)
}

func GetUpstreamConcurrencyLimit() int {
	return globalUpstreamGate.getLimit()
}

func resetUpstreamConcurrencyGateForTest(limit int) {
	globalUpstreamGate.reset(limit)
}

func (g *upstreamConcurrencyGate) setLimit(limit int) {
	if g == nil {
		return
	}
	limit = normalizeUpstreamConcurrencyLimit(limit)
	g.mu.Lock()
	g.limit = limit
	g.mu.Unlock()
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.UpstreamGateLimitSet(limit)
	}
}

func (g *upstreamConcurrencyGate) getLimit() int {
	if g == nil {
		return 1
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.limit
}

func (g *upstreamConcurrencyGate) reset(limit int) {
	if g == nil {
		return
	}
	limit = normalizeUpstreamConcurrencyLimit(limit)
	g.mu.Lock()
	g.limit = limit
	g.inflight = 0
	g.logStates = make(map[string]*upstreamGateLogState)
	g.mu.Unlock()
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.UpstreamGateInFlightSet(0)
		monitor.Rec53Metric.UpstreamGateLimitSet(limit)
	}
}

func (g *upstreamConcurrencyGate) tryAcquire(path upstreamGatePath, qname string) bool {
	if g == nil {
		return true
	}

	g.mu.Lock()
	if g.inflight >= g.limit {
		event := g.buildEventLocked("saturated", path, qname)
		g.mu.Unlock()
		recordUpstreamGateEvent(event)
		return false
	}
	g.inflight++
	inflight := g.inflight
	limit := g.limit
	g.mu.Unlock()

	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.UpstreamGateInFlightSet(inflight)
		monitor.Rec53Metric.UpstreamGateLimitSet(limit)
	}
	return true
}

func (g *upstreamConcurrencyGate) release() {
	if g == nil {
		return
	}

	g.mu.Lock()
	if g.inflight > 0 {
		g.inflight--
	}
	inflight := g.inflight
	limit := g.limit
	g.mu.Unlock()

	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.UpstreamGateInFlightSet(inflight)
		monitor.Rec53Metric.UpstreamGateLimitSet(limit)
	}
}

func (g *upstreamConcurrencyGate) shouldDegradeFanout() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.limit <= 1 {
		return true
	}
	return g.inflight >= g.limit-1
}

func (g *upstreamConcurrencyGate) recordDegraded(path upstreamGatePath, action, qname string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	event := g.buildEventLocked(action, path, qname)
	g.mu.Unlock()
	recordUpstreamGateEvent(event)
}

func (g *upstreamConcurrencyGate) buildEventLocked(action string, path upstreamGatePath, qname string) upstreamGateEvent {
	now := time.Now()
	key := action + "|" + path.metricLabel()
	state := g.logStates[key]
	if state == nil {
		state = &upstreamGateLogState{}
		g.logStates[key] = state
	}
	event := upstreamGateEvent{
		action:   action,
		path:     path,
		qname:    qname,
		inflight: g.inflight,
		limit:    g.limit,
	}
	if state.lastLogAt.IsZero() || now.Sub(state.lastLogAt) >= upstreamGateLogInterval {
		event.shouldLog = true
		event.suppressedCount = state.suppressedCount
		state.lastLogAt = now
		state.suppressedCount = 0
		return event
	}
	state.suppressedCount++
	return event
}

func recordUpstreamGateEvent(event upstreamGateEvent) {
	if event.action == "" {
		return
	}
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.UpstreamGateEventAdd(event.action, event.path.metricLabel())
	}
	if !event.shouldLog || monitor.Rec53Log == nil {
		return
	}
	monitor.Rec53Log.Warnf("[UPSTREAM_GATE] action=%s path=%s inflight=%d limit=%d qname=%s suppressed=%d",
		event.action,
		event.path.logLabel(),
		event.inflight,
		event.limit,
		event.qname,
		event.suppressedCount,
	)
}

func (p upstreamGatePath) metricLabel() string {
	switch p {
	case upstreamGatePathForward:
		return "forward"
	case upstreamGatePathIterative:
		return "iterative"
	case upstreamGatePathHappyEyeballs:
		return "happy_eyeballs"
	case upstreamGatePathNSResolution:
		return "ns_resolution"
	case upstreamGatePathIterativeRetry:
		return "iterative_retry"
	default:
		return "unknown"
	}
}

func (p upstreamGatePath) logLabel() string {
	return p.metricLabel()
}
