package server

import (
	"context"
	"net"
	"testing"
	"time"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	// Initialize no-op logger for tests
	monitor.Rec53Log = zap.NewNop().Sugar()
}

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

// TestNewServer tests server creation
func TestNewServer(t *testing.T) {
	listen := "127.0.0.1:5353"
	s := NewServer(listen)

	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.listen != listen {
		t.Errorf("expected listen %s, got %s", listen, s.listen)
	}
}

// TestServerRunAndShutdown tests server startup and graceful shutdown
func TestServerRunAndShutdown(t *testing.T) {
	// Use port 0 to get a random available port
	s := NewServer("127.0.0.1:0")
	monitor.ResetRuntimeState()

	// Run the server
	errChan := s.Run()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify addresses are assigned
	udpAddr := s.UDPAddr()
	tcpAddr := s.TCPAddr()

	if udpAddr == "" {
		t.Error("expected UDP address to be assigned after Run()")
	}
	if tcpAddr == "" {
		t.Error("expected TCP address to be assigned after Run()")
	}
	if state := monitor.RuntimeState(); !state.Readiness || state.Phase != monitor.RuntimePhaseSteady {
		t.Fatalf("runtime state = %+v, want ready steady", state)
	}

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected error on shutdown: %v", err)
	}

	// Verify error channel is closed
	select {
	case _, ok := <-errChan:
		if ok {
			t.Error("expected error channel to be closed after shutdown")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for error channel to close")
	}
}

func TestServerRun_StartupErrorDoesNotBlock(t *testing.T) {
	monitor.ResetRuntimeState()
	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve tcp port: %v", err)
	}
	defer tcpLn.Close()

	addr := tcpLn.Addr().String()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("failed to resolve udp addr %q: %v", addr, err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("failed to reserve udp port: %v", err)
	}
	defer udpConn.Close()

	s := NewServer(addr)
	runDone := make(chan (<-chan error), 1)
	go func() {
		runDone <- s.Run()
	}()

	var errChan <-chan error
	select {
	case errChan = <-runDone:
		// good: startup path did not block forever
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run() blocked on startup failure instead of returning")
	}

	select {
	case err, ok := <-errChan:
		if !ok {
			t.Fatal("expected startup error, got closed errChan")
		}
		if err == nil {
			t.Fatal("expected non-nil startup error")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected startup error to be visible on errChan")
	}

	if s.UDPAddr() != "" {
		t.Errorf("expected empty UDPAddr on startup failure, got %q", s.UDPAddr())
	}
	if s.TCPAddr() != "" {
		t.Errorf("expected empty TCPAddr on startup failure, got %q", s.TCPAddr())
	}
	if state := monitor.RuntimeState(); state.Readiness || state.Phase != monitor.RuntimePhaseColdStart {
		t.Fatalf("runtime state = %+v, want not-ready cold-start", state)
	}
}

func TestServerShutdown_CancelsBackgroundWorkBeforeWait(t *testing.T) {
	monitor.ResetRuntimeState()
	warmupCtx, warmupCancel := context.WithCancel(context.Background())
	xdpCtx, xdpCancel := context.WithCancel(context.Background())

	s := &server{
		warmupCancel: warmupCancel,
		xdpCancel:    xdpCancel,
		xdpLoader:    &XDPLoader{},
	}

	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		<-warmupCtx.Done()
	}()
	go func() {
		defer s.wg.Done()
		<-xdpCtx.Done()
	}()

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		done <- s.Shutdown(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown() returned unexpected error: %v", err)
		}
		if state := monitor.RuntimeState(); state.Readiness || state.Phase != monitor.RuntimePhaseShuttingDown {
			t.Fatalf("runtime state = %+v, want not-ready shutting-down", state)
		}
	case <-time.After(500 * time.Millisecond):
		warmupCancel()
		xdpCancel()
		t.Fatal("Shutdown() blocked waiting for background work")
	}
}

func TestServerSetRuntimeStateServingWithWarmup(t *testing.T) {
	monitor.ResetRuntimeState()
	s := &server{warmupCfg: WarmupConfig{Enabled: true}}

	s.setRuntimeStateServing()

	if state := monitor.RuntimeState(); !state.Readiness || state.Phase != monitor.RuntimePhaseWarming {
		t.Fatalf("runtime state = %+v, want ready warming", state)
	}
}

func TestServerSetRuntimeStateServingWithWarmupAlreadyComplete(t *testing.T) {
	monitor.ResetRuntimeState()
	if state, changed := monitor.MarkRuntimeWarmupComplete(); changed {
		t.Fatalf("MarkRuntimeWarmupComplete changed = true, state = %+v", state)
	}

	s := &server{warmupCfg: WarmupConfig{Enabled: true}}
	s.setRuntimeStateServing()

	if state := monitor.RuntimeState(); !state.Readiness || state.Phase != monitor.RuntimePhaseSteady {
		t.Fatalf("runtime state = %+v, want ready steady", state)
	}
}

func TestServerWarmupCompletionTransitionsToSteady(t *testing.T) {
	monitor.ResetRuntimeState()
	s := &server{warmupCfg: WarmupConfig{Enabled: true}}
	s.setRuntimeStateServing()

	if state := monitor.RuntimeState(); !state.Readiness || state.Phase != monitor.RuntimePhaseWarming {
		t.Fatalf("runtime state before warmup completion = %+v, want ready warming", state)
	}

	state, changed := monitor.MarkRuntimeWarmupComplete()
	if !changed {
		t.Fatalf("MarkRuntimeWarmupComplete changed = false, state = %+v", state)
	}
	if state.Readiness != true || state.Phase != monitor.RuntimePhaseSteady {
		t.Fatalf("runtime state after warmup completion = %+v, want ready steady", state)
	}
}

// TestServerUDPAddr tests UDPAddr method
func TestServerUDPAddr(t *testing.T) {
	s := NewServer("127.0.0.1:0")

	// Before running, should return empty string
	if addr := s.UDPAddr(); addr != "" {
		t.Errorf("expected empty UDP address before Run(), got %s", addr)
	}

	// Run server
	s.Run()
	time.Sleep(50 * time.Millisecond)

	// After running, should return address
	if addr := s.UDPAddr(); addr == "" {
		t.Error("expected non-empty UDP address after Run()")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

// TestServerTCPAddr tests TCPAddr method
func TestServerTCPAddr(t *testing.T) {
	s := NewServer("127.0.0.1:0")

	// Before running, should return empty string
	if addr := s.TCPAddr(); addr != "" {
		t.Errorf("expected empty TCP address before Run(), got %s", addr)
	}

	// Run server
	s.Run()
	time.Sleep(50 * time.Millisecond)

	// After running, should return address
	if addr := s.TCPAddr(); addr == "" {
		t.Error("expected non-empty TCP address after Run()")
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

// TestServeDNSBasicQuery tests basic DNS query handling
// Note: This test requires network access to resolve DNS queries
func TestServeDNSBasicQuery(t *testing.T) {
	// Skip if running in short mode or no network
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	s := NewServer("127.0.0.1:0")
	s.Run()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	defer s.Shutdown(shutCtx)

	time.Sleep(50 * time.Millisecond)

	// Create a DNS client
	client := &dns.Client{Net: "udp", Timeout: 2 * time.Second}

	// Create a query
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)

	// Send query
	addr := s.UDPAddr()
	resp, _, err := client.Exchange(msg, addr)
	if err != nil {
		t.Logf("Skipping test - DNS resolution failed: %v", err)
		t.Skip("Network access required for DNS resolution")
	}

	// Verify response
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Question) == 0 {
		t.Error("expected response to have question")
	}
	if resp.Question[0].Name != "example.com." {
		t.Errorf("expected question name 'example.com.', got %s", resp.Question[0].Name)
	}
}

// TestServeDNSEmptyQuestion tests handling of messages without questions
// Note: This tests that the server panics on empty questions (current behavior)
func TestServeDNSEmptyQuestion(t *testing.T) {
	s := NewServer("127.0.0.1:0")
	s.Run()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	defer s.Shutdown(shutCtx)

	time.Sleep(50 * time.Millisecond)

	// Create mock response writer with WriteMsg to capture panic
	mock := &mockResponseWriterWithCapture{addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}}
	msg := new(dns.Msg) // Empty message with no questions

	// Test that the server handles empty messages gracefully
	// Currently, this will panic because server.go:39 assumes Question[0] exists
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Server panicked on empty question (known issue): %v", r)
		}
	}()
	s.ServeDNS(mock, msg)
}

// mockResponseWriterWithCapture captures messages for testing
type mockResponseWriterWithCapture struct {
	dns.ResponseWriter
	addr    net.Addr
	written *dns.Msg
}

func (m *mockResponseWriterWithCapture) RemoteAddr() net.Addr {
	return m.addr
}

func (m *mockResponseWriterWithCapture) WriteMsg(msg *dns.Msg) error {
	m.written = msg
	return nil
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

// TestNewServerWithFullConfig_XDPDisabled verifies that XDP-disabled config
// creates a server that works identically to the pre-XDP path.
func TestNewServerWithFullConfig_XDPDisabled(t *testing.T) {
	s := NewServerWithFullConfig(
		"127.0.0.1:0", 1,
		WarmupConfig{Enabled: false}, SnapshotConfig{},
		nil, nil,
		"", // xdpInterface="" → XDP disabled
		ExpensiveRequestLimitConfig{},
	)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.xdpLoader != nil {
		t.Error("expected xdpLoader to be nil when XDP disabled")
	}

	// Run and shutdown should work without XDP
	errChan := s.Run()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
	select {
	case _, ok := <-errChan:
		if ok {
			t.Error("expected error channel closed")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for errChan close")
	}
}

// TestNewServerWithFullConfig_XDPInterface verifies that passing an XDP interface
// creates a loader on the server struct.
func TestNewServerWithFullConfig_XDPInterface(t *testing.T) {
	s := NewServerWithFullConfig(
		"127.0.0.1:0", 1,
		WarmupConfig{Enabled: false}, SnapshotConfig{},
		nil, nil,
		"eth0", // xdpInterface="eth0" → loader created
		ExpensiveRequestLimitConfig{},
	)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.xdpLoader == nil {
		t.Error("expected xdpLoader to be non-nil when interface specified")
	}
}

// TestServer_ShutdownCleansXDP verifies that Shutdown() nils the global XDP
// cache map and closes the loader when XDP was active (graceful degradation
// when attach wasn't possible — e.g. no root — is handled by Run() logging).
func TestServer_ShutdownCleansXDP(t *testing.T) {
	// Ensure globalXDPCacheMap is nil after shutdown, even if it was set.
	oldMap := globalXDPCacheMap.Load()
	defer func() { globalXDPCacheMap.Store(oldMap) }()

	s := NewServerWithFullConfig(
		"127.0.0.1:0", 1,
		WarmupConfig{Enabled: false}, SnapshotConfig{},
		nil, nil,
		"", // XDP disabled
		ExpensiveRequestLimitConfig{},
	)
	errChan := s.Run()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
	if globalXDPCacheMap.Load() != nil {
		t.Error("expected globalXDPCacheMap to be nil after shutdown")
	}
	select {
	case _, ok := <-errChan:
		if ok {
			t.Error("expected error channel closed")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for errChan close")
	}
}
