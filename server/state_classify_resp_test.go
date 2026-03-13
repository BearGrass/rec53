package server

import (
	"context"
	"testing"

	"github.com/miekg/dns"
)

// =============================================================================
// classifyRespState.handle Tests
// =============================================================================

// TestClassifyRespState_NilInput tests error handling for nil request and nil response
func TestClassifyRespState_NilInput(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		resp := new(dns.Msg)
		resp.SetQuestion("example.com.", dns.TypeA)
		state := newClassifyRespState(nil, resp, context.Background())
		ret, err := state.handle(nil, resp)
		if err == nil {
			t.Error("expected error for nil request")
		}
		if ret != CLASSIFY_RESP_COMMON_ERROR {
			t.Errorf("expected CLASSIFY_RESP_COMMON_ERROR (%d), got %d", CLASSIFY_RESP_COMMON_ERROR, ret)
		}
	})

	t.Run("nil response", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		state := newClassifyRespState(req, nil, context.Background())
		ret, err := state.handle(req, nil)
		if err == nil {
			t.Error("expected error for nil response")
		}
		if ret != CLASSIFY_RESP_COMMON_ERROR {
			t.Errorf("expected CLASSIFY_RESP_COMMON_ERROR (%d), got %d", CLASSIFY_RESP_COMMON_ERROR, ret)
		}
	})
}

// TestClassifyRespState_NXDOMAIN tests: empty answer + SOA in authority + RcodeNameError → GET_NEGATIVE
func TestClassifyRespState_NXDOMAIN(t *testing.T) {
	FlushCacheForTest()
	defer FlushCacheForTest()

	req := new(dns.Msg)
	req.SetQuestion("notexist.example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeNameError // NXDOMAIN
	resp.Answer = nil               // No answers
	resp.Ns = []dns.RR{
		&dns.SOA{
			Hdr:    dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 300},
			Ns:     "ns1.example.com.",
			Mbox:   "admin.example.com.",
			Serial: 1,
			Minttl: 300,
		},
	}

	state := newClassifyRespState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != CLASSIFY_RESP_GET_NEGATIVE {
		t.Errorf("expected CLASSIFY_RESP_GET_NEGATIVE (%d), got %d", CLASSIFY_RESP_GET_NEGATIVE, ret)
	}
}

// TestClassifyRespState_NODATA tests: empty answer + SOA in authority + RcodeSuccess → GET_NEGATIVE
func TestClassifyRespState_NODATA(t *testing.T) {
	FlushCacheForTest()
	defer FlushCacheForTest()

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeMX)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeSuccess // NODATA: success but no records of requested type
	resp.Answer = nil             // No answers
	resp.Ns = []dns.RR{
		&dns.SOA{
			Hdr:    dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 300},
			Ns:     "ns1.example.com.",
			Mbox:   "admin.example.com.",
			Serial: 1,
			Minttl: 600,
		},
	}

	state := newClassifyRespState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != CLASSIFY_RESP_GET_NEGATIVE {
		t.Errorf("expected CLASSIFY_RESP_GET_NEGATIVE (%d), got %d", CLASSIFY_RESP_GET_NEGATIVE, ret)
	}
}

// TestClassifyRespState_NoAnswerNoSOA tests: empty answer, no SOA → GET_NS (continue iteration)
func TestClassifyRespState_NoAnswerNoSOA(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeSuccess
	resp.Answer = nil // No answers
	resp.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
			Ns:  "ns1.example.com.",
		},
	}
	// No SOA in authority

	state := newClassifyRespState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != CLASSIFY_RESP_GET_NS {
		t.Errorf("expected CLASSIFY_RESP_GET_NS (%d), got %d", CLASSIFY_RESP_GET_NS, ret)
	}
}

// TestClassifyRespState_MatchingType tests: answer matches qtype → GET_ANS
func TestClassifyRespState_MatchingType(t *testing.T) {
	tests := []struct {
		name  string
		qtype uint16
		ansRR dns.RR
	}{
		{
			name:  "A record matches qtype A",
			qtype: dns.TypeA,
			ansRR: &dns.A{
				Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   []byte{1, 2, 3, 4},
			},
		},
		{
			name:  "MX record matches qtype MX",
			qtype: dns.TypeMX,
			ansRR: &dns.MX{
				Hdr:        dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 300},
				Preference: 10,
				Mx:         "mail.example.com.",
			},
		},
		{
			name:  "CNAME record matches qtype CNAME",
			qtype: dns.TypeCNAME,
			ansRR: &dns.CNAME{
				Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
				Target: "example.com.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := new(dns.Msg)
			req.SetQuestion("example.com.", tt.qtype)

			resp := new(dns.Msg)
			resp.SetReply(req)
			resp.Rcode = dns.RcodeSuccess
			resp.Answer = []dns.RR{tt.ansRR}

			state := newClassifyRespState(req, resp, context.Background())
			ret, err := state.handle(req, resp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ret != CLASSIFY_RESP_GET_ANS {
				t.Errorf("expected CLASSIFY_RESP_GET_ANS (%d), got %d", CLASSIFY_RESP_GET_ANS, ret)
			}
		})
	}
}

// TestClassifyRespState_CNAME_followed tests: answer has CNAME, qtype=A → GET_CNAME
func TestClassifyRespState_CNAME_followed(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("www.example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeSuccess
	// Answer has CNAME but no matching A record
	resp.Answer = []dns.RR{
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: "example.com.",
		},
	}

	state := newClassifyRespState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != CLASSIFY_RESP_GET_CNAME {
		t.Errorf("expected CLASSIFY_RESP_GET_CNAME (%d), got %d", CLASSIFY_RESP_GET_CNAME, ret)
	}
}

// TestClassifyRespState_CNAME_is_answer tests: answer has CNAME, qtype=CNAME → GET_ANS
func TestClassifyRespState_CNAME_is_answer(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("www.example.com.", dns.TypeCNAME)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeSuccess
	resp.Answer = []dns.RR{
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: "example.com.",
		},
	}

	state := newClassifyRespState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When qtype=CNAME and answer has CNAME, it matches the requested type → GET_ANS
	if ret != CLASSIFY_RESP_GET_ANS {
		t.Errorf("expected CLASSIFY_RESP_GET_ANS (%d), got %d", CLASSIFY_RESP_GET_ANS, ret)
	}
}

// TestClassifyRespState_WrongTypeNoMatch tests: answers present but wrong type, no CNAME → GET_NS
func TestClassifyRespState_WrongTypeNoMatch(t *testing.T) {
	// Query for A, but response contains only MX — no CNAME either
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeSuccess
	resp.Answer = []dns.RR{
		&dns.MX{
			Hdr:        dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 300},
			Preference: 10,
			Mx:         "mail.example.com.",
		},
	}

	state := newClassifyRespState(req, resp, context.Background())
	ret, err := state.handle(req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ret != CLASSIFY_RESP_GET_NS {
		t.Errorf("expected CLASSIFY_RESP_GET_NS (%d), got %d", CLASSIFY_RESP_GET_NS, ret)
	}
}
