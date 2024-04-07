package direct

import (
	"context"
	"sync"

	"github.com/itohio/dndm/router"
	"google.golang.org/protobuf/proto"
)

type Interest struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	route  router.Route
	msgC   chan proto.Message
	closer func() error
}

func NewInterest(ctx context.Context, route router.Route, size int, closer func() error) *Interest {
	ctx, cancel := context.WithCancel(ctx)
	return &Interest{
		ctx:    ctx,
		cancel: cancel,
		route:  route,
		closer: closer,
		msgC:   make(chan proto.Message, size),
	}
}

func (i *Interest) Ctx() context.Context {
	return i.ctx
}

func (i *Interest) Close() error {
	i.cancel()
	err := i.closer()
	if err != nil {
		return err
	}
	close(i.msgC) // Might cause panic in `link`, however, it should have been unlinked by then
	return nil
}

func (i *Interest) Route() router.Route {
	return i.route
}

func (i *Interest) C() <-chan proto.Message {
	return i.msgC
}

func (i *Interest) MsgC() chan proto.Message {
	return i.msgC
}
