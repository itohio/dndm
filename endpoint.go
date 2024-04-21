package dndm

import (
	"context"
	"io"
	"log/slog"

	"github.com/itohio/dndm/errors"
)

var (
	_ CloseNotifier = (*Base)(nil)
)

// Endpoint is the interface that describes a End To End route.
// Endpoint registers Interests and Intents and links them together when they match.
type Endpoint interface {
	io.Closer
	Name() string
	// Publish will advertise an intent to publish named and typed data.
	Publish(route Route, opt ...PubOpt) (Intent, error)
	// Subscribe will advertise an interest in named and typed data.
	Subscribe(route Route, opt ...SubOpt) (Interest, error)
	// Init is used by the Router to initialize this endpoint.
	Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Endpoint) error) error
}

type Base struct {
	Ctx            context.Context
	cancel         context.CancelCauseFunc
	name           string
	Log            *slog.Logger
	AddCallback    func(interest Interest, t Endpoint) error
	RemoveCallback func(interest Interest, t Endpoint) error
	Size           int
}

func NewBase(name string, size int) *Base {
	return &Base{
		name: name,
		Size: size,
	}
}

func (t *Base) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Endpoint) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.Log = logger
	t.AddCallback = add
	t.RemoveCallback = remove
	ctx, cancel := context.WithCancelCause(ctx)
	t.Ctx = ctx
	t.cancel = cancel

	return nil
}

func (t *Base) Name() string {
	return t.name
}

func (t *Base) SetName(name string) {
	t.name = name
}

func (t *Base) Close() error {
	t.cancel(nil)
	return nil
}

func (t *Base) CloseCause(err error) error {
	t.cancel(err)
	return nil
}

func (t *Base) OnClose(f func()) {
	if f == nil {
		return
	}
	go func() {
		<-t.Ctx.Done()
		f()
	}()
}
