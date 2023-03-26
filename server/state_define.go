package server

import (
	"fmt"
	"time"

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
	response.SetReply(request)
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
	return s.request
}

func (s *inCacheState) getResponse() *dns.Msg {
	return s.response
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
	return s.request
}

func (s *checkRespState) getResponse() *dns.Msg {
	return s.response
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
}

func newInGlueState(req, resp *dns.Msg) *inGlueState {
	return &inGlueState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *inGlueState) getCurrentState() int {
	return IN_GLUE
}

func (s *inGlueState) getRequest() *dns.Msg {
	return s.request
}

func (s *inGlueState) getResponse() *dns.Msg {
	return s.response
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
	return IN_GLUE_NOT_EXIST, nil
}

type iterState struct {
	request  *dns.Msg
	response *dns.Msg
}

func newIterState(req, resp *dns.Msg) *iterState {
	return &iterState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *iterState) getCurrentState() int {
	return ITER
}

func (s *iterState) getRequest() *dns.Msg {
	return s.request
}

func (s *iterState) getResponse() *dns.Msg {
	return s.response
}

type ipSeed struct {
	ip  string
	ttl int
}

func getBestAddress(response *dns.Msg) (string, error) {
	if response == nil {
		return "", fmt.Errorf("response is nil")
	}
	if len(response.Extra) == 0 {
		return "", fmt.Errorf("response.Extra is nil")
	}
	var bestAddr string
	var bestTTL int = utils.MAX_TIMEOUT
	// Concurrent func to get the best address
	ttlChan := make(chan ipSeed, len(response.Extra))
	cnt := 0
	for _, extra := range response.Extra {
		if extra.Header().Rrtype == dns.TypeA {
			addr := extra.(*dns.A).A.String()
			cnt++
			if bestAddr == "" {
				bestAddr = addr
			}
			//try to ping the addr
			go func(addr string) {
				ttl, err := utils.Hc(addr)
				if err != nil {
					ttlChan <- ipSeed{ip: addr, ttl: utils.MAX_TIMEOUT}
				}
				ttlChan <- ipSeed{ip: addr, ttl: ttl}
			}(addr)
		}
	}
	for i := 0; i < cnt; i++ {
		ttl := <-ttlChan
		if ttl.ttl < bestTTL {
			bestTTL = ttl.ttl
			bestAddr = ttl.ip
		}
	}
	if bestAddr == "" {
		return "", fmt.Errorf("bestAddr is nil")
	}
	return bestAddr, nil
}

func (s *iterState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return ITER_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	newQuery := new(dns.Msg)
	newQuery.SetQuestion(request.Question[0].Name, request.Question[0].Qtype)
	newQuery.RecursionDesired = false
	newQuery.Id = dns.Id()
	//check the best ip in the extra in response
	bestAddr, err := getBestAddress(response)
	if err != nil {
		return ITER_COMMEN_ERROR, err
	}
	dnsClient := &dns.Client{}
	dnsClient.Net = "udp"
	dnsClient.Timeout = 5 * time.Second
	dnsClient.SingleInflight = true

	//send query to the best ip
	newResponse, _, err := dnsClient.Exchange(newQuery, bestAddr+":53")
	if err != nil {
		return ITER_COMMEN_ERROR, err
	}
	//check the response
	if newResponse.Rcode != dns.RcodeSuccess {
		return ITER_COMMEN_ERROR, fmt.Errorf("response.Rcode is not success")
	}
	//check the response is the same as the request
	if newResponse.Question[0].Name != request.Question[0].Name {
		return ITER_COMMEN_ERROR, fmt.Errorf("response.Question is not the same as request")
	}
	s.response.Answer = append(s.response.Answer, newResponse.Answer...)
	s.response.Ns = newResponse.Ns
	s.response.Extra = newResponse.Extra
	return ITER_NO_ERROR, nil
}

type inGlueCacheState struct {
	request  *dns.Msg
	response *dns.Msg
}

func newInGlueCacheState(req, resp *dns.Msg) *inGlueCacheState {
	return &inGlueCacheState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *inGlueCacheState) getCurrentState() int {
	return IN_GLUE_CACHE
}

func (s *inGlueCacheState) getRequest() *dns.Msg {
	return s.request
}

func (s *inGlueCacheState) getResponse() *dns.Msg {
	return s.response
}

func (s *inGlueCacheState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return IN_GLUE_CACHE_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	glueCache := GetCache()
	if glueCache == nil {
		return IN_GLUE_CACHE_COMMEN_ERROR, fmt.Errorf("glueCache is nil")
	}
	zoneList := utils.GetZoneList(request.Question[0].Name)
	for _, zone := range zoneList {
		if _, ok := glueCache[zone]; ok {
			s.response.Ns = append(s.response.Ns, glueCache[zone].Ns...)
			s.response.Extra = append(s.response.Extra, glueCache[zone].Extra...)
			return IN_GLUE_CACHE_HIT_CACHE, nil
		}
	}
	rootGlue := utils.GetRootGlue()
	s.response.Ns = append(s.response.Ns, rootGlue.Ns...)
	s.response.Extra = append(s.response.Extra, rootGlue.Extra...)
	return IN_CACHE_MISS_CACHE, nil
}

type retRespState struct {
	request  *dns.Msg
	response *dns.Msg
}

func newRetRespState(req, resp *dns.Msg) *retRespState {
	return &retRespState{
		request:  req,
		response: resp,
	}
}

//implement stateMachine interface
func (s *retRespState) getCurrentState() int {
	return RET_RESP
}

func (s *retRespState) getRequest() *dns.Msg {
	return s.request
}

func (s *retRespState) getResponse() *dns.Msg {
	return s.response
}

func (s *retRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return RET_RESP_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	return RET_RESP_NO_ERROR, nil
}
