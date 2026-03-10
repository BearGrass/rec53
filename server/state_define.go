package server

import (
	"fmt"
	"time"

	"rec53/monitor"
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

// implement stateMachine interface
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

// implement stateMachine interface
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
	q := request.Question[0]
	monitor.Rec53Log.Debugf("try to get cache %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
	// Use getCacheCopyByType to ensure we get the correct record type
	if msgInCache, ok := getCacheCopyByType(q.Name, q.Qtype); ok {
		monitor.Rec53Log.Debugf("get cache %s (type: %s)", q.Name, dns.TypeToString[q.Qtype])
		if len(msgInCache.Answer) != 0 {
			s.response.Answer = append(s.response.Answer, msgInCache.Answer...)
			return IN_CACHE_HIT_CACHE, nil
		}
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

// implement stateMachine interface
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

	qtype := request.Question[0].Qtype

	// Check if we have any answers
	if len(response.Answer) == 0 {
		// No answers, need to continue iteration
		return CHECK_RESP_GET_NS, nil
	}

	// Check if we have a matching record type in the answers
	for _, rr := range response.Answer {
		if rr.Header().Rrtype == qtype {
			// Found a matching record type, return the answer
			return CHECK_RESP_GET_ANS, nil
		}
	}

	// Check if we have a CNAME record that needs to be followed
	// A CNAME can only exist when querying for A, AAAA, or other types that CNAME points to
	if qtype != dns.TypeCNAME {
		for _, rr := range response.Answer {
			if cname, ok := rr.(*dns.CNAME); ok {
				// Found a CNAME record, need to follow it
				monitor.Rec53Log.Debugf("found CNAME: %s -> %s", rr.Header().Name, cname.Target)
				return CHECK_RESP_GET_CNAME, nil
			}
		}
	}

	// We have answers but none match the requested type and no CNAME found
	// This could happen when:
	// - Querying for a type that doesn't exist (but other types do)
	// - The server returned a partial answer
	// Continue iteration to get the correct type
	monitor.Rec53Log.Debugf("no matching type %s in answers, continuing iteration", dns.TypeToString[qtype])
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

// implement stateMachine interface
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

// implement stateMachine interface
func (s *iterState) getCurrentState() int {
	return ITER
}

func (s *iterState) getRequest() *dns.Msg {
	return s.request
}

func (s *iterState) getResponse() *dns.Msg {
	return s.response
}

func getIPListFromResponse(response *dns.Msg) []string {
	var ipList []string
	for _, extra := range response.Extra {
		if extra.Header().Rrtype == dns.TypeA {
			ipList = append(ipList, extra.(*dns.A).A.String())
		}
	}
	return ipList
}

func getBestAddressAndPrefetchIPs(ipList []string) (string, string, error) {
	if len(ipList) == 0 {
		return "", "", fmt.Errorf("no ip in extra")
	}
	bestIP, oldBestIP := globalIPPool.getBestIPs(ipList)
	if bestIP != "" {
		IPs := globalIPPool.GetPrefetchIPs(bestIP)
		globalIPPool.PrefetchIPs(IPs)
	}
	return bestIP, oldBestIP, nil
}

func (s *iterState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return ITER_COMMEN_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	newQuery := new(dns.Msg)
	newQuery.SetQuestion(request.Question[0].Name, request.Question[0].Qtype)
	newQuery.RecursionDesired = false
	newQuery.Id = dns.Id()
	// Set EDNS0 with larger buffer size to handle larger responses
	newQuery.SetEdns0(4096, false)

	//check the best ip in the extra in response
	ipList := getIPListFromResponse(response)
	bestAddr, secondAddr, err := getBestAddressAndPrefetchIPs(ipList)
	if bestAddr == "" || err != nil {
		return ITER_COMMEN_ERROR, err
	}
	dnsClient := &dns.Client{
		Net:            "udp",
		Timeout:        5 * time.Second,
		SingleInflight: true,
		UDPSize:        4096, // Set larger UDP buffer size for EDNS
	}

	//send query to the best ip
	theBestIP := bestAddr
	monitor.Rec53Metric.InCounterAdd("forward_request", newQuery.Question[0].Name, dns.TypeToString[newQuery.Question[0].Qtype])
	newResponse, rtt, err := dnsClient.Exchange(newQuery, bestAddr+":53")
	if err != nil {
		ipq := globalIPPool.GetIPQuality(bestAddr)
		ipq.SetLatency(MAX_IP_LATENCY)
		//try to use the second ip
		if secondAddr == "" {
			return ITER_COMMEN_ERROR, err
		}
		newResponse, rtt, err = dnsClient.Exchange(newQuery, secondAddr+":53")
		if err != nil {
			ipq := globalIPPool.GetIPQuality(secondAddr)
			ipq.SetLatency(MAX_IP_LATENCY)
			return ITER_COMMEN_ERROR, err
		}
		theBestIP = secondAddr
	}

	if !globalIPPool.isTheIPInit(theBestIP) {
		globalIPPool.UpIPsQuality(ipList)
	}
	//update the ip quality
	globalIPPool.updateIPQuality(theBestIP, int32(rtt/time.Millisecond))
	monitor.Rec53Metric.IPQualityGaugeSet(theBestIP, float64(rtt/time.Millisecond))

	monitor.Rec53Metric.OutCounterAdd("forward_response", newQuery.Question[0].Name, dns.TypeToString[newQuery.Question[0].Qtype], dns.RcodeToString[newResponse.Rcode])
	//check the response
	if newResponse.Rcode != dns.RcodeSuccess {
		// Copy response code and authority section
		s.response.Rcode = newResponse.Rcode
		s.response.Ns = newResponse.Ns

		// Handle different response codes appropriately
		switch newResponse.Rcode {
		case dns.RcodeNameError: // NXDOMAIN - domain does not exist
			// Return normally with NXDOMAIN code preserved
			return ITER_NO_ERROR, nil
		case dns.RcodeSuccess:
			return ITER_NO_ERROR, nil
		default:
			// Other errors (REFUSED, SERVFAIL, etc.) - return as error
			return ITER_COMMEN_ERROR, fmt.Errorf("response rcode: %s",
				dns.RcodeToString[newResponse.Rcode])
		}
	}
	//check the response is the same as the request
	if len(newResponse.Question) == 0 {
		return ITER_COMMEN_ERROR, fmt.Errorf("response has no question")
	}
	if newResponse.Question[0].Name != request.Question[0].Name {
		return ITER_COMMEN_ERROR, fmt.Errorf("response.Question is not the same as request")
	}
	if len(newResponse.Answer) != 0 {
		// Use setCacheCopyByType to store with query type in key
		q := newResponse.Question[0]
		setCacheCopyByType(q.Name, q.Qtype, newResponse, newResponse.Answer[0].Header().Ttl)
		monitor.Rec53Log.Debug("set cache: ", q.Name, " type:", dns.TypeToString[q.Qtype], " ttl:", newResponse.Answer[0].Header().Ttl)
	}
	if len(newResponse.Ns) != 0 && len(newResponse.Extra) != 0 {
		// Use setCacheCopy to store a copy of the message
		setCacheCopy(newResponse.Ns[0].Header().Name, newResponse, newResponse.Ns[0].Header().Ttl)
		monitor.Rec53Log.Debug("set cache: ", newResponse.Ns[0].Header().Name, newResponse.Ns[0].Header().Ttl)
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

// implement stateMachine interface
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
	zoneList := utils.GetZoneList(request.Question[0].Name)
	for _, zone := range zoneList {
		// Use getCacheCopy to avoid modifying cached message
		if msgInCache, ok := getCacheCopy(zone); ok {
			monitor.Rec53Log.Debug("get cache: ", zone, " in inGlueCacheState")
			if len(msgInCache.Ns) != 0 && len(msgInCache.Extra) != 0 {
				s.response.Ns = append(s.response.Ns, msgInCache.Ns...)
				s.response.Extra = append(s.response.Extra, msgInCache.Extra...)
				return IN_GLUE_CACHE_HIT_CACHE, nil
			}
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

// implement stateMachine interface
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
