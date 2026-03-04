package server

import (
	"context"
	"log"
	"sync"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type server struct {
	listen string
	udpSrv *dns.Server
	tcpSrv *dns.Server
	wg     sync.WaitGroup
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

func (s *server) Run() {
	s.wg.Add(2)

	go func() {
		defer s.wg.Done()
		s.udpSrv = &dns.Server{Addr: s.listen, Net: "udp", Handler: s}
		if err := s.udpSrv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	go func() {
		defer s.wg.Done()
		s.tcpSrv = &dns.Server{Addr: s.listen, Net: "tcp", Handler: s}
		if err := s.tcpSrv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()
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

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
