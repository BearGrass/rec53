package server

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// HostEntry represents a single static DNS record in the hosts configuration.
// It is used to answer queries locally without hitting cache or upstream resolvers.
type HostEntry struct {
	// Name is the fully-qualified domain name (FQDN). Trailing dot is optional;
	// it is normalized to a canonical FQDN on load.
	Name string `yaml:"name"`

	// Type is the DNS record type. Supported values: "A", "AAAA", "CNAME".
	Type string `yaml:"type"`

	// Value is the record data:
	//   - For A records: an IPv4 address (e.g. "10.0.0.1")
	//   - For AAAA records: an IPv6 address (e.g. "::1")
	//   - For CNAME records: the target FQDN (e.g. "real.internal")
	Value string `yaml:"value"`

	// TTL is the time-to-live in seconds for the synthesised response.
	// Defaults to 60 if not set.
	TTL uint32 `yaml:"ttl"`
}

const defaultHostsTTL uint32 = 60

// compileHostsEntries pre-compiles host entries into a lookup map and a name set.
// The map key uses the same format as the DNS cache: "fqdn:qtypeNum".
// Multiple entries for the same name+type are merged into a single dns.Msg.
// The name set tracks which FQDNs exist in hosts (regardless of type) for NODATA detection.
func compileHostsEntries(hosts []HostEntry) (map[string]*dns.Msg, map[string]bool) {
	hostsMap := make(map[string]*dns.Msg, len(hosts))
	hostsNames := make(map[string]bool, len(hosts))

	for _, h := range hosts {
		fqdn := dns.Fqdn(h.Name)
		hostsNames[fqdn] = true

		ttl := h.TTL
		if ttl == 0 {
			ttl = defaultHostsTTL
		}

		rr := buildHostRR(fqdn, h.Type, h.Value, ttl)
		if rr == nil {
			continue
		}

		qtype := dns.StringToType[strings.ToUpper(h.Type)]
		key := fmt.Sprintf("%s:%d", fqdn, qtype)

		if existing, ok := hostsMap[key]; ok {
			existing.Answer = append(existing.Answer, rr)
		} else {
			msg := new(dns.Msg)
			msg.Authoritative = true
			msg.Answer = append(msg.Answer, rr)
			hostsMap[key] = msg
		}
	}

	return hostsMap, hostsNames
}

func buildHostRR(fqdn, rrType, value string, ttl uint32) dns.RR {
	switch strings.ToUpper(rrType) {
	case "A":
		return &dns.A{
			Hdr: dns.RR_Header{Name: fqdn, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
			A:   net.ParseIP(value).To4(),
		}
	case "AAAA":
		return &dns.AAAA{
			Hdr:  dns.RR_Header{Name: fqdn, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
			AAAA: net.ParseIP(value),
		}
	case "CNAME":
		return &dns.CNAME{
			Hdr:    dns.RR_Header{Name: fqdn, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
			Target: dns.Fqdn(value),
		}
	default:
		return nil
	}
}
