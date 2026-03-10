package server

import (
	"reflect"
	"testing"

	"github.com/miekg/dns"
)

func TestChange(t *testing.T) {
	type args struct {
		stm stateMachine
	}
	tests := []struct {
		name    string
		args    args
		want    *dns.Msg
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Change(tt.args.stm)
			if (err != nil) != tt.wantErr {
				t.Errorf("Change() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Change() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckRespStateHandle tests the checkRespState.handle() function
func TestCheckRespStateHandle(t *testing.T) {
	tests := []struct {
		name          string
		request       *dns.Msg
		response      *dns.Msg
		expectedRet   int
		expectError   bool
	}{
		{
			name: "matching A record type",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeA)
				return m
			}(),
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
					},
				}
				return m
			}(),
			expectedRet: CHECK_RESP_GET_ANS,
			expectError: false,
		},
		{
			name: "matching AAAA record type",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeAAAA)
				return m
			}(),
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = []dns.RR{
					&dns.AAAA{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
					},
				}
				return m
			}(),
			expectedRet: CHECK_RESP_GET_ANS,
			expectError: false,
		},
		{
			name: "CNAME record - should follow",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("www.example.com.", dns.TypeA)
				return m
			}(),
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = []dns.RR{
					&dns.CNAME{
						Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
						Target: "example.com.",
					},
				}
				return m
			}(),
			expectedRet: CHECK_RESP_GET_CNAME,
			expectError: false,
		},
		{
			name: "CNAME with A record - should find A",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("www.example.com.", dns.TypeA)
				return m
			}(),
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = []dns.RR{
					&dns.CNAME{
						Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
						Target: "example.com.",
					},
					&dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
					},
				}
				return m
			}(),
			expectedRet: CHECK_RESP_GET_ANS,
			expectError: false,
		},
		{
			name: "no answers - should get NS",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeA)
				return m
			}(),
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = nil
				return m
			}(),
			expectedRet: CHECK_RESP_GET_NS,
			expectError: false,
		},
		{
			name: "type mismatch without CNAME - should get NS",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeAAAA)
				return m
			}(),
			response: func() *dns.Msg {
				m := new(dns.Msg)
				m.Answer = []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
					},
				}
				return m
			}(),
			expectedRet: CHECK_RESP_GET_NS,
			expectError: false,
		},
		{
			name: "nil request - error",
			request: nil,
			response: func() *dns.Msg {
				m := new(dns.Msg)
				return m
			}(),
			expectedRet: CHECK_RESP_COMMEN_ERROR,
			expectError: true,
		},
		{
			name: "nil response - error",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				return m
			}(),
			response:    nil,
			expectedRet: CHECK_RESP_COMMEN_ERROR,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newCheckRespState(tt.request, tt.response)
			ret, err := state.handle(tt.request, tt.response)

			if (err != nil) != tt.expectError {
				t.Errorf("handle() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if ret != tt.expectedRet {
				t.Errorf("handle() returned %d, expected %d", ret, tt.expectedRet)
			}
		})
	}
}

// TestInCacheStateHandle tests the inCacheState.handle() function with type-aware caching
func TestInCacheStateHandle(t *testing.T) {
	// Clear cache before tests
	deleteAllCache()

	domain := "cache-test.example.com."

	// Cache an A record
	aMsg := new(dns.Msg)
	aMsg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		},
	}
	setCacheCopyByType(domain, dns.TypeA, aMsg, 300)

	tests := []struct {
		name        string
		request     *dns.Msg
		expectedRet int
	}{
		{
			name: "A record cache hit",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion(domain, dns.TypeA)
				return m
			}(),
			expectedRet: IN_CACHE_HIT_CACHE,
		},
		{
			name: "AAAA record cache miss (different type)",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion(domain, dns.TypeAAAA)
				return m
			}(),
			expectedRet: IN_CACHE_MISS_CACHE,
		},
		{
			name: "MX record cache miss (different type)",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion(domain, dns.TypeMX)
				return m
			}(),
			expectedRet: IN_CACHE_MISS_CACHE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := new(dns.Msg)
			state := newInCacheState(tt.request, response)
			ret, err := state.handle(tt.request, response)

			if err != nil {
				t.Errorf("handle() unexpected error: %v", err)
				return
			}

			if ret != tt.expectedRet {
				t.Errorf("handle() returned %d, expected %d", ret, tt.expectedRet)
			}
		})
	}
}