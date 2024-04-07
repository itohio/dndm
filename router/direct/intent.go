package direct

import (
	"context"
	"sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
	"google.golang.org/protobuf/proto"
)

type Intent struct {
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	route   router.Route
	msgC    chan proto.Message
	notifyC chan router.Route
	closer  func() error
	linked  bool
}

func NewIntent(ctx context.Context, route router.Route, size int, closer func() error) *Intent {
	ctx, cancel := context.WithCancel(ctx)
	intent := &Intent{
		ctx:     ctx,
		cancel:  cancel,
		route:   route,
		notifyC: make(chan router.Route),
		msgC:    make(chan proto.Message, size),
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

func (i *Intent) Route() router.Route {
	return i.route
}

func (i *Intent) Interest() <-chan router.Route {
	return i.notifyC
}

func (i *Intent) MsgC() chan proto.Message {
	return i.msgC
}

func (i *Intent) Send(ctx context.Context, msg proto.Message) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if !i.linked {
		return errors.ErrNoInterest
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-i.ctx.Done():
		return i.ctx.Err()
	case i.msgC <- msg:
	}

	return nil
}

func (i *Intent) SetLinked(l bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.linked = l
}

func (i *Intent) Notify() {
	select {
	case i.notifyC <- i.route:
	default:
	}
}
