package server

import (
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
}

// Change executes the state machine until a final state is reached.
// It includes protection against infinite loops via iteration count and CNAME cycle detection.
func Change(stm stateMachine) (*dns.Msg, error) {
	// Track visited domains for CNAME cycle detection
	visitedDomains := make(map[string]bool)
	iterations := 0

	// Save original question for response
	originalQuestion := stm.getRequest().Question[0]

	for {
		iterations++
		if iterations > MaxIterations {
			monitor.Rec53Log.Errorf("Max iterations (%d) exceeded, possible CNAME loop", MaxIterations)
			return nil, fmt.Errorf("max iterations exceeded, possible CNAME loop")
		}

		st := stm.getCurrentState()
		switch st {
		case STATE_INIT:
			if _, err := stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				monitor.Rec53Log.Errorf("Handle state error %d %v", stm.getCurrentState(), err)
				return nil, fmt.Errorf("handle state error %d %v", stm.getCurrentState(), err)
			}
			inCache := newInCacheState(stm.getRequest(), stm.getResponse())
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
				checkResp := newCheckRespState(stm.getRequest(), stm.getResponse())
				stm = checkResp
			case IN_CACHE_MISS_CACHE:
				inGlue := newInGlueState(stm.getRequest(), stm.getResponse())
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
				stm = newRetRespState(stm.getRequest(), stm.getResponse())
			case CHECK_RESP_GET_CNAME:
				// Find the CNAME record in the answer
				var cnameTarget string
				for _, rr := range stm.getResponse().Answer {
					if cname, ok := rr.(*dns.CNAME); ok {
						cnameTarget = cname.Target
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
					stm.getRequest().Question[0].Name = cnameTarget
				}
				stm = newInCacheState(stm.getRequest(), stm.getResponse())
			case CHECK_RESP_GET_NS:
				stm = newInGlueState(stm.getRequest(), stm.getResponse())
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
				stm = newIterState(stm.getRequest(), stm.getResponse())
			case IN_GLUE_NOT_EXIST:
				stm = newInGlueCacheState(stm.getRequest(), stm.getResponse())
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
				stm = newIterState(stm.getRequest(), stm.getResponse())
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
				stm = newCheckRespState(stm.getRequest(), stm.getResponse())
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
			return resp, nil
		default:
			monitor.Rec53Log.Errorf("Wrong state %d", stm.getCurrentState())
			return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
		}
	}
}
