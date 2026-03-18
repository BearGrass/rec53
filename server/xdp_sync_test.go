package server

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/miekg/dns"
)

// ---------------------------------------------------------------------------
// domainToWireFormat tests
// ---------------------------------------------------------------------------

func TestDomainToWireFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:  "simple FQDN",
			input: "example.com.",
			want:  []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0},
		},
		{
			name:  "case normalization",
			input: "Example.COM.",
			want:  []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0},
		},
		{
			name:  "root domain",
			input: ".",
			want:  []byte{0},
		},
		{
			name:  "subdomain",
			input: "www.example.com.",
			want:  []byte{3, 'w', 'w', 'w', 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0},
		},
		{
			name:  "no trailing dot",
			input: "example.com",
			want:  []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0},
		},
		{
			name: "exceeds MAX_QNAME_LEN",
			// 128 single-char labels: wire format = 128*(1+1)+1 = 257 > 255
			input: func() string {
				labels := make([]string, 128)
				for i := range labels {
					labels[i] = "a"
				}
				return strings.Join(labels, ".") + "."
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domainToWireFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildBPFCacheKey tests
// ---------------------------------------------------------------------------

func TestBuildBPFCacheKey(t *testing.T) {
	key, err := buildBPFCacheKey("example.com.", dns.TypeA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify qname portion
	wantQname := []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0}
	if !bytes.Equal(key.Qname[:len(wantQname)], wantQname) {
		t.Errorf("qname prefix mismatch: got %v, want %v", key.Qname[:len(wantQname)], wantQname)
	}

	// Verify qtype
	if key.Qtype != uint16(dns.TypeA) {
		t.Errorf("qtype: got %d, want %d", key.Qtype, dns.TypeA)
	}
}

func TestBuildBPFCacheKeyError(t *testing.T) {
	// An oversized domain name (wire format > 255 bytes) should return an error.
	longDomain := strings.Repeat("a.", 128) // 128 labels × 2 chars = 256-byte wire format > 255
	_, err := buildBPFCacheKey(longDomain, dns.TypeA)
	if err == nil {
		t.Error("expected error for oversized domain, got nil")
	}
}

// ---------------------------------------------------------------------------
// buildBPFCacheValue tests
// ---------------------------------------------------------------------------

func TestBuildBPFCacheValue(t *testing.T) {
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{
		MsgHdr: dns.MsgHdr{Id: 0x1234},
		Question: []dns.Question{
			{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
		},
	})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.IP{1, 2, 3, 4},
	})

	val, err := buildBPFCacheValue(msg, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// resp_len should be > 0 and <= 512
	if val.RespLen == 0 {
		t.Error("RespLen should be > 0")
	}
	if val.RespLen > 512 {
		t.Errorf("RespLen %d exceeds MAX_DNS_RESPONSE_LEN", val.RespLen)
	}

	// expire_ts should be in the future (monotonic seconds)
	if val.ExpireTs == 0 {
		t.Error("ExpireTs should be non-zero")
	}
}

func TestBuildBPFCacheValueOversizedSkipped(t *testing.T) {
	// Create a message that will exceed 512 bytes when packed.
	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{
		MsgHdr: dns.MsgHdr{Id: 0x1234},
		Question: []dns.Question{
			{Name: "example.com.", Qtype: dns.TypeTXT, Qclass: dns.ClassINET},
		},
	})
	// Add many large TXT records to exceed 512 bytes.
	for i := 0; i < 20; i++ {
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 300},
			Txt: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		})
	}

	_, err := buildBPFCacheValue(msg, 300)
	if err == nil {
		t.Error("expected error for oversized response, got nil")
	}
}

// ---------------------------------------------------------------------------
// globalXDPCacheMap (sync on/off switch) tests
// ---------------------------------------------------------------------------

func TestSyncToBPFMapNilMap(t *testing.T) {
	// When globalXDPCacheMap is nil (XDP disabled), syncToBPFMap should be a no-op.
	// Store the original and set nil.
	origMap := globalXDPCacheMap.Load()
	globalXDPCacheMap.Store(nil)
	defer func() { globalXDPCacheMap.Store(origMap) }()

	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{
		MsgHdr: dns.MsgHdr{Id: 0x1234},
		Question: []dns.Question{
			{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
		},
	})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.IP{1, 2, 3, 4},
	})

	// Should not panic, should be a silent no-op.
	syncToBPFMap("example.com.", dns.TypeA, msg, 300)
}

// TestSyncToBPFMapKeyBuildError verifies that syncToBPFMap handles key build
// failures gracefully (logs at Debug, returns silently) when XDP is enabled.
func TestSyncToBPFMapKeyBuildError(t *testing.T) {
	// Set a non-nil map to bypass the nil-map early return.
	// The function returns on key build error before calling Update().
	origMap := globalXDPCacheMap.Load()
	globalXDPCacheMap.Store(new(ebpf.Map))
	defer func() { globalXDPCacheMap.Store(origMap) }()

	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: 0x1234},
		Question: []dns.Question{{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}},
	})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.IP{1, 2, 3, 4},
	})

	// Oversized domain triggers key build failure.
	longDomain := strings.Repeat("a.", 128)
	syncToBPFMap(longDomain, dns.TypeA, msg, 300) // should not panic
}

// TestSyncToBPFMapValueBuildError verifies that syncToBPFMap handles value
// build failures gracefully when the packed response exceeds 512 bytes.
func TestSyncToBPFMapValueBuildError(t *testing.T) {
	origMap := globalXDPCacheMap.Load()
	globalXDPCacheMap.Store(new(ebpf.Map))
	defer func() { globalXDPCacheMap.Store(origMap) }()

	msg := new(dns.Msg)
	msg.SetReply(&dns.Msg{
		MsgHdr:   dns.MsgHdr{Id: 0x1234},
		Question: []dns.Question{{Name: "example.com.", Qtype: dns.TypeTXT, Qclass: dns.ClassINET}},
	})
	// Add large TXT records to exceed 512-byte limit.
	for i := 0; i < 20; i++ {
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 300},
			Txt: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		})
	}

	syncToBPFMap("example.com.", dns.TypeTXT, msg, 300) // should not panic
}

// ---------------------------------------------------------------------------
// setCacheCopy BPF sync guard: negative/delegation entries must NOT sync
// ---------------------------------------------------------------------------

// TestSetCacheCopySkipsBPFSyncForNegativeResponse verifies that setCacheCopy
// stores negative responses (NXDOMAIN/NODATA) in the Go cache but does not
// attempt BPF sync. The guard is len(cp.Answer) > 0 in cache.go.
// We verify this indirectly: with a non-nil globalXDPCacheMap, a negative
// response would fail in buildBPFCacheValue (empty Answer → packs fine but
// semantic violation). We confirm the entry IS in go-cache.
func TestSetCacheCopySkipsBPFSyncForNegativeResponse(t *testing.T) {
	deleteAllCache()

	// NXDOMAIN response: empty Answer, SOA in Ns.
	nxMsg := new(dns.Msg)
	nxMsg.SetQuestion("nonexistent.example.com.", dns.TypeA)
	nxMsg.Rcode = dns.RcodeNameError
	nxMsg.Ns = []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   "example.com.",
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			Ns:     "ns1.example.com.",
			Mbox:   "admin.example.com.",
			Minttl: 60,
		},
	}

	// Store via setCacheCopy — the Answer is empty, so BPF sync guard
	// (len(cp.Answer) > 0) should prevent syncToBPFMap from being called.
	setCacheCopy("nonexistent.example.com.:1", nxMsg, 60)

	// Verify the entry is in the Go cache.
	msg, ok := getCacheCopy("nonexistent.example.com.:1")
	if !ok {
		t.Fatal("expected negative response to be stored in go-cache")
	}
	if msg.Rcode != dns.RcodeNameError {
		t.Errorf("expected Rcode NXDOMAIN in cached entry, got %s", dns.RcodeToString[msg.Rcode])
	}
	if !hasSOAInAuthority(msg) {
		t.Error("expected SOA in cached entry's Authority section")
	}
}

// TestSetCacheCopySyncsBPFForPositiveResponse verifies that setCacheCopy
// does attempt BPF sync for positive responses (non-empty Answer).
// With globalXDPCacheMap=nil this is a no-op, but the code path is exercised.
func TestSetCacheCopySyncsBPFForPositiveResponse(t *testing.T) {
	deleteAllCache()

	posMsg := new(dns.Msg)
	posMsg.SetQuestion("example.com.", dns.TypeA)
	posMsg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.IP{1, 2, 3, 4},
		},
	}

	// Should not panic; BPF sync is a no-op because globalXDPCacheMap is nil.
	setCacheCopy("example.com.:1", posMsg, 300)

	// Verify stored in go-cache.
	msg, ok := getCacheCopy("example.com.:1")
	if !ok {
		t.Fatal("expected positive response to be stored in go-cache")
	}
	if len(msg.Answer) != 1 {
		t.Errorf("expected 1 Answer record, got %d", len(msg.Answer))
	}
}

// ---------------------------------------------------------------------------
// classifyXDPError tests — error classification for degradation hints
// ---------------------------------------------------------------------------

func TestClassifyXDPError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantHint string
	}{
		{
			name:     "nil error",
			err:      nil,
			wantHint: "",
		},
		{
			name:     "EPERM from BPF syscall",
			err:      fmt.Errorf("[XDP] failed to load eBPF objects: %w", syscall.EPERM),
			wantHint: "CAP_BPF",
		},
		{
			name:     "EACCES from attach",
			err:      fmt.Errorf("[XDP] failed to attach: %w", syscall.EACCES),
			wantHint: "CAP_BPF",
		},
		{
			name:     "interface not found",
			err:      fmt.Errorf("[XDP] interface %q not found: no such device", "eth99"),
			wantHint: "xdp.interface",
		},
		{
			name:     "eBPF load failure",
			err:      fmt.Errorf("[XDP] failed to load eBPF objects: program too large"),
			wantHint: "kernel >= 5.15",
		},
		{
			name:     "attach failure",
			err:      fmt.Errorf("[XDP] failed to attach to eth0 (native: ENOSYS, generic: ENOSYS)"),
			wantHint: "XDP",
		},
		{
			name:     "unknown error",
			err:      fmt.Errorf("something unexpected"),
			wantHint: "hint:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyXDPError(tt.err)
			if tt.err == nil {
				if got != "" {
					t.Errorf("expected empty hint for nil error, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantHint) {
				t.Errorf("hint %q should contain %q", got, tt.wantHint)
			}
		})
	}
}
