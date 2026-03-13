package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type cacheLookupState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newCacheLookupState(req, resp *dns.Msg) *cacheLookupState {
	return &cacheLookupState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newCacheLookupStateWithContext creates a cacheLookupState with a specific context
func newCacheLookupStateWithContext(req, resp *dns.Msg, ctx context.Context) *cacheLookupState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &cacheLookupState{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
}

// implement stateMachine interface
func (s *cacheLookupState) getCurrentState() int {
	return CACHE_LOOKUP
}

func (s *cacheLookupState) getRequest() *dns.Msg {
	return s.request
}

func (s *cacheLookupState) getResponse() *dns.Msg {
	return s.response
}

func (s *cacheLookupState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
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
			s.response.Answer = append(s.response.Answer, msgInCache.Answer...)
			return CACHE_LOOKUP_HIT, nil
		}
	}
	return CACHE_LOOKUP_MISS, nil
}
