package server

import (
	"log"

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
	reply := &dns.Msg{}
	monitor.Rec53Metric.InCounterAdd("start", r.Question[0].Name, dns.TypeToString[r.Question[0].Qtype])
	stm := newStateInitState(r, reply)
	if _, err := Change(stm); err != nil {
		monitor.Rec53Log.Errorf("Change state error: %s", err.Error())
	}
	monitor.Rec53Metric.OutCounterAdd("end", reply.Question[0].Name, dns.TypeToString[reply.Question[0].Qtype], dns.RcodeToString[reply.Rcode])
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
