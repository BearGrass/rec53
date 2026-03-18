package server

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"rec53/monitor"
	"rec53/utils"

	"github.com/miekg/dns"
)

type server struct {
	listen       string
	listeners    int // number of UDP+TCP listener pairs; >1 enables SO_REUSEPORT
	warmupCfg    WarmupConfig
	snapshotCfg  SnapshotConfig
	hostsMap     map[string]*dns.Msg // pre-compiled "fqdn:qtype" → authoritative response
	hostsNames   map[string]bool     // set of FQDNs in hosts (for NODATA detection)
	forwardZones []ForwardZone       // zones sorted by zone length desc (longest match first)
	xdpLoader    *XDPLoader          // eBPF lifecycle manager; nil when XDP disabled
	udpSrvs      []*dns.Server
	tcpSrvs      []*dns.Server
	wg           sync.WaitGroup
	errChan      chan error
	udpReady     chan struct{}      // closed when first UDP listener has started
	tcpReady     chan struct{}      // closed when first TCP listener has started
	udpAddr      string             // set once UDP server is ready; safe to read after Run() returns
	tcpAddr      string             // set once TCP server is ready; safe to read after Run() returns
	warmupCancel context.CancelFunc // cancels the warmup goroutine; nil if warmup disabled
}

func NewServer(listen string) *server {
	// Disable warmup by default for test compatibility
	// Tests that need warmup can use NewServerWithConfig
	warmupCfg := DefaultWarmupConfig
	warmupCfg.Enabled = false
	return &server{
		listen:    listen,
		listeners: 1,
		warmupCfg: warmupCfg,
	}
}

// NewServerWithConfig creates a new server with both listen address and warmup config
func NewServerWithConfig(listen string, warmupCfg WarmupConfig) *server {
	return &server{
		listen:    listen,
		listeners: 1,
		warmupCfg: warmupCfg,
	}
}

// NewServerWithFullConfig creates a server with hosts, forwarding, XDP, and listener configuration.
// Hosts entries are pre-compiled into a lookup map; forwarding zones are sorted by
// zone length descending for longest-suffix matching.
// listeners controls the number of UDP+TCP listener pairs; values ≤1 mean a single
// pair without SO_REUSEPORT; values >1 enable SO_REUSEPORT with N parallel pairs.
// xdpInterface controls the XDP/eBPF cache fast path; when non-empty,
// cache hits are served directly from the kernel via XDP_TX (requires root/CAP_BPF).
func NewServerWithFullConfig(listen string, listeners int, warmupCfg WarmupConfig, snapshotCfg SnapshotConfig, hosts []HostEntry, forwarding []ForwardZone, xdpInterface string) *server {
	hostsMap, hostsNames := compileHostsEntries(hosts)
	fwdZones := sortForwardZones(forwarding)
	if listeners < 1 {
		listeners = 1
	}
	s := &server{
		listen:       listen,
		listeners:    listeners,
		warmupCfg:    warmupCfg,
		snapshotCfg:  snapshotCfg,
		hostsMap:     hostsMap,
		hostsNames:   hostsNames,
		forwardZones: fwdZones,
	}
	// Create XDP loader if interface is specified.
	if xdpInterface != "" {
		s.xdpLoader = NewXDPLoader(xdpInterface)
	}
	setGlobalHostsAndForward(hostsMap, hostsNames, fwdZones)
	return s
}

func (s *server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	startTime := time.Now()
	reply := &dns.Msg{}

	// Guard against malformed requests with no question (QDCOUNT=0) to prevent panic
	if len(r.Question) == 0 {
		reply.SetRcode(r, dns.RcodeFormatError)
		w.WriteMsg(reply)
		return
	}

	// Save original question before any modifications by state machine
	var originalQuestion dns.Question
	if len(r.Question) > 0 {
		originalQuestion = r.Question[0]
	}

	monitor.Rec53Metric.InCounterAdd("request", dns.TypeToString[r.Question[0].Qtype])
	stm := newStateInitState(r, reply, context.Background())
	result, err := Change(stm)
	if err != nil {
		monitor.Rec53Log.Errorf("Change state error: %s", err.Error())
		// Return SERVFAIL on error
		reply.SetRcode(r, dns.RcodeServerFailure)
	} else {
		reply = result
	}

	// Restore original question to ensure response matches query
	// This handles all cases including early returns and error paths
	if len(originalQuestion.Name) > 0 {
		if len(reply.Question) == 0 {
			reply.Question = make([]dns.Question, 1)
		}
		reply.Question[0] = originalQuestion
	}

	// Handle truncation for UDP responses
	if isUDP(w) {
		reply = truncateResponse(reply, r, getMaxUDPSize(r))
	}

	monitor.Rec53Metric.OutCounterAdd("response", dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode])
	monitor.Rec53Metric.LatencyHistogramObserve("latency", dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode], float64(time.Since(startTime).Milliseconds()))
	if err := w.WriteMsg(reply); err != nil {
		monitor.Rec53Log.Errorf("Failed to write response: %v", err)
	}
}

// isUDP checks if the connection is UDP
func isUDP(w dns.ResponseWriter) bool {
	_, ok := w.RemoteAddr().(*net.UDPAddr)
	return ok
}

// getMaxUDPSize returns the maximum UDP response size from EDNS0 or default
func getMaxUDPSize(r *dns.Msg) int {
	const defaultUDPSize = 512
	if opt := r.IsEdns0(); opt != nil {
		size := int(opt.UDPSize())
		if size > 0 {
			return size
		}
	}
	return defaultUDPSize
}

// truncateResponse truncates a DNS response if it exceeds the maximum UDP size
func truncateResponse(reply, request *dns.Msg, maxSize int) *dns.Msg {
	// Check if response fits
	if reply.Len() <= maxSize {
		return reply
	}

	// Set truncated flag
	reply.Truncated = true

	// Try to fit as much as possible by removing answer records
	// Keep removing from the end until it fits or no more answers
	for len(reply.Answer) > 0 && reply.Len() > maxSize {
		reply.Answer = reply.Answer[:len(reply.Answer)-1]
	}

	// If still too large, clear extra section
	if reply.Len() > maxSize {
		reply.Extra = nil
	}

	// If still too large, clear answer section completely
	if reply.Len() > maxSize {
		reply.Answer = nil
	}

	// Clear authoritative section for truncated responses
	// This follows DNS protocol best practices
	reply.Ns = nil

	monitor.Rec53Log.Debugf("Response truncated: original size exceeded %d bytes", maxSize)

	return reply
}

// Run starts the DNS server listeners and returns an error channel.
// Errors during startup or runtime are sent to the returned channel.
// The channel is closed when all servers have stopped.
// When listeners > 1, SO_REUSEPORT is enabled and each listener pair gets
// its own kernel receive queue for parallel packet distribution.
// If warmup is enabled, it runs in the background without blocking startup.
func (s *server) Run() <-chan error {
	n := s.listeners
	if n < 1 {
		n = 1
	}
	reusePort := n > 1

	// Create ready channels and server slices before starting goroutines
	s.udpReady = make(chan struct{})
	s.tcpReady = make(chan struct{})
	var udpOnce, tcpOnce sync.Once

	s.udpSrvs = make([]*dns.Server, n)
	s.tcpSrvs = make([]*dns.Server, n)

	for i := 0; i < n; i++ {
		s.udpSrvs[i] = &dns.Server{
			Addr:      s.listen,
			Net:       "udp",
			Handler:   s,
			ReusePort: reusePort,
		}
		s.tcpSrvs[i] = &dns.Server{
			Addr:      s.listen,
			Net:       "tcp",
			Handler:   s,
			ReusePort: reusePort,
		}
	}

	// Wire NotifyStartedFunc for each listener; sync.Once ensures ready
	// channels are closed exactly once by whichever listener binds first.
	for i := 0; i < n; i++ {
		udp := s.udpSrvs[i]
		udp.NotifyStartedFunc = func() {
			if udp.PacketConn != nil {
				udpOnce.Do(func() {
					s.udpAddr = udp.PacketConn.LocalAddr().String()
					close(s.udpReady)
				})
			}
		}

		tcp := s.tcpSrvs[i]
		tcp.NotifyStartedFunc = func() {
			if tcp.Listener != nil {
				tcpOnce.Do(func() {
					s.tcpAddr = tcp.Listener.Addr().String()
					close(s.tcpReady)
				})
			}
		}
	}

	// Start background IP probe loop for fault recovery
	globalIPPool.StartProbeLoop(utils.ExtractRootIPs())

	// Initialize XDP/eBPF cache fast path if a loader was configured.
	// This must happen before DNS listeners start so cache hits can be
	// served via XDP_TX from the first query.
	// Failure to attach is non-fatal: the server degrades to Go-only cache.
	if s.xdpLoader != nil {
		if err := s.xdpLoader.LoadAndAttach(); err != nil {
			hint := classifyXDPError(err)
			monitor.Rec53Log.Warnf("[XDP] failed to initialize, degrading to Go-only cache mode: %v", err)
			monitor.Rec53Log.Warnf("[XDP] %s", hint)
			s.xdpLoader = nil
			monitor.XDPStatus.Set(0)
		} else {
			globalXDPCacheMap.Store(s.xdpLoader.CacheMap())
			monitor.Rec53Log.Infof("[XDP] cache fast path active on %s", s.xdpLoader.iface)
			monitor.XDPStatus.Set(1)
		}
	} else {
		monitor.Rec53Log.Infof("[XDP] disabled by config, running in Go-only cache mode")
		monitor.XDPStatus.Set(0)
	}

	// Start background NS warmup if enabled (non-blocking).
	// warmupCancel lets Shutdown() stop warmup immediately without waiting for
	// the per-query or overall Duration deadline to expire.
	if s.warmupCfg.Enabled {
		warmupCtx, warmupCancel := context.WithCancel(context.Background())
		s.warmupCancel = warmupCancel
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.warmupNSOnStartup(warmupCtx)
		}()
	}

	s.errChan = make(chan error, 2*n)
	s.wg.Add(2 * n)

	for i := 0; i < n; i++ {
		udp := s.udpSrvs[i]
		tcp := s.tcpSrvs[i]
		idx := i

		go func() {
			defer s.wg.Done()
			if err := udp.ListenAndServe(); err != nil {
				s.errChan <- fmt.Errorf("udp listener[%d]: %w", idx, err)
			}
		}()

		go func() {
			defer s.wg.Done()
			if err := tcp.ListenAndServe(); err != nil {
				s.errChan <- fmt.Errorf("tcp listener[%d]: %w", idx, err)
			}
		}()
	}

	// Close errChan when all servers have stopped
	go func() {
		s.wg.Wait()
		close(s.errChan)
	}()

	// Wait for at least the first UDP and TCP server to be ready before returning.
	// This ensures UDPAddr() and TCPAddr() are safe to call immediately after Run().
	<-s.udpReady
	<-s.tcpReady

	if reusePort {
		monitor.Rec53Log.Infof("Started %d UDP + %d TCP listeners with SO_REUSEPORT on %s", n, n, s.listen)
	}

	return s.errChan
}

// warmupNSOnStartup runs NS warmup in the background on startup.
// It does not block server startup or query handling.
// ctx is derived from the server lifecycle — cancelling it (via Shutdown) stops warmup immediately.
// The warmup process also applies s.warmupCfg.Duration as a hard per-warmup deadline.
// Panics during warmup are caught and logged to prevent server crashes.
func (s *server) warmupNSOnStartup(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			monitor.Rec53Log.Warnf("Panic during NS warmup (non-fatal): %v", r)
		}
	}()

	// Apply the configured Duration as the hard deadline, but respect external cancellation too.
	ctx, cancel := context.WithTimeout(ctx, s.warmupCfg.Duration)
	defer cancel()

	monitor.Rec53Log.Infof("Starting NS warmup with %d TLDs, concurrency: %d (CPU cores: %d)...", len(s.warmupCfg.TLDs), s.warmupCfg.Concurrency, runtime.NumCPU())
	WarmupNSRecords(ctx, s.warmupCfg)
	// Stats are logged inside WarmupNSRecords()
}

// Shutdown gracefully shuts down the DNS server
func (s *server) Shutdown(ctx context.Context) error {
	// Cancel warmup goroutine first so it stops issuing DNS queries before
	// we tear down the IP pool.
	if s.warmupCancel != nil {
		s.warmupCancel()
	}

	var errs []error

	for _, srv := range s.udpSrvs {
		if srv != nil {
			if err := srv.ShutdownContext(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, srv := range s.tcpSrvs {
		if srv != nil {
			if err := srv.ShutdownContext(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}

	s.wg.Wait()

	// Shutdown IP pool prefetch goroutines
	if err := globalIPPool.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}

	// Detach XDP program and close eBPF objects.
	// Must happen after DNS listeners are stopped (no more cache writes)
	// but before snapshot (snapshot doesn't need BPF map).
	if s.xdpLoader != nil {
		globalXDPCacheMap.Store(nil)
		if err := s.xdpLoader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("[XDP] close: %w", err))
		}
		s.xdpLoader = nil
		monitor.XDPStatus.Set(0)
		monitor.Rec53Log.Infof("[XDP] detached and cleaned up")
	}

	// Write cache snapshot on graceful shutdown (SIGTERM, SIGINT, or programmatic Shutdown).
	// Runs after listeners and IP pool are stopped to avoid concurrent cache writes.
	// Write failures are logged but do not affect the Shutdown return value.
	if s.snapshotCfg.Enabled {
		if err := SaveSnapshot(s.snapshotCfg); err != nil {
			monitor.Rec53Log.Errorf("[SNAPSHOT] failed to save snapshot on shutdown: %v", err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// UDPAddr returns the UDP server's listening address.
// Returns empty string if server is not running.
// Safe to call immediately after Run() returns.
func (s *server) UDPAddr() string {
	return s.udpAddr
}

// WaitUntilReady blocks until the UDP server has started listening.
// Run() already calls this internally, so this is only needed if you
// need to synchronize with the server from a separate goroutine.
func (s *server) WaitUntilReady() {
	<-s.udpReady
}

// TCPAddr returns the TCP server's listening address.
// Returns empty string if server is not running.
// Safe to call immediately after Run() returns.
func (s *server) TCPAddr() string {
	return s.tcpAddr
}

// XDPLoaderForTest returns the XDP loader for testing purposes.
// Returns nil if XDP is not enabled.
func (s *server) XDPLoaderForTest() *XDPLoader {
	return s.xdpLoader
}
