package server

import (
	"context"
	"fmt"

	"rec53/monitor"
	"rec53/utils"

	"github.com/miekg/dns"
)

type lookupNSCacheState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newLookupNSCacheState(req, resp *dns.Msg) *lookupNSCacheState {
	return &lookupNSCacheState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newLookupNSCacheStateWithContext creates a lookupNSCacheState with a specific context
func newLookupNSCacheStateWithContext(req, resp *dns.Msg, ctx context.Context) *lookupNSCacheState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &lookupNSCacheState{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
}

// implement stateMachine interface
func (s *lookupNSCacheState) getCurrentState() int {
	return LOOKUP_NS_CACHE
}

func (s *lookupNSCacheState) getRequest() *dns.Msg {
	return s.request
}

func (s *lookupNSCacheState) getResponse() *dns.Msg {
	return s.response
}

func (s *lookupNSCacheState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *lookupNSCacheState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return LOOKUP_NS_CACHE_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	zoneList := utils.GetZoneList(request.Question[0].Name)
	for _, zone := range zoneList {
		// Use getCacheCopy to avoid modifying cached message
		if msgInCache, ok := getCacheCopy(zone); ok {
			monitor.Rec53Log.Debug("get cache: ", zone, " in lookupNSCacheState")
			if len(msgInCache.Ns) != 0 && len(msgInCache.Extra) != 0 {
				s.response.Ns = append(s.response.Ns, msgInCache.Ns...)
				s.response.Extra = append(s.response.Extra, msgInCache.Extra...)
				return LOOKUP_NS_CACHE_HIT, nil
			}
		}
	}
	rootGlue := utils.GetRootGlue()
	s.response.Ns = append(s.response.Ns, rootGlue.Ns...)
	s.response.Extra = append(s.response.Extra, rootGlue.Extra...)
	return LOOKUP_NS_CACHE_MISS, nil
}
