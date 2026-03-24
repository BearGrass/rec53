package server

import (
	"context"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

const (
	ExpensiveRequestLimitModeDisabled = "disabled"
	ExpensiveRequestLimitModeEnabled  = "enabled"
)

const expensiveRequestLogInterval = 30 * time.Second

type ExpensiveRequestLimitConfig struct {
	Mode                string   `yaml:"expensive_request_limit_mode"`
	Limit               int      `yaml:"expensive_request_limit"`
	HotZoneBaseSuffixes []string `yaml:"hot_zone_base_suffixes"`
	ObserveWouldRefuse  bool     `yaml:"-"`
}

type expensivePath uint8

const (
	expensivePathForward expensivePath = iota
	expensivePathIterative
)

type expensiveRequestLimiter struct {
	mode               string
	limit              int
	shards             []expensiveRequestShard
	observeWouldRefuse bool
	hotZone            *hotZoneController
}

type expensiveRequestShard struct {
	mu      sync.Mutex
	clients map[netip.Addr]*clientState
}

type clientState struct {
	inflight        int
	lastLogAt       time.Time
	suppressedCount int
}

type expensiveRequestLimitEvent struct {
	shouldLog       bool
	action          string
	path            expensivePath
	inflight        int
	limit           int
	suppressedCount int
	clientIP        netip.Addr
}

type expensiveRequestHolder struct {
	mu            sync.Mutex
	clientIP      netip.Addr
	held          bool
	hotZoneHeld   bool
	hotZoneTicket uint64
}

const contextKeyExpensiveRequestHolder contextKeyType = "expensiveRequestHolder"
const contextKeyExpensiveRequestLimiter contextKeyType = "expensiveRequestLimiter"

func normalizeExpensiveRequestLimitConfig(cfg ExpensiveRequestLimitConfig) ExpensiveRequestLimitConfig {
	if cfg.Mode == "" {
		cfg.Mode = ExpensiveRequestLimitModeDisabled
	}
	if cfg.Limit <= 0 {
		cfg.Limit = runtime.NumCPU()
		if cfg.Limit < 1 {
			cfg.Limit = 1
		}
	}
	return cfg
}

func newExpensiveRequestLimiter(cfg ExpensiveRequestLimitConfig) *expensiveRequestLimiter {
	cfg = normalizeExpensiveRequestLimitConfig(cfg)
	if cfg.Mode != ExpensiveRequestLimitModeEnabled {
		return nil
	}
	shardCount := runtime.NumCPU()
	if shardCount < 1 {
		shardCount = 1
	}
	shards := make([]expensiveRequestShard, shardCount)
	for i := range shards {
		shards[i].clients = make(map[netip.Addr]*clientState)
	}
	return &expensiveRequestLimiter{
		mode:               cfg.Mode,
		limit:              cfg.Limit,
		shards:             shards,
		observeWouldRefuse: cfg.ObserveWouldRefuse,
		hotZone:            newHotZoneController(cfg.HotZoneBaseSuffixes),
	}
}

func newExpensiveRequestHolder(clientIP netip.Addr) *expensiveRequestHolder {
	return &expensiveRequestHolder{clientIP: clientIP}
}

func withExpensiveRequestLimiter(ctx context.Context, limiter *expensiveRequestLimiter) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if limiter == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKeyExpensiveRequestLimiter, limiter)
}

func withExpensiveRequestHolder(ctx context.Context, holder *expensiveRequestHolder) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if holder == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKeyExpensiveRequestHolder, holder)
}

func expensiveRequestLimiterFromContext(ctx context.Context) *expensiveRequestLimiter {
	if ctx == nil {
		return nil
	}
	limiter, _ := ctx.Value(contextKeyExpensiveRequestLimiter).(*expensiveRequestLimiter)
	return limiter
}

func expensiveRequestHolderFromContext(ctx context.Context) *expensiveRequestHolder {
	if ctx == nil {
		return nil
	}
	holder, _ := ctx.Value(contextKeyExpensiveRequestHolder).(*expensiveRequestHolder)
	return holder
}

func tryAcquireExpensiveRequest(ctx context.Context, path expensivePath, qname, matchedForwardZone string) bool {
	limiter := expensiveRequestLimiterFromContext(ctx)
	holder := expensiveRequestHolderFromContext(ctx)
	if limiter == nil || holder == nil {
		return true
	}
	return holder.TryAcquire(limiter, path, qname, matchedForwardZone)
}

func (h *expensiveRequestHolder) TryAcquire(limiter *expensiveRequestLimiter, path expensivePath, qname, matchedForwardZone string) bool {
	if h == nil || limiter == nil || !h.clientIP.IsValid() {
		return true
	}

	h.mu.Lock()
	if h.held || h.hotZoneHeld {
		h.mu.Unlock()
		return true
	}
	h.mu.Unlock()

	hotZoneAllowed, hotZoneTicket := limiter.TryAcquireHotZone(qname, matchedForwardZone)
	if !hotZoneAllowed {
		return false
	}

	allowed := limiter.TryAcquire(h.clientIP, path)
	if !allowed {
		limiter.ReleaseHotZone(hotZoneTicket)
		return false
	}

	h.mu.Lock()
	if h.held || h.hotZoneHeld {
		h.mu.Unlock()
		limiter.Release(h.clientIP)
		limiter.ReleaseHotZone(hotZoneTicket)
		return true
	}
	h.held = true
	h.hotZoneHeld = true
	h.hotZoneTicket = hotZoneTicket
	h.mu.Unlock()
	return true
}

func (h *expensiveRequestHolder) ReleaseIfHeld(limiter *expensiveRequestLimiter) {
	if h == nil || limiter == nil || !h.clientIP.IsValid() {
		return
	}

	h.mu.Lock()
	if !h.held && !h.hotZoneHeld {
		h.mu.Unlock()
		return
	}
	hotZoneHeld := h.hotZoneHeld
	hotZoneTicket := h.hotZoneTicket
	h.held = false
	h.hotZoneHeld = false
	h.hotZoneTicket = 0
	h.mu.Unlock()

	limiter.Release(h.clientIP)
	if hotZoneHeld {
		limiter.ReleaseHotZone(hotZoneTicket)
	}
}

func (l *expensiveRequestLimiter) TryAcquire(clientIP netip.Addr, path expensivePath) bool {
	if l == nil || l.mode != ExpensiveRequestLimitModeEnabled || !clientIP.IsValid() {
		return true
	}

	shard := l.shardFor(clientIP)
	var event expensiveRequestLimitEvent

	shard.mu.Lock()
	state := shard.clients[clientIP]
	if state == nil {
		state = &clientState{}
		shard.clients[clientIP] = state
	}
	if state.inflight >= l.limit {
		action := "refused"
		allowed := false
		if l.observeWouldRefuse {
			action = "would_refuse"
			allowed = true
		}
		event = buildExpensiveRequestLimitEvent(state, clientIP, path, action, l.limit)
		shard.mu.Unlock()
		recordExpensiveRequestLimitEvent(event)
		return allowed
	}
	state.inflight++
	shard.mu.Unlock()
	return true
}

func (l *expensiveRequestLimiter) Release(clientIP netip.Addr) {
	if l == nil || l.mode != ExpensiveRequestLimitModeEnabled || !clientIP.IsValid() {
		return
	}

	shard := l.shardFor(clientIP)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	state := shard.clients[clientIP]
	if state == nil {
		return
	}
	if state.inflight > 0 {
		state.inflight--
	}
	if state.inflight == 0 {
		delete(shard.clients, clientIP)
	}
}

func (l *expensiveRequestLimiter) TryAcquireHotZone(qname, matchedForwardZone string) (bool, uint64) {
	if l == nil || l.mode != ExpensiveRequestLimitModeEnabled || l.hotZone == nil {
		return true, 0
	}
	return l.hotZone.TryEnter(qname, matchedForwardZone)
}

func (l *expensiveRequestLimiter) ReleaseHotZone(ticket uint64) {
	if l == nil || l.mode != ExpensiveRequestLimitModeEnabled || l.hotZone == nil {
		return
	}
	l.hotZone.Release(ticket)
}

func (l *expensiveRequestLimiter) shardFor(clientIP netip.Addr) *expensiveRequestShard {
	idx := expensiveRequestShardIndex(clientIP, len(l.shards))
	return &l.shards[idx]
}

func buildExpensiveRequestLimitEvent(state *clientState, clientIP netip.Addr, path expensivePath, action string, limit int) expensiveRequestLimitEvent {
	now := time.Now()
	event := expensiveRequestLimitEvent{
		action:   action,
		path:     path,
		inflight: state.inflight,
		limit:    limit,
		clientIP: clientIP,
	}
	if state.lastLogAt.IsZero() || now.Sub(state.lastLogAt) >= expensiveRequestLogInterval {
		event.shouldLog = true
		event.suppressedCount = state.suppressedCount
		state.lastLogAt = now
		state.suppressedCount = 0
		return event
	}
	state.suppressedCount++
	return event
}

func recordExpensiveRequestLimitEvent(event expensiveRequestLimitEvent) {
	if event.action == "" {
		return
	}
	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.ExpensiveRequestLimitAdd(event.action, event.path.metricLabel())
	}
	if !event.shouldLog || monitor.Rec53Log == nil {
		return
	}
	monitor.Rec53Log.Warnf("[LIMIT] expensive request %s for client %s (path=%s inflight=%d limit=%d suppressed=%d)",
		event.action,
		event.clientIP.String(),
		event.path.logLabel(),
		event.inflight,
		event.limit,
		event.suppressedCount,
	)
}

func expensiveRequestShardIndex(clientIP netip.Addr, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	b := clientIP.As16()
	var hash uint32 = 2166136261
	for _, v := range b {
		hash ^= uint32(v)
		hash *= 16777619
	}
	return int(hash % uint32(shardCount))
}

func (p expensivePath) metricLabel() string {
	switch p {
	case expensivePathForward:
		return "forward"
	case expensivePathIterative:
		return "iterative"
	default:
		return "unknown"
	}
}

func (p expensivePath) logLabel() string {
	return p.metricLabel()
}

func extractClientIP(addr net.Addr) netip.Addr {
	if addr == nil {
		return netip.Addr{}
	}
	switch a := addr.(type) {
	case *net.UDPAddr:
		if ip, ok := netip.AddrFromSlice(a.IP); ok {
			return ip.Unmap()
		}
	case *net.TCPAddr:
		if ip, ok := netip.AddrFromSlice(a.IP); ok {
			return ip.Unmap()
		}
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return netip.Addr{}
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			return ip.Unmap()
		}
	}
	return netip.Addr{}
}

func buildRefusedResponse(request *dns.Msg) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetRcode(request, dns.RcodeRefused)
	return msg
}
