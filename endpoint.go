package dndm

import (
	"context"
	"io"
	"log/slog"

	"github.com/itohio/dndm/errors"
)

// Endpoint is the interface that describes a End To End route.
// Endpoint registers Interests and Intents and links them together when they match.
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

type RemoteEndpoint interface {
	// Local returns the name of the local peer
	Local() Peer
	// Remote returns the name of the remote peer
	Remote() Peer
}

type Base struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
}

// NewBase creates Base. This must be used only once when initializing
// the embedded Base struct.
func NewBase() Base {
	return Base{}
}

// NewBaseWithCtx creates Base. This must be used only once when initializing
// the embedded Base struct.
func NewBaseWithCtx(ctx context.Context) Base {
	ret := Base{}
	ret.Init(ctx)
	return ret
}

func (t *Base) Init(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	t.ctx = ctx
	t.cancel = cancel
	return nil
}

func (t *Base) Close() error {
	return t.CloseCause(nil)
}

func (t *Base) CloseCause(err error) error {
	t.cancel(err)
	return nil
}

func (t *Base) Ctx() context.Context {
	return t.ctx
}

func (t *Base) AddOnClose(f func()) {
	if f == nil {
		return
	}
	go func() {
		<-t.ctx.Done()
		f()
	}()
}

type BaseEndpoint struct {
	Base
	name          string
	Log           *slog.Logger
	OnAddIntent   IntentCallback
	OnAddInterest InterestCallback
	Size          int
}

func NewEndpointBase(name string, size int) BaseEndpoint {
	return BaseEndpoint{
		Base: NewBase(),
		name: name,
		Size: size,
	}
}

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

func (t *BaseEndpoint) Name() string {
	return t.name
}

func (t *BaseEndpoint) SetName(name string) {
	t.name = name
}
