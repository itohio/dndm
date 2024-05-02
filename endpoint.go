package dndm

import (
	"context"
	"io"
	"log/slog"

	"github.com/itohio/dndm/errors"
)

// Endpoint describes a component that can register, manage, and link intents and interests
// based on data routes. It provides methods for initialization, publishing intents, subscribing interests,
// and managing its lifecycle.
type Endpoint interface {
	io.Closer
	OnClose(func()) Endpoint
	Name() string
	// Publish will advertise an intent to publish named and typed data.
	Publish(route Route, opt ...PubOpt) (Intent, error)
	// Subscribe will advertise an interest in named and typed data.
	Subscribe(route Route, opt ...SubOpt) (Interest, error)
	// Init is used by the Router to initialize this endpoint.
	Init(ctx context.Context, logger *slog.Logger, addIntent IntentCallback, addInterest InterestCallback) error
}

// RemoteEndpoint extends Endpoint with methods to retrieve local and remote peer information.
type RemoteEndpoint interface {
	// Local returns the name of the local peer
	Local() Peer
	// Remote returns the name of the remote peer
	Remote() Peer
}

// Base provides basic context management functionalities for components that require
// initialization with a context, cancellation, and cleanup operations.
type Base struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
}

// NewBase initializes a new Base instance without a specific context.
func NewBase() Base {
	return Base{}
}

// NewBaseWithCtx initializes a new Base instance with the provided context.
func NewBaseWithCtx(ctx context.Context) Base {
	ret := Base{}
	ret.Init(ctx)
	return ret
}

// Init sets up the Base instance with a cancellation context.
func (t *Base) Init(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	t.ctx = ctx
	t.cancel = cancel
	return nil
}

// Close cleans up resources and cancels the context without a specific cause.
func (t *Base) Close() error {
	return t.CloseCause(nil)
}

// CloseCause cleans up resources and cancels the context with a specified error cause.
func (t *Base) CloseCause(err error) error {
	t.cancel(err)
	return nil
}

// Ctx returns the current context associated with the Base instance.
func (t *Base) Ctx() context.Context {
	return t.ctx
}

// AddOnClose registers a function to be called upon the context's cancellation.
func (t *Base) AddOnClose(f func()) {
	if f == nil {
		return
	}
	go func() {
		<-t.ctx.Done()
		f()
	}()
}

// BaseEndpoint is a concrete implementation of the Endpoint interface that provides
// methods for endpoint initialization, managing lifecycle, and handling intents and interests.
type BaseEndpoint struct {
	Base
	name          string
	Log           *slog.Logger
	OnAddIntent   IntentCallback
	OnAddInterest InterestCallback
	Size          int
}

// NewEndpointBase creates a new BaseEndpoint with a specified name and size.
func NewEndpointBase(name string, size int) BaseEndpoint {
	return BaseEndpoint{
		Base: NewBase(),
		name: name,
		Size: size,
	}
}

// Init initializes the BaseEndpoint with necessary callbacks and logging capabilities.
func (t *BaseEndpoint) Init(ctx context.Context, logger *slog.Logger, addIntent IntentCallback, addInterest InterestCallback) error {
	if logger == nil || addIntent == nil || addInterest == nil {
		return errors.ErrBadArgument
	}
	t.Base.Init(ctx)
	t.Log = logger
	t.OnAddIntent = addIntent
	t.OnAddInterest = addInterest

	return nil
}

// Name returns the name of the endpoint.
func (t *BaseEndpoint) Name() string {
	return t.name
}

// SetName sets or updates the name of the endpoint.
func (t *BaseEndpoint) SetName(name string) {
	t.name = name
}
