package server

/*
func (i *Iterator) Resolve(ctx *server.QueryContext) (*g53.Message, error) {
	event := NewEvent(ctx, &ctx.Request, InitQuery, Finished)
	for {
		if ctx.TraceQuery {
			logger.GetLogger().Info("handle event",
				zap.String("state", event.CurrentState().String()),
				zap.String("question", event.Request().Question.String()))
		}

		switch event.CurrentState() {
		case InitQuery:
			event = i.processInitQuery(event)
		case QueryTarget:
			event = i.processQueryTarget(event)
		case QueryResponse:
			event = i.processQueryResponse(event)
		case PrimeResponse:
			event = i.processPrimeResponse(event)
		case TargetResponse:
			event = i.processTargetResponse(event)
		case Finished:
			resp := i.processFinished(event)
			ReleaseEvent(event)
			return resp, nil
		default:
			panic("unreachable branche")
		}
	}
}
*/
