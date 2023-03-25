package server

import (
	"fmt"

	"rec53/utils"

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

type checkRespState struct {
	request  *dns.Msg
	response *dns.Msg
}

func newCheckRespState(req, resp *dns.Msg) *checkRespState {
	return &checkRespState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *checkRespState) getCurrentState() int {
	return CHECK_RESP
}

func (s *checkRespState) getRequest() *dns.Msg {
	return nil
}

func (s *checkRespState) getResponse() *dns.Msg {
	return nil
}

func (s *checkRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return CHECK_RESP_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	if len(response.Answer) != 0 {
		if response.Answer[len(response.Answer)-1].Header().Rrtype == request.Question[0].Qtype {
			return CHECK_RESP_GET_ANS, nil
		}
		//TODO: another type
		return CHECK_RESP_GET_CNAME, nil
	}
	return CHECK_RESP_GET_NS, nil
}

type inGlueState struct {
	request  *dns.Msg
	response *dns.Msg
	zoneList []string
}

func newInGlueState(req, resp *dns.Msg) *inGlueState {
	return &inGlueState{
		request:  req,
		response: resp,
		zoneList: []string{},
	}
}

//implement stateMachine interface
func (s *inGlueState) getCurrentState() int {
	return IN_GLUE
}

func (s *inGlueState) getRequest() *dns.Msg {
	return nil
}

func (s *inGlueState) getResponse() *dns.Msg {
	return nil
}

func (s *inGlueState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return IN_GLUE_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	if len(response.Ns) != 0 && len(response.Extra) != 0 {
		//We got glue from cache or iterater
		//get zone list
		return IN_GLUE_EXIST, nil
	}
	s.zoneList = utils.GetZoneList(request.Question[0].Name)
	return IN_GLUE_NOT_EXIST, nil
}
