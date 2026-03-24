package server

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

const (
	hotZoneWindowDuration           = 5 * time.Second
	hotZoneRecentWindowCount        = 3
	hotZoneEvaluateInterval         = time.Second
	hotZoneObserveCPUThreshold      = 70.0
	hotZoneObserveConcurrencyFactor = 0.75
	hotZoneChildDominanceThreshold  = 0.80
	hotZoneProtectConfirmWindows    = 3
	hotZoneExitBaselineFactor       = 1.05
	hotZoneLogInterval              = 30 * time.Second
)

type hotZoneController struct {
	mu sync.Mutex

	nowFn      func() time.Time
	cpuUsageFn func() float64
	numCPUFn   func() int
	evalEvery  time.Duration

	baseSuffixes []string
	active       map[uint64]*hotZoneActiveRequest
	nextID       uint64
	windows      []hotZoneWindow
	lastEvalAt   time.Time

	observeMode         bool
	protectedZone       string
	candidateZone       string
	candidateStreak     int
	lastCandidateWindow time.Time
	lastNormalAvg       float64
	preTriggerBaseline  float64

	logState map[string]*hotZoneLogState
}

type hotZoneActiveRequest struct {
	id          uint64
	qname       string
	coarseKey   string
	startedAt   time.Time
	accountedAt time.Time
}

type hotZoneWindow struct {
	start           time.Time
	globalOccupancy time.Duration
	coarseOccupancy map[string]time.Duration
	childOccupancy  map[string]map[string]time.Duration
}

type hotZoneSnapshot struct {
	currentWindowStart time.Time
	currentElapsed     time.Duration
	currentGlobalAvg   float64
	aggregatedGlobal   time.Duration
	aggregatedCoarse   map[string]time.Duration
	aggregatedChild    map[string]map[string]time.Duration
}

type hotZoneLogState struct {
	lastAt          time.Time
	suppressedCount int
}

type cpuUsageSampler struct {
	mu         sync.Mutex
	lastReadAt time.Time
	lastTotal  uint64
	lastIdle   uint64
	lastValue  float64
}

func newHotZoneController(baseSuffixes []string) *hotZoneController {
	return &hotZoneController{
		nowFn:        time.Now,
		cpuUsageFn:   newCPUUsageSampler().CPUUsage,
		numCPUFn:     runtime.NumCPU,
		evalEvery:    hotZoneEvaluateInterval,
		baseSuffixes: normalizeHotZoneBaseSuffixes(baseSuffixes),
		active:       make(map[uint64]*hotZoneActiveRequest),
		logState:     make(map[string]*hotZoneLogState),
	}
}

func (c *hotZoneController) TryEnter(qname, matchedForwardZone string) (bool, uint64) {
	if c == nil {
		return true, 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.nowFn()
	rotated := c.rotateLocked(now)
	c.maybeEvaluateLocked(now, rotated)

	fqdn := dns.Fqdn(qname)
	if c.protectedZone != "" && dns.IsSubDomain(c.protectedZone, fqdn) {
		c.recordRefusedLocked(fqdn)
		return false, 0
	}

	c.nextID++
	id := c.nextID
	c.active[id] = &hotZoneActiveRequest{
		id:          id,
		qname:       fqdn,
		coarseKey:   hotZoneCoarseKey(fqdn, matchedForwardZone, c.baseSuffixes),
		startedAt:   now,
		accountedAt: now,
	}
	return true, id
}

func (c *hotZoneController) Release(id uint64) {
	if c == nil || id == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.nowFn()
	rotated := c.rotateLocked(now)
	active := c.active[id]
	if active != nil {
		c.addSampleToCurrentWindowLocked(active, now)
		delete(c.active, id)
	}
	c.maybeEvaluateLocked(now, rotated)
}

func (c *hotZoneController) rotateLocked(now time.Time) bool {
	current := c.ensureCurrentWindowLocked(now)
	rotated := false
	for now.Sub(current.start) >= hotZoneWindowDuration {
		boundary := current.start.Add(hotZoneWindowDuration)
		for _, active := range c.active {
			c.addSampleSegmentLocked(current, active, boundary)
			active.accountedAt = boundary
		}
		current = c.advanceWindowLocked(boundary)
		rotated = true
	}
	return rotated
}

func (c *hotZoneController) maybeEvaluateLocked(now time.Time, rotated bool) {
	interval := c.evalEvery
	if interval <= 0 {
		interval = hotZoneEvaluateInterval
	}
	if rotated || c.lastEvalAt.IsZero() || now.Sub(c.lastEvalAt) >= interval {
		c.evaluateLocked(now)
		c.lastEvalAt = now
	}
}

func (c *hotZoneController) ensureCurrentWindowLocked(now time.Time) *hotZoneWindow {
	if len(c.windows) == 0 {
		start := now.Truncate(hotZoneWindowDuration)
		if start.IsZero() {
			start = now
		}
		c.windows = append(c.windows, newHotZoneWindow(start))
	}
	return &c.windows[len(c.windows)-1]
}

func (c *hotZoneController) advanceWindowLocked(start time.Time) *hotZoneWindow {
	c.windows = append(c.windows, newHotZoneWindow(start))
	if len(c.windows) > hotZoneRecentWindowCount {
		copy(c.windows, c.windows[len(c.windows)-hotZoneRecentWindowCount:])
		c.windows = c.windows[:hotZoneRecentWindowCount]
	}
	return &c.windows[len(c.windows)-1]
}

func newHotZoneWindow(start time.Time) hotZoneWindow {
	return hotZoneWindow{
		start:           start,
		coarseOccupancy: make(map[string]time.Duration),
		childOccupancy:  make(map[string]map[string]time.Duration),
	}
}

func (c *hotZoneController) addSampleToCurrentWindowLocked(active *hotZoneActiveRequest, now time.Time) {
	if active == nil || !now.After(active.accountedAt) {
		return
	}
	current := c.ensureCurrentWindowLocked(now)
	c.addSampleSegmentLocked(current, active, now)
	active.accountedAt = now
}

func (c *hotZoneController) addSampleSegmentLocked(window *hotZoneWindow, active *hotZoneActiveRequest, end time.Time) {
	if window == nil || active == nil || !end.After(active.accountedAt) {
		return
	}
	delta := end.Sub(active.accountedAt)
	window.globalOccupancy += delta
	window.coarseOccupancy[active.coarseKey] += delta
	child := hotZoneDirectChild(active.qname, active.coarseKey)
	if child != "" {
		if window.childOccupancy[active.coarseKey] == nil {
			window.childOccupancy[active.coarseKey] = make(map[string]time.Duration)
		}
		window.childOccupancy[active.coarseKey][child] += delta
	}
}

func (c *hotZoneController) snapshotLocked(now time.Time) hotZoneSnapshot {
	c.ensureCurrentWindowLocked(now)
	current := c.windows[len(c.windows)-1]
	snapshot := hotZoneSnapshot{
		currentWindowStart: current.start,
		currentElapsed:     now.Sub(current.start),
		aggregatedCoarse:   make(map[string]time.Duration),
		aggregatedChild:    make(map[string]map[string]time.Duration),
	}
	if snapshot.currentElapsed <= 0 {
		snapshot.currentElapsed = time.Second
	}

	windows := make([]hotZoneWindow, len(c.windows))
	copy(windows, c.windows)
	for i := range windows {
		if windows[i].coarseOccupancy == nil {
			windows[i].coarseOccupancy = make(map[string]time.Duration)
		}
		if windows[i].childOccupancy == nil {
			windows[i].childOccupancy = make(map[string]map[string]time.Duration)
		}
	}
	for _, active := range c.active {
		if !now.After(active.accountedAt) {
			continue
		}
		idx := len(windows) - 1
		if idx < 0 {
			continue
		}
		window := &windows[idx]
		delta := now.Sub(active.accountedAt)
		window.globalOccupancy += delta
		window.coarseOccupancy[active.coarseKey] += delta
		child := hotZoneDirectChild(active.qname, active.coarseKey)
		if child != "" {
			if window.childOccupancy[active.coarseKey] == nil {
				window.childOccupancy[active.coarseKey] = make(map[string]time.Duration)
			}
			window.childOccupancy[active.coarseKey][child] += delta
		}
	}

	for i, window := range windows {
		snapshot.aggregatedGlobal += window.globalOccupancy
		for key, dur := range window.coarseOccupancy {
			snapshot.aggregatedCoarse[key] += dur
		}
		for parent, childMap := range window.childOccupancy {
			if snapshot.aggregatedChild[parent] == nil {
				snapshot.aggregatedChild[parent] = make(map[string]time.Duration)
			}
			for child, dur := range childMap {
				snapshot.aggregatedChild[parent][child] += dur
			}
		}
		if i == len(windows)-1 {
			snapshot.currentGlobalAvg = float64(window.globalOccupancy) / float64(snapshot.currentElapsed)
		}
	}

	return snapshot
}

func (c *hotZoneController) evaluateLocked(now time.Time) {
	snapshot := c.snapshotLocked(now)
	avg := snapshot.currentGlobalAvg
	numCPU := 1
	if c.numCPUFn != nil {
		numCPU = c.numCPUFn()
	}
	threshold := hotZoneObserveConcurrencyFactor * float64(max(1, numCPU))
	cpu := c.cpuUsageLocked()
	pressure := avg >= threshold && cpu >= hotZoneObserveCPUThreshold

	if !c.observeMode {
		if !pressure {
			c.lastNormalAvg = avg
			c.publishMetricsLocked(snapshot)
			return
		}
		c.observeMode = true
		c.preTriggerBaseline = c.lastNormalAvg
		if c.preTriggerBaseline <= 0 {
			c.preTriggerBaseline = avg
		}
		c.recordEventLocked("observe_enter", fmt.Sprintf("enter observe mode avg=%.2f cpu=%.1f baseline=%.2f", avg, cpu, c.preTriggerBaseline))
	}

	if c.protectedZone != "" {
		if avg <= c.preTriggerBaseline*hotZoneExitBaselineFactor {
			zone := c.protectedZone
			c.observeMode = false
			c.protectedZone = ""
			c.candidateZone = ""
			c.candidateStreak = 0
			c.lastCandidateWindow = time.Time{}
			c.lastNormalAvg = avg
			c.recordEventLocked("protect_exit", fmt.Sprintf("exit protected zone=%s avg=%.2f baseline=%.2f", zone, avg, c.preTriggerBaseline))
		}
		c.publishMetricsLocked(snapshot)
		return
	}

	if !pressure {
		c.observeMode = false
		c.candidateZone = ""
		c.candidateStreak = 0
		c.lastCandidateWindow = time.Time{}
		c.lastNormalAvg = avg
		c.recordEventLocked("observe_exit", fmt.Sprintf("leave observe mode avg=%.2f cpu=%.1f", avg, cpu))
		c.publishMetricsLocked(snapshot)
		return
	}

	candidate := snapshot.selectCandidate()
	if candidate == "" {
		c.candidateZone = ""
		c.candidateStreak = 0
		c.lastCandidateWindow = snapshot.currentWindowStart
		c.publishMetricsLocked(snapshot)
		return
	}

	if candidate != c.candidateZone {
		c.candidateZone = candidate
		c.candidateStreak = 1
		c.lastCandidateWindow = snapshot.currentWindowStart
		c.recordEventLocked("candidate_change", fmt.Sprintf("candidate zone=%s streak=%d", c.candidateZone, c.candidateStreak))
	} else if !snapshot.currentWindowStart.Equal(c.lastCandidateWindow) {
		c.candidateStreak++
		c.lastCandidateWindow = snapshot.currentWindowStart
		c.recordEventLocked("candidate_confirm", fmt.Sprintf("candidate zone=%s streak=%d", c.candidateZone, c.candidateStreak))
	}

	if c.candidateStreak >= hotZoneProtectConfirmWindows {
		c.protectedZone = c.candidateZone
		c.recordEventLocked("protect_enter", fmt.Sprintf("enter protected zone=%s streak=%d avg=%.2f baseline=%.2f", c.protectedZone, c.candidateStreak, avg, c.preTriggerBaseline))
	}
	c.publishMetricsLocked(snapshot)
}

func (c *hotZoneController) cpuUsageLocked() float64 {
	if c.cpuUsageFn == nil {
		return 0
	}
	return c.cpuUsageFn()
}

func (c *hotZoneController) recordRefusedLocked(qname string) {
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.HotZoneEventAdd("refused")
	}
	if monitor.Rec53Log == nil || c.protectedZone == "" {
		return
	}
	c.logRateLimitedLocked("refused:"+c.protectedZone, fmt.Sprintf("[HOT_ZONE] refused expensive path for %s because protected zone=%s", qname, c.protectedZone))
}

func (c *hotZoneController) recordEventLocked(event, detail string) {
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.HotZoneEventAdd(event)
	}
	if monitor.Rec53Log == nil {
		return
	}
	c.logRateLimitedLocked(event, "[HOT_ZONE] "+detail)
}

func (c *hotZoneController) logRateLimitedLocked(key, msg string) {
	state := c.logState[key]
	if state == nil {
		state = &hotZoneLogState{}
		c.logState[key] = state
	}
	now := c.nowFn()
	if state.lastAt.IsZero() || now.Sub(state.lastAt) >= hotZoneLogInterval {
		suppressed := state.suppressedCount
		state.lastAt = now
		state.suppressedCount = 0
		monitor.Rec53Log.Warnf("%s (suppressed=%d)", msg, suppressed)
		return
	}
	state.suppressedCount++
}

func (c *hotZoneController) publishMetricsLocked(snapshot hotZoneSnapshot) {
	if monitor.Rec53Metric == nil {
		return
	}
	if c.protectedZone == "" && !c.observeMode && c.candidateStreak == 0 && c.preTriggerBaseline == 0 && snapshot.currentGlobalAvg == 0 {
		return
	}
	monitor.Rec53Metric.HotZoneObserveModeSet(c.observeMode)
	monitor.Rec53Metric.HotZoneProtectedSet(c.protectedZone != "")
	monitor.Rec53Metric.HotZoneAvgExpensiveConcurrencySet(snapshot.currentGlobalAvg)
	monitor.Rec53Metric.HotZoneBaselineConcurrencySet(c.preTriggerBaseline)
	monitor.Rec53Metric.HotZoneCandidateStreakSet(c.candidateStreak)
}

func (s hotZoneSnapshot) selectCandidate() string {
	var hottest string
	var hottestDur time.Duration
	for key, dur := range s.aggregatedCoarse {
		if dur <= 0 || hotZoneIsPublicSuffixLike(key) {
			continue
		}
		if dur > hottestDur || (dur == hottestDur && key < hottest) {
			hottest = key
			hottestDur = dur
		}
	}
	if hottest == "" || hottestDur <= 0 {
		return ""
	}
	children := s.aggregatedChild[hottest]
	var hottestChild string
	var hottestChildDur time.Duration
	for child, dur := range children {
		if dur > hottestChildDur || (dur == hottestChildDur && child < hottestChild) {
			hottestChild = child
			hottestChildDur = dur
		}
	}
	if hottestChild == "" || hottestChildDur <= 0 {
		return hottest
	}
	if float64(hottestChildDur) >= float64(hottestDur)*hotZoneChildDominanceThreshold {
		return hottestChild
	}
	return hottest
}

func hotZoneCoarseKey(qname, matchedForwardZone string, baseSuffixes []string) string {
	if matchedForwardZone != "" {
		return dns.Fqdn(matchedForwardZone)
	}
	fqdn := dns.Fqdn(qname)
	if suffix := hotZoneMatchedBaseSuffix(fqdn, baseSuffixes); suffix != "" {
		return hotZoneBusinessRootAboveSuffix(fqdn, suffix)
	}
	return hotZoneFallbackLevel3(fqdn)
}

func hotZoneMatchedBaseSuffix(qname string, baseSuffixes []string) string {
	for _, suffix := range baseSuffixes {
		if dns.IsSubDomain(suffix, qname) {
			return suffix
		}
	}
	return ""
}

func hotZoneBusinessRootAboveSuffix(qname, suffix string) string {
	labels := dns.SplitDomainName(qname)
	suffixLabels := dns.SplitDomainName(suffix)
	if len(labels) <= len(suffixLabels) {
		return dns.Fqdn(suffix)
	}
	start := len(labels) - len(suffixLabels) - 1
	return strings.Join(labels[start:], ".") + "."
}

func hotZoneFallbackLevel3(qname string) string {
	labels := dns.SplitDomainName(qname)
	if len(labels) <= 2 {
		return dns.Fqdn(qname)
	}
	return strings.Join(labels[len(labels)-2:], ".") + "."
}

func hotZoneDirectChild(qname, parent string) string {
	fqdn := dns.Fqdn(qname)
	parent = dns.Fqdn(parent)
	if fqdn == parent || !dns.IsSubDomain(parent, fqdn) {
		return ""
	}
	labels := dns.SplitDomainName(fqdn)
	parentLabels := dns.SplitDomainName(parent)
	if len(labels) <= len(parentLabels) {
		return ""
	}
	start := len(labels) - len(parentLabels) - 1
	return strings.Join(labels[start:], ".") + "."
}

func hotZoneIsPublicSuffixLike(zone string) bool {
	return len(dns.SplitDomainName(zone)) <= 1
}

func normalizeHotZoneBaseSuffixes(extra []string) []string {
	seen := make(map[string]struct{})
	var suffixes []string
	for _, value := range DefaultCuratedTLDs {
		suffix := dns.Fqdn(value)
		if _, ok := seen[suffix]; ok {
			continue
		}
		seen[suffix] = struct{}{}
		suffixes = append(suffixes, suffix)
	}
	for _, value := range extra {
		suffix := dns.Fqdn(strings.TrimSpace(value))
		if suffix == "." {
			continue
		}
		if _, ok := seen[suffix]; ok {
			continue
		}
		seen[suffix] = struct{}{}
		suffixes = append(suffixes, suffix)
	}
	sort.Slice(suffixes, func(i, j int) bool {
		if len(suffixes[i]) == len(suffixes[j]) {
			return suffixes[i] < suffixes[j]
		}
		return len(suffixes[i]) > len(suffixes[j])
	})
	return suffixes
}

func newCPUUsageSampler() *cpuUsageSampler {
	return &cpuUsageSampler{}
}

func (s *cpuUsageSampler) CPUUsage() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.lastReadAt.IsZero() && now.Sub(s.lastReadAt) < time.Second {
		return s.lastValue
	}
	total, idle, err := readCPUStat()
	if err != nil {
		return s.lastValue
	}
	if s.lastTotal == 0 || total <= s.lastTotal {
		s.lastReadAt = now
		s.lastTotal = total
		s.lastIdle = idle
		return s.lastValue
	}
	totalDelta := float64(total - s.lastTotal)
	idleDelta := float64(idle - s.lastIdle)
	usage := 100.0 * (1 - idleDelta/totalDelta)
	if math.IsNaN(usage) || math.IsInf(usage, 0) {
		usage = s.lastValue
	}
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	s.lastReadAt = now
	s.lastTotal = total
	s.lastIdle = idle
	s.lastValue = usage
	return usage
}

func readCPUStat() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("unexpected /proc/stat cpu line")
	}
	var total uint64
	var values [8]uint64
	for i := 1; i < len(fields) && i <= len(values); i++ {
		if _, err := fmt.Sscanf(fields[i], "%d", &values[i-1]); err != nil {
			return 0, 0, fmt.Errorf("parse /proc/stat: %w", err)
		}
		total += values[i-1]
	}
	idle := values[3] + values[4]
	return total, idle, nil
}
