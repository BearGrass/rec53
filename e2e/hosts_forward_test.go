package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"rec53/server"
	"rec53/utils"

	"github.com/miekg/dns"
)

// =============================================================================
// Hosts E2E Tests
// =============================================================================

// TestHostsLookupARecord verifies that a query matching a hosts A entry returns
// the correct IP, AA=true, RCODE=NOERROR, and does NOT reach iterative resolution.
func TestHostsLookupARecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build a mock hierarchy that resolves test.example.com to a DIFFERENT IP.
	// If the hosts path works correctly, the mock server should never be queried.
	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("test.example.com", "9.9.9.9", 300),
		},
		dns.TypeNS: {
			NS("example.com.", "ns1.example.com.", 172800),
		},
	})

	mockSrv, rootGlue := hierarchy.Build(t)

	// Set globals BEFORE server starts to avoid data race
	server.SetHostsAndForwardForTest(
		[]server.HostEntry{
			{Name: "test.example.com", Type: "A", Value: "10.0.0.1", TTL: 300},
		},
		nil, // no forwarding
	)
	// Register cleanup BEFORE setupResolverWithMockRoot so LIFO order ensures:
	// server shutdown first (from setupResolverWithMockRoot cleanup), then globals reset
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("test.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if !resp.Authoritative {
		t.Error("expected AA flag to be set for hosts response")
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected at least one answer RR")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("10.0.0.1")) {
		t.Errorf("expected 10.0.0.1, got %s", a.A)
	}

	// Verify mock server was NOT queried (hosts should short-circuit)
	if mockSrv.RequestCount() != 0 {
		t.Errorf("expected 0 requests to mock server (hosts should short-circuit), got %d", mockSrv.RequestCount())
	}
}

// TestHostsLookupAAAARecord verifies hosts AAAA entries work end-to-end.
func TestHostsLookupAAAARecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {
			NS("example.com.", "ns1.example.com.", 172800),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	server.SetHostsAndForwardForTest(
		[]server.HostEntry{
			{Name: "ipv6.example.com", Type: "AAAA", Value: "fd00::1", TTL: 120},
		},
		nil,
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("ipv6.example.com", dns.TypeAAAA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if !resp.Authoritative {
		t.Error("expected AA flag")
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	aaaa, ok := resp.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("expected AAAA record, got %T", resp.Answer[0])
	}
	if !aaaa.AAAA.Equal(net.ParseIP("fd00::1")) {
		t.Errorf("expected fd00::1, got %s", aaaa.AAAA)
	}
}

// TestHostsTypeMismatchNODATA verifies that querying a hosts name with the
// wrong record type returns NODATA (NOERROR + empty answer), not a miss.
func TestHostsTypeMismatchNODATA(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeAAAA: {
			AAAA("nodata.example.com", "2001:db8::1", 300),
		},
		dns.TypeNS: {
			NS("example.com.", "ns1.example.com.", 172800),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Only A record configured in hosts
	server.SetHostsAndForwardForTest(
		[]server.HostEntry{
			{Name: "nodata.example.com", Type: "A", Value: "10.0.0.2", TTL: 300},
		},
		nil,
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// Query AAAA — name exists in hosts but type doesn't match → NODATA
	resp, err := env.query("nodata.example.com", dns.TypeAAAA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR (NODATA), got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) != 0 {
		t.Errorf("expected empty answer section for NODATA, got %d answers", len(resp.Answer))
	}
	if !resp.Authoritative {
		t.Error("expected AA flag for NODATA from hosts")
	}

	// NODATA path should not touch mock server
	if mockSrv.RequestCount() != 0 {
		t.Errorf("expected 0 requests to mock server for NODATA, got %d", mockSrv.RequestCount())
	}
}

// TestHostsMissFallsThrough verifies that a query NOT matching any hosts entry
// falls through to iterative resolution and returns the upstream answer.
func TestHostsMissFallsThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeA: {
			A("other.example.com", "93.184.216.34", 300),
		},
		dns.TypeNS: {
			NS("example.com.", "ns1.example.com.", 172800),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Hosts only has "internal.example.com"; "other.example.com" is NOT in hosts.
	server.SetHostsAndForwardForTest(
		[]server.HostEntry{
			{Name: "internal.example.com", Type: "A", Value: "10.0.0.99", TTL: 60},
		},
		nil,
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("other.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR from iterative resolution, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer from iterative resolution")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("93.184.216.34")) {
		t.Errorf("expected 93.184.216.34 from iterative, got %s", a.A)
	}

	// Mock server should have been queried for iterative resolution
	if mockSrv.RequestCount() == 0 {
		t.Error("expected mock server to be queried for non-hosts domain")
	}
}

// =============================================================================
// Forwarding E2E Tests
// =============================================================================

// newMockUpstreamServer creates a simple DNS server that answers A queries for
// the given domain with the given IP. Useful as a forwarding upstream target.
func newMockUpstreamServer(t *testing.T, domain string, ip string) *MockAuthorityServer {
	t.Helper()
	zone := &Zone{
		Origin: dns.Fqdn(domain),
		Records: map[uint16][]dns.RR{
			dns.TypeA: {
				A(domain, ip, 300),
			},
		},
	}
	return NewMockAuthorityServer(t, zone)
}

// TestForwardingZoneMatch verifies that a query matching a forwarding zone is
// sent to the configured upstream and returns the upstream's answer.
func TestForwardingZoneMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start a mock upstream that answers queries for "app.corp.example.com" → 172.16.0.1
	upstream := newMockUpstreamServer(t, "app.corp.example.com", "172.16.0.1")
	defer upstream.Stop()

	// Build a standard hierarchy (for non-forwarded queries)
	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {
			NS("example.com.", "ns1.example.com.", 172800),
		},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Configure forwarding for corp.example.com → our mock upstream
	server.SetHostsAndForwardForTest(
		nil, // no hosts
		[]server.ForwardZone{
			{Zone: "corp.example.com", Upstreams: []string{upstream.Addr()}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("app.corp.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer from forwarding upstream")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("172.16.0.1")) {
		t.Errorf("expected 172.16.0.1, got %s", a.A)
	}

	// Verify the upstream received the query
	if upstream.RequestCount() == 0 {
		t.Error("expected upstream to receive the forwarded query")
	}

	// Verify the mock root hierarchy was NOT queried (forwarding short-circuits iterative)
	if mockSrv.RequestCount() != 0 {
		t.Errorf("expected 0 requests to mock root (forwarding should short-circuit), got %d",
			mockSrv.RequestCount())
	}
}

// TestForwardingLongestSuffix verifies that when multiple zones match, the
// longest suffix wins.
func TestForwardingLongestSuffix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Broad zone upstream: example.com → returns 1.1.1.1
	broadUpstream := newMockUpstreamServer(t, "app.deep.example.com", "1.1.1.1")
	defer broadUpstream.Stop()

	// Specific zone upstream: deep.example.com → returns 2.2.2.2
	specificUpstream := newMockUpstreamServer(t, "app.deep.example.com", "2.2.2.2")
	defer specificUpstream.Stop()

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	server.SetHostsAndForwardForTest(
		nil,
		[]server.ForwardZone{
			{Zone: "example.com", Upstreams: []string{broadUpstream.Addr()}},
			{Zone: "deep.example.com", Upstreams: []string{specificUpstream.Addr()}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("app.deep.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer from specific upstream")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	// Should get 2.2.2.2 from the more-specific "deep.example.com" zone
	if !a.A.Equal(net.ParseIP("2.2.2.2")) {
		t.Errorf("expected 2.2.2.2 (specific zone), got %s", a.A)
	}

	// The specific upstream should have been used, not the broad one
	if specificUpstream.RequestCount() == 0 {
		t.Error("expected specific upstream to receive query")
	}
	if broadUpstream.RequestCount() != 0 {
		t.Errorf("expected broad upstream NOT to be queried, got %d requests", broadUpstream.RequestCount())
	}
}

// TestForwardingAllUpstreamsFail verifies that when all configured upstreams
// are unreachable, the resolver returns SERVFAIL and does NOT fall back to
// iterative resolution (design constraint D-5).
func TestForwardingAllUpstreamsFail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeA:  {A("fail.corp.example.com", "93.184.216.34", 300)},
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Use unreachable addresses as upstreams
	server.SetHostsAndForwardForTest(
		nil,
		[]server.ForwardZone{
			{Zone: "corp.example.com", Upstreams: []string{
				"127.0.0.1:1", // port 1 — nothing listening
				"127.0.0.1:2", // port 2 — nothing listening
			}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("fail.corp.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// D-5: all upstreams fail → SERVFAIL, no fallback to iterative
	if resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("expected SERVFAIL, got %s", dns.RcodeToString[resp.Rcode])
	}

	// Verify mock root was NOT queried (no fallback to iterative)
	if mockSrv.RequestCount() != 0 {
		t.Errorf("expected 0 requests to mock root (no iterative fallback), got %d",
			mockSrv.RequestCount())
	}
}

// TestForwardingUpstreamFailover verifies that when the first upstream fails,
// the resolver tries the next one and succeeds.
func TestForwardingUpstreamFailover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Second upstream works
	upstream := newMockUpstreamServer(t, "failover.corp.example.com", "172.16.0.2")
	defer upstream.Stop()

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// First upstream is unreachable; second is the mock
	server.SetHostsAndForwardForTest(
		nil,
		[]server.ForwardZone{
			{Zone: "corp.example.com", Upstreams: []string{
				"127.0.0.1:1", // unreachable
				upstream.Addr(),
			}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("failover.corp.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR after failover, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer after failover")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("172.16.0.2")) {
		t.Errorf("expected 172.16.0.2, got %s", a.A)
	}
}

// TestForwardingNoMatchFallsThrough verifies that a query NOT matching any
// forwarding zone falls through to iterative resolution.
func TestForwardingNoMatchFallsThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeA:  {A("www.example.com", "93.184.216.34", 300)},
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Forwarding configured for corp.example.com only
	upstream := newMockUpstreamServer(t, "www.example.com", "10.0.0.1")
	defer upstream.Stop()

	server.SetHostsAndForwardForTest(
		nil,
		[]server.ForwardZone{
			{Zone: "corp.example.com", Upstreams: []string{upstream.Addr()}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	// Query www.example.com — not in corp.example.com zone, should go iterative
	resp, err := env.query("www.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR from iterative, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer from iterative resolution")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("93.184.216.34")) {
		t.Errorf("expected 93.184.216.34 from iterative, got %s", a.A)
	}

	// Mock root SHOULD be queried (iterative path)
	if mockSrv.RequestCount() == 0 {
		t.Error("expected mock root to be queried for non-forwarded domain")
	}

	// Upstream mock should NOT be queried (query doesn't match forwarding zone)
	if upstream.RequestCount() != 0 {
		t.Errorf("expected 0 requests to forwarding upstream for non-matching query, got %d",
			upstream.RequestCount())
	}
}

// =============================================================================
// Priority / Integration Tests
// =============================================================================

// TestPriorityHostsOverForwarding verifies that hosts entries take precedence
// over forwarding zones. If both hosts and forwarding match the same name,
// hosts wins.
func TestPriorityHostsOverForwarding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Forwarding upstream returns a different IP
	upstream := newMockUpstreamServer(t, "priority.corp.example.com", "172.16.0.99")
	defer upstream.Stop()

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Both hosts AND forwarding match the same domain
	server.SetHostsAndForwardForTest(
		[]server.HostEntry{
			{Name: "priority.corp.example.com", Type: "A", Value: "10.0.0.42", TTL: 300},
		},
		[]server.ForwardZone{
			{Zone: "corp.example.com", Upstreams: []string{upstream.Addr()}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("priority.corp.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if !resp.Authoritative {
		t.Error("expected AA flag (hosts response)")
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	// Hosts answer (10.0.0.42) should win over forwarding (172.16.0.99)
	if !a.A.Equal(net.ParseIP("10.0.0.42")) {
		t.Errorf("expected 10.0.0.42 (hosts), got %s", a.A)
	}

	// Upstream should NOT be queried (hosts short-circuits before forwarding)
	if upstream.RequestCount() != 0 {
		t.Errorf("expected 0 upstream requests (hosts has priority), got %d", upstream.RequestCount())
	}

	// Mock root should NOT be queried
	if mockSrv.RequestCount() != 0 {
		t.Errorf("expected 0 root requests, got %d", mockSrv.RequestCount())
	}
}

// TestPriorityForwardingOverIterative verifies that forwarding takes precedence
// over iterative resolution when the domain matches a forwarding zone.
func TestPriorityForwardingOverIterative(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Iterative resolution via mock hierarchy returns 93.184.216.34
	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeA:  {A("iter.corp.example.com", "93.184.216.34", 300)},
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	// Forwarding upstream returns 172.16.0.5
	upstream := newMockUpstreamServer(t, "iter.corp.example.com", "172.16.0.5")
	defer upstream.Stop()

	server.SetHostsAndForwardForTest(
		nil,
		[]server.ForwardZone{
			{Zone: "corp.example.com", Upstreams: []string{upstream.Addr()}},
		},
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("iter.corp.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer from forwarding")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	// Forwarding (172.16.0.5) should win over iterative (93.184.216.34)
	if !a.A.Equal(net.ParseIP("172.16.0.5")) {
		t.Errorf("expected 172.16.0.5 (forwarding), got %s", a.A)
	}

	// Mock root should NOT be queried (forwarding short-circuits)
	if mockSrv.RequestCount() != 0 {
		t.Errorf("expected 0 root requests (forwarding has priority), got %d",
			mockSrv.RequestCount())
	}
}

// TestHostsCNAMERecord verifies CNAME hosts entries work end-to-end.
// A CNAME in hosts should be returned directly without further resolution.
func TestHostsCNAMERecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	server.SetHostsAndForwardForTest(
		[]server.HostEntry{
			{Name: "alias.example.com", Type: "CNAME", Value: "real.example.com", TTL: 300},
		},
		nil,
	)
	t.Cleanup(func() { server.ResetHostsAndForwardForTest() })

	env := setupResolverWithMockRoot(t, mockSrv, rootGlue)

	resp, err := env.query("alias.example.com", dns.TypeCNAME)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if !resp.Authoritative {
		t.Error("expected AA flag")
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected CNAME answer")
	}
	cname, ok := resp.Answer[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("expected CNAME record, got %T", resp.Answer[0])
	}
	if cname.Target != "real.example.com." {
		t.Errorf("expected target real.example.com., got %s", cname.Target)
	}
}

// =============================================================================
// setupResolverWithHostsForward helper
// =============================================================================

// setupResolverWithHostsForward creates a rec53 server with hosts and forwarding
// config using NewServerWithFullConfig. This is an alternative to the
// SetHostsAndForwardForTest approach when you need the server itself to be
// initialized with the config (e.g., testing NewServerWithFullConfig path).
func setupResolverWithHostsForward(
	t *testing.T,
	mockSrv *MultiZoneMockServer,
	rootGlue *dns.Msg,
	hosts []server.HostEntry,
	forwarding []server.ForwardZone,
) *ResolverTestEnv {
	t.Helper()

	utils.SetRootGlue(rootGlue)
	server.SetIterPort(mockSrv.Port())
	server.FlushCacheForTest()
	server.ResetIPPoolForTest()

	warmupCfg := server.DefaultWarmupConfig
	warmupCfg.Enabled = false

	srv := server.NewServerWithFullConfig("127.0.0.1:0", warmupCfg, server.SnapshotConfig{}, hosts, forwarding)
	errChan := srv.Run()

	addr := srv.UDPAddr()
	if addr == "" {
		t.Fatal("rec53 server failed to start: no UDP address")
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		go func() {
			for range errChan {
			}
		}()
		mockSrv.Stop()
		utils.ResetRootGlue()
		server.ResetIterPort()
		server.FlushCacheForTest()
		server.ResetIPPoolForTest()
		server.ResetHostsAndForwardForTest()
	})

	return &ResolverTestEnv{
		MockSrv: mockSrv,
		Addr:    addr,
	}
}

// TestNewServerWithFullConfigE2E exercises the full constructor path by creating
// a server with NewServerWithFullConfig and verifying hosts resolution works.
func TestNewServerWithFullConfigE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	hierarchy := BuildStandardHierarchy("com.", "example.com.", map[uint16][]dns.RR{
		dns.TypeNS: {NS("example.com.", "ns1.example.com.", 172800)},
	})
	mockSrv, rootGlue := hierarchy.Build(t)

	env := setupResolverWithHostsForward(t, mockSrv, rootGlue,
		[]server.HostEntry{
			{Name: "fullconfig.example.com", Type: "A", Value: "10.0.0.77", TTL: 300},
		},
		nil,
	)

	resp, err := env.query("fullconfig.example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer from hosts via NewServerWithFullConfig")
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !a.A.Equal(net.ParseIP("10.0.0.77")) {
		t.Errorf("expected 10.0.0.77, got %s", a.A)
	}
}
