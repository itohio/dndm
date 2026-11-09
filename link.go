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
	unlinked chan struct{}
}

// NewLink creates a new Link instance initialized with the provided context,
// intent, and interest. It sets up an onClose behavior to sever the link cleanly
// when the Link object is closed.
func NewLink(ctx context.Context, intent IntentInternal, interest InterestInternal) *Link {
	ret := &Link{
		Base:     NewBaseWithCtx(ctx),
		intent:   intent,
		interest: interest,
		unlinked: make(chan struct{}),
	}
	ret.AddOnClose(func() {
		intent.Link(nil)
		close(ret.unlinked)
	})
	return ret
}

// Close closes the link and returns once the intent is unlinked, also when another
// goroutine is already closing it. After that the intent no longer sends on the
// interest's channel, so the interest may close it.
func (l *Link) Close() error {
	err := l.Base.Close()
	<-l.unlinked
	return err
}

// Link links the intent to the interest's channel and notifies the intent, once.
// It runs in the caller's goroutine: an asynchronous link could run after Close and
// re-link a closed link's intent. A link that is already closed does nothing.
func (l *Link) Link() {
	l.once.Do(func() {
		if l.ctx.Err() != nil {
			return
		}
		l.intent.Link(l.interest.MsgC())
		l.intent.Notify()
	})
}

// Notify sends a notification to the intent, typically used to indicate state changes
// or updates in the interest that need to be communicated to the intent.
func (l *Link) Notify() {
	l.intent.Notify()
}
