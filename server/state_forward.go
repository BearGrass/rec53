package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type forwardLookupState struct {
	baseState
}

// newForwardLookupState creates a forwardLookupState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newForwardLookupState(req, resp *dns.Msg, ctx context.Context) *forwardLookupState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &forwardLookupState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *forwardLookupState) getCurrentState() int {
	return FORWARD_LOOKUP
}

// handle checks whether the query name matches any configured forwarding zone.
//
// The globalForwardZones slice is pre-sorted by zone length descending, so the
// first match found by linear scan is the longest (most-specific) suffix match.
//
// Match found:  try each upstream in order (sequential, RD=1); first success → HIT.
//
//	All upstreams fail → SERVFAIL (no fallback to iterative resolution).
//
// No match:     MISS → continue to CACHE_LOOKUP.
//
// Design constraint D-4: forwarded results are NOT written to globalDnsCache.
// Design constraint D-5: if all upstreams fail, return SERVFAIL — do not fall back to iterative.
func (s *forwardLookupState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return FORWARD_LOOKUP_COMMON_ERROR, fmt.Errorf("request is nil or response is nil in %s", "FORWARD_LOOKUP")
	}

	snap := globalHostsForward.Load()

	if len(snap.forwardZones) == 0 {
		return FORWARD_LOOKUP_MISS, nil
	}

	q := request.Question[0]
	qname := q.Name // already FQDN from the DNS library

	// Longest-suffix match: linear scan over zones sorted by length descending.
	var matched *ForwardZone
	for i := range snap.forwardZones {
		if dns.IsSubDomain(snap.forwardZones[i].Zone, qname) {
			matched = &snap.forwardZones[i]
			break
		}
	}

	if matched == nil {
		monitor.Rec53Log.Debugf("[FORWARD_LOOKUP] miss for %s — no matching zone", qname)
		return FORWARD_LOOKUP_MISS, nil
	}

	monitor.Rec53Log.Debugf("[FORWARD_LOOKUP] zone match: %s → zone %s (%d upstreams)",
		qname, matched.Zone, len(matched.Upstreams))

	// Build forwarding query with RD=1.
	fwdQuery := new(dns.Msg)
	fwdQuery.SetQuestion(q.Name, q.Qtype)
	fwdQuery.RecursionDesired = true
	fwdQuery.Id = dns.Id()

	// Try each upstream in order; first success wins.
	client := &dns.Client{
		Net:     "udp",
		Timeout: GetUpstreamTimeout(),
		UDPSize: 4096,
	}

	var lastErr error
	for _, upstream := range matched.Upstreams {
		monitor.Rec53Log.Debugf("[FORWARD_LOOKUP] trying upstream %s for %s", upstream, qname)

		resp, _, err := client.ExchangeContext(s.ctx, fwdQuery, upstream)
		if err != nil {
			monitor.Rec53Log.Debugf("[FORWARD_LOOKUP] upstream %s failed: %v", upstream, err)
			lastErr = err
			continue
		}

		// Success — copy response to s.response.
		// Copy the full MsgHdr so all upstream flags (TC, AA, RA, AD, CD…)
		// are preserved. Then fix Id and Response to match our client's query.
		// D-4: do NOT write to globalDnsCache.
		response.MsgHdr = resp.MsgHdr
		response.Id = request.Id
		response.Response = true
		response.Answer = resp.Answer
		response.Ns = resp.Ns
		response.Extra = resp.Extra

		monitor.Rec53Log.Debugf("[FORWARD_LOOKUP] hit from upstream %s for %s (rcode=%s, answers=%d, tc=%v)",
			upstream, qname, dns.RcodeToString[resp.Rcode], len(resp.Answer), resp.Truncated)
		return FORWARD_LOOKUP_HIT, nil
	}

	// All upstreams failed → SERVFAIL, no fallback to iterative resolution (D-5).
	monitor.Rec53Log.Errorf("[FORWARD_LOOKUP] all upstreams failed for %s (zone %s): %v",
		qname, matched.Zone, lastErr)
	return FORWARD_LOOKUP_SERVFAIL, nil
}
