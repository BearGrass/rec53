package server

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestCache(t *testing.T) {
	// Basic smoke test for cache operations
	deleteAllCache()

	// Verify cache is empty
	if getCacheSize() != 0 {
		t.Errorf("expected empty cache, got %d items", getCacheSize())
	}
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

// TestSetAndGetCache tests basic cache set and get operations
func TestSetAndGetCache(t *testing.T) {
	deleteAllCache()

	key := "test.example.com.:1"
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   "test.example.com.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: net.ParseIP("192.0.2.1"),
	})

	// Test set and get
	setCacheCopy(key, msg, 300)
	retrieved, found := getCacheCopy(key)
	if !found {
		t.Fatal("expected to find cached message")
	}
	if len(retrieved.Answer) != 1 {
		t.Errorf("expected 1 answer, got %d", len(retrieved.Answer))
	}

	// Test non-existent key
	_, found = getCacheCopy("nonexistent.:1")
	if found {
		t.Error("expected not to find non-existent key")
	}

	// Test cache size
	if getCacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", getCacheSize())
	}
}

// TestCacheExpiration tests that cache entries expire correctly
func TestCacheExpiration(t *testing.T) {
	deleteAllCache()

	domain := "expire-test.example.com."
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   domain,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    1, // 1 second TTL
		},
		A: net.ParseIP("192.0.2.1"),
	})

	// Set with very short expiration (1 second)
	setCacheCopyByType(domain, dns.TypeA, msg, 1)

	// Should be found immediately
	_, found := getCacheCopyByType(domain, dns.TypeA)
	if !found {
		t.Fatal("expected to find cached message immediately after set")
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Force delete expired items
	deleteExpiredCache()

	// Should not be found after expiration
	_, found = getCacheCopyByType(domain, dns.TypeA)
	if found {
		t.Error("expected cache entry to be expired")
	}
}

// TestCacheConcurrentAccess tests concurrent read/write operations
func TestCacheConcurrentAccess(t *testing.T) {
	deleteAllCache()

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			domain := "concurrent.example.com."
			for j := 0; j < iterations; j++ {
				msg := new(dns.Msg)
				msg.SetReply(&dns.Msg{})
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   domain,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP("192.0.2.1"),
				})
				setCacheCopyByType(domain, dns.TypeA, msg, 300)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = getCacheCopyByType("concurrent.example.com.", dns.TypeA)
			}
		}()
	}

	// Concurrent flush operations
	for i := 0; i < goroutines/10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations/10; j++ {
				deleteExpiredCache()
			}
		}()
	}

	wg.Wait()
}

// TestCacheFlush tests the deleteAllCache function
func TestCacheFlush(t *testing.T) {
	deleteAllCache()

	// Add multiple entries
	for i := 0; i < 10; i++ {
		domain := "flush-test.example.com."
		msg := new(dns.Msg)
		msg.SetReply(&dns.Msg{})
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP("192.0.2.1"),
		})
		setCacheCopyByType(domain, dns.TypeA, msg, 300)
	}

	// Verify cache has entries
	if getCacheSize() == 0 {
		t.Fatal("expected cache to have entries before flush")
	}

	// Flush cache
	deleteAllCache()

	// Verify cache is empty
	if getCacheSize() != 0 {
		t.Errorf("expected empty cache after flush, got %d items", getCacheSize())
	}
}

// TestCacheMultipleTypesSameDomain tests caching multiple record types for the same domain
func TestCacheMultipleTypesSameDomain(t *testing.T) {
	deleteAllCache()

	domain := "multi.example.com."

	// Cache A record
	aMsg := new(dns.Msg)
	aMsg.Answer = append(aMsg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A: net.ParseIP("192.0.2.1"),
	})
	setCacheCopyByType(domain, dns.TypeA, aMsg, 300)

	// Cache AAAA record
	aaaaMsg := new(dns.Msg)
	aaaaMsg.Answer = append(aaaaMsg.Answer, &dns.AAAA{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
		AAAA: net.ParseIP("2001:db8::1"),
	})
	setCacheCopyByType(domain, dns.TypeAAAA, aaaaMsg, 300)

	// Cache MX record
	mxMsg := new(dns.Msg)
	mxMsg.Answer = append(mxMsg.Answer, &dns.MX{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 300},
		Mx: "mail.example.com.",
		Preference: 10,
	})
	setCacheCopyByType(domain, dns.TypeMX, mxMsg, 300)

	// Verify all types are cached separately
	if getCacheSize() != 3 {
		t.Errorf("expected cache size 3, got %d", getCacheSize())
	}

	// Verify each type can be retrieved
	for _, tc := range []struct {
		qtype    uint16
		typeName string
	}{
		{dns.TypeA, "A"},
		{dns.TypeAAAA, "AAAA"},
		{dns.TypeMX, "MX"},
	} {
		_, found := getCacheCopyByType(domain, tc.qtype)
		if !found {
			t.Errorf("expected to find %s record", tc.typeName)
		}
	}
}