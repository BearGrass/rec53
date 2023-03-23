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
				//TODO: new a state to handle cache hit
			case IN_CACHE_MISS_CACHE:
				//TODO: new a state to handle cache miss
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
