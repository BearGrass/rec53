package server

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
)

type returnRespState struct {
	baseState
}

// newReturnRespState creates a returnRespState with a specific context.
// Pass context.Background() if no deadline or cancellation is needed.
func newReturnRespState(req, resp *dns.Msg, ctx context.Context) *returnRespState {
	if ctx == nil {
		ctx = context.Background()
	}
	return &returnRespState{baseState{request: req, response: resp, ctx: ctx}}
}

// implement stateMachine interface
func (s *returnRespState) getCurrentState() int {
	return RETURN_RESP
}

func (s *returnRespState) handle(request *dns.Msg, response *dns.Msg) (int, error) {
	if request == nil || response == nil {
		return RETURN_RESP_COMMON_ERROR, fmt.Errorf("request is nil or response is nil")
	}
	return RETURN_RESP_NO_ERROR, nil
}
