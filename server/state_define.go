package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"rec53/monitor"
	"rec53/utils"

	"github.com/miekg/dns"
)

// contextKeyType is the type for context keys
type contextKeyType string

// contextKeyWarmupDeadline is the context key for storing the warmup deadline
const contextKeyWarmupDeadline contextKeyType = "warmupDeadline"

// contextKeyNSResolutionDepth tracks recursive NS resolution depth to prevent deadlock.
// When resolveNSIPsConcurrently is resolving NS names, this key is set so that
// nested iterState.handle calls know not to recursively resolve NS names again.
const contextKeyNSResolutionDepth contextKeyType = "nsResolutionDepth"

// DefaultNegativeCacheTTL is the default TTL for negative responses (NXDOMAIN/NODATA)
// when SOA minimum is not available or is zero.
// TODO: make this configurable via config file or command-line flag
const DefaultNegativeCacheTTL = 60

// extractSOAFromAuthority extracts the SOA record from the Authority section.
// Returns the SOA record and its TTL (or DefaultNegativeCacheTTL if SOA.Minttl is 0).
// Returns nil, 0 if no SOA is found.
func extractSOAFromAuthority(response *dns.Msg) (*dns.SOA, uint32) {
	for _, rr := range response.Ns {
		if soa, ok := rr.(*dns.SOA); ok {
			ttl := soa.Minttl
			if ttl == 0 {
				ttl = DefaultNegativeCacheTTL
			}
			return soa, ttl
		}
	}
	return nil, 0
}

// hasSOAInAuthority checks if the Authority section contains a SOA record.
// This is used to identify negative responses (NXDOMAIN/NODATA).
func hasSOAInAuthority(response *dns.Msg) bool {
	for _, rr := range response.Ns {
		if _, ok := rr.(*dns.SOA); ok {
			return true
		}
	}
	return false
}

type stateInitState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newStateInitState(req, resp *dns.Msg) *stateInitState {
	return &stateInitState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newStateInitStateWithContext creates a stateInitState with a specific context
func newStateInitStateWithContext(req, resp *dns.Msg, ctx context.Context) *stateInitState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &stateInitState{
		request:  req,
		response: resp,
		ctx:      ctx,
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

func (s *stateInitState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *stateInitState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return STATE_INIT_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	// RFC 1035 Section 4.1.1: validate basic message fields
	if len(request.Question) != 1 || request.Response || request.Opcode != dns.OpcodeQuery {
		monitor.Rec53Log.Debugf("[STATE_INIT] FORMERR: qdcount=%d qr=%v opcode=%d",
			len(request.Question), request.Response, request.Opcode)
		response.SetRcode(request, dns.RcodeFormatError)
		return STATE_INIT_FORMERR, nil
	}
	response.SetReply(request)
	s.request = request
	return STATE_INIT_NO_ERROR, nil
}

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

type classifyRespState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newClassifyRespState(req, resp *dns.Msg) *classifyRespState {
	return &classifyRespState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newClassifyRespStateWithContext creates a classifyRespState with a specific context
func newClassifyRespStateWithContext(req, resp *dns.Msg, ctx context.Context) *classifyRespState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &classifyRespState{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
}

// implement stateMachine interface
func (s *classifyRespState) getCurrentState() int {
	return CLASSIFY_RESP
}

func (s *classifyRespState) getRequest() *dns.Msg {
	return s.request
}

func (s *classifyRespState) getResponse() *dns.Msg {
	return s.response
}

func (s *classifyRespState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
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

// iterPortOverride allows tests to inject a custom port for upstream queries.
// When non-empty, iterState uses this port instead of the default "53".
var iterPortOverride string

// SetIterPort overrides the port used by iterState for upstream DNS queries.
// This is intended for testing with mock servers on non-standard ports.
func SetIterPort(port string) {
	iterPortOverride = port
}

// ResetIterPort clears the port override so iterState uses port 53 again.
func ResetIterPort() {
	iterPortOverride = ""
}

// getIterPort returns the port to use for upstream DNS queries.
func getIterPort() string {
	if iterPortOverride != "" {
		return iterPortOverride
	}
	return "53"
}

type queryUpstreamState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newQueryUpstreamState(req, resp *dns.Msg) *queryUpstreamState {
	return &queryUpstreamState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newQueryUpstreamStateWithContext creates a queryUpstreamState with a specific context
func newQueryUpstreamStateWithContext(req, resp *dns.Msg, ctx context.Context) *queryUpstreamState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &queryUpstreamState{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
}

// implement stateMachine interface
func (s *queryUpstreamState) getCurrentState() int {
	return QUERY_UPSTREAM
}

func (s *queryUpstreamState) getRequest() *dns.Msg {
	return s.request
}

func (s *queryUpstreamState) getResponse() *dns.Msg {
	return s.response
}

func (s *queryUpstreamState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
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

// getNSNamesFromResponse extracts NS domain names from the Ns section
func getNSNamesFromResponse(response *dns.Msg) []string {
	var nsNames []string
	for _, ns := range response.Ns {
		if nsRR, ok := ns.(*dns.NS); ok {
			nsNames = append(nsNames, nsRR.Ns)
		}
	}
	return nsNames
}

// resolveNSIPs attempts to resolve IP addresses for NS names from cache
func resolveNSIPs(nsNames []string) []string {
	var ipList []string
	for _, nsName := range nsNames {
		// Try to get A record from cache
		if msgInCache, ok := getCacheCopyByType(nsName, dns.TypeA); ok {
			for _, ans := range msgInCache.Answer {
				if a, ok := ans.(*dns.A); ok {
					ipList = append(ipList, a.A.String())
				}
			}
		}
	}
	return ipList
}

// nsResult holds the result of resolving a single NS name
type nsResult struct {
	nsName string
	ips    []string
}

// resolveNSIPsRecursively resolves NS names using the state machine recursively.
// This is the correct approach for a recursive resolver - we use the same
// resolution mechanism to resolve NS names as we do for any other query.
// The provided context allows warmup deadlines to propagate to nested resolutions.
func resolveNSIPsRecursively(ctx context.Context, nsNames []string) []string {
	return resolveNSIPsConcurrently(ctx, nsNames)
}

// resolveNSIPsConcurrently resolves multiple NS names in parallel with a configurable
// concurrency limit (default 5). Returns the first successful response immediately
// while background goroutine updates cache for remaining IPs.
// The provided context allows warmup deadlines to propagate and cancel all nested goroutines.
const maxConcurrentNSQueries = 5

func resolveNSIPsConcurrently(ctx context.Context, nsNames []string) []string {
	if len(nsNames) == 0 {
		return nil
	}

	// Detect recursive NS resolution to prevent deadlock (B2 fix).
	// When iterState.handle resolves NS names recursively, it calls resolveNSIPsConcurrently.
	// If the nested state machine then encounters another NS without glue, it would call
	// resolveNSIPsConcurrently again — but the semaphore slots are already held by the outer
	// layer, causing a deadlock. We break the cycle by marking depth in context.
	currentDepth := 0
	if d, ok := ctx.Value(contextKeyNSResolutionDepth).(int); ok {
		currentDepth = d
	}
	if currentDepth > 0 {
		// Already inside NS resolution — do not recurse further to avoid deadlock.
		monitor.Rec53Log.Debugf("[ITER] resolveNSIPsConcurrently: skipping recursive NS resolution (depth=%d)", currentDepth)
		return nil
	}
	ctx = context.WithValue(ctx, contextKeyNSResolutionDepth, currentDepth+1)
	monitor.Rec53Log.Debugf("[ITER] resolveNSIPsConcurrently: starting NS resolution (depth=%d, names=%v)", currentDepth+1, nsNames)

	// Use context with cancellation on first successful response
	// If ctx has a deadline (e.g., warmup timeout), it will be preserved
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to collect successful IPs
	resultChan := make(chan nsResult, len(nsNames))
	var wg sync.WaitGroup

	// Limit concurrency with semaphore pattern
	// NOTE: do NOT close(semaphore) here. The channel is GC'd when all goroutines
	// have released it. Closing while goroutines may still be sending causes panic.
	semaphore := make(chan struct{}, maxConcurrentNSQueries)

	// Launch goroutines for each NS name
	for _, nsName := range nsNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Acquire semaphore slot — must be context-aware so that a cancelled
			// context (deadline or first-result cancel) doesn't leave goroutines
			// blocked here indefinitely, which would prevent wg.Wait() from returning.
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				monitor.Rec53Log.Debugf("[ITER] Concurrent NS resolution cancelled for %s (deadline or first response)", name)
				return
			}
			defer func() { <-semaphore }()

			// Create a new query for NS A record
			req := new(dns.Msg)
			req.SetQuestion(name, dns.TypeA)
			req.RecursionDesired = false
			resp := new(dns.Msg)

			// Use the state machine to resolve the NS name
			// Pass context through to nested resolutions
			stm := newStateInitStateWithContext(req, resp, ctx)
			result, err := Change(stm)
			if err != nil {
				monitor.Rec53Log.Debugf("[ITER] Failed to resolve NS %s: %v", name, err)
				return
			}

			// Extract IP addresses from the result
			var ips []string
			for _, ans := range result.Answer {
				if a, ok := ans.(*dns.A); ok {
					ips = append(ips, a.A.String())
				}
			}

			if len(ips) > 0 {
				monitor.Rec53Log.Debugf("[ITER] Resolved NS %s to IPs: %v", name, ips)
				// Send result (non-blocking)
				select {
				case resultChan <- nsResult{nsName: name, ips: ips}:
				case <-ctx.Done():
					// Context cancelled, don't block
					return
				}
			}
		}(nsName)
	}

	// Close resultChan after all workers finish.
	// This is the only place resultChan is closed, ensuring the range below terminates.
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Single consumer: collect all results that arrive before the channel closes.
	// Calling cancel() after the first result signals remaining workers to exit early,
	// which causes them to either skip sending or return without sending to resultChan.
	// Workers that already sent before cancel() is called may still appear in allResults.
	var allResults []nsResult
	for result := range resultChan {
		allResults = append(allResults, result)
		if len(allResults) == 1 {
			// Got the first usable IP set — cancel all remaining NS queries.
			monitor.Rec53Log.Debugf("[ITER] First NS resolved: %s -> %v", result.nsName, result.ips)
			cancel()
		}
	}

	if len(allResults) == 0 {
		return nil
	}

	// Background-update cache with any additional results that arrived after the first.
	// fire-and-forget: cache writes are idempotent and safe to skip on shutdown.
	if len(allResults) > 1 {
		go updateNSIPsCache(allResults[1:])
	}

	return allResults[0].ips
}

// updateNSIPsCache is a helper function that caches resolved NS IPs in the background.
// Called after first response is returned to avoid blocking the main query path.
func updateNSIPsCache(results []nsResult) {
	for _, result := range results {
		// Create cache entry for this NS
		cacheMsg := new(dns.Msg)
		cacheMsg.SetQuestion(result.nsName, dns.TypeA)
		cacheMsg.Response = true

		// Add A records to Answer section
		for _, ip := range result.ips {
			a := new(dns.A)
			a.Hdr = dns.RR_Header{
				Name:   result.nsName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300, // Standard 5-minute TTL for NS IPs
			}
			a.A = net.ParseIP(ip)
			cacheMsg.Answer = append(cacheMsg.Answer, a)
		}

		// Store in cache with 5-minute TTL
		setCacheCopyByType(result.nsName, dns.TypeA, cacheMsg, 300)
		monitor.Rec53Log.Debugf("[ITER] Updated NS IP cache for %s: %v", result.nsName, result.ips)
	}
}

func getBestAddressAndPrefetchIPs(ipList []string) (string, string, error) {
	if len(ipList) == 0 {
		return "", "", fmt.Errorf("no ip in extra")
	}
	bestIP, backupIP := globalIPPool.GetBestIPsV2(ipList)
	if bestIP != "" {
		IPs := globalIPPool.GetPrefetchIPs(bestIP)
		globalIPPool.PrefetchIPs(IPs)
	}
	return bestIP, backupIP, nil
}

func (s *queryUpstreamState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}

	// Check context before doing any work — exit early if already cancelled
	if err := s.ctx.Err(); err != nil {
		monitor.Rec53Log.Debugf("[ITER] Context cancelled before query for %s: %v", request.Question[0].Name, err)
		return QUERY_UPSTREAM_COMMON_ERROR, err
	}

	monitor.Rec53Log.Debugf("[ITER] Querying: %s (type: %s)", request.Question[0].Name, dns.TypeToString[request.Question[0].Qtype])

	newQuery := new(dns.Msg)
	newQuery.SetQuestion(request.Question[0].Name, request.Question[0].Qtype)
	newQuery.RecursionDesired = false
	newQuery.Id = dns.Id()
	// Set EDNS0 with larger buffer size to handle larger responses
	newQuery.SetEdns0(4096, false)

	dnsClient := &dns.Client{
		Net:            "udp",
		Timeout:        5 * time.Second,
		SingleInflight: true,
		UDPSize:        4096, // Set larger UDP buffer size for EDNS
	}

	//check the best ip in the extra in response
	ipList := getIPListFromResponse(response)
	monitor.Rec53Log.Debugf("[ITER] IP list from Extra: %v", ipList)

	// If no IP from Extra (no glue records), try to resolve NS names
	if len(ipList) == 0 && len(response.Ns) > 0 {
		nsNames := getNSNamesFromResponse(response)
		monitor.Rec53Log.Debugf("[ITER] No glue records, trying to resolve NS names: %v", nsNames)

		// First, try to get NS IPs from cache
		ipList = resolveNSIPs(nsNames)
		monitor.Rec53Log.Debugf("[ITER] Resolved IPs from cache: %v", ipList)

		// If still no IPs, resolve NS names using recursive state machine
		if len(ipList) == 0 {
			monitor.Rec53Log.Debugf("[ITER] Resolving NS names recursively: %v", nsNames)
			ipList = resolveNSIPsRecursively(s.ctx, nsNames)
			monitor.Rec53Log.Debugf("[ITER] Resolved NS IPs: %v", ipList)
		}
	}

	bestAddr, secondAddr, err := getBestAddressAndPrefetchIPs(ipList)
	if bestAddr == "" || err != nil {
		return QUERY_UPSTREAM_COMMON_ERROR, err
	}

	//send query to the best ip
	theBestIP := bestAddr
	monitor.Rec53Metric.InCounterAdd("forward_request", newQuery.Question[0].Name, dns.TypeToString[newQuery.Question[0].Qtype])
	port := getIterPort()
	monitor.Rec53Log.Debugf("[ITER] Sending query to %s:%s for %s", bestAddr, port, request.Question[0].Name)
	newResponse, rtt, err := dnsClient.ExchangeContext(s.ctx, newQuery, bestAddr+":"+port)
	if err != nil {
		monitor.Rec53Log.Debugf("[ITER] Query to %s failed: %v", bestAddr, err)
		// Record failure in V2 only (V1 deprecated)
		iqv2 := globalIPPool.GetIPQualityV2(bestAddr)
		if iqv2 != nil {
			iqv2.RecordFailure()
		}
		//try to use the second ip
		if secondAddr == "" {
			monitor.Rec53Log.Debugf("[ITER] No second IP available, returning error")
			return QUERY_UPSTREAM_COMMON_ERROR, err
		}
		monitor.Rec53Log.Debugf("[ITER] Retrying with second IP: %s", secondAddr)
		newResponse, rtt, err = dnsClient.ExchangeContext(s.ctx, newQuery, secondAddr+":"+port)
		if err != nil {
			monitor.Rec53Log.Debugf("[ITER] Query to second IP %s also failed: %v", secondAddr, err)
			// Record failure in V2 only (V1 deprecated)
			iqv2 := globalIPPool.GetIPQualityV2(secondAddr)
			if iqv2 != nil {
				iqv2.RecordFailure()
			}
			return QUERY_UPSTREAM_COMMON_ERROR, err
		}
		theBestIP = secondAddr
	}

	// Record latency and metrics using V2 only (V1 deprecated)
	iqv2 := globalIPPool.GetIPQualityV2(theBestIP)
	if iqv2 != nil {
		iqv2.RecordLatency(int32(rtt / time.Millisecond))
		// Export V2 percentile metrics to Prometheus
		monitor.Rec53Metric.IPQualityV2GaugeSet(theBestIP,
			float64(iqv2.GetP50Latency()),
			float64(iqv2.GetP95Latency()),
			float64(iqv2.GetP99Latency()),
		)
	}

	monitor.Rec53Metric.OutCounterAdd("forward_response", newQuery.Question[0].Name, dns.TypeToString[newQuery.Question[0].Qtype], dns.RcodeToString[newResponse.Rcode])

	monitor.Rec53Log.Debugf("[ITER] Response from %s: Rcode=%s, Answers=%d, Ns=%d, Extra=%d",
		theBestIP, dns.RcodeToString[newResponse.Rcode], len(newResponse.Answer), len(newResponse.Ns), len(newResponse.Extra))

	//check the response
	if newResponse.Rcode != dns.RcodeSuccess {
		// Copy response code and authority section
		s.response.Rcode = newResponse.Rcode
		s.response.Ns = newResponse.Ns

		// Handle different response codes appropriately
		switch newResponse.Rcode {
		case dns.RcodeNameError: // NXDOMAIN - domain does not exist
			monitor.Rec53Log.Debugf("[ITER] NXDOMAIN received for %s", request.Question[0].Name)
			// Return normally with NXDOMAIN code preserved
			return QUERY_UPSTREAM_NO_ERROR, nil
		case dns.RcodeSuccess:
			return QUERY_UPSTREAM_NO_ERROR, nil
		case dns.RcodeServerFailure, dns.RcodeRefused, dns.RcodeFormatError, dns.RcodeNotImplemented:
			// B-013: Bad Rcodes (SERVFAIL, REFUSED, FORMERR, NOTIMPL) should trigger server switch
			monitor.Rec53Log.Debugf("[ITER] Bad Rcode %s from %s, marking as failed and retrying with secondary IP",
				dns.RcodeToString[newResponse.Rcode], theBestIP)

			// Record failure in IP quality tracking
			iqv2 := globalIPPool.GetIPQualityV2(theBestIP)
			if iqv2 != nil {
				iqv2.RecordFailure()
				monitor.Rec53Log.Debugf("[ITER] Recorded failure for IP %s", theBestIP)
			}

			// Try secondary IP if available
			if secondAddr == "" {
				monitor.Rec53Log.Debugf("[ITER] No secondary IP available for retry, returning bad Rcode")
				return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("bad response rcode: %s, no secondary IP",
					dns.RcodeToString[newResponse.Rcode])
			}

			monitor.Rec53Log.Debugf("[ITER] Retrying with secondary IP %s for bad Rcode %s",
				secondAddr, dns.RcodeToString[newResponse.Rcode])

			// Retry query with secondary IP
			newResponse, rtt, err = dnsClient.ExchangeContext(s.ctx, newQuery, secondAddr+":"+port)
			if err != nil {
				monitor.Rec53Log.Debugf("[ITER] Query to secondary IP %s failed: %v", secondAddr, err)
				// Record failure for secondary IP
				iqv2 = globalIPPool.GetIPQualityV2(secondAddr)
				if iqv2 != nil {
					iqv2.RecordFailure()
				}
				return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("bad response rcode: %s, secondary IP also failed: %v",
					dns.RcodeToString[newResponse.Rcode], err)
			}

			// Secondary IP succeeded, update tracking
			theBestIP = secondAddr
			monitor.Rec53Log.Debugf("[ITER] Secondary IP %s succeeded after bad Rcode from primary", secondAddr)

			// Record latency for secondary IP
			iqv2 = globalIPPool.GetIPQualityV2(theBestIP)
			if iqv2 != nil {
				iqv2.RecordLatency(int32(rtt / time.Millisecond))
				// Export V2 percentile metrics to Prometheus
				monitor.Rec53Metric.IPQualityV2GaugeSet(theBestIP,
					float64(iqv2.GetP50Latency()),
					float64(iqv2.GetP95Latency()),
					float64(iqv2.GetP99Latency()),
				)
			}

			// After successful secondary IP retry, check its response code
			s.response.Rcode = newResponse.Rcode
			s.response.Ns = newResponse.Ns

			// If secondary IP also returned bad Rcode, give up
			if newResponse.Rcode != dns.RcodeSuccess {
				if newResponse.Rcode == dns.RcodeNameError {
					monitor.Rec53Log.Debugf("[ITER] Secondary IP returned NXDOMAIN")
					return QUERY_UPSTREAM_NO_ERROR, nil
				}
				monitor.Rec53Log.Debugf("[ITER] Secondary IP also returned bad Rcode: %s",
					dns.RcodeToString[newResponse.Rcode])
				return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("both primary and secondary IPs returned bad rcode: %s",
					dns.RcodeToString[newResponse.Rcode])
			}
			// Secondary IP succeeded, continue to process response
		default:
			// Other unknown errors - return as error
			monitor.Rec53Log.Debugf("[ITER] Non-success Rcode: %s", dns.RcodeToString[newResponse.Rcode])
			return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("response rcode: %s",
				dns.RcodeToString[newResponse.Rcode])
		}
	}
	//check the response is the same as the request
	if len(newResponse.Question) == 0 {
		monitor.Rec53Log.Debugf("[ITER] Response has no question section")
		return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("response has no question")
	}
	if newResponse.Question[0].Name != request.Question[0].Name {
		monitor.Rec53Log.Debugf("[ITER] Question mismatch: response=%s, request=%s", newResponse.Question[0].Name, request.Question[0].Name)
		return QUERY_UPSTREAM_COMMON_ERROR, fmt.Errorf("response.Question is not the same as request")
	}
	monitor.Rec53Log.Debugf("[ITER] Response validated, Answers: %d, Ns: %d, Extra: %d",
		len(newResponse.Answer), len(newResponse.Ns), len(newResponse.Extra))
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
	monitor.Rec53Log.Debugf("[ITER] State complete, total answers: %d", len(s.response.Answer))
	return QUERY_UPSTREAM_NO_ERROR, nil
}

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

type returnRespState struct {
	request  *dns.Msg
	response *dns.Msg
	ctx      context.Context
}

func newReturnRespState(req, resp *dns.Msg) *returnRespState {
	return &returnRespState{
		request:  req,
		response: resp,
		ctx:      context.Background(),
	}
}

// newReturnRespStateWithContext creates a returnRespState with a specific context
func newReturnRespStateWithContext(req, resp *dns.Msg, ctx context.Context) *returnRespState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &returnRespState{
		request:  req,
		response: resp,
		ctx:      ctx,
	}
}

// implement stateMachine interface
func (s *returnRespState) getCurrentState() int {
	return RETURN_RESP
}

func (s *returnRespState) getRequest() *dns.Msg {
	return s.request
}

func (s *returnRespState) getResponse() *dns.Msg {
	return s.response
}

func (s *returnRespState) getContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *returnRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return RETURN_RESP_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	return RETURN_RESP_NO_ERROR, nil
}
