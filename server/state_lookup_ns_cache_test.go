package server

import (
	"context"
	"testing"

	"github.com/miekg/dns"
)

func TestLookupNSCacheState_GluelessHit(t *testing.T) {
	FlushCacheForTest()
	defer FlushCacheForTest()

	nsZone := "glueless.test."
	nsName := "ns1.glueless.test."

	// Write a glueless NS referral into cache (Ns present, Extra absent).
	cached := new(dns.Msg)
	cached.SetQuestion(nsZone, dns.TypeNS)
	cached.Response = true
	cached.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   nsZone,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns: nsName,
		},
	}
	// No Extra records.
	setCacheCopy(nsZone, cached, 3600)

	req := new(dns.Msg)
	req.SetQuestion("www.glueless.test.", dns.TypeA)

	resp := new(dns.Msg)

	state := newLookupNSCacheState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != LOOKUP_NS_CACHE_HIT {
		t.Fatalf("expected LOOKUP_NS_CACHE_HIT (%d), got %d", LOOKUP_NS_CACHE_HIT, next)
	}
	if len(resp.Ns) == 0 {
		t.Fatal("expected Ns records in response after glueless cache hit, got none")
	}
	nsRR, ok := resp.Ns[0].(*dns.NS)
	if !ok {
		t.Fatalf("expected *dns.NS, got %T", resp.Ns[0])
	}
	if nsRR.Ns != nsName {
		t.Errorf("expected NS name %q, got %q", nsName, nsRR.Ns)
	}
	if len(resp.Extra) != 0 {
		t.Errorf("expected empty Extra for glueless hit, got %d records", len(resp.Extra))
	}
}

func TestLookupNSCacheState_GluedHit(t *testing.T) {
	FlushCacheForTest()
	defer FlushCacheForTest()

	nsZone := "glued.test."
	nsName := "ns1.glued.test."
	glueIP := "1.2.3.4"

	// Write a glued NS referral into cache (Ns + Extra present).
	cached := new(dns.Msg)
	cached.SetQuestion(nsZone, dns.TypeNS)
	cached.Response = true
	cached.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   nsZone,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns: nsName,
		},
	}
	cached.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   nsName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			A: []byte{1, 2, 3, 4},
		},
	}
	setCacheCopy(nsZone, cached, 3600)

	req := new(dns.Msg)
	req.SetQuestion("www.glued.test.", dns.TypeA)

	resp := new(dns.Msg)

	state := newLookupNSCacheState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != LOOKUP_NS_CACHE_HIT {
		t.Fatalf("expected LOOKUP_NS_CACHE_HIT (%d), got %d", LOOKUP_NS_CACHE_HIT, next)
	}
	if len(resp.Ns) == 0 {
		t.Fatal("expected Ns records")
	}
	if len(resp.Extra) == 0 {
		t.Fatal("expected Extra records for glued hit")
	}
	a, ok := resp.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("expected *dns.A in Extra, got %T", resp.Extra[0])
	}
	if a.A.String() != glueIP {
		t.Errorf("expected glue IP %s, got %s", glueIP, a.A.String())
	}
}

func TestLookupNSCacheState_Miss(t *testing.T) {
	FlushCacheForTest()
	defer FlushCacheForTest()

	req := new(dns.Msg)
	req.SetQuestion("www.uncached.test.", dns.TypeA)

	resp := new(dns.Msg)

	state := newLookupNSCacheState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != LOOKUP_NS_CACHE_MISS {
		t.Fatalf("expected LOOKUP_NS_CACHE_MISS (%d), got %d", LOOKUP_NS_CACHE_MISS, next)
	}
}

func TestLookupNSCacheState_SOANotHit(t *testing.T) {
	FlushCacheForTest()
	defer FlushCacheForTest()

	// Write a cache entry where Ns contains a SOA (NODATA response, not a delegation).
	// This must NOT be treated as a valid NS delegation cache hit.
	nsZone := "soa.test."
	cached := new(dns.Msg)
	cached.SetQuestion(nsZone, dns.TypeA)
	cached.Response = true
	cached.Ns = []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   nsZone,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			Ns:      "ns1.soa.test.",
			Mbox:    "hostmaster.soa.test.",
			Serial:  1,
			Refresh: 3600,
			Retry:   900,
			Expire:  604800,
			Minttl:  300,
		},
	}
	setCacheCopy(nsZone, cached, 300)

	req := new(dns.Msg)
	req.SetQuestion("www.soa.test.", dns.TypeA)

	resp := new(dns.Msg)

	state := newLookupNSCacheState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must fall through to MISS because the cached Ns[0] is SOA, not NS.
	if next != LOOKUP_NS_CACHE_MISS {
		t.Fatalf("expected LOOKUP_NS_CACHE_MISS for SOA in cache, got %d", next)
	}
}
