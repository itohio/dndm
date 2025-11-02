package dnbus

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/itohio/dndm"
	"google.golang.org/protobuf/proto"
)

// Input represents an input to a processing module.
type Input[T proto.Message] struct {
	consumer *Consumer[T]
	path     string
}

// Receive calls the handler for each received message.
// If the handler returns an error, the interest is terminated.
func (i *Input[T]) Receive(ctx context.Context, handler func(ctx context.Context, msg T) error) error {
	return i.consumer.Receive(ctx, handler)
}

// C returns the typed message channel for this input.
func (i *Input[T]) C() <-chan T {
	return i.consumer.C()
}

// Output represents an output from a processing module.
type Output[T proto.Message] struct {
	producer *Producer[T]
	path     string
}

// Send waits for interest and then calls the handler callback.
// The handler can call the provided send function to send messages.
// When the handler returns, the intent is terminated.
func (o *Output[T]) Send(ctx context.Context, handler func(ctx context.Context, send func(msg T) error) error) error {
	return o.producer.Send(ctx, handler)
}

// SendWithTimeout waits for interest and then calls the handler callback with a timeout.
func (o *Output[T]) SendWithTimeout(ctx context.Context, timeout time.Duration, handler func(ctx context.Context, send func(msg T) error) error) error {
	return o.producer.SendWithTimeout(ctx, timeout, handler)
}

// Module represents a processing module with multiple inputs/outputs.
type Module struct {
	ctx     context.Context
	router  *dndm.Router
	mu      sync.Mutex
	inputs  []io.Closer
	outputs []io.Closer
}

// NewModule creates a new processing module.
func NewModule(ctx context.Context, router *dndm.Router) *Module {
	return &Module{
		ctx:     ctx,
		router:  router,
		inputs:  make([]io.Closer, 0),
		outputs: make([]io.Closer, 0),
	}
}

// AddInput adds an input consumer to the module.
func (m *Module) AddInput[T proto.Message](path string) (*Input[T], error) {
	consumer, err := NewConsumer[T](m.ctx, m.router, path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.inputs = append(m.inputs, consumer)
	m.mu.Unlock()

	return &Input[T]{
		consumer: consumer,
		path:     path,
	}, nil
}

// AddOutput adds an output producer to the module.
func (m *Module) AddOutput[T proto.Message](path string) (*Output[T], error) {
	producer, err := NewProducer[T](m.ctx, m.router, path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.outputs = append(m.outputs, producer)
	m.mu.Unlock()

	return &Output[T]{
		producer: producer,
		path:     path,
	}, nil
}

// Close closes all inputs and outputs.
func (m *Module) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error
	for _, input := range m.inputs {
		if closeErr := input.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	for _, output := range m.outputs {
		if closeErr := output.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	m.inputs = nil
	m.outputs = nil
	return err
}

// Run runs a processing function. The function should handle all input/output
// operations using the Input/Output objects added to the module.
func (m *Module) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

