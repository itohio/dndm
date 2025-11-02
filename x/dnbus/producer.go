package dnbus

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/itohio/dndm"
	"google.golang.org/protobuf/proto"
)

// Producer wraps an Intent with type-safe methods for sending messages.
type Producer[T proto.Message] struct {
	intent           dndm.Intent
	router           *dndm.Router
	path             string
	mu               sync.Mutex
	interestReceived bool
}

// NewProducer creates a typed producer for sending messages of type T.
// The producer will automatically wait for interest on the first Send() call,
// or you can explicitly call WaitForInterest().
func NewProducer[T proto.Message](
	ctx context.Context,
	router *dndm.Router,
	path string,
) (*Producer[T], error) {
	var zero T
	intent, err := router.Publish(path, zero)
	if err != nil {
		return nil, err
	}

	return &Producer[T]{
		intent:           intent,
		router:           router,
		path:             path,
		interestReceived: false,
	}, nil
}

// Send waits for interest and then calls the handler callback.
// The handler can call the provided send function to send messages.
// When the handler returns, the intent is terminated.
func (p *Producer[T]) Send(ctx context.Context, handler func(ctx context.Context, send func(msg T) error) error) error {
	// Wait for interest if not already linked
	p.mu.Lock()
	interestReceived := p.interestReceived
	p.mu.Unlock()

	if !interestReceived {
		if err := p.WaitForInterest(ctx); err != nil {
			return err
		}
	}

	// Create send function that calls intent.Send
	sendFunc := func(msg T) error {
		return p.intent.Send(ctx, msg)
	}

	// Call handler - when it returns, we're done
	err := handler(ctx, sendFunc)
	if err != nil {
		// Close intent if handler returns error
		_ = p.intent.Close()
		return err
	}

	// Handler returned successfully - close intent
	return p.intent.Close()
}

// SendWithTimeout waits for interest and then calls the handler callback with a timeout.
func (p *Producer[T]) SendWithTimeout(ctx context.Context, timeout time.Duration, handler func(ctx context.Context, send func(msg T) error) error) error {
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.Send(sendCtx, handler)
}

// WaitForInterest waits for the first consumer to express interest.
// This is optional - Send() will automatically wait on the first call.
func (p *Producer[T]) WaitForInterest(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.interestReceived {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.intent.Interest():
		p.interestReceived = true
		return nil
	}
}

// Close closes the producer and releases associated resources.
func (p *Producer[T]) Close() error {
	return p.intent.Close()
}

// Route returns the route associated with this producer.
func (p *Producer[T]) Route() dndm.Route {
	return p.intent.Route()
}

// SendDirect sends a message directly without closing the intent.
// This is useful when you need to send multiple messages and manage the intent lifecycle yourself.
// You must call WaitForInterest() before calling SendDirect.
func (p *Producer[T]) SendDirect(ctx context.Context, msg T) error {
	p.mu.Lock()
	interestReceived := p.interestReceived
	p.mu.Unlock()

	if !interestReceived {
		if err := p.WaitForInterest(ctx); err != nil {
			return err
		}
	}

	return p.intent.Send(ctx, msg)
}
