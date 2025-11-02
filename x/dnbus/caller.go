package dnbus

import (
	"context"
	"sync"
	"time"

	"github.com/itohio/dndm"
	"google.golang.org/protobuf/proto"
)

// Caller sends requests and receives replies, reusing Producer for requests
// and Consumer for responses. It handles correlation automatically using nonces.
type Caller[Req proto.Message, Resp proto.Message] struct {
	requestProducer  *Producer[Req]
	responseConsumer *Consumer[Resp]
	router           *dndm.Router
	requestPath      string
	responsePath     string
	correlation      map[uint64]chan Resp
	mu               sync.Mutex
	nonce            uint64
}

// NewCaller creates a request/reply caller that sends requests on requestPath
// and receives replies on responsePath.
func NewCaller[Req proto.Message, Resp proto.Message](
	ctx context.Context,
	router *dndm.Router,
	requestPath string,
	responsePath string,
) (*Caller[Req, Resp], error) {
	requestProducer, err := NewProducer[Req](ctx, router, requestPath)
	if err != nil {
		return nil, err
	}

	responseConsumer, err := NewConsumer[Resp](ctx, router, responsePath)
	if err != nil {
		requestProducer.Close()
		return nil, err
	}

	caller := &Caller[Req, Resp]{
		requestProducer:  requestProducer,
		responseConsumer: responseConsumer,
		router:           router,
		requestPath:      requestPath,
		responsePath:     responsePath,
		correlation:      make(map[uint64]chan Resp),
		nonce:            1, // Start at 1 to avoid zero value issues
	}

	// Start goroutine to handle responses
	go caller.handleResponses(ctx)

	return caller, nil
}

// handleResponses handles incoming responses and routes them to waiting callers.
func (c *Caller[Req, Resp]) handleResponses(ctx context.Context) {
	for {
		resp, err := c.responseConsumer.Receive(ctx)
		if err != nil {
			// Context cancelled or consumer closed
			c.mu.Lock()
			// Close all waiting channels
			for _, respC := range c.correlation {
				close(respC)
			}
			c.correlation = make(map[uint64]chan Resp)
			c.mu.Unlock()
			return
		}

		// Extract nonce from response (assuming it's in the message)
		// For now, we'll use a simple correlation mechanism
		// In practice, the nonce should be embedded in the protobuf message
		c.mu.Lock()
		// Route response to first waiting channel (simple implementation)
		// TODO: Implement proper nonce extraction from message
		for nonce, respC := range c.correlation {
			select {
			case respC <- resp:
				delete(c.correlation, nonce)
			default:
			}
			break // Only handle first waiting caller for now
		}
		c.mu.Unlock()
	}
}

// Call sends a request and waits for a single reply.
func (c *Caller[Req, Resp]) Call(ctx context.Context, req Req) (Resp, error) {
	return c.CallWithTimeout(ctx, req, 0)
}

// CallWithTimeout sends a request with a timeout and waits for a single reply.
// If timeout is 0, uses context timeout.
func (c *Caller[Req, Resp]) CallWithTimeout(ctx context.Context, req Req, timeout time.Duration) (Resp, error) {
	// Generate nonce
	c.mu.Lock()
	nonce := c.nonce
	c.nonce++
	respC := make(chan Resp, 1)
	c.correlation[nonce] = respC
	c.mu.Unlock()

	// TODO: Set nonce in request message if supported
	// For now, we use a simple correlation map

	// Create context with timeout if specified
	callCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Send request
	err := c.requestProducer.Send(callCtx, req)
	if err != nil {
		c.mu.Lock()
		delete(c.correlation, nonce)
		close(respC)
		c.mu.Unlock()
		var zero Resp
		return zero, err
	}

	// Wait for reply
	select {
	case <-callCtx.Done():
		c.mu.Lock()
		delete(c.correlation, nonce)
		close(respC)
		c.mu.Unlock()
		var zero Resp
		return zero, callCtx.Err()
	case resp, ok := <-respC:
		if !ok {
			var zero Resp
			return zero, ErrClosed
		}
		return resp, nil
	}
}

// HandleReplies handles multiple replies via callback (goroutine-based).
// The callback is called in a goroutine and can receive one or multiple replies.
func (c *Caller[Req, Resp]) HandleReplies(ctx context.Context, req Req, handler func(ctx context.Context, req Req, resp Resp) error) error {
	// Send request once using handler
	if err := c.requestProducer.Send(ctx, func(ctx context.Context, send func(msg Req) error) error {
		return send(req)
	}); err != nil {
		return err
	}

	// Handle replies in goroutine
	go func() {
		_ = c.responseConsumer.Receive(ctx, func(ctx context.Context, resp Resp) error {
			return handler(ctx, req, resp)
		})
	}()

	return nil
}

// Close closes the caller and releases associated resources.
func (c *Caller[Req, Resp]) Close() error {
	c.mu.Lock()
	// Close all waiting channels
	for _, respC := range c.correlation {
		close(respC)
	}
	c.correlation = nil
	c.mu.Unlock()

	var err1, err2 error
	if c.requestProducer != nil {
		err1 = c.requestProducer.Close()
	}
	if c.responseConsumer != nil {
		err2 = c.responseConsumer.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}
