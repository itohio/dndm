package direct

import (
	"context"
	"log/slog"
	"reflect"
	"sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	"google.golang.org/protobuf/proto"
)

type Intent struct {
	ctx    context.Context
	cancel context.CancelFunc
	route  routers.Route
	// msgC    chan proto.Message
	notifyC chan routers.Route
	closer  func() error

	mu      sync.RWMutex
	linkedC chan<- proto.Message
}

func NewIntent(ctx context.Context, route routers.Route, size int, closer func() error) *Intent {
	ctx, cancel := context.WithCancel(ctx)
	intent := &Intent{
		ctx:     ctx,
		cancel:  cancel,
		route:   route,
		notifyC: make(chan routers.Route, size),
		closer:  closer,
	}
	return intent
}

func (i *Intent) Ctx() context.Context {
	return i.ctx
}

func (i *Intent) Close() error {
	err := i.closer()
	if err != nil {
		return err
	}
	i.cancel()
	close(i.notifyC)
	return nil
}

func (i *Intent) Route() routers.Route {
	return i.route
}

func (i *Intent) Interest() <-chan routers.Route {
	return i.notifyC
}

func (i *Intent) Send(ctx context.Context, msg proto.Message) error {
	i.mu.RLock()
	linkedC := i.linkedC
	i.mu.RUnlock()
	if linkedC == nil {
		return errors.ErrNoInterest
	}
	if reflect.TypeOf(msg) != i.route.Type() {
		return errors.ErrInvalidType
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-i.ctx.Done():
		return i.ctx.Err()
	case linkedC <- msg:
	}

	return nil
}

func (i *Intent) Link(c chan<- proto.Message) {
	i.mu.Lock()
	i.linkedC = c
	i.mu.Unlock()
}

func (i *Intent) Notify() {
	select {
	case i.notifyC <- i.route:
		slog.Info("Intent.Notify SEND", "route", i.route)
	default:
		slog.Info("Intent.Notify SKIP", "route", i.route)
	}
}
