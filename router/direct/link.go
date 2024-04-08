package direct

import (
	"context"

	"github.com/itohio/dndm/router"
)

type Link struct {
	ctx      context.Context
	cancel   context.CancelFunc
	intent   router.IntentInternal
	interest router.InterestInternal
	closer   func() error
	done     chan struct{}
}

func NewLink(ctx context.Context, intent router.IntentInternal, interest router.InterestInternal, closer func() error) *Link {
	ctx, cancel := context.WithCancel(ctx)
	ret := &Link{
		ctx:      ctx,
		cancel:   cancel,
		intent:   intent,
		interest: interest,
		closer:   closer,
		done:     make(chan struct{}),
	}

	return ret
}

func (l Link) Link() {
	go func() {
		defer func() {
			l.intent.Link(nil)
			l.closer()
			l.done <- struct{}{}
		}()
		l.intent.Link(l.interest.MsgC())
		l.intent.Notify()
		select {
		case <-l.ctx.Done():
			return
		case <-l.intent.Ctx().Done():
			return
		case <-l.interest.Ctx().Done():
			return
		}
	}()
}

func (l Link) Unlink() {
	l.cancel()
	<-l.done
}

func (l Link) Notify() {
	l.intent.Notify()
}
