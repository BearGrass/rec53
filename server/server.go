package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type server struct {
	listen  string
	udpSrv  *dns.Server
	tcpSrv  *dns.Server
	wg      sync.WaitGroup
	errChan chan error
}

func NewServer(listen string) *server {
	return &server{
		listen: listen,
	}
}

func (s *server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	startTime := time.Now()
	reply := &dns.Msg{}
	monitor.Rec53Metric.InCounterAdd("request", r.Question[0].Name, dns.TypeToString[r.Question[0].Qtype])
	stm := newStateInitState(r, reply)
	if _, err := Change(stm); err != nil {
		monitor.Rec53Log.Errorf("Change state error: %s", err.Error())
	}
	monitor.Rec53Metric.OutCounterAdd("response", reply.Question[0].Name, dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode])
	monitor.Rec53Metric.LatencyHistogramObserve("latency", reply.Question[0].Name, dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode], float64(time.Since(startTime).Milliseconds()))
	w.WriteMsg(reply)
}

// Run starts the DNS server listeners and returns an error channel.
// Errors during startup or runtime are sent to the returned channel.
// The channel is closed when both UDP and TCP servers have stopped.
func (s *server) Run() <-chan error {
	// Create servers before starting goroutines to avoid race with Shutdown()
	s.udpSrv = &dns.Server{Addr: s.listen, Net: "udp", Handler: s}
	s.tcpSrv = &dns.Server{Addr: s.listen, Net: "tcp", Handler: s}

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
