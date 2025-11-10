package bus

import (
	"context"
	"io"
	"sync"

	"github.com/itohio/dndm"
	"google.golang.org/protobuf/proto"
)

// Consumer wraps an Interest with type-safe methods for receiving messages.
type Consumer[T proto.Message] struct {
	interest dndm.Interest
	router   *dndm.Router
	path     string
	typedC   <-chan T
	mu       sync.Mutex
	closed   bool
}

// NewConsumer creates a typed consumer for receiving messages of type T.
// It creates a type-safe channel that converts proto.Message to T automatically.
func NewConsumer[T proto.Message](
	ctx context.Context,
	router *dndm.Router,
	path string,
) (*Consumer[T], error) {
	var zero T
	interest, err := router.Subscribe(path, zero)
	if err != nil {
		return nil, err
	}

	// Create typed channel with same capacity as interest channel
	interestC := interest.C()
	typedC := make(chan T, cap(interestC))

	// Start goroutine to convert types
	go func() {
		defer close(typedC)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-interestC:
				if !ok {
					return
				}
				typedMsg, ok := msg.(T)
				if !ok {
					// Type assertion failed - skip this message
					continue
				}
				select {
				case <-ctx.Done():
					return
				case typedC <- typedMsg:
				}
			}
		}
	}()

	return &Consumer[T]{
		interest: interest,
		router:   router,
		path:     path,
		typedC:   typedC,
		closed:   false,
	}, nil
}

// Receive returns the next message or an error if the consumer is closed or the context is cancelled.
func (c *Consumer[T]) Receive(ctx context.Context) (T, error) {
	var zero T

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return zero, ErrClosed
	}
	typedC := c.typedC
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case msg, ok := <-typedC:
		if !ok {
			c.mu.Lock()
			c.closed = true
			c.mu.Unlock()
			return zero, io.EOF
		}
		return msg, nil
	}
}

// C returns the typed message channel for advanced use cases.
// The channel will be closed when the consumer is closed.
func (c *Consumer[T]) C() <-chan T {
	return c.typedC
}

// Close closes the consumer and releases associated resources.
func (c *Consumer[T]) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return c.interest.Close()
}

// Route returns the route associated with this consumer.
func (c *Consumer[T]) Route() dndm.Route {
	return c.interest.Route()
}
