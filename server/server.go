package server

import (
	"log"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type server struct {
	listen string
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
	go func() {
		srv := &dns.Server{Addr: s.listen, Net: "udp", Handler: s}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{Addr: s.listen, Net: "tcp", Handler: s}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()
}
