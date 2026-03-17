package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type hostsLookupState struct {
	baseState
}

// newHostsLookupState creates a hostsLookupState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newHostsLookupState(req, resp *dns.Msg, ctx context.Context) *hostsLookupState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &hostsLookupState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *hostsLookupState) getCurrentState() int {
	return HOSTS_LOOKUP
}

// handle looks up the query in the compiled hosts map.
//
// Hit:           snap.hostsMap[key] exists → copy answer RRs, set AA=true, return HIT.
// NODATA:        name exists in snap.hostsNames but no matching qtype → NOERROR + empty answer, return HIT.
// Miss:          name not in hosts at all → return MISS (pass to next state).
func (s *hostsLookupState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return HOSTS_LOOKUP_COMMON_ERROR, fmt.Errorf("request is nil or response is nil in %s", "HOSTS_LOOKUP")
	}

	snap := globalHostsForward.Load()

	if len(snap.hostsMap) == 0 {
		return HOSTS_LOOKUP_MISS, nil
	}

	q := request.Question[0]
	key := fmt.Sprintf("%s:%d", q.Name, q.Qtype)

	if cached, ok := snap.hostsMap[key]; ok {
		monitor.Rec53Log.Debugf("[HOSTS_LOOKUP] hit for %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
		// Copy answer RRs to avoid mutating the compiled hosts map.
		for _, rr := range cached.Answer {
			response.Answer = append(response.Answer, dns.Copy(rr))
		}
		response.Authoritative = true
		response.Rcode = dns.RcodeSuccess
		return HOSTS_LOOKUP_HIT, nil
	}

	// Name exists but qtype does not match → NODATA (NOERROR + empty answer).
	// Per spec: "Type mismatch on hosts entry → RCODE=NOERROR with no answers (NODATA)"
	if snap.hostsNames[q.Name] {
		monitor.Rec53Log.Debugf("[HOSTS_LOOKUP] NODATA for %s (type: %s) — name exists, type mismatch",
			q.Name, dns.TypeToString[q.Qtype])
		response.Authoritative = true
		response.Rcode = dns.RcodeSuccess
		return HOSTS_LOOKUP_HIT, nil
	}

	monitor.Rec53Log.Debugf("[HOSTS_LOOKUP] miss for %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
	return HOSTS_LOOKUP_MISS, nil
}
