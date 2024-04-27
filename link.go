package dndm

import (
	"context"
	"sync"
)

type Link struct {
	Base
	intent   IntentInternal
	interest InterestInternal
	once     sync.Once
}

func NewLink(ctx context.Context, intent IntentInternal, interest InterestInternal) *Link {
	ret := &Link{
		Base:     NewBaseWithCtx(ctx),
		intent:   intent,
		interest: interest,
	}
	ret.AddOnClose(func() { intent.Link(nil) })
	return ret
}

// Link links intent with interest by configuring the intent to
// send messages to interest's MsgC channel.
//
// Link notifies intent.
func (l *Link) Link() {
	l.once.Do(func() {
		go func() {
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
	})
}

func (l *Link) Notify() {
	l.intent.Notify()
}
