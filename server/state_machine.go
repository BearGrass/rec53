package server

import (
	"fmt"

	"rec53/logger"

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
		switch stm.getCurrentState() {
		case STATE_INIT:
			if _, err := stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				logger.Rec53Log.Sugar().Errorf("Handle state error %d", stm.getCurrentState())
				return nil, fmt.Errorf("handle state error %d", stm.getCurrentState())
			}
			inCache := newInCacheState(stm.getRequest(), stm.getResponse())
			stm = inCache
		case IN_CACHE:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				logger.Rec53Log.Sugar().Errorf("Handle state error %d", stm.getCurrentState())
				return nil, fmt.Errorf("handle state error %d", stm.getCurrentState())
			}
			switch ret {
			case IN_CACHE_HIT_CACHE:
				checkResp := newCheckRespState(stm.getRequest(), stm.getResponse())
				stm = checkResp
			case IN_CACHE_MISS_CACHE:
				inGlue := newInGlueState(stm.getRequest(), stm.getResponse())
				stm = inGlue
			default:
				logger.Rec53Log.Sugar().Errorf("Wrong state %d", stm.getCurrentState())
				return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
			}
		case CHECK_RESP:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				logger.Rec53Log.Sugar().Errorf("Handle state error %d", stm.getCurrentState())
				return nil, fmt.Errorf("handle state error %d", stm.getCurrentState())
			}
			switch ret {
			case CHECK_RESP_COMMEN_ERROR:
				return stm.getResponse(), nil
			case CHECK_RESP_GET_ANS:
				//TODO: new a state to handle get answer
			case CHECK_RESP_GET_CNAME:
				//TODO: new a state to handle get cname
			case CHECK_RESP_GET_NS:
				//TODO: new a state to handle get ns
			default:
				logger.Rec53Log.Sugar().Errorf("Wrong state %d", stm.getCurrentState())
				return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
			}
		case IN_GLUE:
			var (
				ret int
				err error
			)
			if ret, err = stm.handle(stm.getRequest(), stm.getResponse()); err != nil {
				logger.Rec53Log.Sugar().Errorf("Handle state error %d", stm.getCurrentState())
				return nil, fmt.Errorf("handle state error %d", stm.getCurrentState())
			}
			switch ret {
			case IN_GLUE_EXIST:
				//TODO: new a state to handle exist glue
			case IN_GLUE_NOT_EXIST:
				//TODO: new a state to handle not exist glue
			default:
				logger.Rec53Log.Sugar().Errorf("Wrong state %d", stm.getCurrentState())
				return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
			}
		default:
			logger.Rec53Log.Sugar().Errorf("Wrong state %d", stm.getCurrentState())
			return nil, fmt.Errorf("wrong state %d", stm.getCurrentState())
		}
	}

}
