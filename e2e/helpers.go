// Package e2e provides end-to-end integration tests for the rec53 DNS resolver.
// These tests validate the complete system behavior including network interactions.
package e2e

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// MockAuthorityServer simulates a DNS authority server for testing.
// It can be configured to return specific responses or simulate various DNS scenarios.
type MockAuthorityServer struct {
	server    *dns.Server
	addr      string
	zone      *Zone
	mu        sync.RWMutex
	reqCount  int64
	questions []dns.Question
}

// Zone represents a DNS zone with records.
type Zone struct {
	Origin   string
	Records  map[uint16][]dns.RR
	NSEC     bool // Enable NSEC for negative responses
	Referral bool // Return referral instead of answer
}

// NewMockAuthorityServer creates a new mock DNS server.
func NewMockAuthorityServer(t *testing.T, zone *Zone) *MockAuthorityServer {
	m := &MockAuthorityServer{
		zone: zone,
	}

	m.server = &dns.Server{
		Addr:    "127.0.0.1:0",
		Net:     "udp",
		Handler: dns.HandlerFunc(m.handleDNS),
	}

	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			t.Logf("mock server stopped: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Get actual address
	if m.server.PacketConn != nil {
		m.addr = m.server.PacketConn.LocalAddr().String()
	}

	return m
}

// Addr returns the server's listening address.
func (m *MockAuthorityServer) Addr() string {
	return m.addr
}

// Stop shuts down the mock server.
func (m *MockAuthorityServer) Stop() {
	if m.server != nil {
		m.server.Shutdown()
	}
}

// RequestCount returns the number of requests received.
func (m *MockAuthorityServer) RequestCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reqCount
}

// Questions returns all questions received by the server.
func (m *MockAuthorityServer) Questions() []dns.Question {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]dns.Question{}, m.questions...)
}

// handleDNS handles incoming DNS queries.
func (m *MockAuthorityServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m.mu.Lock()
	m.reqCount++
	m.questions = append(m.questions, r.Question...)
	m.mu.Unlock()

	reply := new(dns.Msg)
	reply.SetReply(r)

	qname := r.Question[0].Name
	qtype := r.Question[0].Qtype

	m.mu.RLock()
	zone := m.zone
	m.mu.RUnlock()

	if zone == nil {
		// Return NXDOMAIN for unknown zones
		reply.SetRcode(r, dns.RcodeNameError)
		w.WriteMsg(reply)
		return
	}

	// Check if query is for this zone
	if !dns.IsSubDomain(zone.Origin, qname) {
		// Return referral to parent
		m.handleReferral(w, r, reply)
		return
	}

	if zone.Referral {
		m.handleReferral(w, r, reply)
		return
	}

	// Look for exact match records
	if records, ok := zone.Records[qtype]; ok {
		for _, rr := range records {
			if rr.Header().Name == qname {
				reply.Answer = append(reply.Answer, rr)
			}
		}
	}

	if len(reply.Answer) > 0 {
		w.WriteMsg(reply)
		return
	}

	// Look for CNAME
	if cnames, ok := zone.Records[dns.TypeCNAME]; ok {
		for _, rr := range cnames {
			if cname, ok := rr.(*dns.CNAME); ok && cname.Header().Name == qname {
				reply.Answer = append(reply.Answer, cname)
				// Follow CNAME chain
				target := cname.Target
				if arecords, ok := zone.Records[dns.TypeA]; ok {
					for _, arr := range arecords {
						if a, ok := arr.(*dns.A); ok && a.Header().Name == target {
							reply.Answer = append(reply.Answer, a)
						}
					}
				}
			}
		}
		if len(reply.Answer) > 0 {
			w.WriteMsg(reply)
			return
		}
	}

	// No records found
	if zone.NSEC {
		reply.SetRcode(r, dns.RcodeNameError)
	} else {
		// Return authoritative NOERROR with no answer
		reply.Ns = m.getAuthoritySOA(zone)
	}

	w.WriteMsg(reply)
}

func (m *MockAuthorityServer) handleReferral(w dns.ResponseWriter, r *dns.Msg, reply *dns.Msg) {
	// Return NS records and glue
	m.mu.RLock()
	zone := m.zone
	m.mu.RUnlock()

	if zone == nil {
		reply.SetRcode(r, dns.RcodeNameError)
		w.WriteMsg(reply)
		return
	}

	// Add NS records
	if nsrecords, ok := zone.Records[dns.TypeNS]; ok {
		reply.Ns = append(reply.Ns, nsrecords...)
	}

	// Add glue records (A records for nameservers)
	if arecords, ok := zone.Records[dns.TypeA]; ok {
		for _, rr := range arecords {
			if a, ok := rr.(*dns.A); ok {
				// Check if this is glue for a nameserver
				for _, ns := range reply.Ns {
					if nsrr, ok := ns.(*dns.NS); ok && a.Header().Name == nsrr.Ns {
						reply.Extra = append(reply.Extra, rr)
						break
					}
				}
			}
		}
	}

	w.WriteMsg(reply)
}

func (m *MockAuthorityServer) getAuthoritySOA(zone *Zone) []dns.RR {
	return []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   zone.Origin,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns:      "ns1." + zone.Origin,
			Mbox:    "admin." + zone.Origin,
			Serial:  2024010101,
			Refresh: 3600,
			Retry:   1800,
			Expire:  86400,
			Minttl:  300,
		},
	}
}

// SetZone updates the zone data.
func (m *MockAuthorityServer) SetZone(zone *Zone) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.zone = zone
}

// A creates an A record.
func A(name string, ip string, ttl uint32) *dns.A {
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		A: net.ParseIP(ip),
	}
}

// AAAA creates an AAAA record.
func AAAA(name string, ip string, ttl uint32) *dns.AAAA {
	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		AAAA: net.ParseIP(ip),
	}
}

// CNAME creates a CNAME record.
func CNAME(name string, target string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: dns.Fqdn(target),
	}
}

// MX creates an MX record.
func MX(name string, mx string, preference uint16, ttl uint32) *dns.MX {
	return &dns.MX{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeMX,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Mx:         dns.Fqdn(mx),
		Preference: preference,
	}
}

// TXT creates a TXT record.
func TXT(name string, txt string, ttl uint32) *dns.TXT {
	return &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Txt: []string{txt},
	}
}

// NS creates an NS record.
func NS(domain string, nameserver string, ttl uint32) *dns.NS {
	return &dns.NS{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(domain),
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns: dns.Fqdn(nameserver),
	}
}

// SOA creates an SOA record.
func SOA(domain string, nameserver string, mbox string, ttl uint32) *dns.SOA {
	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(domain),
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns:      dns.Fqdn(nameserver),
		Mbox:    dns.Fqdn(mbox),
		Serial:  2024010101,
		Refresh: 3600,
		Retry:   1800,
		Expire:  86400,
		Minttl:  300,
	}
}

// TestResolver wraps a rec53 server for testing.
type TestResolver struct {
	server   *dns.Server
	addr     string
	errChan  <-chan error
	handler  dns.Handler
}

// NewTestResolver creates a test resolver listening on a random port.
func NewTestResolver(handler dns.Handler) (*TestResolver, error) {
	tr := &TestResolver{
		handler: handler,
	}

	tr.server = &dns.Server{
		Addr:    "127.0.0.1:0",
		Net:     "udp",
		Handler: handler,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := tr.server.ListenAndServe(); err != nil {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	if tr.server.PacketConn != nil {
		tr.addr = tr.server.PacketConn.LocalAddr().String()
	}

	tr.errChan = errChan
	return tr, nil
}

// Addr returns the resolver's address.
func (tr *TestResolver) Addr() string {
	return tr.addr
}

// Stop shuts down the resolver.
func (tr *TestResolver) Stop() {
	if tr.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tr.server.ShutdownContext(ctx)
	}
}

// Query performs a DNS query against the resolver.
func (tr *TestResolver) Query(qname string, qtype uint16) (*dns.Msg, error) {
	client := &dns.Client{
		Net:     "udp",
		Timeout: 5 * time.Second,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), qtype)
	msg.RecursionDesired = true

	resp, _, err := client.Exchange(msg, tr.addr)
	return resp, err
}

// QueryWithClient performs a DNS query with a custom client.
func (tr *TestResolver) QueryWithClient(client *dns.Client, qname string, qtype uint16) (*dns.Msg, time.Duration, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), qtype)
	msg.RecursionDesired = true

	return client.Exchange(msg, tr.addr)
}

// WaitForError waits for a server error or timeout.
func (tr *TestResolver) WaitForError(timeout time.Duration) error {
	select {
	case err := <-tr.errChan:
		return err
	case <-time.After(timeout):
		return nil
	}
}