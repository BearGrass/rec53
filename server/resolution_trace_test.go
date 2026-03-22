package server

import (
	"context"
	"strings"
	"testing"

	"rec53/monitor"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func init() {
	if monitor.Rec53Log == nil {
		monitor.Rec53Log = zap.NewNop().Sugar()
	}
}

func TestTraceDomainCapturesCacheHitSuccess(t *testing.T) {
	setupStateMachineTransitionTest(t)

	setCacheCopyByType("trace-cache.example.", dns.TypeA, makeAMsg("trace-cache.example.", 60), 60)

	resp, trace, err := TraceDomain(context.Background(), "trace-cache.example.", dns.TypeA)
	if err != nil {
		t.Fatalf("TraceDomain cache hit: %v", err)
	}
	if resp == nil || resp.Rcode != dns.RcodeSuccess {
		t.Fatalf("response = %#v, want successful reply", resp)
	}
	if trace == nil {
		t.Fatal("trace is nil")
	}
	if trace.QueryName != "trace-cache.example." {
		t.Fatalf("query name = %q, want fqdn", trace.QueryName)
	}
	if trace.QueryType != "A" {
		t.Fatalf("query type = %q, want A", trace.QueryType)
	}
	if trace.Terminal != monitor.StateMachineSuccessExit {
		t.Fatalf("terminal = %q, want %q", trace.Terminal, monitor.StateMachineSuccessExit)
	}
	if len(trace.States) < 3 {
		t.Fatalf("states = %v, want at least init/hosts/return", trace.States)
	}
	if trace.States[0] != monitor.StateMachineStateInit {
		t.Fatalf("first state = %q, want %q", trace.States[0], monitor.StateMachineStateInit)
	}
	if !strings.Contains(trace.Format(), "terminal: success_exit") {
		t.Fatalf("formatted trace missing success terminal:\n%s", trace.Format())
	}
}

func TestTraceDomainCapturesUpstreamDrivenServfail(t *testing.T) {
	setupStateMachineTransitionTest(t)

	port, closePort := reserveUnusedUDPPort(t)
	closePort()
	SetHostsAndForwardForTest(nil, []ForwardZone{
		{Zone: "trace-servfail.test.", Upstreams: []string{"127.0.0.1:" + port}},
	})

	resp, trace, err := TraceDomain(context.Background(), "www.trace-servfail.test.", dns.TypeA)
	if err != nil {
		t.Fatalf("TraceDomain upstream servfail: %v", err)
	}
	if resp == nil || resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("response = %#v, want SERVFAIL", resp)
	}
	if trace == nil {
		t.Fatal("trace is nil")
	}
	if trace.Terminal != monitor.StateMachineServfailExit {
		t.Fatalf("terminal = %q, want %q", trace.Terminal, monitor.StateMachineServfailExit)
	}
	if !containsTraceState(trace.States, monitor.StateMachineForwardLookup) {
		t.Fatalf("states = %v, want forward_lookup", trace.States)
	}
	if !strings.Contains(trace.Format(), "rcode: SERVFAIL") {
		t.Fatalf("formatted trace missing SERVFAIL rcode:\n%s", trace.Format())
	}
}

func TestTraceDomainCapturesRevisitedStateSequence(t *testing.T) {
	setupStateMachineTransitionTest(t)

	setCacheCopyByType("alias-trace.example.", dns.TypeA, makeCNAMEMsg("alias-trace.example.", "target-trace.example.", 60), 60)
	setCacheCopyByType("target-trace.example.", dns.TypeA, makeAMsg("target-trace.example.", 60), 60)

	_, trace, err := TraceDomain(context.Background(), "alias-trace.example.", dns.TypeA)
	if err != nil {
		t.Fatalf("TraceDomain cname revisit: %v", err)
	}
	if trace == nil {
		t.Fatal("trace is nil")
	}
	if trace.Terminal != monitor.StateMachineSuccessExit {
		t.Fatalf("terminal = %q, want %q", trace.Terminal, monitor.StateMachineSuccessExit)
	}
	if countTraceState(trace.States, monitor.StateMachineCacheLookup) < 2 {
		t.Fatalf("states = %v, want repeated cache_lookup", trace.States)
	}
	if countTraceState(trace.States, monitor.StateMachineClassifyResp) < 2 {
		t.Fatalf("states = %v, want repeated classify_resp", trace.States)
	}
}

func containsTraceState(states []string, want string) bool {
	for _, state := range states {
		if state == want {
			return true
		}
	}
	return false
}

func countTraceState(states []string, want string) int {
	count := 0
	for _, state := range states {
		if state == want {
			count++
		}
	}
	return count
}
