package dndm

import (
	"context"
	"io"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

var (
	_ Interest         = (*LocalInterest)(nil)
	_ InterestInternal = (*LocalInterest)(nil)
	_ Interest         = (*interestWrapper)(nil)
	_ Interest         = (*InterestRouter)(nil)
	_ CloseNotifier    = (*LocalInterest)(nil)
	_ CloseNotifier    = (*InterestRouter)(nil)
)

// Interest is an interface to describe an interest in named data.
// User should consume C of the interest until it is closed or no longer needed.
// Messages will be delivered only when a corresponding Intent is discovered.
type Interest interface {
	io.Closer
	Route() Route
	// C returns a channel that contains messages. Users should typecast to specific message type that
	// was registered with the interest.
	C() <-chan proto.Message
}

// InterestInternal is an interface to describe an internal interest data structure. Should not be used by users.
type InterestInternal interface {
	Interest
	Ctx() context.Context
	MsgC() chan<- proto.Message
}

type LocalInterest struct {
	ctx    context.Context
	cancel context.CancelFunc
	route  Route
	msgC   chan proto.Message
	closer func() error
	once   sync.Once
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
	if i.closer != nil {
		err := i.closer()
		if err != nil {
			return err
		}
	}
	i.once.Do(func() {
		i.cancel()
		close(i.msgC)
		i.msgC = nil
	})
	return nil
}

func (t *LocalInterest) OnClose(f func()) {
	if f == nil {
		return
	}
	go func() {
		<-t.ctx.Done()
		f()
	}()
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

// InterestRouter keeps track of same type interests and multiple subscribers.
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
	once      sync.Once
}

func NewInterestRouter(ctx context.Context, route Route, closer func() error, size int, interests ...Interest) (*InterestRouter, error) {
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
		if !i.Route().Equal(route) {
			return nil, errors.ErrInvalidRoute
		}
		if err := ret.AddInterest(i); err != nil {
			return nil, err
		}
	}

	return ret, nil
}

// Wrap returns a wrapped interest that collects messages from all registered interests.
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

// AddInterest registers an interest and sets up the routing.
func (i *InterestRouter) AddInterest(interest Interest) error {
	if !i.route.Equal(interest.Route()) {
		return errors.ErrInvalidRoute
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if idx := slices.Index(i.interests, interest); idx >= 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(i.ctx)
	i.interests = append(i.interests, interest)
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
				i.RemoveInterest(interest)
				return
			}
		}
	}
}

// routeMsg routes message received by some interest to all wrappers.
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
	if i.closer != nil {
		err := i.closer()
		if err != nil {
			return err
		}
	}
	i.once.Do(func() {
		i.cancel()
		i.wg.Wait()
		close(i.c)
	})
	return nil
}

func (i *InterestRouter) OnClose(f func()) {
	if f == nil {
		return
	}
	go func() {
		<-i.ctx.Done()
		f()
	}()
}

func (i *InterestRouter) Route() Route {
	return i.route
}

func (i *InterestRouter) C() <-chan proto.Message {
	return i.c
}
