package server

import (
	"context"
	"testing"

	"github.com/miekg/dns"
)

func TestExtractGlueState_GluelessZoneMatch(t *testing.T) {
	// Glueless NS referral where NS zone IS an ancestor of the query domain.
	// Expected: EXTRACT_GLUE_EXIST so state machine proceeds to QUERY_UPSTREAM.
	req := new(dns.Msg)
	req.SetQuestion("www.glueless.example.", dns.TypeA)

	resp := new(dns.Msg)
	resp.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   "glueless.example.",
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns: "ns1.glueless.example.",
		},
	}
	// No Extra — glueless.

	state := newExtractGlueState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != EXTRACT_GLUE_EXIST {
		t.Fatalf("expected EXTRACT_GLUE_EXIST (%d), got %d", EXTRACT_GLUE_EXIST, next)
	}
	if len(resp.Ns) == 0 {
		t.Error("expected Ns records to be preserved after EXTRACT_GLUE_EXIST")
	}
}

func TestExtractGlueState_GluelessZoneMismatch(t *testing.T) {
	// Glueless NS referral where NS zone is NOT an ancestor of the query domain
	// (stale delegation from a previous CNAME hop).
	// Expected: EXTRACT_GLUE_NOT_EXIST and Ns/Extra cleared.
	req := new(dns.Msg)
	req.SetQuestion("www.other-domain.example.", dns.TypeA)

	resp := new(dns.Msg)
	resp.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   "unrelated.zone.",
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns: "ns1.unrelated.zone.",
		},
	}
	// No Extra — glueless but wrong zone.

	state := newExtractGlueState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != EXTRACT_GLUE_NOT_EXIST {
		t.Fatalf("expected EXTRACT_GLUE_NOT_EXIST (%d), got %d", EXTRACT_GLUE_NOT_EXIST, next)
	}
	if len(resp.Ns) != 0 {
		t.Errorf("expected Ns to be cleared on zone mismatch, got %d records", len(resp.Ns))
	}
	if len(resp.Extra) != 0 {
		t.Errorf("expected Extra to be cleared on zone mismatch, got %d records", len(resp.Extra))
	}
}

func TestExtractGlueState_GluedZoneMatch(t *testing.T) {
	// Original behaviour: glued NS referral with matching zone → EXTRACT_GLUE_EXIST.
	req := new(dns.Msg)
	req.SetQuestion("www.glued.example.", dns.TypeA)

	resp := new(dns.Msg)
	resp.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   "glued.example.",
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns: "ns1.glued.example.",
		},
	}
	resp.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "ns1.glued.example.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			A: []byte{1, 2, 3, 4},
		},
	}

	state := newExtractGlueState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != EXTRACT_GLUE_EXIST {
		t.Fatalf("expected EXTRACT_GLUE_EXIST (%d), got %d", EXTRACT_GLUE_EXIST, next)
	}
}

func TestExtractGlueState_GluedZoneMismatch(t *testing.T) {
	// Original behaviour: glued NS referral but zone does NOT match query domain.
	// Expected: EXTRACT_GLUE_NOT_EXIST and Ns/Extra cleared.
	req := new(dns.Msg)
	req.SetQuestion("www.other.example.", dns.TypeA)

	resp := new(dns.Msg)
	resp.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   "stale.zone.",
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			Ns: "ns1.stale.zone.",
		},
	}
	resp.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   "ns1.stale.zone.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			A: []byte{9, 9, 9, 9},
		},
	}

	state := newExtractGlueState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != EXTRACT_GLUE_NOT_EXIST {
		t.Fatalf("expected EXTRACT_GLUE_NOT_EXIST (%d), got %d", EXTRACT_GLUE_NOT_EXIST, next)
	}
	if len(resp.Ns) != 0 {
		t.Errorf("expected Ns cleared, got %d records", len(resp.Ns))
	}
	if len(resp.Extra) != 0 {
		t.Errorf("expected Extra cleared, got %d records", len(resp.Extra))
	}
}

func TestExtractGlueState_NoNs(t *testing.T) {
	// No Ns at all → EXTRACT_GLUE_NOT_EXIST.
	req := new(dns.Msg)
	req.SetQuestion("www.example.", dns.TypeA)

	resp := new(dns.Msg)

	state := newExtractGlueState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != EXTRACT_GLUE_NOT_EXIST {
		t.Fatalf("expected EXTRACT_GLUE_NOT_EXIST (%d), got %d", EXTRACT_GLUE_NOT_EXIST, next)
	}
}

func TestExtractGlueState_SOAInNsSection(t *testing.T) {
	// SOA record in Ns section (NODATA/NXDOMAIN response) must NOT be treated as
	// delegation glue → EXTRACT_GLUE_NOT_EXIST, and Ns/Extra must be cleared.
	req := new(dns.Msg)
	req.SetQuestion("www.example.", dns.TypeA)

	resp := new(dns.Msg)
	resp.Ns = []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   "example.",
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			Ns:      "ns1.example.",
			Mbox:    "hostmaster.example.",
			Serial:  1,
			Refresh: 3600,
			Retry:   900,
			Expire:  604800,
			Minttl:  300,
		},
	}

	state := newExtractGlueState(req, resp, context.Background())
	next, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != EXTRACT_GLUE_NOT_EXIST {
		t.Fatalf("expected EXTRACT_GLUE_NOT_EXIST for SOA in Ns, got %d", next)
	}
	if len(resp.Ns) != 0 {
		t.Errorf("expected Ns cleared after SOA detection, got %d records", len(resp.Ns))
	}
}
