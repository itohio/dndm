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

// Receive returns the next message from the underlying consumer.
func (i *Input[T]) Receive(ctx context.Context) (T, error) {
	return i.consumer.Receive(ctx)
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

// Send waits for interest (on the first call) and delivers the provided message.
func (o *Output[T]) Send(ctx context.Context, msg T) error {
	return o.producer.Send(ctx, msg)
}

// SendWithTimeout behaves like Send but bounds the send and interest wait time.
func (o *Output[T]) SendWithTimeout(ctx context.Context, msg T, timeout time.Duration) error {
	return o.producer.SendWithTimeout(ctx, msg, timeout)
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

// AddInput adds an input consumer to the module. This is exposed as a package-level
// generic helper to remain compatible with Go 1.21 (method type parameters are not supported).
func AddInput[T proto.Message](m *Module, path string) (*Input[T], error) {
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

// AddOutput adds an output producer to the module. Exposed as package-level helper
// for the same reason as AddInput.
func AddOutput[T proto.Message](m *Module, path string) (*Output[T], error) {
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
