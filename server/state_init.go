package server

import (
	"context"
	"fmt"

	"rec53/monitor"

	"github.com/miekg/dns"
)

type stateInitState struct {
	baseState
}

// newStateInitState creates a stateInitState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newStateInitState(req, resp *dns.Msg, ctx context.Context) *stateInitState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &stateInitState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *stateInitState) getCurrentState() int {
	return STATE_INIT
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
