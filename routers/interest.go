package routers

import (
	"context"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

type LocalInterest struct {
	ctx    context.Context
	cancel context.CancelFunc
	route  Route
	msgC   chan proto.Message
	closer func() error
}

func NewInterest(ctx context.Context, route Route, size int, closer func() error) *LocalInterest {
	ctx, cancel := context.WithCancel(ctx)
	return &LocalInterest{
		ctx:    ctx,
		cancel: cancel,
		route:  route,
		closer: closer,
		msgC:   make(chan proto.Message, size),
	}
}

func (i *LocalInterest) Ctx() context.Context {
	return i.ctx
}

func (i *LocalInterest) Close() error {
	err := i.closer()
	if err != nil {
		return err
	}
	i.cancel()
	close(i.msgC) // FIXME: Might cause panic in `link`, however, it should have been unlinked by then
	i.msgC = nil
	return nil
}

func (i *LocalInterest) Route() Route {
	return i.route
}

func (i *LocalInterest) C() <-chan proto.Message {
	return i.msgC
}

func (i *LocalInterest) MsgC() chan<- proto.Message {
	return i.msgC
}

type interestWrapper struct {
	router *InterestRouter
	c      chan proto.Message
}

func (w *interestWrapper) Close() error {
	return w.router.removeWrapper(w)
}
func (w *interestWrapper) Route() Route {
	return w.router.route
}
func (w *interestWrapper) C() <-chan proto.Message {
	return w.c
}

type InterestRouter struct {
	mu        sync.RWMutex
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	route     Route
	closer    func() error
	c         chan proto.Message
	interests []Interest
	cancels   []context.CancelFunc
	wrappers  []*interestWrapper
	size      int
}

func NewInterestRouter(ctx context.Context, route Route, closer func() error, size int, interests ...Interest) *InterestRouter {
	ctx, cancel := context.WithCancel(ctx)
	ret := &InterestRouter{
		ctx:    ctx,
		cancel: cancel,
		closer: closer,
		route:  route,
		c:      make(chan proto.Message, size),
		size:   size,
	}

	for _, i := range interests {
		ret.AddInterest(i)
	}

	return ret
}

func (i *InterestRouter) Wrap() *interestWrapper {
	ret := &interestWrapper{
		router: i,
		c:      make(chan proto.Message, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *InterestRouter) addWrapper(w *interestWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.wrappers = append(i.wrappers, w)
}

func (i *InterestRouter) removeWrapper(w *interestWrapper) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	idx := slices.Index(i.wrappers, w)
	w.router = nil
	if idx >= 0 {
		i.wrappers = slices.Delete(i.wrappers, idx, idx+1)
	}
	close(w.c)

	if len(i.wrappers) > 0 {
		return nil
	}

	return i.Close()
}

func (i *InterestRouter) AddInterest(interest Interest) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.route.Equal(interest.Route()) {
		return errors.ErrInvalidRoute
	}
	if idx := slices.Index(i.interests, interest); idx >= 0 {
		return nil
	}

	i.interests = append(i.interests, interest)
	ctx, cancel := context.WithCancel(i.ctx)
	i.cancels = append(i.cancels, cancel)
	i.wg.Add(1)
	go i.recvRunner(ctx, interest)
	return nil
}

func (i *InterestRouter) RemoveInterest(interest Interest) {
	i.mu.Lock()
	defer i.mu.Unlock()
	idx := slices.Index(i.interests, interest)
	if idx < 0 {
		return
	}
	i.cancels[idx]()
	i.interests = slices.Delete(i.interests, idx, idx+1)
	i.cancels = slices.Delete(i.cancels, idx, idx+1)
}

func (i *InterestRouter) recvRunner(ctx context.Context, interest Interest) {
	defer i.wg.Done()
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ctx.Done():
			return
		case msg := <-interest.C():
			if err := i.routeMsg(ctx, msg); err != nil {
				return
			}
		}
	}
}

func (i *InterestRouter) routeMsg(ctx context.Context, msg proto.Message) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, w := range i.wrappers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-i.ctx.Done():
			return i.ctx.Err()
		case w.c <- msg:
		default:
		}
	}
	return nil
}

func (i *InterestRouter) Close() error {
	i.cancel()
	err := i.closer()
	if err != nil {
		return err
	}
	i.wg.Wait()
	return nil
}

func (i *InterestRouter) Route() Route {
	return i.route
}

func (i *InterestRouter) C() <-chan proto.Message {
	return i.c
}
