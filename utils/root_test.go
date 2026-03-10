package utils

import (
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