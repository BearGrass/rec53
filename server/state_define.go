package server

import (
	"fmt"

	"github.com/miekg/dns"
)

type stateInitState struct {
	request  *dns.Msg
	response *dns.Msg
}

func newStateInitState(req, resp *dns.Msg) *stateInitState {
	return &stateInitState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *stateInitState) getCurrentState() int {
	return STATE_INIT
}

func (s *stateInitState) getRequest() *dns.Msg {
	return s.request
}

func (s *stateInitState) getResponse() *dns.Msg {
	return s.response
}

func (s *stateInitState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return STATE_INIT_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	s.response = request.SetReply(response)
	s.request = request
	return STATE_INIT_NO_ERROR, nil
}

type inCacheState struct {
	request  *dns.Msg
	response *dns.Msg
}

func newInCacheState(req, resp *dns.Msg) *inCacheState {
	return &inCacheState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *inCacheState) getCurrentState() int {
	return IN_CACHE
}

func (s *inCacheState) getRequest() *dns.Msg {
	return nil
}

func (s *inCacheState) getResponse() *dns.Msg {
	return nil
}

func (s *inCacheState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return IN_CACHE_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	requestCache := GetCache()
	if requestCache == nil {
		return IN_CACHE_COMMEN_ERROR, fmt.Errorf("requestCache is nil")
	}
	if _, ok := requestCache[request.Question[0].String()]; ok {
		s.response.Answer = append(s.response.Answer, requestCache[request.Question[0].String()].Answer...)
		return IN_CACHE_HIT_CACHE, nil
	}
	return IN_CACHE_MISS_CACHE, nil
}
