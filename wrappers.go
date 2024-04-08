package dndm

import (
	"context"
	"reflect"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	"google.golang.org/protobuf/proto"
)

var _ routers.Intent = (*intentWrapper)(nil)
var _ routers.Interest = (*interestWrapper)(nil)

type intentWrapper struct {
	router  *intentRouter
	notifyC chan routers.Route
}

func (w *intentWrapper) Route() routers.Route {
	return w.router.route
}
func (w *intentWrapper) Close() error {
	return w.router.removeWrapper(w)
}
func (w *intentWrapper) Interest() <-chan routers.Route {
	return w.notifyC
}

func (w *intentWrapper) Send(ctx context.Context, msg proto.Message) error {
	return w.router.Send(ctx, msg)
}

type intentRouter struct {
	mu       sync.RWMutex
	wg       sync.WaitGroup
	ctx      context.Context
	route    routers.Route
	cancel   context.CancelFunc
	closer   func() error
	intents  []routers.Intent
	cancels  []context.CancelFunc
	wrappers []*intentWrapper
	size     int
}

func makeIntentRouter(ctx context.Context, route routers.Route, closer func() error, size int, intents ...routers.Intent) *intentRouter {
	ctx, cancel := context.WithCancel(ctx)
	ret := &intentRouter{
		ctx:    ctx,
		cancel: cancel,
		route:  route,
		size:   size,
		closer: closer,
	}
	for _, i := range intents {
		ret.addIntent(i)
	}
	return ret
}

func (i *intentRouter) wrap() *intentWrapper {
	ret := &intentWrapper{
		router:  i,
		notifyC: make(chan routers.Route, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *intentRouter) addWrapper(w *intentWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.wrappers = append(i.wrappers, w)
}

func (i *intentRouter) removeWrapper(w *intentWrapper) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	idx := slices.Index(i.wrappers, w)
	w.router = nil
	if idx >= 0 {
		i.wrappers = slices.Delete(i.wrappers, idx, idx+1)
	}
	close(w.notifyC)
	if len(i.wrappers) > 0 {
		return nil
	}

	return i.Close()
}

func (i *intentRouter) addIntent(intent routers.Intent) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.route.Equal(intent.Route()) {
		return errors.ErrInvalidRoute
	}
	ctx, cancel := context.WithCancel(i.ctx)
	i.wg.Add(1)
	go i.notifyRunner(ctx, intent)
	i.intents = append(i.intents, intent)
	i.cancels = append(i.cancels, cancel)
	return nil
}

func (i *intentRouter) removeIntent(intent routers.Intent) {
	i.mu.Lock()
	defer i.mu.Unlock()
	idx := slices.Index(i.intents, intent)
	if idx < 0 {
		return
	}
	i.cancels[idx]()
	i.intents = slices.Delete(i.intents, idx, idx+1)
	i.cancels = slices.Delete(i.cancels, idx, idx+1)
}

func (i *intentRouter) notifyRunner(ctx context.Context, intent routers.Intent) {
	defer i.wg.Done()
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ctx.Done():
			return
		case notification := <-intent.Interest():
			if err := i.notifyWrappers(ctx, notification); err != nil {
				return
			}
		}
	}
}

func (i *intentRouter) notifyWrappers(ctx context.Context, route routers.Route) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, w := range i.wrappers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-i.ctx.Done():
			return i.ctx.Err()
		case w.notifyC <- route:
			// slog.Info("SEND notify", "route", route)
		default:
			// slog.Info("SKIP notify", "route", route)
		}
	}
	return nil
}

func (i *intentRouter) Close() error {
	i.closer()
	i.cancel()
	errarr := make([]error, len(i.intents))
	for i, intent := range i.intents {
		errarr[i] = intent.Close()
	}
	i.wg.Wait()
	return errors.Join(errarr...)
}

func (i *intentRouter) Route() routers.Route {
	return i.route
}

func (i *intentRouter) Send(ctx context.Context, msg proto.Message) error {
	if reflect.TypeOf(msg) != i.route.Type() {
		return errors.ErrInvalidType
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	var wg sync.WaitGroup
	errarr := make([]error, len(i.intents))
	for i, intent := range i.intents {
		wg.Add(1)
		go func(i int, intent routers.Intent) {
			defer wg.Done()
			errarr[i] = intent.Send(ctx, msg)
		}(i, intent)
	}
	wg.Wait()
	noInterest := 0
	for i, err := range errarr {
		if errors.Is(err, errors.ErrNoInterest) {
			noInterest++
			errarr[i] = nil
		}
	}
	if noInterest == len(i.intents) {
		return errors.ErrNoInterest
	}
	return errors.Join(errarr...)
}

type interestWrapper struct {
	router *interestRouter
	c      chan proto.Message
}

func (w *interestWrapper) Close() error {
	return w.router.removeWrapper(w)
}
func (w *interestWrapper) Route() routers.Route {
	return w.router.route
}
func (w *interestWrapper) C() <-chan proto.Message {
	return w.c
}

type interestRouter struct {
	mu        sync.RWMutex
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	route     routers.Route
	closer    func() error
	c         chan proto.Message
	interests []routers.Interest
	cancels   []context.CancelFunc
	wrappers  []*interestWrapper
	size      int
}

func makeInterestRouter(ctx context.Context, route routers.Route, closer func() error, size int, interests ...routers.Interest) *interestRouter {
	ctx, cancel := context.WithCancel(ctx)
	ret := &interestRouter{
		ctx:    ctx,
		cancel: cancel,
		closer: closer,
		route:  route,
		c:      make(chan proto.Message, size),
		size:   size,
	}

	for _, i := range interests {
		ret.addInterest(i)
	}

	return ret
}

func (i *interestRouter) wrap() *interestWrapper {
	ret := &interestWrapper{
		router: i,
		c:      make(chan proto.Message, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *interestRouter) addWrapper(w *interestWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.wrappers = append(i.wrappers, w)
}

func (i *interestRouter) removeWrapper(w *interestWrapper) error {
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

func (i *interestRouter) addInterest(interest routers.Interest) error {
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

func (i *interestRouter) removeInterest(interest routers.Interest) {
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

func (i *interestRouter) recvRunner(ctx context.Context, interest routers.Interest) {
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

func (i *interestRouter) routeMsg(ctx context.Context, msg proto.Message) error {
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

func (i *interestRouter) Close() error {
	i.cancel()
	err := i.closer()
	if err != nil {
		return err
	}
	i.wg.Wait()
	return nil
}

func (i *interestRouter) Route() routers.Route {
	return i.route
}

func (i *interestRouter) C() <-chan proto.Message {
	return i.c
}
