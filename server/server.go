package server

import (
	"log"

	"rec53/logger"

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
	stm := newStateInitState(r, reply)
	if _, err := Change(stm); err != nil {
		logger.Rec53Log.Sugar().Errorf("Change state error: %s", err.Error())
	}
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
