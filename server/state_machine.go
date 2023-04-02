package server

import (
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type stateMachine interface {
	getCurrentState() int
	getRequest() *dns.Msg
	getResponse() *dns.Msg
	handle(request *dns.Msg, response *dns.Msg) (int, error)
}

func Change(stm stateMachine) (*dns.Msg, error) {
	for {
		st := stm.getCurrentState()
		//fmt.Println("===================debug\n", stm.getRequest(), stm.getResponse())
		//fmt.Println("===================debug")
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
			case CHECK_RESP_COMMEN_ERROR:
				return stm.getResponse(), nil
			case CHECK_RESP_GET_ANS:
				stm = newRetRespState(stm.getRequest(), stm.getResponse())
			case CHECK_RESP_GET_CNAME:
				lastCname := stm.getResponse().Answer[len(stm.getResponse().Answer)-1]
				stm.getRequest().Question[0].Name = lastCname.(*dns.CNAME).Target
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
			case ITER_COMMEN_ERROR:
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
			return stm.getResponse(), nil
		default:
			monitor.Rec53Log.Errorf("Wrong state %d", stm.getCurrentState())
			return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
		}
	}
}
