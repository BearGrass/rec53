package server

import (
	"rec53/logger"

	"github.com/miekg/dns"
)

type stateMachine interface {
	getCurrentState() int
	getRequest() *dns.Msg
	getResponse() *dns.Msg
	handle(request *dns.Msg, response *dns.Msg)
}

func Change(stm stateMachine) {
	for {
		switch stm.getCurrentState() {
		case STATE_INIT:
			req := stm.getRequest()
			resp := new(dns.Msg)
			resp.SetReply(req)
			stm.handle(req, resp)
		default:
			logger.Rec53Log.Sugar().Errorf("Wrong state %d", stm.getCurrentState())
			return
		}
	}

}
