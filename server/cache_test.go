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

// TestCacheIsolation tests that the write-side deep copy protects the cached
// entry from mutations to the caller's original message. With shallow copy on
// read, callers MUST NOT modify individual RR fields on cache-read values
// (see cache safety invariant in cache.go). This test verifies the write-side
// guarantee only: mutating the source message after setCacheCopy does not
// affect the cached entry.
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

	// Mutate the caller's original message AFTER caching
	originalMsg.Answer[0].(*dns.A).A = net.ParseIP("192.0.2.100")

	// Verify cached entry still has the original IP (write-side deep copy)
	cached, _ := getCacheCopyByType(domain, dns.TypeA)
	cachedIP := cached.Answer[0].(*dns.A).A
	if cachedIP.Equal(net.ParseIP("192.0.2.100")) {
		t.Error("Mutating original message after setCacheCopy affected cache — write-side isolation broken")
	}
	if !cachedIP.Equal(net.ParseIP("192.0.2.1")) {
		t.Errorf("Expected cached IP 192.0.2.1, got %v", cachedIP)
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
		A:   net.ParseIP("192.0.2.1"),
	})
	setCacheCopyByType(domain, dns.TypeA, aMsg, 300)

	// Cache AAAA record
	aaaaMsg := new(dns.Msg)
	aaaaMsg.Answer = append(aaaaMsg.Answer, &dns.AAAA{
		Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
		AAAA: net.ParseIP("2001:db8::1"),
	})
	setCacheCopyByType(domain, dns.TypeAAAA, aaaaMsg, 300)

	// Cache MX record
	mxMsg := new(dns.Msg)
	mxMsg.Answer = append(mxMsg.Answer, &dns.MX{
		Hdr:        dns.RR_Header{Name: domain, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 300},
		Mx:         "mail.example.com.",
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

// ---------------------------------------------------------------------------
// Task 1.3: OPT stripping tests
// ---------------------------------------------------------------------------

// TestStripOPT covers: OPT removed, non-OPT preserved, no-OPT no-op, multiple OPT stripped.
func TestStripOPT(t *testing.T) {
	glueA := &dns.A{
		Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.53"),
	}
	glueAAAA := &dns.AAAA{
		Hdr:  dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
		AAAA: net.ParseIP("2001:db8::53"),
	}
	opt1 := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	opt2 := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}

	tests := []struct {
		name      string
		extra     []dns.RR
		wantLen   int
		wantTypes []uint16
	}{
		{
			name:      "single OPT removed, glue preserved",
			extra:     []dns.RR{glueA, opt1},
			wantLen:   1,
			wantTypes: []uint16{dns.TypeA},
		},
		{
			name:      "no OPT present — no-op",
			extra:     []dns.RR{glueA, glueAAAA},
			wantLen:   2,
			wantTypes: []uint16{dns.TypeA, dns.TypeAAAA},
		},
		{
			name:    "empty Extra — no-op",
			extra:   nil,
			wantLen: 0,
		},
		{
			name:      "multiple OPT stripped",
			extra:     []dns.RR{opt1, glueAAAA, opt2},
			wantLen:   1,
			wantTypes: []uint16{dns.TypeAAAA},
		},
		{
			name:    "only OPT records — all removed",
			extra:   []dns.RR{opt1, opt2},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &dns.Msg{}
			// Make a copy of the extra slice so tests are independent
			msg.Extra = make([]dns.RR, len(tt.extra))
			copy(msg.Extra, tt.extra)

			stripOPT(msg)

			if len(msg.Extra) != tt.wantLen {
				t.Fatalf("Extra length = %d, want %d", len(msg.Extra), tt.wantLen)
			}
			for i, wantType := range tt.wantTypes {
				if msg.Extra[i].Header().Rrtype != wantType {
					t.Errorf("Extra[%d] type = %d, want %d", i, msg.Extra[i].Header().Rrtype, wantType)
				}
			}
		})
	}
}

// TestStripOPTOnCacheWrite verifies that setCacheCopy strips OPT from the cached
// entry while preserving non-OPT Extra records and leaving the caller's message untouched.
func TestStripOPTOnCacheWrite(t *testing.T) {
	deleteAllCache()

	domain := "opt-strip.example.com."
	msg := &dns.Msg{}
	msg.SetQuestion(domain, dns.TypeA)
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.1"),
	})
	glue := &dns.A{
		Hdr: dns.RR_Header{Name: "ns1." + domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.53"),
	}
	opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
	msg.Extra = []dns.RR{glue, opt}

	setCacheCopyByType(domain, dns.TypeA, msg, 300)

	// Caller's message should still have OPT
	if len(msg.Extra) != 2 {
		t.Fatalf("caller's Extra was modified: len=%d, want 2", len(msg.Extra))
	}

	// Cached entry should have OPT stripped
	cached, found := getCacheCopyByType(domain, dns.TypeA)
	if !found {
		t.Fatal("expected cached entry")
	}
	for _, rr := range cached.Extra {
		if _, ok := rr.(*dns.OPT); ok {
			t.Error("cached entry contains OPT record — stripOPT failed on write")
		}
	}
	if len(cached.Extra) != 1 {
		t.Fatalf("cached Extra length = %d, want 1 (glue only)", len(cached.Extra))
	}
	if cached.Extra[0].Header().Rrtype != dns.TypeA {
		t.Errorf("cached Extra[0] type = %d, want A record", cached.Extra[0].Header().Rrtype)
	}
}

// ---------------------------------------------------------------------------
// Task 2.3: Shallow copy correctness tests
// ---------------------------------------------------------------------------

// TestShallowCopyMsg verifies: independent slice headers, shared RR pointers, all fields preserved.
// Question is intentionally NOT copied by shallowCopyMsg — callers do not use it
// and server.go restores the original question from the client request.
func TestShallowCopyMsg(t *testing.T) {
	original := &dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: 1234, Response: true, Authoritative: true},
		Compress: true,
	}
	original.Question = []dns.Question{
		{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
	}
	aRR := &dns.A{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.1"),
	}
	nsRR := &dns.NS{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
		Ns:  "ns1.example.com.",
	}
	glueRR := &dns.A{
		Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.53"),
	}
	original.Answer = []dns.RR{aRR}
	original.Ns = []dns.RR{nsRR}
	original.Extra = []dns.RR{glueRR}

	cp := shallowCopyMsg(original)

	// Fields preserved
	if cp.MsgHdr.Id != 1234 {
		t.Errorf("Id = %d, want 1234", cp.MsgHdr.Id)
	}
	if !cp.MsgHdr.Response {
		t.Error("Response flag not preserved")
	}
	if !cp.Compress {
		t.Error("Compress flag not preserved")
	}

	// Question is intentionally NOT copied — shallowCopyMsg skips it
	// to save 1 alloc/op; callers rely on server.go to restore Question.
	if len(cp.Question) != 0 {
		t.Fatalf("Question len = %d, want 0 (not copied)", len(cp.Question))
	}

	// Answer, Ns, Extra slice lengths preserved
	if len(cp.Answer) != 1 {
		t.Fatalf("Answer len = %d, want 1", len(cp.Answer))
	}
	if len(cp.Ns) != 1 {
		t.Fatalf("Ns len = %d, want 1", len(cp.Ns))
	}
	if len(cp.Extra) != 1 {
		t.Fatalf("Extra len = %d, want 1", len(cp.Extra))
	}

	// Shared RR pointers (same address)
	if cp.Answer[0] != original.Answer[0] {
		t.Error("Answer[0] pointer differs — expected shared RR pointer")
	}
	if cp.Ns[0] != original.Ns[0] {
		t.Error("Ns[0] pointer differs — expected shared RR pointer")
	}
	if cp.Extra[0] != original.Extra[0] {
		t.Error("Extra[0] pointer differs — expected shared RR pointer")
	}
}

// TestShallowCopyMsgEmpty verifies shallowCopyMsg handles empty/nil slices.
func TestShallowCopyMsgEmpty(t *testing.T) {
	original := &dns.Msg{MsgHdr: dns.MsgHdr{Id: 42}}
	cp := shallowCopyMsg(original)

	if cp.MsgHdr.Id != 42 {
		t.Errorf("Id = %d, want 42", cp.MsgHdr.Id)
	}
	if cp.Question != nil {
		t.Error("expected nil Question slice for empty original")
	}
	if cp.Answer != nil {
		t.Error("expected nil Answer slice for empty original")
	}
	if cp.Ns != nil {
		t.Error("expected nil Ns slice for empty original")
	}
	if cp.Extra != nil {
		t.Error("expected nil Extra slice for empty original")
	}
}

// ---------------------------------------------------------------------------
// Task 2.4: Shallow copy slice isolation
// ---------------------------------------------------------------------------

// TestShallowCopySliceIsolation verifies that append/nil on the returned slice
// does not affect the cached entry.
func TestShallowCopySliceIsolation(t *testing.T) {
	deleteAllCache()

	domain := "slice-isolation.example.com."
	msg := &dns.Msg{}
	msg.SetQuestion(domain, dns.TypeA)
	rr1 := &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.1"),
	}
	rr2 := &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.2"),
	}
	msg.Answer = []dns.RR{rr1, rr2}
	setCacheCopyByType(domain, dns.TypeA, msg, 300)

	// Read 1: append to returned slice
	read1, _ := getCacheCopyByType(domain, dns.TypeA)
	extraRR := &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.ParseIP("10.0.0.1"),
	}
	read1.Answer = append(read1.Answer, extraRR)

	// Read 2: nil the slice
	read2, _ := getCacheCopyByType(domain, dns.TypeA)
	read2.Answer = nil

	// Read 3: verify original cached entry is unaffected
	read3, found := getCacheCopyByType(domain, dns.TypeA)
	if !found {
		t.Fatal("cached entry disappeared")
	}
	if len(read3.Answer) != 2 {
		t.Fatalf("cached Answer length = %d, want 2 (slice isolation broken)", len(read3.Answer))
	}
}

// ---------------------------------------------------------------------------
// Task 3.1: Concurrent read + Pack() race test
// ---------------------------------------------------------------------------

// TestCacheConcurrentReadPack verifies that 100 goroutines can concurrently
// read the same cache key, append RRs to a response, and call Pack() without
// any data race. Must be run with -race.
func TestCacheConcurrentReadPack(t *testing.T) {
	deleteAllCache()

	domain := "race-pack.example.com."
	msg := &dns.Msg{}
	msg.SetQuestion(domain, dns.TypeA)
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.1"),
	})
	msg.Ns = append(msg.Ns, &dns.NS{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
		Ns:  "ns1.example.com.",
	})
	msg.Extra = append(msg.Extra, &dns.A{
		Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.53"),
	})
	setCacheCopyByType(domain, dns.TypeA, msg, 300)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cached, found := getCacheCopyByType(domain, dns.TypeA)
			if !found {
				t.Error("cache miss in concurrent read")
				return
			}

			// Build a response using cached RRs — mimics real handler behavior
			resp := new(dns.Msg)
			resp.SetReply(&dns.Msg{MsgHdr: dns.MsgHdr{Id: 9999}})
			resp.Answer = append(resp.Answer, cached.Answer...)
			resp.Ns = append(resp.Ns, cached.Ns...)
			resp.Extra = append(resp.Extra, cached.Extra...)

			// Pack() is the operation that must be race-free on shared RRs
			wire, err := resp.Pack()
			if err != nil {
				t.Errorf("Pack() failed: %v", err)
				return
			}
			if len(wire) == 0 {
				t.Error("Pack() returned empty wire")
			}
		}()
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Task 3.2: Concurrent read/write race test
// ---------------------------------------------------------------------------

// TestCacheConcurrentReadWrite verifies that N readers + 1 writer on the same
// key produce no data race and readers always get valid messages.
func TestCacheConcurrentReadWrite(t *testing.T) {
	deleteAllCache()

	domain := "race-rw.example.com."

	// Seed the cache
	seedMsg := &dns.Msg{}
	seedMsg.SetQuestion(domain, dns.TypeA)
	seedMsg.Answer = append(seedMsg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.1"),
	})
	setCacheCopyByType(domain, dns.TypeA, seedMsg, 300)

	const readers = 50
	const writerIters = 200
	const readerIters = 200

	var wg sync.WaitGroup

	// 1 writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < writerIters; j++ {
			m := &dns.Msg{}
			m.SetQuestion(domain, dns.TypeA)
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   net.ParseIP("192.0.2.1"),
			})
			setCacheCopyByType(domain, dns.TypeA, m, 300)
		}
	}()

	// N reader goroutines
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < readerIters; j++ {
				cached, found := getCacheCopyByType(domain, dns.TypeA)
				if !found {
					// Writer might be replacing; tolerate misses
					continue
				}
				if len(cached.Answer) < 1 {
					t.Error("cached message has no Answer — invalid state")
				}
				// Pack to verify message integrity
				if _, err := cached.Pack(); err != nil {
					t.Errorf("Pack() failed on cache read: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Task 4.1: Wire-format equivalence test
// ---------------------------------------------------------------------------

// TestShallowVsDeepCopyWireFormat verifies that shallowCopyMsg produces a
// message that can be packed without error for representative message types.
//
// Wire bytes deliberately differ from deep copy because shallowCopyMsg omits
// the Question section (saved 1 alloc/op); server.go restores Question from
// the original client request before packing the final reply.
func TestShallowVsDeepCopyWireFormat(t *testing.T) {
	messages := map[string]*dns.Msg{
		"A record": func() *dns.Msg {
			m := &dns.Msg{}
			m.SetQuestion("example.com.", dns.TypeA)
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   net.ParseIP("192.0.2.1"),
			})
			return m
		}(),
		"NS delegation": func() *dns.Msg {
			m := &dns.Msg{}
			m.SetQuestion("example.com.", dns.TypeNS)
			m.Ns = append(m.Ns, &dns.NS{
				Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 86400},
				Ns:  "ns1.example.com.",
			})
			m.Extra = append(m.Extra, &dns.A{
				Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 86400},
				A:   net.ParseIP("192.0.2.53"),
			})
			return m
		}(),
		"CNAME chain": func() *dns.Msg {
			m := &dns.Msg{}
			m.SetQuestion("www.example.com.", dns.TypeA)
			m.Answer = append(m.Answer,
				&dns.CNAME{
					Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
					Target: "example.com.",
				},
				&dns.A{
					Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
					A:   net.ParseIP("192.0.2.1"),
				},
			)
			return m
		}(),
		"NXDOMAIN": func() *dns.Msg {
			m := &dns.Msg{}
			m.SetQuestion("nonexistent.example.com.", dns.TypeA)
			m.MsgHdr.Rcode = dns.RcodeNameError
			m.Ns = append(m.Ns, &dns.SOA{
				Hdr:     dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 300},
				Ns:      "ns1.example.com.",
				Mbox:    "admin.example.com.",
				Serial:  2024010101,
				Refresh: 3600,
				Retry:   900,
				Expire:  604800,
				Minttl:  86400,
			})
			return m
		}(),
	}

	for name, original := range messages {
		t.Run(name, func(t *testing.T) {
			shallow := shallowCopyMsg(original)

			// Shallow copy must pack cleanly (no error).
			shallowWire, err := shallow.Pack()
			if err != nil {
				t.Fatalf("shallow Pack() error: %v", err)
			}
			if len(shallowWire) == 0 {
				t.Fatal("shallow Pack() returned empty wire bytes")
			}

			// Shallow wire must be shorter than deep copy because the
			// Question section is intentionally omitted.
			deepWire, err := original.Copy().Pack()
			if err != nil {
				t.Fatalf("deep Pack() error: %v", err)
			}
			if len(shallowWire) >= len(deepWire) {
				t.Fatalf("expected shallow wire (%d bytes) < deep wire (%d bytes): Question section not omitted?", len(shallowWire), len(deepWire))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Task 4.2: Writer mutation does not affect cache
// ---------------------------------------------------------------------------

// TestWriterMutationDoesNotAffectCache writes an entry, mutates the caller's
// message, then verifies the cached entry is unchanged.
func TestWriterMutationDoesNotAffectCache(t *testing.T) {
	deleteAllCache()

	domain := "writer-mutation.example.com."
	msg := &dns.Msg{}
	msg.SetQuestion(domain, dns.TypeA)
	originalRR := &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("192.0.2.1"),
	}
	msg.Answer = append(msg.Answer, originalRR)
	setCacheCopyByType(domain, dns.TypeA, msg, 300)

	// Mutate the caller's message in multiple ways
	msg.Answer[0].(*dns.A).A = net.ParseIP("10.0.0.99")
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.ParseIP("10.0.0.100"),
	})
	msg.MsgHdr.Rcode = dns.RcodeServerFailure

	// Verify cached entry is unaffected
	cached, found := getCacheCopyByType(domain, dns.TypeA)
	if !found {
		t.Fatal("cached entry not found")
	}
	if cached.MsgHdr.Rcode != dns.RcodeSuccess {
		t.Errorf("cached Rcode = %d, want NOERROR", cached.MsgHdr.Rcode)
	}
	if len(cached.Answer) != 1 {
		t.Fatalf("cached Answer len = %d, want 1", len(cached.Answer))
	}
	cachedIP := cached.Answer[0].(*dns.A).A
	if !cachedIP.Equal(net.ParseIP("192.0.2.1")) {
		t.Errorf("cached IP = %v, want 192.0.2.1", cachedIP)
	}
}
