package server

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
)

type extractGlueState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newExtractGlueState(req, resp *dns.Msg) *extractGlueState {
	return &extractGlueState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newExtractGlueStateWithContext creates an extractGlueState with a specific context
func newExtractGlueStateWithContext(req, resp *dns.Msg, ctx context.Context) *extractGlueState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &extractGlueState{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
}

// implement stateMachine interface
func (s *extractGlueState) getCurrentState() int {
	return EXTRACT_GLUE
}

func (s *extractGlueState) getRequest() *dns.Msg {
	return s.request
}

func (s *extractGlueState) getResponse() *dns.Msg {
	return s.response
}

func (s *extractGlueState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *extractGlueState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return EXTRACT_GLUE_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	if len(response.Ns) != 0 && len(response.Extra) != 0 {
		// We got glue from cache or iterator.
		// Validate that the NS zone is relevant to the current query domain.
		// If the NS zone is not an ancestor of the query domain, the glue belongs
		// to a different delegation zone (e.g., a prior CNAME hop) and must not be used.
		nsZone := response.Ns[0].Header().Name
		queryName := request.Question[0].Name
		if dns.IsSubDomain(nsZone, queryName) {
			return EXTRACT_GLUE_EXIST, nil
		}
		// NS zone is unrelated to query domain; clear stale glue and re-delegate.
		response.Ns = nil
		response.Extra = nil
	}
	return EXTRACT_GLUE_NOT_EXIST, nil
}
