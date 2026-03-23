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

func stateMetricName(state int) string {
	switch state {
	case STATE_INIT:
		return monitor.StateMachineStateInit
	case HOSTS_LOOKUP:
		return monitor.StateMachineHostsLookup
	case FORWARD_LOOKUP:
		return monitor.StateMachineForwardLookup
	case CACHE_LOOKUP:
		return monitor.StateMachineCacheLookup
	case CLASSIFY_RESP:
		return monitor.StateMachineClassifyResp
	case EXTRACT_GLUE:
		return monitor.StateMachineExtractGlue
	case LOOKUP_NS_CACHE:
		return monitor.StateMachineLookupNSCache
	case QUERY_UPSTREAM:
		return monitor.StateMachineQueryUpstream
	case RETURN_RESP:
		return monitor.StateMachineReturnResp
	default:
		return monitor.StateMachineUnknownState
	}
}

func recordStateTransition(from, to int) {
	if monitor.Rec53Metric == nil {
		return
	}
	monitor.Rec53Metric.StateMachineTransitionAdd(stateMetricName(from), stateMetricName(to))
}

func recordTraceState(ctx context.Context, state string) {
	if recorder := resolutionTraceFromContext(ctx); recorder != nil {
		recorder.recordState(state)
	}
}

func recordTerminalExit(ctx context.Context, from int, exit string) {
	if monitor.Rec53Metric == nil {
		recordTraceTerminal(ctx, exit)
		return
	}
	monitor.Rec53Metric.StateMachineTransitionAdd(stateMetricName(from), exit)
	recordTraceTerminal(ctx, exit)
}

func recordTraceTerminal(ctx context.Context, exit string) {
	if recorder := resolutionTraceFromContext(ctx); recorder != nil {
		recorder.recordTerminal(exit)
	}
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

// handleState invokes the current state's handle method and wraps any error with state context.
func handleState(stm stateMachine) (int, error) {
	ret, err := stm.handle(stm.getRequest(), stm.getResponse())
	if err != nil {
		return ret, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
	}
	return ret, nil
}

// followCNAME processes a CNAME record from the current response: detects cycles, appends to
// the chain, optionally clears stale NS/Extra delegation records, and updates the request
// Question to point at the CNAME target so the next loop iteration resolves it.
func followCNAME(stm stateMachine, cnameChain *[]dns.RR, visited map[string]bool) error {
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

	if cnameTarget == "" {
		return nil
	}

	// Check for CNAME cycle
	if visited[cnameTarget] {
		monitor.Rec53Log.Errorf("CNAME cycle detected: %s", cnameTarget)
		return fmt.Errorf("CNAME cycle detected: %s", cnameTarget)
	}
	visited[cnameTarget] = true

	// Per RFC1034 Section 3.6.2, preserve CNAME records in the chain
	// The CNAME record must be included in the final response
	if cnameRecord != nil {
		*cnameChain = append(*cnameChain, dns.Copy(cnameRecord))
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

	return nil
}

// buildFinalResponse restores the original question and prepends any accumulated CNAME chain
// records to the answer section, returning the response ready for the client.
func buildFinalResponse(stm stateMachine, origQ dns.Question, chain []dns.RR) *dns.Msg {
	resp := stm.getResponse()
	resp.Question[0] = origQ

	// Per RFC1034 Section 3.6.2: prepend CNAME chain to the answer section
	// The CNAME records must appear before the final records
	if len(chain) > 0 {
		resp.Answer = append(chain, resp.Answer...)
		monitor.Rec53Log.Debugf("Prepended %d CNAME records to answer section", len(chain))
	}

	return resp
}

// Change executes the state machine until a final state is reached.
// It includes protection against infinite loops via iteration count and CNAME cycle detection.
// Per RFC1034 Section 3.6.2, CNAME chains are preserved in the response.
func Change(stm stateMachine) (*dns.Msg, error) {
	// Track visited domains for CNAME cycle detection
	visitedDomains := make(map[string]bool)

	// Save original question for response (may be empty for malformed requests)
	var originalQuestion dns.Question
	if len(stm.getRequest().Question) > 0 {
		originalQuestion = stm.getRequest().Question[0]
	}

	// Accumulate CNAME records for RFC1034 compliant responses
	// The CNAME chain must be included in the Answer section
	var cnameChain []dns.RR

	for iterations := 1; iterations <= MaxIterations; iterations++ {
		st := stm.getCurrentState()
		recordTraceState(stm.getContext(), stateMetricName(st))
		if monitor.Rec53Metric != nil {
			monitor.Rec53Metric.StateMachineStageAdd(stateMetricName(st))
		}
		queryName := ""
		if len(stm.getRequest().Question) > 0 {
			queryName = stm.getRequest().Question[0].Name
		}
		monitor.Rec53Log.Debugf("[STATE_MACHINE] Iteration %d, current state: %d, query: %s",
			iterations, st, queryName)
		switch st {
		case STATE_INIT:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("state_init_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			if ret == STATE_INIT_FORMERR {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("formerr")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineFormerrExit)
				return stm.getResponse(), nil
			}
			recordStateTransition(st, HOSTS_LOOKUP)
			stm = newHostsLookupState(stm.getRequest(), stm.getResponse(), stm.getContext())
		case HOSTS_LOOKUP:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("hosts_lookup_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case HOSTS_LOOKUP_HIT:
				recordStateTransition(st, RETURN_RESP)
				stm = newReturnRespState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case HOSTS_LOOKUP_MISS:
				recordStateTransition(st, FORWARD_LOOKUP)
				stm = newForwardLookupState(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("hosts_lookup_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case FORWARD_LOOKUP:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("forward_lookup_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case FORWARD_LOOKUP_HIT:
				recordStateTransition(st, RETURN_RESP)
				stm = newReturnRespState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case FORWARD_LOOKUP_MISS:
				recordStateTransition(st, CACHE_LOOKUP)
				stm = newCacheLookupState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case FORWARD_LOOKUP_SERVFAIL:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("forward_lookup_servfail")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineServfailExit)
				msg := new(dns.Msg)
				msg.SetRcode(stm.getRequest(), dns.RcodeServerFailure)
				return msg, nil
			case FORWARD_LOOKUP_REFUSED:
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineRefusedExit)
				return buildRefusedResponse(stm.getRequest()), nil
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("forward_lookup_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case CACHE_LOOKUP:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("cache_lookup_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case CACHE_LOOKUP_HIT:
				recordStateTransition(st, CLASSIFY_RESP)
				stm = newClassifyRespState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case CACHE_LOOKUP_MISS:
				if !tryAcquireExpensiveRequest(stm.getContext(), expensivePathIterative) {
					recordTerminalExit(stm.getContext(), st, monitor.StateMachineRefusedExit)
					return buildRefusedResponse(stm.getRequest()), nil
				}
				recordStateTransition(st, EXTRACT_GLUE)
				stm = newExtractGlueState(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("cache_lookup_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case CLASSIFY_RESP:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("classify_resp_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case CLASSIFY_RESP_COMMON_ERROR:
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				return stm.getResponse(), nil
			case CLASSIFY_RESP_GET_ANS, CLASSIFY_RESP_GET_NEGATIVE:
				// Negative response (NXDOMAIN/NODATA) - return directly to client
				recordStateTransition(st, RETURN_RESP)
				stm = newReturnRespState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case CLASSIFY_RESP_GET_CNAME:
				if err := followCNAME(stm, &cnameChain, visitedDomains); err != nil {
					if monitor.Rec53Metric != nil {
						monitor.Rec53Metric.StateMachineFailureAdd("cname_cycle")
					}
					recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
					return nil, err
				}
				recordStateTransition(st, CACHE_LOOKUP)
				stm = newCacheLookupState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case CLASSIFY_RESP_GET_NS:
				recordStateTransition(st, EXTRACT_GLUE)
				stm = newExtractGlueState(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("classify_resp_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case EXTRACT_GLUE:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("extract_glue_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case EXTRACT_GLUE_EXIST:
				recordStateTransition(st, QUERY_UPSTREAM)
				stm = newQueryUpstreamState(stm.getRequest(), stm.getResponse(), stm.getContext())
			case EXTRACT_GLUE_NOT_EXIST:
				recordStateTransition(st, LOOKUP_NS_CACHE)
				stm = newLookupNSCacheState(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("extract_glue_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case LOOKUP_NS_CACHE:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("lookup_ns_cache_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case LOOKUP_NS_CACHE_HIT,
				LOOKUP_NS_CACHE_MISS:
				recordStateTransition(st, QUERY_UPSTREAM)
				stm = newQueryUpstreamState(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("lookup_ns_cache_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case QUERY_UPSTREAM:
			ret, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("query_upstream_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			switch ret {
			case QUERY_UPSTREAM_COMMON_ERROR:
				// return servfail response
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("query_upstream_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineServfailExit)
				msg := new(dns.Msg)
				msg.SetRcode(stm.getRequest(), dns.RcodeServerFailure)
				return msg, nil
			case QUERY_UPSTREAM_NO_ERROR:
				recordStateTransition(st, CLASSIFY_RESP)
				stm = newClassifyRespState(stm.getRequest(), stm.getResponse(), stm.getContext())
			default:
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("query_upstream_wrong_state")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("Wrong state %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("wrong state %d %v", stm.getCurrentState(), err)
			}
		case RETURN_RESP:
			_, err := handleState(stm)
			if err != nil {
				if monitor.Rec53Metric != nil {
					monitor.Rec53Metric.StateMachineFailureAdd("return_resp_handle_error")
				}
				recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
				monitor.Rec53Log.Errorf("%v", err)
				return nil, err
			}
			recordTerminalExit(stm.getContext(), st, monitor.StateMachineSuccessExit)
			return buildFinalResponse(stm, originalQuestion, cnameChain), nil
		default:
			if monitor.Rec53Metric != nil {
				monitor.Rec53Metric.StateMachineFailureAdd("unknown_state")
			}
			recordTerminalExit(stm.getContext(), st, monitor.StateMachineErrorExit)
			monitor.Rec53Log.Errorf("Wrong state %d", stm.getCurrentState())
			return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
		}
	}

	if monitor.Rec53Metric != nil {
		monitor.Rec53Metric.StateMachineFailureAdd("max_iterations")
	}
	recordTerminalExit(stm.getContext(), stm.getCurrentState(), monitor.StateMachineMaxItersExit)
	monitor.Rec53Log.Errorf("Max iterations (%d) exceeded, possible CNAME loop", MaxIterations)
	return nil, fmt.Errorf("max iterations exceeded, possible CNAME loop")
}
