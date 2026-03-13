// Package e2e provides end-to-end integration tests for the rec53 DNS resolver.
// These tests validate the complete system behavior including network interactions.
package e2e

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"rec53/server"
	"rec53/utils"

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

	started := make(chan struct{})
	m.server.NotifyStartedFunc = func() { close(started) }
	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			t.Logf("mock server stopped: %v", err)
		}
	}()

	// Wait for server to bind the socket (happens-before guarantee)
	<-started

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
	server  *dns.Server
	addr    string
	errChan <-chan error
	handler dns.Handler
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
	started := make(chan struct{})
	tr.server.NotifyStartedFunc = func() { close(started) }
	go func() {
		if err := tr.server.ListenAndServe(); err != nil {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Wait for server to bind the socket (happens-before guarantee)
	<-started

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

// =============================================================================
// Multi-Zone Mock DNS Server
// =============================================================================

// MockZone represents a single DNS zone in a multi-zone mock server.
// It describes the zone's own records and how to refer queries for child zones.
type MockZone struct {
	Origin    string              // e.g., "com." or "example.com."
	Records   map[uint16][]dns.RR // Records owned by this zone
	Referrals []MockReferral      // Child zone delegations
	NSEC      bool                // Enable NSEC for NXDOMAIN responses
	TC        bool                // Force TC (truncation) flag on all responses
}

// MockReferral describes a delegation from a parent zone to a child zone.
type MockReferral struct {
	ChildOrigin string   // e.g., "example.com." delegated from "com."
	NSRecords   []dns.RR // NS records for the child zone
	Glue        []dns.RR // A/AAAA glue records (empty = glueless referral)
}

// MultiZoneMockServer is a single DNS server that handles multiple zones.
// It routes queries to the appropriate zone based on the query name,
// returning authoritative answers or referrals as appropriate.
type MultiZoneMockServer struct {
	server    *dns.Server
	addr      string
	zones     []*MockZone // ordered from most-specific to least-specific
	mu        sync.RWMutex
	reqCount  int64
	questions []dns.Question
}

// NewMultiZoneMockServer creates a mock DNS server with multiple zones.
// Zones should be added from least-specific to most-specific, but the
// server will sort them internally by label count (most-specific first).
func NewMultiZoneMockServer(t *testing.T, zones []*MockZone) *MultiZoneMockServer {
	m := &MultiZoneMockServer{
		zones: zones,
	}

	// Sort zones by label count descending (most-specific first).
	// When routing a query, we find the most-specific zone that contains
	// the qname. Within that zone, handleZoneQuery checks referrals first:
	// if the zone delegates the qname to a child, a referral is returned.
	// Otherwise, the zone answers authoritatively.
	for i := 0; i < len(m.zones); i++ {
		for j := i + 1; j < len(m.zones); j++ {
			if dns.CountLabel(m.zones[i].Origin) < dns.CountLabel(m.zones[j].Origin) {
				m.zones[i], m.zones[j] = m.zones[j], m.zones[i]
			}
		}
	}

	m.server = &dns.Server{
		Addr:    "127.0.0.1:0",
		Net:     "udp",
		Handler: dns.HandlerFunc(m.handleDNS),
	}

	started := make(chan struct{})
	m.server.NotifyStartedFunc = func() { close(started) }
	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			t.Logf("multi-zone mock server stopped: %v", err)
		}
	}()

	// Wait for server to bind the socket (happens-before guarantee)
	<-started

	if m.server.PacketConn != nil {
		m.addr = m.server.PacketConn.LocalAddr().String()
	}

	return m
}

// Addr returns the server's listening address.
func (m *MultiZoneMockServer) Addr() string {
	return m.addr
}

// Port returns just the port number from the listening address.
func (m *MultiZoneMockServer) Port() string {
	_, port, _ := net.SplitHostPort(m.addr)
	return port
}

// Stop shuts down the mock server.
func (m *MultiZoneMockServer) Stop() {
	if m.server != nil {
		m.server.Shutdown()
	}
}

// RequestCount returns the number of requests received.
func (m *MultiZoneMockServer) RequestCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reqCount
}

// Questions returns all questions received by the server.
func (m *MultiZoneMockServer) Questions() []dns.Question {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]dns.Question{}, m.questions...)
}

// handleDNS routes queries to the appropriate zone.
func (m *MultiZoneMockServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m.mu.Lock()
	m.reqCount++
	m.questions = append(m.questions, r.Question...)
	m.mu.Unlock()

	if len(r.Question) == 0 {
		reply := new(dns.Msg)
		reply.SetRcode(r, dns.RcodeFormatError)
		w.WriteMsg(reply)
		return
	}

	qname := r.Question[0].Name

	m.mu.RLock()
	zones := m.zones
	m.mu.RUnlock()

	// Find the most-specific zone that contains this qname.
	// Zones are sorted most-specific first, so the first match wins.
	// This means for "www.example.com.", the "example.com." zone is checked
	// before "com." or ".". Within the zone, handleZoneQuery checks referrals
	// first, so delegations to child zones are respected.
	for _, zone := range zones {
		if dns.IsSubDomain(zone.Origin, qname) {
			m.handleZoneQuery(w, r, zone, qname)
			return
		}
	}

	// No matching zone found — return REFUSED
	reply := new(dns.Msg)
	reply.SetRcode(r, dns.RcodeRefused)
	w.WriteMsg(reply)
}

// handleZoneQuery handles a query within a specific zone.
func (m *MultiZoneMockServer) handleZoneQuery(w dns.ResponseWriter, r *dns.Msg, zone *MockZone, qname string) {
	reply := new(dns.Msg)
	reply.SetReply(r)

	// Check TC flag first — if set, return truncated response immediately
	if zone.TC {
		reply.Truncated = true
		reply.Rcode = dns.RcodeSuccess
		w.WriteMsg(reply)
		return
	}

	qtype := r.Question[0].Qtype

	// Check if this query should be referred to a child zone
	for _, ref := range zone.Referrals {
		if dns.IsSubDomain(ref.ChildOrigin, qname) {
			// This qname belongs to a child zone — send referral
			reply.Authoritative = false
			reply.Ns = append(reply.Ns, ref.NSRecords...)
			reply.Extra = append(reply.Extra, ref.Glue...)
			w.WriteMsg(reply)
			return
		}
	}

	// This zone is authoritative for the qname.
	reply.Authoritative = true

	// Look for exact match records of the requested type
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

	// Look for CNAME (only when not querying for CNAME type itself)
	if qtype != dns.TypeCNAME {
		if cnames, ok := zone.Records[dns.TypeCNAME]; ok {
			for _, rr := range cnames {
				if cname, ok := rr.(*dns.CNAME); ok && cname.Header().Name == qname {
					reply.Answer = append(reply.Answer, cname)
					// If the CNAME target is in the same zone, include the A record
					target := cname.Target
					if dns.IsSubDomain(zone.Origin, target) {
						if arecords, ok := zone.Records[dns.TypeA]; ok {
							for _, arr := range arecords {
								if a, ok := arr.(*dns.A); ok && a.Header().Name == target {
									reply.Answer = append(reply.Answer, a)
								}
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
	}

	// No matching records found — negative response
	if zone.NSEC {
		// NXDOMAIN: domain does not exist
		reply.SetRcode(r, dns.RcodeNameError)
		reply.Ns = makeSOA(zone.Origin)
	} else {
		// NODATA: domain exists but no records of the requested type
		reply.Rcode = dns.RcodeSuccess
		reply.Ns = makeSOA(zone.Origin)
	}

	w.WriteMsg(reply)
}

// makeSOA creates a SOA record for negative responses.
func makeSOA(origin string) []dns.RR {
	return []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   origin,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			Ns:      "ns1." + origin,
			Mbox:    "admin." + origin,
			Serial:  2024010101,
			Refresh: 3600,
			Retry:   1800,
			Expire:  86400,
			Minttl:  300,
		},
	}
}

// =============================================================================
// MockDNSHierarchy — convenient builder for root→TLD→auth chains
// =============================================================================

// MockDNSHierarchy builds a multi-layer DNS hierarchy for testing.
// All zones are served by a single MultiZoneMockServer, with the root
// glue pointing to 127.0.0.1 and the server's actual port.
type MockDNSHierarchy struct {
	zones []*MockZone
}

// NewMockDNSHierarchy creates a new hierarchy builder.
func NewMockDNSHierarchy() *MockDNSHierarchy {
	return &MockDNSHierarchy{}
}

// AddZone adds a zone to the hierarchy. Returns the hierarchy for chaining.
func (h *MockDNSHierarchy) AddZone(zone *MockZone) *MockDNSHierarchy {
	h.zones = append(h.zones, zone)
	return h
}

// Build creates the MultiZoneMockServer and returns it along with the
// root glue message suitable for utils.SetRootGlue().
func (h *MockDNSHierarchy) Build(t *testing.T) (*MultiZoneMockServer, *dns.Msg) {
	srv := NewMultiZoneMockServer(t, h.zones)

	// Build root glue message pointing to this server
	rootGlue := new(dns.Msg)
	rootGlue.SetUpdate(".")
	rootGlue.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 0},
			Ns:  "ns.mock-root.",
		},
	}
	rootGlue.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "ns.mock-root.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
			A:   net.ParseIP("127.0.0.1"),
		},
	}

	return srv, rootGlue
}

// =============================================================================
// setupResolverWithMockRoot — full test environment setup
// =============================================================================

// ResolverTestEnv holds references to a rec53 server and mock hierarchy
// for end-to-end testing.
type ResolverTestEnv struct {
	MockSrv *MultiZoneMockServer
	Addr    string // rec53 resolver address for client queries
}

// setupResolverWithMockRoot creates a rec53 server configured to use a mock
// root DNS hierarchy instead of real root servers. It:
//   - Flushes the global DNS cache
//   - Resets the global IP pool
//   - Injects the mock root glue via utils.SetRootGlue()
//   - Overrides the iter port via server.SetIterPort()
//   - Starts a rec53 server on a random port
//   - Registers t.Cleanup() for teardown
//
// Returns the ResolverTestEnv with the rec53 server address and mock server.
func setupResolverWithMockRoot(t *testing.T, mockSrv *MultiZoneMockServer, rootGlue *dns.Msg) *ResolverTestEnv {
	t.Helper()

	// Inject mock root glue
	utils.SetRootGlue(rootGlue)

	// Override iter port to use mock server's port
	server.SetIterPort(mockSrv.Port())

	// Flush cache and reset IP pool
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	// Start rec53 server — Run() blocks until UDP is ready, so UDPAddr() is safe immediately
	srv := server.NewServer("127.0.0.1:0")
	errChan := srv.Run()

	addr := srv.UDPAddr()
	if addr == "" {
		t.Fatal("rec53 server failed to start: no UDP address")
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		// Drain error channel
		go func() {
			for range errChan {
			}
		}()

		mockSrv.Stop()
		utils.ResetRootGlue()
		server.ResetIterPort()
		server.FlushCacheForTest()
		server.ResetIPPoolForTest()
	})

	return &ResolverTestEnv{
		MockSrv: mockSrv,
		Addr:    addr,
	}
}

// query performs a DNS query against the resolver environment.
func (env *ResolverTestEnv) query(qname string, qtype uint16) (*dns.Msg, error) {
	client := &dns.Client{
		Net:     "udp",
		Timeout: 10 * time.Second,
		UDPSize: 4096,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(qname), qtype)
	msg.RecursionDesired = true
	msg.SetEdns0(4096, false)

	resp, _, err := client.Exchange(msg, env.Addr)
	return resp, err
}

// =============================================================================
// Convenience builders for common DNS hierarchy patterns
// =============================================================================

// BuildStandardHierarchy creates a standard root→TLD→auth hierarchy.
// The auth zone contains the specified records.
// All zones refer to the next layer via NS+glue pointing to 127.0.0.1.
//
// Parameters:
//   - tld: e.g., "com."
//   - authZone: e.g., "example.com."
//   - authRecords: records for the authoritative zone
//
// Returns the hierarchy builder (call Build to get the server).
func BuildStandardHierarchy(tld, authZone string, authRecords map[uint16][]dns.RR) *MockDNSHierarchy {
	nsName := fmt.Sprintf("ns1.%s", authZone)

	return NewMockDNSHierarchy().
		AddZone(&MockZone{
			Origin: ".",
			Referrals: []MockReferral{
				{
					ChildOrigin: tld,
					NSRecords: []dns.RR{
						NS(tld, fmt.Sprintf("ns1.%s", tld), 172800),
					},
					Glue: []dns.RR{
						A(fmt.Sprintf("ns1.%s", tld), "127.0.0.1", 172800),
					},
				},
			},
		}).
		AddZone(&MockZone{
			Origin: tld,
			Referrals: []MockReferral{
				{
					ChildOrigin: authZone,
					NSRecords: []dns.RR{
						NS(authZone, nsName, 172800),
					},
					Glue: []dns.RR{
						A(nsName, "127.0.0.1", 172800),
					},
				},
			},
		}).
		AddZone(&MockZone{
			Origin:  authZone,
			Records: authRecords,
		})
}
