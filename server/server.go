package server

import (
	"log"

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
	w.WriteMsg(r)
	return
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
