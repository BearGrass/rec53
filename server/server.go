package server

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type server struct {
	listen    string
	warmupCfg WarmupConfig
	udpSrv    *dns.Server
	tcpSrv    *dns.Server
	wg        sync.WaitGroup
	errChan   chan error
}

func NewServer(listen string) *server {
	// Disable warmup by default for test compatibility
	// Tests that need warmup can use NewServerWithConfig
	warmupCfg := DefaultWarmupConfig
	warmupCfg.Enabled = false
	return &server{
		listen:    listen,
		warmupCfg: warmupCfg,
	}
}

// NewServerWithConfig creates a new server with both listen address and warmup config
func NewServerWithConfig(listen string, warmupCfg WarmupConfig) *server {
	return &server{
		listen:    listen,
		warmupCfg: warmupCfg,
	}
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

	monitor.Rec53Metric.InCounterAdd("request", r.Question[0].Name, dns.TypeToString[r.Question[0].Qtype])
	stm := newStateInitState(r, reply)
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

	monitor.Rec53Metric.OutCounterAdd("response", reply.Question[0].Name, dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode])
	monitor.Rec53Metric.LatencyHistogramObserve("latency", reply.Question[0].Name, dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode], float64(time.Since(startTime).Milliseconds()))
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
// The channel is closed when both UDP and TCP servers have stopped.
// If warmup is enabled, it runs in the background without blocking startup.
func (s *server) Run() <-chan error {
	// Create servers before starting goroutines to avoid race with Shutdown()
	s.udpSrv = &dns.Server{Addr: s.listen, Net: "udp", Handler: s}
	s.tcpSrv = &dns.Server{Addr: s.listen, Net: "tcp", Handler: s}

	// Start background IP probe loop for fault recovery
	globalIPPool.StartProbeLoop()

	// Start background NS warmup if enabled (non-blocking)
	if s.warmupCfg.Enabled {
		go s.warmupNSOnStartup()
	}

	s.errChan = make(chan error, 2)
	s.wg.Add(2)

	go func() {
		defer s.wg.Done()
		if err := s.udpSrv.ListenAndServe(); err != nil {
			s.errChan <- fmt.Errorf("udp listener: %w", err)
		}
	}()

	go func() {
		defer s.wg.Done()
		if err := s.tcpSrv.ListenAndServe(); err != nil {
			s.errChan <- fmt.Errorf("tcp listener: %w", err)
		}
	}()

	// Close errChan when both servers have stopped
	go func() {
		s.wg.Wait()
		close(s.errChan)
	}()

	return s.errChan
}

// warmupNSOnStartup runs NS warmup in the background on startup.
// It does not block server startup or query handling.
// The warmup process runs for at most s.warmupCfg.Duration, and all goroutines
// are cancelled when the deadline is reached.
// Panics during warmup are caught and logged to prevent server crashes.
func (s *server) warmupNSOnStartup() {
	defer func() {
		if r := recover(); r != nil {
			monitor.Rec53Log.Warnf("Panic during NS warmup (non-fatal): %v", r)
		}
	}()

	// Create a context with the configured duration as the hard deadline
	ctx, cancel := context.WithTimeout(context.Background(), s.warmupCfg.Duration)
	defer cancel()

	monitor.Rec53Log.Infof("Starting NS warmup with %d TLDs, concurrency: %d (CPU cores: %d)...", len(s.warmupCfg.TLDs), s.warmupCfg.Concurrency, runtime.NumCPU())
	WarmupNSRecords(ctx, s.warmupCfg)
	// Stats are logged inside WarmupNSRecords()
}

// Shutdown gracefully shuts down the DNS server
func (s *server) Shutdown(ctx context.Context) error {
	var errs []error

	if s.udpSrv != nil {
		if err := s.udpSrv.ShutdownContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if s.tcpSrv != nil {
		if err := s.tcpSrv.ShutdownContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	s.wg.Wait()

	// Shutdown IP pool prefetch goroutines
	if err := globalIPPool.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// UDPAddr returns the UDP server's listening address.
// Returns empty string if server is not running.
func (s *server) UDPAddr() string {
	if s.udpSrv != nil && s.udpSrv.PacketConn != nil {
		return s.udpSrv.PacketConn.LocalAddr().String()
	}
	return ""
}

// TCPAddr returns the TCP server's listening address.
// Returns empty string if server is not running.
func (s *server) TCPAddr() string {
	if s.tcpSrv != nil && s.tcpSrv.Listener != nil {
		return s.tcpSrv.Listener.Addr().String()
	}
	return ""
}
