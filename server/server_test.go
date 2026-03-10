package server

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func Test_server_ServeDNS(t *testing.T) {
	type fields struct {
		listen string
	}
	type args struct {
		w dns.ResponseWriter
		r *dns.Msg
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		//Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{
				listen: tt.fields.listen,
			}
			s.ServeDNS(tt.args.w, tt.args.r)
		})
	}
}

func TestIsUDP(t *testing.T) {
	// Create a real UDP address
	udpAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
	tcpAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}

	// Test with UDP address
	if !isUDP(&mockResponseWriter{addr: udpAddr}) {
		t.Error("Expected isUDP to return true for UDP address")
	}

	// Test with TCP address
	if isUDP(&mockResponseWriter{addr: tcpAddr}) {
		t.Error("Expected isUDP to return false for TCP address")
	}
}

// mockResponseWriter implements dns.ResponseWriter for testing
type mockResponseWriter struct {
	dns.ResponseWriter
	addr net.Addr
}

func (m *mockResponseWriter) RemoteAddr() net.Addr {
	return m.addr
}

func TestGetMaxUDPSize(t *testing.T) {
	tests := []struct {
		name     string
		msg      *dns.Msg
		expected int
	}{
		{
			name:     "no EDNS - default size",
			msg:      new(dns.Msg),
			expected: 512,
		},
		{
			name: "EDNS with 4096 buffer",
			msg: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetEdns0(4096, false)
				return m
			}(),
			expected: 4096,
		},
		{
			name: "EDNS with 1232 buffer",
			msg: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetEdns0(1232, false)
				return m
			}(),
			expected: 1232,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := getMaxUDPSize(tt.msg)
			if size != tt.expected {
				t.Errorf("getMaxUDPSize() = %d, want %d", size, tt.expected)
			}
		})
	}
}

func TestTruncateResponse(t *testing.T) {
	tests := []struct {
		name          string
		setupReply    func() *dns.Msg
		maxSize       int
		expectTrunc   bool
		expectAnswers int
	}{
		{
			name: "small response - no truncation",
			setupReply: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
						A:   net.ParseIP("192.0.2.1"),
					},
				}
				return m
			},
			maxSize:       512,
			expectTrunc:   false,
			expectAnswers: 1,
		},
		{
			name: "large response - truncation required",
			setupReply: func() *dns.Msg {
				m := new(dns.Msg)
				// Add many answers to exceed 512 bytes
				for i := 0; i < 30; i++ {
					m.Answer = append(m.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
						A:   net.ParseIP("192.0.2.1"),
					})
				}
				return m
			},
			maxSize:       512,
			expectTrunc:   true,
			expectAnswers: 0, // Will be truncated to fit or cleared
		},
		{
			name: "EDNS 4096 - no truncation",
			setupReply: func() *dns.Msg {
				m := new(dns.Msg)
				for i := 0; i < 50; i++ {
					m.Answer = append(m.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
						A:   net.ParseIP("192.0.2.1"),
					})
				}
				return m
			},
			maxSize:       4096,
			expectTrunc:   false,
			expectAnswers: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := tt.setupReply()
			request := new(dns.Msg)

			result := truncateResponse(reply, request, tt.maxSize)

			if result.Truncated != tt.expectTrunc {
				t.Errorf("Truncated = %v, want %v", result.Truncated, tt.expectTrunc)
			}

			if tt.expectTrunc {
				// Verify response fits within max size
				if result.Len() > tt.maxSize {
					t.Errorf("Truncated response size %d exceeds max %d", result.Len(), tt.maxSize)
				}
			}

			if len(result.Answer) != tt.expectAnswers && !tt.expectTrunc {
				t.Errorf("Answer count = %d, want %d", len(result.Answer), tt.expectAnswers)
			}
		})
	}
}