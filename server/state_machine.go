package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

const (
	// MaxIterations limits the number of state machine iterations to prevent infinite loops
	MaxIterations = 50
)

type stateMachine interface {
	getCurrentState() int
	getRequest() *dns.Msg
	getResponse() *dns.Msg
	handle(request *dns.Msg, response *dns.Msg) (int, error)
	getContext() context.Context
}

// isNSRelevantForCNAME checks if NS records are relevant for resolving a CNAME target.
// NS records are relevant if their zone matches or is a parent of the CNAME target.
// This enables smart preservation of valid delegation info when following CNAME chains.
func isNSRelevantForCNAME(nsRecords []dns.RR, cnameTarget string) bool {
	if len(nsRecords) == 0 {
		return false
	}

	// Get the zone name from the first NS record
	nsZone := nsRecords[0].Header().Name

	// Check if CNAME target is a subdomain of the NS zone
	// e.g., nsZone="akadns.net.", cnameTarget="www.huawei.com.akadns.net." -> true
	// e.g., nsZone="zone1.com.", cnameTarget="target.zone2.com." -> false
	return dns.IsSubDomain(nsZone, cnameTarget)
}

// Change executes the state machine until a final state is reached.
// It includes protection against infinite loops via iteration count and CNAME cycle detection.
// Per RFC1034 Section 3.6.2, CNAME chains are preserved in the response.
func Change(stm stateMachine) (*dns.Msg, error) {
	// Track visited domains for CNAME cycle detection
	visitedDomains := make(map[string]bool)
	iterations := 0

	// Save original question for response (may be empty for malformed requests)
	var originalQuestion dns.Question
	if len(stm.getRequest().Question) > 0 {
		originalQuestion = stm.getRequest().Question[0]
	}

	// Accumulate CNAME records for RFC1034 compliant responses
	// The CNAME chain must be included in the Answer section
	var cnameChain []dns.RR

	for {
		iterations++
		if iterations > MaxIterations {
			monitor.Rec53Log.Errorf("Max iterations (%d) exceeded, possible CNAME loop", MaxIterations)
			return nil, fmt.Errorf("max iterations exceeded, possible CNAME loop")
		}

		st := stm.getCurrentState()
		queryName := ""
		if len(stm.getRequest().Question) > 0 {
			queryName = stm.getRequest().Question[0].Name
		}
		monitor.Rec53Log.Debugf("[STATE_MACHINE] Iteration %d, current state: %d, query: %s",
			iterations, st, queryName)
		switch st {
		case STATE_INIT:
			ret, err := stm.handle(stm.getRequest(), stm.getResponse())
			if err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			if ret == STATE_INIT_FORMERR {
				return stm.getResponse(), nil
			}
			inCache := newInCacheStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			stm = inCache
		case IN_CACHE:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			switch ret {
			case IN_CACHE_HIT_CACHE:
				checkResp := newCheckRespStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
				stm = checkResp
			case IN_CACHE_MISS_CACHE:
				inGlue := newInGlueStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
				stm = inGlue
			default:
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case CHECK_RESP:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			switch ret {
			case CHECK_RESP_COMMON_ERROR:
				return stm.getResponse(), nil
			case CHECK_RESP_GET_ANS:
				stm = newRetRespStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			case CHECK_RESP_GET_NEGATIVE:
				// Negative response (NXDOMAIN/NODATA) - return directly to client
				stm = newRetRespStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			case CHECK_RESP_GET_CNAME:
				// Find the CNAME record in the answer
				var cnameTarget string
				var cnameRecord *dns.CNAME
				for _, rr := range stm.getResponse().Answer {
					if cname, ok := rr.(*dns.CNAME); ok {
						cnameTarget = cname.Target
						cnameRecord = cname
						break
					}
				}
				if cnameTarget != "" {
					// Check for CNAME cycle
					if visitedDomains[cnameTarget] {
						monitor.Rec53Log.Errorf("CNAME cycle detected: %s", cnameTarget)
						return nil, fmt.Errorf("CNAME cycle detected: %s", cnameTarget)
					}
					visitedDomains[cnameTarget] = true

					// Per RFC1034 Section 3.6.2, preserve CNAME records in the chain
					// The CNAME record must be included in the final response
					if cnameRecord != nil {
						cnameChain = append(cnameChain, dns.Copy(cnameRecord))
						monitor.Rec53Log.Debugf("CNAME chain: added %s -> %s", cnameRecord.Hdr.Name, cnameRecord.Target)
					}

					// Smart NS/Extra handling for CNAME following (B-004):
					// Only clear delegation records if they are NOT relevant to the CNAME target.
					// This preserves valid NS delegation from upstream when the NS zone matches
					// or is a parent of the CNAME target's zone.
					if !isNSRelevantForCNAME(stm.getResponse().Ns, cnameTarget) {
						monitor.Rec53Log.Debugf("Clearing stale NS/Extra for CNAME target %s (NS zone mismatch)", cnameTarget)
						stm.getResponse().Ns = nil
						stm.getResponse().Extra = nil
					} else {
						monitor.Rec53Log.Debugf("Preserving NS/Extra for CNAME target %s (NS zone matches)", cnameTarget)
					}
					// Clear the Answer section before following CNAME to avoid stale records
					// being misinterpreted as new responses. CNAME records are preserved in cnameChain.
					stm.getResponse().Answer = nil
					stm.getRequest().Question[0].Name = cnameTarget
				}
				stm = newInCacheStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			case CHECK_RESP_GET_NS:
				stm = newInGlueStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case IN_GLUE:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			switch ret {
			case IN_GLUE_EXIST:
				stm = newIterStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			case IN_GLUE_NOT_EXIST:
				stm = newInGlueCacheStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case IN_GLUE_CACHE:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			switch ret {
			case IN_GLUE_CACHE_HIT_CACHE,
				IN_GLUE_CACHE_MISS_CACHE:
				stm = newIterStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case ITER:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			switch ret {
			case ITER_COMMON_ERROR:
				//return servfail response
				msg := new(dns.Msg)
				msg.SetRcode(stm.getRequest(), dns.RcodeServerFailure)
				return msg, nil
			case ITER_NO_ERROR:
				stm = newCheckRespStateWithContext(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case RET_RESP:
			var (
				err error
			)
			if _, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			// Restore original question before returning response
			resp := stm.getResponse()
			resp.Question[0] = originalQuestion

			// Per RFC1034 Section 3.6.2: prepend CNAME chain to the answer section
			// The CNAME records must appear before the final records
			if len(cnameChain) > 0 {
				resp.Answer = append(cnameChain, resp.Answer...)
				monitor.Rec53Log.Debugf("Prepended %d CNAME records to answer section", len(cnameChain))
			}

			return resp, nil
		default:
			monitor.Rec53Log.Errorf("Wrong state %d", stm.getCurrentState())
			return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
		}
	}
}
