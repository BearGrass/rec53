package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type cacheLookupState struct {
	baseState
}

// newCacheLookupState creates a cacheLookupState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newCacheLookupState(req, resp *dns.Msg, ctx context.Context) *cacheLookupState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &cacheLookupState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *cacheLookupState) getCurrentState() int {
	return CACHE_LOOKUP
}

func (s *cacheLookupState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return CACHE_LOOKUP_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	q := request.Question[0]
	monitor.Rec53Log.Debugf("try to get cache %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
	// Use getCacheCopyByType to ensure we get the correct record type
	if msgInCache, ok := getCacheCopyByType(q.Name, q.Qtype); ok {
		monitor.Rec53Log.Debugf("get cache %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
		if len(msgInCache.Answer) != 0 {
			// Positive cache hit: copy Answer records.
			s.response.Answer = append(s.response.Answer, msgInCache.Answer...)
			return CACHE_LOOKUP_HIT, nil
		}
		// Negative cache hit: NXDOMAIN/NODATA entries have empty Answer
		// but contain SOA in Authority (Ns). Copy Rcode and Ns so that
		// classifyRespState can detect the negative response via
		// hasSOAInAuthority() and return it to the client.
		if hasSOAInAuthority(msgInCache) {
			s.response.Rcode = msgInCache.Rcode
			s.response.Ns = append(s.response.Ns, msgInCache.Ns...)
			monitor.Rec53Log.Debugf("[CACHE_LOOKUP] negative cache hit for %s (type: %s, rcode: %s)",
				q.Name, dns.TypeToString[q.Qtype], dns.RcodeToString[msgInCache.Rcode])
			return CACHE_LOOKUP_HIT, nil
		}
	}
	return CACHE_LOOKUP_MISS, nil
}
