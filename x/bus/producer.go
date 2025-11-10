package bus

import (
	"context"
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

// Send waits for interest (on the first call) and sends the provided message.
func (p *Producer[T]) Send(ctx context.Context, msg T) error {
	if err := p.ensureInterest(ctx); err != nil {
		return err
	}
	return p.intent.Send(ctx, msg)
}

// SendWithTimeout behaves like Send but bounds the send and interest wait time.
func (p *Producer[T]) SendWithTimeout(ctx context.Context, msg T, timeout time.Duration) error {
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.Send(sendCtx, msg)
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
	if err := p.ensureInterest(ctx); err != nil {
		return err
	}
	return p.intent.Send(ctx, msg)
}

func (p *Producer[T]) ensureInterest(ctx context.Context) error {
	p.mu.Lock()
	if p.interestReceived {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()
	return p.WaitForInterest(ctx)
}
