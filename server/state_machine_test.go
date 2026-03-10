package server

import (
	"net"
	"reflect"
	"testing"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	// Initialize no-op logger for tests
	monitor.Rec53Log = zap.NewNop().Sugar()
}

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
		name        string
		request     *dns.Msg
		response    *dns.Msg
		expectedRet int
		expectError bool
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
			name:    "nil request - error",
			request: nil,
			response: func() *dns.Msg {
				m := new(dns.Msg)
				return m
			}(),
			expectedRet: CHECK_RESP_COMMON_ERROR,
			expectError: true,
		},
		{
			name: "nil response - error",
			request: func() *dns.Msg {
				m := new(dns.Msg)
				return m
			}(),
			response:    nil,
			expectedRet: CHECK_RESP_COMMON_ERROR,
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

// TestStateInitState tests the stateInitState
func TestStateInitState(t *testing.T) {
	t.Run("successful initialization", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		resp := new(dns.Msg)

		state := newStateInitState(req, resp)
		ret, err := state.handle(req, resp)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ret != STATE_INIT_NO_ERROR {
			t.Errorf("expected %d, got %d", STATE_INIT_NO_ERROR, ret)
		}
		if state.getCurrentState() != STATE_INIT {
			t.Errorf("expected state %d, got %d", STATE_INIT, state.getCurrentState())
		}
	})

	t.Run("nil request error", func(t *testing.T) {
		resp := new(dns.Msg)
		state := newStateInitState(nil, resp)
		_, err := state.handle(nil, resp)

		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("nil response error", func(t *testing.T) {
		req := new(dns.Msg)
		state := newStateInitState(req, nil)
		_, err := state.handle(req, nil)

		if err == nil {
			t.Error("expected error for nil response")
		}
	})
}

// TestInGlueState tests the inGlueState
func TestInGlueState(t *testing.T) {
	t.Run("glue exists", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		resp := new(dns.Msg)
		resp.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
				Ns:  "ns1.example.com.",
			},
		}
		resp.Extra = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   parseTestIP("192.0.2.1"),
			},
		}

		state := newInGlueState(req, resp)
		ret, err := state.handle(req, resp)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ret != IN_GLUE_EXIST {
			t.Errorf("expected %d, got %d", IN_GLUE_EXIST, ret)
		}
	})

	t.Run("glue does not exist", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		resp := new(dns.Msg)

		state := newInGlueState(req, resp)
		ret, err := state.handle(req, resp)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ret != IN_GLUE_NOT_EXIST {
			t.Errorf("expected %d, got %d", IN_GLUE_NOT_EXIST, ret)
		}
	})

	t.Run("nil request error", func(t *testing.T) {
		resp := new(dns.Msg)
		state := newInGlueState(nil, resp)
		_, err := state.handle(nil, resp)

		if err == nil {
			t.Error("expected error for nil request")
		}
	})
}

// TestRetRespState tests the retRespState
func TestRetRespState(t *testing.T) {
	t.Run("successful return", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		resp := new(dns.Msg)
		resp.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   parseTestIP("192.0.2.1"),
			},
		}

		state := newRetRespState(req, resp)
		ret, err := state.handle(req, resp)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ret != RET_RESP_NO_ERROR {
			t.Errorf("expected %d, got %d", RET_RESP_NO_ERROR, ret)
		}
		if state.getCurrentState() != RET_RESP {
			t.Errorf("expected state %d, got %d", RET_RESP, state.getCurrentState())
		}
	})

	t.Run("nil request error", func(t *testing.T) {
		resp := new(dns.Msg)
		state := newRetRespState(nil, resp)
		_, err := state.handle(nil, resp)

		if err == nil {
			t.Error("expected error for nil request")
		}
	})
}

// TestInGlueCacheState tests the inGlueCacheState
func TestInGlueCacheState(t *testing.T) {
	deleteAllCache()

	t.Run("cache hit for zone", func(t *testing.T) {
		deleteAllCache()

		// Cache glue for a zone
		zone := "example.com."
		cachedMsg := new(dns.Msg)
		cachedMsg.Ns = []dns.RR{
			&dns.NS{
				Hdr: dns.RR_Header{Name: zone, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 300},
				Ns:  "ns1.example.com.",
			},
		}
		cachedMsg.Extra = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   parseTestIP("192.0.2.1"),
			},
		}
		setCacheCopy(zone, cachedMsg, 300)

		req := new(dns.Msg)
		req.SetQuestion("www.example.com.", dns.TypeA)
		resp := new(dns.Msg)

		state := newInGlueCacheState(req, resp)
		ret, err := state.handle(req, resp)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ret != IN_GLUE_CACHE_HIT_CACHE {
			t.Errorf("expected %d, got %d", IN_GLUE_CACHE_HIT_CACHE, ret)
		}
	})

	t.Run("cache miss - use root glue", func(t *testing.T) {
		deleteAllCache()

		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		resp := new(dns.Msg)

		state := newInGlueCacheState(req, resp)
		ret, err := state.handle(req, resp)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should return IN_CACHE_MISS_CACHE and use root glue
		if ret != IN_CACHE_MISS_CACHE {
			t.Errorf("expected %d, got %d", IN_CACHE_MISS_CACHE, ret)
		}
		// Verify root glue was added
		if len(resp.Ns) == 0 {
			t.Error("expected root NS records to be added")
		}
	})

	t.Run("nil request error", func(t *testing.T) {
		resp := new(dns.Msg)
		state := newInGlueCacheState(nil, resp)
		_, err := state.handle(nil, resp)

		if err == nil {
			t.Error("expected error for nil request")
		}
	})
}

// TestGetIPListFromResponse tests extracting IP addresses from response
func TestGetIPListFromResponse(t *testing.T) {
	t.Run("extract IPs from extra section", func(t *testing.T) {
		resp := new(dns.Msg)
		resp.Extra = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   parseTestIP("192.0.2.1"),
			},
			&dns.A{
				Hdr: dns.RR_Header{Name: "ns2.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   parseTestIP("192.0.2.2"),
			},
		}

		ips := getIPListFromResponse(resp)

		if len(ips) != 2 {
			t.Errorf("expected 2 IPs, got %d", len(ips))
		}
	})

	t.Run("empty extra section", func(t *testing.T) {
		resp := new(dns.Msg)
		ips := getIPListFromResponse(resp)

		if len(ips) != 0 {
			t.Errorf("expected 0 IPs, got %d", len(ips))
		}
	})

	t.Run("skip non-A records", func(t *testing.T) {
		resp := new(dns.Msg)
		resp.Extra = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   parseTestIP("192.0.2.1"),
			},
			&dns.AAAA{
				Hdr:  dns.RR_Header{Name: "ns1.example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
				AAAA: parseTestIP("2001:db8::1"),
			},
		}

		ips := getIPListFromResponse(resp)

		if len(ips) != 1 {
			t.Errorf("expected 1 IP (A record only), got %d", len(ips))
		}
	})
}

// TestGetBestAddressAndPrefetchIPs tests IP selection
func TestGetBestAddressAndPrefetchIPs(t *testing.T) {
	t.Run("empty IP list returns error", func(t *testing.T) {
		_, _, err := getBestAddressAndPrefetchIPs([]string{})
		if err == nil {
			t.Error("expected error for empty IP list")
		}
	})

	t.Run("single IP returns that IP", func(t *testing.T) {
		globalIPPool = NewIPPool() // Reset pool for clean state
		bestIP, secondIP, err := getBestAddressAndPrefetchIPs([]string{"192.0.2.1"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if bestIP != "192.0.2.1" {
			t.Errorf("expected bestIP 192.0.2.1, got %s", bestIP)
		}
		// Note: secondIP is bestIPWithoutInit, which is the best IP not yet initialized
		// For a single IP that gets initialized during the call, secondIP may be empty or the same IP
		// depending on initialization state
		_ = secondIP // Don't assert on secondIP for single IP case
	})
}

// TestChangeMaxIterations tests that Change stops after MaxIterations
// Note: This test is complex because Change creates its own state transitions.
// We skip this test as it would require mocking internal state creation.
func TestChangeMaxIterations(t *testing.T) {
	t.Skip("Skipping - Change function creates internal states that cannot be easily mocked")
}

// TestIterState tests the iterState error handling
func TestIterState(t *testing.T) {
	t.Run("nil request error", func(t *testing.T) {
		resp := new(dns.Msg)
		state := newIterState(nil, resp)
		_, err := state.handle(nil, resp)

		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("nil response error", func(t *testing.T) {
		req := new(dns.Msg)
		state := newIterState(req, nil)
		_, err := state.handle(req, nil)

		if err == nil {
			t.Error("expected error for nil response")
		}
	})

	t.Run("empty extra section error", func(t *testing.T) {
		req := new(dns.Msg)
		req.SetQuestion("example.com.", dns.TypeA)
		resp := new(dns.Msg)

		state := newIterState(req, resp)
		ret, err := state.handle(req, resp)

		if err == nil {
			t.Error("expected error for empty extra section")
		}
		if ret != ITER_COMMON_ERROR {
			t.Errorf("expected %d, got %d", ITER_COMMON_ERROR, ret)
		}
	})
}

// parseTestIP helper function
func parseTestIP(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4
	}
	return ip
}

// =============================================================================
// Change Function Tests
// =============================================================================
//
// Note: Testing the Change() function directly is complex because:
// 1. It accesses stm.getRequest().Question[0] at line 31, which panics if request is nil
// 2. It creates new state instances internally after STATE_INIT and other states,
//    which make real network calls
//
// The only directly testable state is RET_RESP (terminal state with no state transitions).
// Other states (STATE_INIT, IN_CACHE, ITER, etc.) are tested via their individual handle() methods.
//
// For full integration testing of Change(), see e2e tests.

// TestChange_RetRespState tests the RET_RESP state behavior - the terminal state
// that returns the final DNS response.
func TestChange_RetRespState(t *testing.T) {
	originalDomain := "original.example.com."
	req := new(dns.Msg)
	req.SetQuestion(originalDomain, dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: originalDomain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   parseTestIP("192.0.2.1"),
		},
	}

	// Test RET_RESP state directly - this is a terminal state
	retState := newRetRespState(req, resp)
	result, err := Change(retState)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	// Verify original question is preserved
	if len(result.Question) == 0 {
		t.Error("Expected question in response")
		return
	}

	if result.Question[0].Name != originalDomain {
		t.Errorf("Expected question name '%s', got '%s'", originalDomain, result.Question[0].Name)
	}

	// Verify answer is present
	if len(result.Answer) == 0 {
		t.Error("Expected answer in response")
		return
	}
}

// TestChange_MultipleAnswerRecords tests that Change correctly handles
// responses with multiple answer records
func TestChange_MultipleAnswerRecords(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("multi.example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "multi.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   parseTestIP("192.0.2.1"),
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "multi.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   parseTestIP("192.0.2.2"),
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "multi.example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   parseTestIP("192.0.2.3"),
		},
	}

	retState := newRetRespState(req, resp)
	result, err := Change(retState)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	if len(result.Answer) != 3 {
		t.Errorf("Expected 3 answers, got %d", len(result.Answer))
	}
}

// TestChange_EmptyAnswer tests that Change handles empty answer responses
func TestChange_EmptyAnswer(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("empty.example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	// No answers

	retState := newRetRespState(req, resp)
	result, err := Change(retState)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	if len(result.Answer) != 0 {
		t.Errorf("Expected 0 answers, got %d", len(result.Answer))
	}
}

// TestChange_CNAMEInAnswer tests that Change preserves CNAME records in response
func TestChange_CNAMEInAnswer(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("www.example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = []dns.RR{
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: "example.com.",
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   parseTestIP("192.0.2.1"),
		},
	}

	retState := newRetRespState(req, resp)
	result, err := Change(retState)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if len(result.Answer) != 2 {
		t.Errorf("Expected 2 answers (CNAME + A), got %d", len(result.Answer))
		return
	}

	// Verify CNAME record is preserved
	cname, ok := result.Answer[0].(*dns.CNAME)
	if !ok {
		t.Error("Expected first record to be CNAME")
		return
	}
	if cname.Target != "example.com." {
		t.Errorf("Expected CNAME target 'example.com.', got '%s'", cname.Target)
	}
}

// TestChange_NXDOMAINResponse tests that Change handles NXDOMAIN responses
func TestChange_NXDOMAINResponse(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("nonexistent.example.com.", dns.TypeA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Rcode = dns.RcodeNameError // NXDOMAIN

	retState := newRetRespState(req, resp)
	result, err := Change(retState)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	if result.Rcode != dns.RcodeNameError {
		t.Errorf("Expected RcodeNameError (NXDOMAIN), got %d", result.Rcode)
	}
}

// TestChange_AAAARecord tests that Change handles AAAA records
func TestChange_AAAARecord(t *testing.T) {
	req := new(dns.Msg)
	req.SetQuestion("ipv6.example.com.", dns.TypeAAAA)

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = []dns.RR{
		&dns.AAAA{
			Hdr:  dns.RR_Header{Name: "ipv6.example.com.", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: parseTestIP("2001:db8::1"),
		},
	}

	retState := newRetRespState(req, resp)
	result, err := Change(retState)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	if len(result.Answer) != 1 {
		t.Errorf("Expected 1 answer, got %d", len(result.Answer))
		return
	}

	aaaa, ok := result.Answer[0].(*dns.AAAA)
	if !ok {
		t.Error("Expected AAAA record")
		return
	}

	if aaaa.AAAA.String() != "2001:db8::1" {
		t.Errorf("Expected IPv6 address '2001:db8::1', got '%s'", aaaa.AAAA.String())
	}
}
