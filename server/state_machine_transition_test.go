package server

import (
	"context"
	"fmt"
	"net"
	"testing"

	"rec53/monitor"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

type transitionSample struct {
	from  string
	to    string
	value float64
}

func TestStateMachineTransitions_CacheHitAndCNAMELoopback(t *testing.T) {
	setupStateMachineTransitionTest(t)

	setCacheCopyByType("cache-hit.example.", dns.TypeA, makeAMsg("cache-hit.example.", 60), 60)
	setCacheCopyByType("alias.example.", dns.TypeA, makeCNAMEMsg("alias.example.", "target.example.", 60), 60)
	setCacheCopyByType("target.example.", dns.TypeA, makeAMsg("target.example.", 60), 60)

	beforeHit := transitionValues(
		monitor.StateMachineStateInit, monitor.StateMachineHostsLookup,
		monitor.StateMachineHostsLookup, monitor.StateMachineForwardLookup,
		monitor.StateMachineForwardLookup, monitor.StateMachineCacheLookup,
		monitor.StateMachineCacheLookup, monitor.StateMachineClassifyResp,
		monitor.StateMachineClassifyResp, monitor.StateMachineReturnResp,
		monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit,
	)

	req := new(dns.Msg)
	req.SetQuestion("cache-hit.example.", dns.TypeA)
	if _, err := Change(newStateInitState(req, new(dns.Msg), context.Background())); err != nil {
		t.Fatalf("Change cache-hit: %v", err)
	}

	assertTransitionDelta(t, beforeHit, monitor.StateMachineStateInit, monitor.StateMachineHostsLookup, 1)
	assertTransitionDelta(t, beforeHit, monitor.StateMachineHostsLookup, monitor.StateMachineForwardLookup, 1)
	assertTransitionDelta(t, beforeHit, monitor.StateMachineForwardLookup, monitor.StateMachineCacheLookup, 1)
	assertTransitionDelta(t, beforeHit, monitor.StateMachineCacheLookup, monitor.StateMachineClassifyResp, 1)
	assertTransitionDelta(t, beforeHit, monitor.StateMachineClassifyResp, monitor.StateMachineReturnResp, 1)
	assertTransitionDelta(t, beforeHit, monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit, 1)

	beforeCNAME := transitionValues(
		monitor.StateMachineClassifyResp, monitor.StateMachineCacheLookup,
		monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit,
	)
	cnameReq := new(dns.Msg)
	cnameReq.SetQuestion("alias.example.", dns.TypeA)
	if _, err := Change(newStateInitState(cnameReq, new(dns.Msg), context.Background())); err != nil {
		t.Fatalf("Change cname-loopback: %v", err)
	}

	assertTransitionDelta(t, beforeCNAME, monitor.StateMachineClassifyResp, monitor.StateMachineCacheLookup, 1)
	assertTransitionDelta(t, beforeCNAME, monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit, 1)
}

func TestStateMachineTransitions_ForwardHitAndIterativeSuccess(t *testing.T) {
	setupStateMachineTransitionTest(t)

	forwardResp := makeAMsg("www.forward.test.", 60)
	forwardResp.Answer[0].(*dns.A).A = net.ParseIP("192.0.2.44").To4()
	forwardServer, err := NewMockDNSServer("udp", &MockDNSHandler{response: forwardResp})
	if err != nil {
		t.Fatalf("NewMockDNSServer forward: %v", err)
	}
	defer forwardServer.Stop()

	SetHostsAndForwardForTest(nil, []ForwardZone{
		{Zone: "forward.test.", Upstreams: []string{forwardServer.Addr}},
	})

	beforeForward := transitionValues(
		monitor.StateMachineHostsLookup, monitor.StateMachineForwardLookup,
		monitor.StateMachineForwardLookup, monitor.StateMachineReturnResp,
		monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit,
	)

	forwardReq := new(dns.Msg)
	forwardReq.SetQuestion("www.forward.test.", dns.TypeA)
	if _, err := Change(newStateInitState(forwardReq, new(dns.Msg), context.Background())); err != nil {
		t.Fatalf("Change forward-hit: %v", err)
	}

	assertTransitionDelta(t, beforeForward, monitor.StateMachineHostsLookup, monitor.StateMachineForwardLookup, 1)
	assertTransitionDelta(t, beforeForward, monitor.StateMachineForwardLookup, monitor.StateMachineReturnResp, 1)
	assertTransitionDelta(t, beforeForward, monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit, 1)

	ResetHostsAndForwardForTest()

	iterResp := makeAMsg("www.iter.test.", 60)
	iterResp.Answer[0].(*dns.A).A = net.ParseIP("198.51.100.9").To4()
	iterServer, err := NewMockDNSServer("udp", &MockDNSHandler{response: iterResp})
	if err != nil {
		t.Fatalf("NewMockDNSServer iter: %v", err)
	}
	defer iterServer.Stop()

	setCachedDelegation("iter.test.", "ns1.iter.test.", "127.0.0.1")
	SetIterPort(portOfAddr(iterServer.Addr))

	beforeIter := transitionValues(
		monitor.StateMachineCacheLookup, monitor.StateMachineExtractGlue,
		monitor.StateMachineExtractGlue, monitor.StateMachineLookupNSCache,
		monitor.StateMachineLookupNSCache, monitor.StateMachineQueryUpstream,
		monitor.StateMachineQueryUpstream, monitor.StateMachineClassifyResp,
		monitor.StateMachineClassifyResp, monitor.StateMachineReturnResp,
		monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit,
	)

	iterReq := new(dns.Msg)
	iterReq.SetQuestion("www.iter.test.", dns.TypeA)
	if _, err := Change(newStateInitState(iterReq, new(dns.Msg), context.Background())); err != nil {
		t.Fatalf("Change iterative-success: %v", err)
	}

	assertTransitionDelta(t, beforeIter, monitor.StateMachineCacheLookup, monitor.StateMachineExtractGlue, 1)
	assertTransitionDelta(t, beforeIter, monitor.StateMachineExtractGlue, monitor.StateMachineLookupNSCache, 1)
	assertTransitionDelta(t, beforeIter, monitor.StateMachineLookupNSCache, monitor.StateMachineQueryUpstream, 1)
	assertTransitionDelta(t, beforeIter, monitor.StateMachineQueryUpstream, monitor.StateMachineClassifyResp, 1)
	assertTransitionDelta(t, beforeIter, monitor.StateMachineClassifyResp, monitor.StateMachineReturnResp, 1)
	assertTransitionDelta(t, beforeIter, monitor.StateMachineReturnResp, monitor.StateMachineSuccessExit, 1)
}

func TestStateMachineTransitions_TerminalExitsRemainReconcilable(t *testing.T) {
	setupStateMachineTransitionTest(t)

	reqFormerr := new(dns.Msg)
	reqFormerr.SetQuestion("formerr.example.", dns.TypeA)
	reqFormerr.Response = true
	respFormerr := new(dns.Msg)
	formerrBefore := transitionValue(monitor.StateMachineStateInit, monitor.StateMachineFormerrExit)
	formerrFailBefore := testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("formerr"))
	if _, err := Change(newStateInitState(reqFormerr, respFormerr, context.Background())); err != nil {
		t.Fatalf("Change formerr: %v", err)
	}
	assertCounterDelta(t, "state_init -> formerr_exit", transitionValue(monitor.StateMachineStateInit, monitor.StateMachineFormerrExit)-formerrBefore, 1)
	assertCounterDelta(t, "formerr failure", testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("formerr"))-formerrFailBefore, 1)

	servfailPort, closeServfail := reserveUnusedUDPPort(t)
	closeServfail()
	SetHostsAndForwardForTest(nil, []ForwardZone{
		{Zone: "servfail.test.", Upstreams: []string{"127.0.0.1:" + servfailPort}},
	})

	servfailBefore := transitionValue(monitor.StateMachineForwardLookup, monitor.StateMachineServfailExit)
	servfailFailBefore := testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("forward_lookup_servfail"))
	servfailReq := new(dns.Msg)
	servfailReq.SetQuestion("www.servfail.test.", dns.TypeA)
	resp, err := Change(newStateInitState(servfailReq, new(dns.Msg), context.Background()))
	if err != nil {
		t.Fatalf("Change servfail: %v", err)
	}
	if resp == nil || resp.Rcode != dns.RcodeServerFailure {
		t.Fatalf("servfail response = %#v, want SERVFAIL", resp)
	}
	assertCounterDelta(t, "forward_lookup -> servfail_exit", transitionValue(monitor.StateMachineForwardLookup, monitor.StateMachineServfailExit)-servfailBefore, 1)
	assertCounterDelta(t, "forward_lookup_servfail failure", testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("forward_lookup_servfail"))-servfailFailBefore, 1)

	validReq := new(dns.Msg)
	validReq.SetQuestion("error-exit.example.", dns.TypeA)
	errorBefore := transitionValue(monitor.StateMachineStateInit, monitor.StateMachineErrorExit)
	errorFailBefore := testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("state_init_handle_error"))
	if _, err := Change(newStateInitState(validReq, nil, context.Background())); err == nil {
		t.Fatal("expected nil-response Change to fail")
	}
	assertCounterDelta(t, "state_init -> error_exit", transitionValue(monitor.StateMachineStateInit, monitor.StateMachineErrorExit)-errorBefore, 1)
	assertCounterDelta(t, "state_init_handle_error failure", testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("state_init_handle_error"))-errorFailBefore, 1)

	maxBefore := sumExitTransitions(t, monitor.StateMachineMaxItersExit)
	maxFailBefore := testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("max_iterations"))
	seedLongCNAMEChain(t, 30)
	maxReq := new(dns.Msg)
	maxReq.SetQuestion("loop-00.example.", dns.TypeA)
	if _, err := Change(newStateInitState(maxReq, new(dns.Msg), context.Background())); err == nil {
		t.Fatal("expected max-iterations Change to fail")
	}
	assertCounterDelta(t, "max_iterations exit", sumExitTransitions(t, monitor.StateMachineMaxItersExit)-maxBefore, 1)
	assertCounterDelta(t, "max_iterations failure", testutil.ToFloat64(monitor.StateMachineFailuresTotal.WithLabelValues("max_iterations"))-maxFailBefore, 1)
}

func TestStateMachineTransitionLabelsRemainCanonicalAndBounded(t *testing.T) {
	setupStateMachineTransitionTest(t)

	setCacheCopyByType("bounded.example.", dns.TypeA, makeAMsg("bounded.example.", 60), 60)
	req := new(dns.Msg)
	req.SetQuestion("bounded.example.", dns.TypeA)
	if _, err := Change(newStateInitState(req, new(dns.Msg), context.Background())); err != nil {
		t.Fatalf("Change bounded: %v", err)
	}

	allowedFrom := make(map[string]struct{}, len(monitor.StateMachineCanonicalStates))
	for _, label := range monitor.StateMachineCanonicalStates {
		allowedFrom[label] = struct{}{}
	}
	allowedTo := make(map[string]struct{}, len(monitor.StateMachineCanonicalStates)+len(monitor.StateMachineCanonicalTerminals))
	for _, label := range monitor.StateMachineCanonicalStates {
		allowedTo[label] = struct{}{}
	}
	for _, label := range monitor.StateMachineCanonicalTerminals {
		allowedTo[label] = struct{}{}
	}

	samples := collectTransitionSamples(t)
	if len(samples) == 0 {
		t.Fatal("expected transition samples to exist")
	}
	for _, sample := range samples {
		if _, ok := allowedFrom[sample.from]; !ok {
			t.Fatalf("unexpected from label %q in transition metric", sample.from)
		}
		if _, ok := allowedTo[sample.to]; !ok {
			t.Fatalf("unexpected to label %q in transition metric", sample.to)
		}
	}
}

func setupStateMachineTransitionTest(t *testing.T) {
	t.Helper()
	monitor.InitMetricForTest()
	deleteAllCache()
	ResetHostsAndForwardForTest()
	ResetIterPort()
	t.Cleanup(func() {
		deleteAllCache()
		ResetHostsAndForwardForTest()
		ResetIterPort()
	})
}

func setCachedDelegation(zone, ns, ip string) {
	msg := makeNSMsg(zone, ns, 3600)
	msg.Extra = append(msg.Extra, &dns.A{
		Hdr: dns.RR_Header{Name: ns, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600},
		A:   net.ParseIP(ip).To4(),
	})
	setCacheCopy(zone, msg, 3600)
}

func reserveUnusedUDPPort(t *testing.T) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	port := fmt.Sprintf("%d", conn.LocalAddr().(*net.UDPAddr).Port)
	return port, func() {
		conn.Close()
	}
}

func portOfAddr(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return port
}

func seedLongCNAMEChain(t *testing.T, length int) {
	t.Helper()
	if length < 2 {
		t.Fatalf("length = %d, want >= 2", length)
	}
	for i := 0; i < length-1; i++ {
		name := fmt.Sprintf("loop-%02d.example.", i)
		target := fmt.Sprintf("loop-%02d.example.", i+1)
		setCacheCopyByType(name, dns.TypeA, makeCNAMEMsg(name, target, 60), 60)
	}
	last := fmt.Sprintf("loop-%02d.example.", length-1)
	setCacheCopyByType(last, dns.TypeA, makeAMsg(last, 60), 60)
}

func transitionValues(labels ...string) map[string]float64 {
	values := make(map[string]float64, len(labels)/2)
	for i := 0; i+1 < len(labels); i += 2 {
		key := labels[i] + "->" + labels[i+1]
		values[key] = transitionValue(labels[i], labels[i+1])
	}
	return values
}

func transitionValue(from, to string) float64 {
	return testutil.ToFloat64(monitor.StateMachineTransitionTotal.WithLabelValues(from, to))
}

func assertTransitionDelta(t *testing.T, before map[string]float64, from, to string, want float64) {
	t.Helper()
	key := from + "->" + to
	assertCounterDelta(t, key, transitionValue(from, to)-before[key], want)
}

func assertCounterDelta(t *testing.T, name string, got, want float64) {
	t.Helper()
	if got != want {
		t.Fatalf("%s delta = %f, want %f", name, got, want)
	}
}

func sumExitTransitions(t *testing.T, exit string) float64 {
	t.Helper()
	total := 0.0
	for _, sample := range collectTransitionSamples(t) {
		if sample.to == exit {
			total += sample.value
		}
	}
	return total
}

func collectTransitionSamples(t *testing.T) []transitionSample {
	t.Helper()
	ch := make(chan prometheus.Metric, 64)
	go func() {
		monitor.StateMachineTransitionTotal.Collect(ch)
		close(ch)
	}()

	var samples []transitionSample
	for metric := range ch {
		dtoMetric := new(dto.Metric)
		if err := metric.Write(dtoMetric); err != nil {
			t.Fatalf("metric.Write: %v", err)
		}
		sample := transitionSample{value: dtoMetric.GetCounter().GetValue()}
		for _, label := range dtoMetric.GetLabel() {
			switch label.GetName() {
			case "from":
				sample.from = label.GetValue()
			case "to":
				sample.to = label.GetValue()
			}
		}
		samples = append(samples, sample)
	}
	return samples
}
