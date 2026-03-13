package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type classifyRespState struct {
	baseState
}

// newClassifyRespState creates a classifyRespState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newClassifyRespState(req, resp *dns.Msg, ctx context.Context) *classifyRespState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &classifyRespState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *classifyRespState) getCurrentState() int {
	return CLASSIFY_RESP
}

func (s *classifyRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return CLASSIFY_RESP_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}

	qtype := request.Question[0].Qtype
	qname := request.Question[0].Name
	monitor.Rec53Log.Debugf("[CHECK_RESP] Checking response for %s (type: %s), Rcode: %s, Answers: %d, Ns: %d, Extra: %d",
		qname, dns.TypeToString[qtype], dns.RcodeToString[response.Rcode], len(response.Answer), len(response.Ns), len(response.Extra))

	// Priority 1: Check for negative responses (NXDOMAIN or NODATA)
	// These are authoritative responses from upstream that must be passed to the client
	if len(response.Answer) == 0 && hasSOAInAuthority(response) {
		// Negative response detected: empty Answer + SOA in Authority
		if response.Rcode == dns.RcodeNameError {
			// NXDOMAIN: domain does not exist
			monitor.Rec53Log.Debugf("[CHECK_RESP] NXDOMAIN detected for %s, returning negative response", qname)
			// Cache the negative response
			if soa, ttl := extractSOAFromAuthority(response); soa != nil {
				setCacheCopyByType(qname, qtype, response, ttl)
				monitor.Rec53Log.Debugf("[CHECK_RESP] Cached NXDOMAIN for %s (type: %s) with TTL: %d", qname, dns.TypeToString[qtype], ttl)
			}
			return CLASSIFY_RESP_GET_NEGATIVE, nil
		} else if response.Rcode == dns.RcodeSuccess {
			// NODATA: domain exists but has no records of the requested type
			monitor.Rec53Log.Debugf("[CHECK_RESP] NODATA detected for %s (type: %s), returning negative response", qname, dns.TypeToString[qtype])
			// Cache the negative response
			if soa, ttl := extractSOAFromAuthority(response); soa != nil {
				setCacheCopyByType(qname, qtype, response, ttl)
				monitor.Rec53Log.Debugf("[CHECK_RESP] Cached NODATA for %s (type: %s) with TTL: %d", qname, dns.TypeToString[qtype], ttl)
			}
			return CLASSIFY_RESP_GET_NEGATIVE, nil
		}
	}

	// Priority 2: Check if we have any answers
	if len(response.Answer) == 0 {
		// No answers and no SOA (not a negative response), need to continue iteration
		monitor.Rec53Log.Debugf("[CHECK_RESP] No answers (and no SOA), continuing to IN_GLUE")
		return CLASSIFY_RESP_GET_NS, nil
	}

	// Priority 3: Check if we have a matching record type in the answers
	for _, rr := range response.Answer {
		if rr.Header().Rrtype == qtype {
			// Found a matching record type, return the answer
			monitor.Rec53Log.Debugf("[CHECK_RESP] Found matching type %s, returning answer", dns.TypeToString[qtype])
			return CLASSIFY_RESP_GET_ANS, nil
		}
	}

	// Priority 4: Check if we have a CNAME record that needs to be followed
	// A CNAME can only exist when querying for A, AAAA, or other types that CNAME points to
	if qtype != dns.TypeCNAME {
		for _, rr := range response.Answer {
			if cname, ok := rr.(*dns.CNAME); ok {
				// Found a CNAME record, need to follow it
				monitor.Rec53Log.Debugf("[CHECK_RESP] Found CNAME: %s -> %s", rr.Header().Name, cname.Target)
				return CLASSIFY_RESP_GET_CNAME, nil
			}
		}
	}

	// Priority 5: We have answers but none match the requested type and no CNAME found
	// This could happen when:
	// - Querying for a type that doesn't exist (but other types do)
	// - The server returned a partial answer
	// Continue iteration to get the correct type
	monitor.Rec53Log.Debugf("[CHECK_RESP] No matching type %s in answers, continuing to IN_GLUE", dns.TypeToString[qtype])
	return CLASSIFY_RESP_GET_NS, nil
}
