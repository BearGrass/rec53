package utils

import (
	"net"

	"github.com/miekg/dns"
)

func GetRootGlue() *dns.Msg {
	rootGlue := new(dns.Msg)
	rootGlue.SetUpdate(".")
	rootGlue.Ns = make([]dns.RR, 0)
	rootGlueHeader := dns.RR_Header{
		Name:   ".",
		Rrtype: dns.TypeNS,
		Class:  dns.ClassINET,
		Ttl:    0,
	}
	rootGlue.Ns = append(rootGlue.Ns, []dns.RR{
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "a.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "b.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "c.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "d.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "e.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "f.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "g.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "h.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "i.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "j.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "k.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "l.root-servers.net.",
		},
		&dns.NS{
			Hdr: rootGlueHeader,
			Ns:  "m.root-servers.net.",
		},
	}...)
	rootGlue.Extra = make([]dns.RR, 0)
	rootGlue.Extra = append(rootGlue.Extra, []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "a.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("198.41.0.4"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "b.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("199.9.14.201"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "c.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("192.33.4.12"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "d.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("199.7.91.13"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "e.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("192.203.230.10"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "f.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("192.5.5.241"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "g.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("192.112.36.4"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "h.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("198.97.190.53"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "i.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("192.36.148.17"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "j.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("192.58.128.30"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "k.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("193.0.14.129"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "l.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("199.7.83.42"),
		},
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "m.root-servers.net.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP("202.12.27.33"),
		},
	}...)
	return rootGlue
}
