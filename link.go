package dndm

import (
	"context"
	"sync"
)

// Link represents a managed connection between an intent and an interest.
// It handles synchronization between these components, ensuring that messages
// from the intent are directed to the interest's channel and notifying each
// other of changes in state or context.
type Link struct {
	Base
	intent   IntentInternal
	interest InterestInternal
	once     sync.Once
}

// NewLink creates a new Link instance initialized with the provided context,
// intent, and interest. It sets up an onClose behavior to sever the link cleanly
// when the Link object is closed.
func NewLink(ctx context.Context, intent IntentInternal, interest InterestInternal) *Link {
	ret := &Link{
		Base:     NewBaseWithCtx(ctx),
		intent:   intent,
		interest: interest,
	}
	ret.AddOnClose(func() { intent.Link(nil) })
	return ret
}

// Link starts the process of linking the intent with the interest.
// It configures the intent to send messages to the channel of the interest
// and ensures that notifications are sent to the intent to signal the establishment
// of the link. This method ensures that the link operation is performed only once.
// The operation runs concurrently and listens for context cancellation signals
// from the Link itself or either the intent or interest to properly manage resource cleanup.
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

// Notify sends a notification to the intent, typically used to indicate state changes
// or updates in the interest that need to be communicated to the intent.
func (l *Link) Notify() {
	l.intent.Notify()
}
