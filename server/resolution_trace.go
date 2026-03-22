package server

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

const maxTraceStates = 128

const contextKeyResolutionTrace contextKeyType = "resolutionTrace"

type ResolutionTrace struct {
	QueryName string
	QueryType string
	States    []string
	Terminal  string
	Rcode     string
	Error     string
	Truncated bool
}

type resolutionTraceRecorder struct {
	mu        sync.Mutex
	queryName string
	queryType string
	states    []string
	terminal  string
	truncated bool
}

func newResolutionTraceRecorder(queryName string, qtype uint16) *resolutionTraceRecorder {
	queryType := dns.TypeToString[qtype]
	if queryType == "" {
		queryType = "UNKNOWN"
	}
	return &resolutionTraceRecorder{
		queryName: queryName,
		queryType: queryType,
	}
}

func withResolutionTrace(ctx context.Context, recorder *resolutionTraceRecorder) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKeyResolutionTrace, recorder)
}

func resolutionTraceFromContext(ctx context.Context) *resolutionTraceRecorder {
	if ctx == nil {
		return nil
	}
	recorder, _ := ctx.Value(contextKeyResolutionTrace).(*resolutionTraceRecorder)
	return recorder
}

func (r *resolutionTraceRecorder) recordState(state string) {
	if r == nil || state == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.states) >= maxTraceStates {
		r.truncated = true
		return
	}
	r.states = append(r.states, state)
}

func (r *resolutionTraceRecorder) recordTerminal(exit string) {
	if r == nil || exit == "" {
		return
	}
	r.mu.Lock()
	r.terminal = exit
	r.mu.Unlock()
}

func (r *resolutionTraceRecorder) snapshot(response *dns.Msg, err error) *ResolutionTrace {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	trace := &ResolutionTrace{
		QueryName: r.queryName,
		QueryType: r.queryType,
		States:    append([]string(nil), r.states...),
		Terminal:  r.terminal,
		Truncated: r.truncated,
	}
	if response != nil {
		trace.Rcode = dns.RcodeToString[response.Rcode]
		if trace.Rcode == "" {
			trace.Rcode = "UNKNOWN"
		}
	}
	if err != nil {
		trace.Error = err.Error()
	}
	return trace
}

func (t *ResolutionTrace) Format() string {
	if t == nil {
		return ""
	}
	lines := []string{
		"query: " + t.QueryName + " " + t.QueryType,
	}
	if len(t.States) == 0 {
		lines = append(lines, "states: (none)")
	} else {
		lines = append(lines, "states:")
		for i, state := range t.States {
			lines = append(lines, "  "+itoa(i+1)+". "+state)
		}
	}
	lines = append(lines, "terminal: "+fallbackTraceText(t.Terminal))
	lines = append(lines, "rcode: "+fallbackTraceText(t.Rcode))
	if t.Truncated {
		lines = append(lines, "note: state list truncated")
	}
	if t.Error != "" {
		lines = append(lines, "error: "+t.Error)
	}
	return strings.Join(lines, "\n")
}

func fallbackTraceText(value string) string {
	if value == "" {
		return "UNKNOWN"
	}
	return value
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func TraceDomain(ctx context.Context, domain string, qtype uint16) (*dns.Msg, *ResolutionTrace, error) {
	fqdn := dns.Fqdn(domain)
	req := new(dns.Msg)
	req.SetQuestion(fqdn, qtype)
	resp := new(dns.Msg)

	recorder := newResolutionTraceRecorder(fqdn, qtype)
	traceCtx := withResolutionTrace(ctx, recorder)

	result, err := Change(newStateInitState(req, resp, traceCtx))
	trace := recorder.snapshot(result, err)
	return result, trace, err
}
