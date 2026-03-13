package server

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
)

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
