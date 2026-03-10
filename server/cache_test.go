package server

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestCache(t *testing.T) {
}

// TestGetCacheKey tests the cache key generation with query type
func TestGetCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		qtype    uint16
		expected string
	}{
		{
			name:     "A record",
			domain:   "example.com.",
			qtype:    dns.TypeA,
			expected: "example.com.:1",
		},
		{
			name:     "AAAA record",
			domain:   "example.com.",
			qtype:    dns.TypeAAAA,
			expected: "example.com.:28",
		},
		{
			name:     "MX record",
			domain:   "google.com.",
			qtype:    dns.TypeMX,
			expected: "google.com.:15",
		},
		{
			name:     "same domain different types",
			domain:   "test.com.",
			qtype:    dns.TypeTXT,
			expected: "test.com.:16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := getCacheKey(tt.domain, tt.qtype)
			if key != tt.expected {
				t.Errorf("getCacheKey(%s, %d) = %s, want %s", tt.domain, tt.qtype, key, tt.expected)
			}
		})
	}
}

// TestCacheByType tests that different record types are cached separately
func TestCacheByType(t *testing.T) {
	// Clear cache before test
	deleteAllCache()

	domain := "cache-type-test.example.com."

	// Create and cache an A record
	aMsg := new(dns.Msg)
	aMsg.SetReply(&dns.Msg{})
	aMsg.Answer = append(aMsg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: net.ParseIP("192.0.2.1"),
	})
	setCacheCopyByType(domain, dns.TypeA, aMsg, 300)

	// Create and cache an AAAA record for the same domain
	aaaaMsg := new(dns.Msg)
	aaaaMsg.SetReply(&dns.Msg{})
	aaaaMsg.Answer = append(aaaaMsg.Answer, &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		AAAA: net.ParseIP("2001:db8::1"),
	})
	setCacheCopyByType(domain, dns.TypeAAAA, aaaaMsg, 300)

	// Retrieve A record
	retrievedA, found := getCacheCopyByType(domain, dns.TypeA)
	if !found {
		t.Fatal("A record not found in cache")
	}
	if len(retrievedA.Answer) != 1 {
		t.Fatalf("Expected 1 A answer, got %d", len(retrievedA.Answer))
	}
	if aRecord, ok := retrievedA.Answer[0].(*dns.A); ok {
		if !aRecord.A.Equal(net.ParseIP("192.0.2.1")) {
			t.Errorf("A record IP mismatch: got %v, want 192.0.2.1", aRecord.A)
		}
	} else {
		t.Error("Retrieved record is not an A record")
	}

	// Retrieve AAAA record
	retrievedAAAA, found := getCacheCopyByType(domain, dns.TypeAAAA)
	if !found {
		t.Fatal("AAAA record not found in cache")
	}
	if len(retrievedAAAA.Answer) != 1 {
		t.Fatalf("Expected 1 AAAA answer, got %d", len(retrievedAAAA.Answer))
	}
	if aaaaRecord, ok := retrievedAAAA.Answer[0].(*dns.AAAA); ok {
		if !aaaaRecord.AAAA.Equal(net.ParseIP("2001:db8::1")) {
			t.Errorf("AAAA record IP mismatch: got %v, want 2001:db8::1", aaaaRecord.AAAA)
		}
	} else {
		t.Error("Retrieved record is not an AAAA record")
	}

	// Verify A record doesn't return when querying for AAAA
	wrongType, found := getCacheCopyByType(domain, dns.TypeMX)
	if found && len(wrongType.Answer) > 0 {
		t.Error("MX query should not return A/AAAA records")
	}
}

// TestCacheIsolation tests that cached messages are not shared
func TestCacheIsolation(t *testing.T) {
	deleteAllCache()

	domain := "isolation-test.example.com."

	// Create and cache a message
	originalMsg := new(dns.Msg)
	originalMsg.SetReply(&dns.Msg{})
	originalMsg.Answer = append(originalMsg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: net.ParseIP("192.0.2.1"),
	})
	setCacheCopyByType(domain, dns.TypeA, originalMsg, 300)

	// Get a copy from cache
	copy1, _ := getCacheCopyByType(domain, dns.TypeA)

	// Modify the copy
	copy1.Answer[0].(*dns.A).A = net.ParseIP("192.0.2.100")

	// Get another copy and verify it's not affected
	copy2, _ := getCacheCopyByType(domain, dns.TypeA)
	originalIP := copy2.Answer[0].(*dns.A).A
	if originalIP.Equal(net.ParseIP("192.0.2.100")) {
		t.Error("Modifying cached copy affected other copies - cache isolation broken")
	}
	if !originalIP.Equal(net.ParseIP("192.0.2.1")) {
		t.Errorf("Expected original IP 192.0.2.1, got %v", originalIP)
	}
}