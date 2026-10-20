package components

import (
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

type aiStreamUpdate struct {
	run    uint64
	result ai.Result
}

func (session *aiSession) startStreamUI() {
	session.streamQueue = make(chan aiStreamUpdate, 1)
	application := app.App.Application
	go func() {
		for {
			select {
			case <-session.ctx.Done():
				return
			case update := <-session.streamQueue:
				if session.ctx.Err() != nil {
					return
				}
				application.QueueUpdateDraw(func() {
					if !session.active() || session.run != update.run || !session.busy {
						return
					}
					session.streaming = update.result
					if update.result.SQL != "" || update.result.Message != "" || update.result.Clarification != "" || update.result.Missing != "" {
						session.status.SetText("Responding…")
					}
					session.renderChat()
				})
			}
		}
	}()
}

func (session *aiSession) stream(run uint64, result ai.Result) {
	if session.ctx.Err() != nil {
		return
	}
	update := aiStreamUpdate{run: run, result: result}
	select {
	case session.streamQueue <- update:
		return
	default:
	}
	select {
	case <-session.streamQueue:
	default:
	}
	select {
	case session.streamQueue <- update:
	default:
	}
}
