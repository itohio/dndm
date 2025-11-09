package dnbus

import (
	"context"
	"sync"
	"time"

	"github.com/itohio/dndm"
	"google.golang.org/protobuf/proto"
)

type waiter[Resp proto.Message] struct {
	ch     chan Resp
	stream bool
	once   sync.Once
}

func (w *waiter[Resp]) close() {
	w.once.Do(func() {
		close(w.ch)
	})
}

type callerOptions[Req proto.Message, Resp proto.Message] struct {
	setRequestNonce  func(Req, uint64)
	getResponseNonce func(Resp) (uint64, bool)
}

// CallerOption configures optional behaviour of Caller.
type CallerOption[Req proto.Message, Resp proto.Message] func(*callerOptions[Req, Resp])

// WithCallerRequestNonce registers a hook that injects the generated nonce into outgoing requests.
func WithCallerRequestNonce[Req proto.Message, Resp proto.Message](fn func(Req, uint64)) CallerOption[Req, Resp] {
	return func(opts *callerOptions[Req, Resp]) {
		opts.setRequestNonce = fn
	}
}

// WithCallerResponseNonce registers a hook that extracts a nonce from incoming responses.
func WithCallerResponseNonce[Req proto.Message, Resp proto.Message](fn func(Resp) (uint64, bool)) CallerOption[Req, Resp] {
	return func(opts *callerOptions[Req, Resp]) {
		opts.getResponseNonce = fn
	}
}

// Caller sends requests and receives replies, reusing Producer for requests
// and Consumer for responses. It handles correlation automatically using nonces.
type Caller[Req proto.Message, Resp proto.Message] struct {
	requestProducer  *Producer[Req]
	responseConsumer *Consumer[Resp]
	router           *dndm.Router
	requestPath      string
	responsePath     string

	mu      sync.Mutex
	waiters map[uint64]*waiter[Resp]
	order   []uint64
	nonce   uint64
	opts    callerOptions[Req, Resp]
}

// NewCaller creates a request/reply caller that sends requests on requestPath
// and receives replies on responsePath.
func NewCaller[Req proto.Message, Resp proto.Message](
	ctx context.Context,
	router *dndm.Router,
	requestPath string,
	responsePath string,
	options ...CallerOption[Req, Resp],
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

	callOpts := callerOptions[Req, Resp]{}
	for _, optFn := range options {
		optFn(&callOpts)
	}

	caller := &Caller[Req, Resp]{
		requestProducer:  requestProducer,
		responseConsumer: responseConsumer,
		router:           router,
		requestPath:      requestPath,
		responsePath:     responsePath,
		waiters:          make(map[uint64]*waiter[Resp]),
		order:            make([]uint64, 0),
		nonce:            1, // Start at 1 to avoid zero value issues
		opts:             callOpts,
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
			c.failAll()
			return
		}

		delivered := false
		if c.opts.getResponseNonce != nil {
			if nonce, ok := c.opts.getResponseNonce(resp); ok {
				if w := c.takeWaiterForNonce(nonce); w != nil {
					delivered = true
					c.deliver(resp, w)
				}
			}
		}
		if !delivered {
			if w := c.takeNextWaiter(); w != nil {
				delivered = true
				c.deliver(resp, w)
			}
		}
	}
}

// Call sends a request and waits for a single reply.
func (c *Caller[Req, Resp]) Call(ctx context.Context, req Req) (Resp, error) {
	return c.CallWithTimeout(ctx, req, 0)
}

// CallWithTimeout sends a request with a timeout and waits for a single reply.
// If timeout is 0, uses context timeout.
func (c *Caller[Req, Resp]) CallWithTimeout(ctx context.Context, req Req, timeout time.Duration) (Resp, error) {
	// Create context with timeout if specified
	callCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	w, nonce := c.registerWaiter(false)
	respC := w.ch

	if setter := c.opts.setRequestNonce; setter != nil {
		setter(req, nonce)
	}

	// Send request
	err := c.requestProducer.Send(callCtx, req)
	if err != nil {
		if removed := c.unregisterWaiter(nonce); removed != nil {
			removed.close()
		} else {
			w.close()
		}
		var zero Resp
		return zero, err
	}

	// Wait for reply
	select {
	case <-callCtx.Done():
		if removed := c.unregisterWaiter(nonce); removed != nil {
			removed.close()
		} else {
			w.close()
		}
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
	w, nonce := c.registerWaiter(true)
	ch := w.ch

	if setter := c.opts.setRequestNonce; setter != nil {
		setter(req, nonce)
	}

	// Send request once
	if err := c.requestProducer.Send(ctx, req); err != nil {
		if removed := c.unregisterWaiter(nonce); removed != nil {
			removed.close()
		} else {
			w.close()
		}
		return err
	}

	// Handle replies in goroutine
	go func() {
		defer func() {
			if removed := c.unregisterWaiter(nonce); removed != nil {
				removed.close()
			} else {
				w.close()
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case resp, ok := <-ch:
				if !ok {
					return
				}
				if err := handler(ctx, req, resp); err != nil {
					return
				}
			}
		}
	}()

	return nil
}

// Close closes the caller and releases associated resources.
func (c *Caller[Req, Resp]) Close() error {
	c.mu.Lock()
	// Close all waiting channels
	for nonce, w := range c.waiters {
		w.close()
		delete(c.waiters, nonce)
	}
	c.order = nil
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

func (c *Caller[Req, Resp]) deliver(resp Resp, w *waiter[Resp]) {
	if w.stream {
		w.ch <- resp
		return
	}

	w.ch <- resp
	w.close()
}

func (c *Caller[Req, Resp]) registerWaiter(stream bool) (*waiter[Resp], uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.waiters == nil {
		c.waiters = make(map[uint64]*waiter[Resp])
	}
	if c.order == nil {
		c.order = make([]uint64, 0)
	}

	nonce := c.nonce
	c.nonce++
	w := &waiter[Resp]{ch: make(chan Resp, 1), stream: stream}
	c.waiters[nonce] = w
	c.order = append(c.order, nonce)
	return w, nonce
}

func (c *Caller[Req, Resp]) unregisterWaiter(nonce uint64) *waiter[Resp] {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.waiters[nonce]
	if !ok {
		return nil
	}
	delete(c.waiters, nonce)
	for idx, id := range c.order {
		if id == nonce {
			c.order = append(c.order[:idx], c.order[idx+1:]...)
			break
		}
	}
	return w
}

func (c *Caller[Req, Resp]) takeWaiterForNonce(nonce uint64) *waiter[Resp] {
	c.mu.Lock()
	defer c.mu.Unlock()

	w, ok := c.waiters[nonce]
	if !ok {
		return nil
	}

	if w.stream {
		c.moveToBackLocked(nonce)
		return w
	}

	delete(c.waiters, nonce)
	c.removeLocked(nonce)
	return w
}

func (c *Caller[Req, Resp]) takeNextWaiter() *waiter[Resp] {
	c.mu.Lock()
	defer c.mu.Unlock()

	for len(c.order) > 0 {
		nonce := c.order[0]
		c.order = c.order[1:]
		w, ok := c.waiters[nonce]
		if !ok {
			continue
		}
		if w.stream {
			c.order = append(c.order, nonce)
		} else {
			delete(c.waiters, nonce)
		}
		return w
	}

	return nil
}

func (c *Caller[Req, Resp]) removeLocked(nonce uint64) {
	for idx, id := range c.order {
		if id == nonce {
			c.order = append(c.order[:idx], c.order[idx+1:]...)
			break
		}
	}
}

func (c *Caller[Req, Resp]) moveToBackLocked(nonce uint64) {
	c.removeLocked(nonce)
	c.order = append(c.order, nonce)
}

func (c *Caller[Req, Resp]) failAll() {
	c.mu.Lock()
	waiters := c.waiters
	c.waiters = make(map[uint64]*waiter[Resp])
	c.order = nil
	c.mu.Unlock()

	for _, w := range waiters {
		w.close()
	}
}
