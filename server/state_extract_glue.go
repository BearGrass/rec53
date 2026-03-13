package server

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
)

type extractGlueState struct {
	baseState
}

// newExtractGlueState creates an extractGlueState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newExtractGlueState(req, resp *dns.Msg, ctx context.Context) *extractGlueState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &extractGlueState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *extractGlueState) getCurrentState() int {
	return EXTRACT_GLUE
}

func (s *extractGlueState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return EXTRACT_GLUE_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	if len(response.Ns) != 0 {
		// Only treat actual NS delegation records as glue/delegation candidates.
		// SOA records in NODATA/NXDOMAIN responses also appear in the Ns section
		// but are not delegations and must not be forwarded as such.
		if _, isNS := response.Ns[0].(*dns.NS); isNS {
			// Validate that the NS zone is relevant to the current query domain.
			// If the NS zone is not an ancestor of the query domain, the delegation
			// belongs to a different zone (e.g., a prior CNAME hop) and must not be used.
			nsZone := response.Ns[0].Header().Name
			queryName := request.Question[0].Name
			if dns.IsSubDomain(nsZone, queryName) {
				// Both glued (Extra present) and glueless (Extra absent) NS referrals are
				// accepted here. QUERY_UPSTREAM already handles the glueless case by calling
				// resolveNSIPs / resolveNSIPsConcurrently when ipList is empty.
				return EXTRACT_GLUE_EXIST, nil
			}
		}
		// NS zone is unrelated to query domain, or Ns contains non-delegation records
		// (e.g. SOA); clear stale data and re-delegate from root.
		response.Ns = nil
		response.Extra = nil
	}
	return EXTRACT_GLUE_NOT_EXIST, nil
}
