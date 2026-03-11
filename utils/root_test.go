package utils

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestGetRootGlue(t *testing.T) {
	rootGlue := GetRootGlue()

	if rootGlue == nil {
		t.Fatal("GetRootGlue returned nil")
	}

	// Test NS records
	if len(rootGlue.Ns) != 13 {
		t.Errorf("Expected 13 NS records, got %d", len(rootGlue.Ns))
	}

	// Test Extra (A) records
	if len(rootGlue.Extra) != 13 {
		t.Errorf("Expected 13 A records in Extra, got %d", len(rootGlue.Extra))
	}

	// Verify all NS records are for root
	for i, ns := range rootGlue.Ns {
		if ns.Header().Name != "." {
			t.Errorf("NS record %d has wrong name: %s", i, ns.Header().Name)
		}
		if ns.Header().Rrtype != dns.TypeNS {
			t.Errorf("NS record %d has wrong type: %d", i, ns.Header().Rrtype)
		}
	}

	// Verify all A records have valid IPs
	for i, extra := range rootGlue.Extra {
		if extra.Header().Rrtype != dns.TypeA {
			t.Errorf("Extra record %d has wrong type: %d", i, extra.Header().Rrtype)
		}
		aRecord, ok := extra.(*dns.A)
		if !ok {
			t.Errorf("Extra record %d is not an A record", i)
			continue
		}
		if aRecord.A == nil {
			t.Errorf("Extra record %d has nil IP", i)
		}
	}
}

func TestGetRootGlueNSNames(t *testing.T) {
	rootGlue := GetRootGlue()

	expectedNS := []string{
		"a.root-servers.net.",
		"b.root-servers.net.",
		"c.root-servers.net.",
		"d.root-servers.net.",
		"e.root-servers.net.",
		"f.root-servers.net.",
		"g.root-servers.net.",
		"h.root-servers.net.",
		"i.root-servers.net.",
		"j.root-servers.net.",
		"k.root-servers.net.",
		"l.root-servers.net.",
		"m.root-servers.net.",
	}

	for i, ns := range rootGlue.Ns {
		nsRecord, ok := ns.(*dns.NS)
		if !ok {
			t.Errorf("NS record %d is not an NS record", i)
			continue
		}
		if nsRecord.Ns != expectedNS[i] {
			t.Errorf("NS record %d: got %s, expected %s", i, nsRecord.Ns, expectedNS[i])
		}
	}
}

func TestGetRootGlueConsistency(t *testing.T) {
	// Test that calling the function multiple times gives consistent results
	rootGlue1 := GetRootGlue()
	rootGlue2 := GetRootGlue()

	if len(rootGlue1.Ns) != len(rootGlue2.Ns) {
		t.Error("GetRootGlue returned inconsistent NS count")
	}

	if len(rootGlue1.Extra) != len(rootGlue2.Extra) {
		t.Error("GetRootGlue returned inconsistent Extra count")
	}
}

func TestSetRootGlue(t *testing.T) {
	// Ensure we restore default after test
	defer ResetRootGlue()

	// Build a custom root glue with a single mock root server
	custom := new(dns.Msg)
	custom.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
			Ns:  "mock-root.test.",
		},
	}
	custom.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "mock-root.test.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("127.0.0.1"),
		},
	}

	SetRootGlue(custom)

	got := GetRootGlue()
	if got == nil {
		t.Fatal("GetRootGlue returned nil after SetRootGlue")
	}
	if len(got.Ns) != 1 {
		t.Fatalf("expected 1 NS record, got %d", len(got.Ns))
	}
	ns, ok := got.Ns[0].(*dns.NS)
	if !ok {
		t.Fatal("NS record is not *dns.NS")
	}
	if ns.Ns != "mock-root.test." {
		t.Errorf("expected NS=mock-root.test., got %s", ns.Ns)
	}
	if len(got.Extra) != 1 {
		t.Fatalf("expected 1 Extra record, got %d", len(got.Extra))
	}
	a, ok := got.Extra[0].(*dns.A)
	if !ok {
		t.Fatal("Extra record is not *dns.A")
	}
	if a.A.String() != "127.0.0.1" {
		t.Errorf("expected A=127.0.0.1, got %s", a.A.String())
	}
}

func TestSetRootGlueDeepCopy(t *testing.T) {
	defer ResetRootGlue()

	custom := new(dns.Msg)
	custom.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
			Ns:  "mock-root.test.",
		},
	}
	custom.Extra = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "mock-root.test.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("127.0.0.1"),
		},
	}

	SetRootGlue(custom)

	// Mutate the original — should not affect stored override
	custom.Ns[0].(*dns.NS).Ns = "mutated.test."

	got := GetRootGlue()
	ns := got.Ns[0].(*dns.NS)
	if ns.Ns != "mock-root.test." {
		t.Errorf("SetRootGlue did not deep-copy: got %s after mutating original", ns.Ns)
	}
}

func TestGetRootGlueReturnsCopy(t *testing.T) {
	defer ResetRootGlue()

	custom := new(dns.Msg)
	custom.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
			Ns:  "mock-root.test.",
		},
	}
	custom.Extra = []dns.RR{}

	SetRootGlue(custom)

	// Mutate the returned value — should not affect subsequent calls
	got1 := GetRootGlue()
	got1.Ns[0].(*dns.NS).Ns = "mutated.test."

	got2 := GetRootGlue()
	ns := got2.Ns[0].(*dns.NS)
	if ns.Ns != "mock-root.test." {
		t.Errorf("GetRootGlue did not return a copy: got %s after mutating previous return", ns.Ns)
	}
}

func TestResetRootGlue(t *testing.T) {
	defer ResetRootGlue()

	custom := new(dns.Msg)
	custom.Ns = []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
			Ns:  "mock-root.test.",
		},
	}
	custom.Extra = []dns.RR{}

	SetRootGlue(custom)
	ResetRootGlue()

	got := GetRootGlue()
	if len(got.Ns) != 13 {
		t.Errorf("after ResetRootGlue expected 13 NS records (default), got %d", len(got.Ns))
	}
}
