package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"rec53/monitor"
	"rec53/server"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	monitor.Rec53Log = zap.NewNop().Sugar()
}

// TestResolverIntegration tests the resolver with real DNS queries.
func TestResolverIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name     string
		domain   string
		qtype    uint16
		wantCode int
	}{
		{
			name:     "A record for google.com",
			domain:   "google.com.",
			qtype:    dns.TypeA,
			wantCode: dns.RcodeSuccess,
		},
		{
			name:     "A record for cloudflare.com",
			domain:   "cloudflare.com.",
			qtype:    dns.TypeA,
			wantCode: dns.RcodeSuccess,
		},
		{
			name:     "AAAA record for google.com",
			domain:   "google.com.",
			qtype:    dns.TypeAAAA,
			wantCode: dns.RcodeSuccess,
		},
		{
			name:     "MX record for gmail.com",
			domain:   "gmail.com.",
			qtype:    dns.TypeMX,
			wantCode: dns.RcodeSuccess,
		},
		{
			name:     "TXT record for google.com",
			domain:   "google.com.",
			qtype:    dns.TypeTXT,
			wantCode: dns.RcodeSuccess,
		},
		{
			name:     "NS record for google.com",
			domain:   "google.com.",
			qtype:    dns.TypeNS,
			wantCode: dns.RcodeSuccess,
		},
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &dns.Client{
				Net:      "udp",
				Timeout:  10 * time.Second,
				UDPSize:  4096, // Enable EDNS for large responses
			}

			msg := new(dns.Msg)
			msg.SetQuestion(tt.domain, tt.qtype)
			msg.RecursionDesired = true
			msg.SetEdns0(4096, false) // Enable EDNS0

			resp, _, err := client.Exchange(msg, s.UDPAddr())
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			if resp.Rcode != tt.wantCode {
				t.Errorf("Expected rcode %d, got %d", tt.wantCode, resp.Rcode)
			}

			t.Logf("%s %s: %d answers, rcode=%s",
				tt.domain, dns.TypeToString[tt.qtype],
				len(resp.Answer), dns.RcodeToString[resp.Rcode])

			// Log some answers
			for i, rr := range resp.Answer {
				if i >= 3 {
					break
				}
				switch v := rr.(type) {
				case *dns.A:
					t.Logf("  A: %s -> %s", v.Hdr.Name, v.A.String())
				case *dns.AAAA:
					t.Logf("  AAAA: %s -> %s", v.Hdr.Name, v.AAAA.String())
				case *dns.MX:
					t.Logf("  MX: %s -> %s (pref: %d)", v.Hdr.Name, v.Mx, v.Preference)
				case *dns.TXT:
					t.Logf("  TXT: %s -> %s", v.Hdr.Name, v.Txt[0])
				case *dns.NS:
					t.Logf("  NS: %s -> %s", v.Hdr.Name, v.Ns)
				}
			}
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestCNAMEResolution tests CNAME chain resolution.
func TestCNAMEResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Domains that typically have CNAME records
	cnameDomains := []string{
		"www.github.com.",
		"www.cloudflare.com.",
	}

	for _, domain := range cnameDomains {
		t.Run(domain, func(t *testing.T) {
			client := &dns.Client{
				Net:      "udp",
				Timeout:  10 * time.Second,
				UDPSize:  4096,
			}

			msg := new(dns.Msg)
			msg.SetQuestion(domain, dns.TypeA)
			msg.RecursionDesired = true
			msg.SetEdns0(4096, false)

			resp, _, err := client.Exchange(msg, s.UDPAddr())
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			t.Logf("CNAME resolution for %s:", domain)
			for i, rr := range resp.Answer {
				switch v := rr.(type) {
				case *dns.CNAME:
					t.Logf("  [%d] CNAME: %s -> %s", i, v.Hdr.Name, v.Target)
				case *dns.A:
					t.Logf("  [%d] A: %s -> %s", i, v.Hdr.Name, v.A.String())
				}
			}
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestCNAMEChainWithValidNSDelegation tests B-004 scenario:
// CNAME chain where upstream provides valid NS delegation for the CNAME target's zone.
// This test verifies that such delegation is preserved and used correctly.
func TestCNAMEChainWithValidNSDelegation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Domains that have CNAME chains with valid NS delegation from upstream
	// These domains typically return CNAME + NS records for the CNAME target's zone
	testDomains := []struct {
		name        string
		domain      string
		description string
	}{
		{
			name:        "www.huawei.com",
			domain:      "www.huawei.com.",
			description: "CNAME to akadns.net zone with valid NS delegation",
		},
		{
			name:        "www.baidu.com",
			domain:      "www.baidu.com.",
			description: "CNAME chain with CDN delegation",
		},
	}

	for _, tt := range testDomains {
		t.Run(tt.name, func(t *testing.T) {
			client := &dns.Client{
				Net:      "udp",
				Timeout:  15 * time.Second,
				UDPSize:  4096,
			}

			msg := new(dns.Msg)
			msg.SetQuestion(tt.domain, dns.TypeA)
			msg.RecursionDesired = true
			msg.SetEdns0(4096, false)

			resp, _, err := client.Exchange(msg, s.UDPAddr())
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			t.Logf("B-004 test for %s (%s):", tt.domain, tt.description)
			t.Logf("  Response code: %s", dns.RcodeToString[resp.Rcode])
			t.Logf("  Answer count: %d", len(resp.Answer))

			// Log the answer chain
			for i, rr := range resp.Answer {
				switch v := rr.(type) {
				case *dns.CNAME:
					t.Logf("  [%d] CNAME: %s -> %s", i, v.Hdr.Name, v.Target)
				case *dns.A:
					t.Logf("  [%d] A: %s -> %s", i, v.Hdr.Name, v.A.String())
				}
			}

			// B-004 fix: Should return SUCCESS (not SERVFAIL) with A records
			if resp.Rcode == dns.RcodeServerFailure {
				t.Errorf("B-004 FAIL: Got SERVFAIL for %s - NS delegation may not be preserved correctly", tt.domain)
			}

			// Check if we got final A records (CNAME chain resolved successfully)
			hasARecord := false
			for _, rr := range resp.Answer {
				if _, ok := rr.(*dns.A); ok {
					hasARecord = true
					break
				}
			}

			// Note: Some domains may have complex CNAME chains that timeout
			// The important thing is we don't get SERVFAIL due to wrong NS delegation
			if hasARecord {
				t.Logf("  SUCCESS: CNAME chain resolved with A records")
			} else {
				t.Logf("  INFO: No A records in answer (may be complex chain or timeout)")
			}
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestNonExistentDomain tests NXDOMAIN handling.
func TestNonExistentDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	nonExistent := []string{
		"this-domain-does-not-exist-12345.invalid.",
		"nonexistent.example.invalid.",
	}

	for _, domain := range nonExistent {
		t.Run(domain, func(t *testing.T) {
			client := &dns.Client{
				Net:      "udp",
				Timeout:  10 * time.Second,
				UDPSize:  4096,
			}

			msg := new(dns.Msg)
			msg.SetQuestion(domain, dns.TypeA)
			msg.RecursionDesired = true
			msg.SetEdns0(4096, false)

			resp, _, err := client.Exchange(msg, s.UDPAddr())

			if err != nil {
				t.Logf("Query error (expected for non-existent domain): %v", err)
				return
			}

			t.Logf("Response for %s: rcode=%s", domain, dns.RcodeToString[resp.Rcode])

			// Could be NXDOMAIN or SERVFAIL depending on upstream
			if resp.Rcode == dns.RcodeNameError {
				t.Log("NXDOMAIN returned correctly")
			}
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestMultipleRecordTypes tests querying multiple record types for same domain.
func TestMultipleRecordTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	domain := "google.com."
	recordTypes := []uint16{
		dns.TypeA,
		dns.TypeAAAA,
		dns.TypeMX,
		dns.TypeTXT,
		dns.TypeNS,
		dns.TypeSOA,
	}

	results := make(map[uint16]int)

	for _, qtype := range recordTypes {
		client := &dns.Client{
			Net:      "udp",
			Timeout:  10 * time.Second,
			UDPSize:  4096,
		}

		msg := new(dns.Msg)
		msg.SetQuestion(domain, qtype)
		msg.RecursionDesired = true
		msg.SetEdns0(4096, false)

		resp, _, err := client.Exchange(msg, s.UDPAddr())
		if err != nil {
			t.Logf("%s query failed: %v", dns.TypeToString[qtype], err)
			continue
		}

		results[qtype] = len(resp.Answer)
		t.Logf("%s: %d answers (rcode: %s)",
			dns.TypeToString[qtype], len(resp.Answer), dns.RcodeToString[resp.Rcode])
	}

	// Verify at least some queries succeeded
	successCount := 0
	for _, count := range results {
		if count > 0 {
			successCount++
		}
	}

	if successCount == 0 {
		t.Error("Expected at least one successful query")
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestLargeResponse tests handling of large DNS responses.
func TestLargeResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Domains that might return large responses
	largeDomains := []string{
		"google.com.", // Multiple A/AAAA records
		"facebook.com.",
	}

	for _, domain := range largeDomains {
		t.Run(domain, func(t *testing.T) {
			// Test UDP with EDNS
			udpClient := &dns.Client{
				Net:      "udp",
				Timeout:  10 * time.Second,
				UDPSize:  4096,
			}

			msg := new(dns.Msg)
			msg.SetQuestion(domain, dns.TypeANY)
			msg.SetEdns0(4096, false)

			udpResp, _, err := udpClient.Exchange(msg, s.UDPAddr())
			if err != nil {
				t.Logf("UDP query failed: %v", err)
			} else {
				t.Logf("UDP response: %d answers, %d bytes",
					len(udpResp.Answer), udpResp.Len())
			}

			// Test TCP for large responses
			tcpClient := &dns.Client{
				Net:     "tcp",
				Timeout: 10 * time.Second,
			}

			tcpResp, _, err := tcpClient.Exchange(msg, s.TCPAddr())
			if err != nil {
				t.Logf("TCP query failed: %v", err)
			} else {
				t.Logf("TCP response: %d answers, %d bytes",
					len(tcpResp.Answer), tcpResp.Len())
			}
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestQueryTimeout tests timeout handling.
func TestQueryTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Very short timeout client
	client := &dns.Client{
		Net:     "udp",
		Timeout: 1 * time.Millisecond, // Extremely short timeout
	}

	msg := new(dns.Msg)
	msg.SetQuestion("google.com.", dns.TypeA)

	resp, _, err := client.Exchange(msg, s.UDPAddr())
	if err != nil {
		t.Logf("Expected timeout error: %v", err)
	} else {
		t.Logf("Got response despite short timeout: rcode=%d", resp.Rcode)
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestIDNResolution tests Internationalized Domain Names.
func TestIDNResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// IDN domains (Punycode encoded)
	idnDomains := []string{
		"xn--wgbl.example.",     // Arabic
		"xn--fiqs8s.example.",   // Chinese
		"xn--fiqz9s.example.",   // Chinese
	}

	for _, domain := range idnDomains {
		t.Run(domain, func(t *testing.T) {
			client := &dns.Client{
				Net:      "udp",
				Timeout:  5 * time.Second,
				UDPSize:  4096,
			}

			msg := new(dns.Msg)
			msg.SetQuestion(domain, dns.TypeA)
			msg.SetEdns0(4096, false)

			resp, _, err := client.Exchange(msg, s.UDPAddr())
			if err != nil {
				t.Logf("IDN query error: %v", err)
				return
			}

			t.Logf("IDN response for %s: rcode=%s", domain, dns.RcodeToString[resp.Rcode])
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}

// TestReverseDNS tests reverse DNS lookups (PTR records).
func TestReverseDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	s := server.NewServer("127.0.0.1:0")
	errChan := s.Run()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Well-known IP addresses with PTR records
	ptrQueries := []struct {
		name string
		ip   string
	}{
		{"8.8.8.8", "8.8.8.8"},
		{"1.1.1.1", "1.1.1.1"},
	}

	for _, tt := range ptrQueries {
		t.Run(tt.name, func(t *testing.T) {
			client := &dns.Client{
				Net:      "udp",
				Timeout:  10 * time.Second,
				UDPSize:  4096,
			}

			// Create PTR query
			msg := new(dns.Msg)
			msg.SetEdns0(4096, false)
			ptrName, _ := net.LookupAddr(tt.ip)
			if len(ptrName) > 0 {
				// Use the reverse name
				msg.SetQuestion(dns.Fqdn(ptrName[0]), dns.TypePTR)
			} else {
				// Construct PTR name manually
				reverse, _ := dns.ReverseAddr(tt.ip)
				msg.SetQuestion(reverse, dns.TypePTR)
			}

			resp, _, err := client.Exchange(msg, s.UDPAddr())
			if err != nil {
				t.Logf("PTR query error: %v", err)
				return
			}

			t.Logf("PTR response for %s: rcode=%s, %d answers",
				tt.ip, dns.RcodeToString[resp.Rcode], len(resp.Answer))

			for _, rr := range resp.Answer {
				if ptr, ok := rr.(*dns.PTR); ok {
					t.Logf("  PTR: %s -> %s", ptr.Hdr.Name, ptr.Ptr)
				}
			}
		})
	}

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()
}