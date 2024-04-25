package dndm

import (
	"context"
	"io"
	"log/slog"
	"sync"

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

type BaseCtx struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}
	once   sync.Once
}

func NewBaseCtx() BaseCtx {
	return BaseCtx{
		done: make(chan struct{}),
	}
}

func NewBaseCtxWithCtx(ctx context.Context) BaseCtx {
	ret := BaseCtx{
		done: make(chan struct{}),
	}
	ret.Init(ctx)
	return ret
}

func (t *BaseCtx) Init(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	t.ctx = ctx
	t.cancel = cancel
	return nil
}

func (t *BaseCtx) Close() error {
	return t.CloseCause(nil)
}

func (t *BaseCtx) CloseCause(err error) error {
	t.cancel(err)
	t.once.Do(func() { close(t.done) })
	return nil
}

func (t *BaseCtx) Ctx() context.Context {
	return t.ctx
}

func (t *BaseCtx) AddOnClose(f func()) {
	if f == nil {
		return
	}
	go func() {
		select {
		case <-t.ctx.Done():
		case <-t.done:
		}
		f()
	}()
	return
}

type BaseEndpoint struct {
	BaseCtx
	name          string
	Log           *slog.Logger
	OnAddIntent   IntentCallback
	OnAddInterest InterestCallback
	Size          int
}

func NewBase(name string, size int) *BaseEndpoint {
	return &BaseEndpoint{
		BaseCtx: NewBaseCtx(),
		name:    name,
		Size:    size,
	}
}

func (t *BaseEndpoint) Init(ctx context.Context, logger *slog.Logger, addIntent IntentCallback, addInterest InterestCallback) error {
	if logger == nil || addIntent == nil || addInterest == nil {
		return errors.ErrBadArgument
	}
	t.BaseCtx.Init(ctx)
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
