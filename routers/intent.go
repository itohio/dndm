package routers

import (
	"context"
	"reflect"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

type LocalIntent struct {
	ctx    context.Context
	cancel context.CancelFunc
	route  Route
	// msgC    chan proto.Message
	notifyC chan Route
	closer  func() error

	mu      sync.RWMutex
	linkedC chan<- proto.Message
}

func NewIntent(ctx context.Context, route Route, size int, closer func() error) *LocalIntent {
	ctx, cancel := context.WithCancel(ctx)
	intent := &LocalIntent{
		ctx:     ctx,
		cancel:  cancel,
		route:   route,
		notifyC: make(chan Route, size),
		closer:  closer,
	}
	return intent
}

func (i *LocalIntent) Ctx() context.Context {
	return i.ctx
}

func (i *LocalIntent) Close() error {
	err := i.closer()
	if err != nil {
		return err
	}
	i.cancel()
	close(i.notifyC)
	return nil
}

func (i *LocalIntent) Route() Route {
	return i.route
}

func (i *LocalIntent) Interest() <-chan Route {
	return i.notifyC
}

func (i *LocalIntent) Send(ctx context.Context, msg proto.Message) error {
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

func (i *LocalIntent) Link(c chan<- proto.Message) {
	i.mu.Lock()
	i.linkedC = c
	i.mu.Unlock()
}

func (i *LocalIntent) Notify() {
	select {
	case i.notifyC <- i.route:
	default:
	}
}

type intentWrapper struct {
	router  *IntentRouter
	notifyC chan Route
}

func (w *intentWrapper) Route() Route {
	return w.router.route
}
func (w *intentWrapper) Close() error {
	return w.router.removeWrapper(w)
}
func (w *intentWrapper) Interest() <-chan Route {
	return w.notifyC
}

func (w *intentWrapper) Send(ctx context.Context, msg proto.Message) error {
	return w.router.Send(ctx, msg)
}

type IntentRouter struct {
	mu       sync.RWMutex
	wg       sync.WaitGroup
	ctx      context.Context
	route    Route
	cancel   context.CancelFunc
	closer   func() error
	intents  []Intent
	cancels  []context.CancelFunc
	wrappers []*intentWrapper
	size     int
}

func NewIntentRouter(ctx context.Context, route Route, closer func() error, size int, intents ...Intent) *IntentRouter {
	ctx, cancel := context.WithCancel(ctx)
	ret := &IntentRouter{
		ctx:    ctx,
		cancel: cancel,
		route:  route,
		size:   size,
		closer: closer,
	}
	for _, i := range intents {
		ret.AddIntent(i)
	}
	return ret
}

func (i *IntentRouter) Wrap() *intentWrapper {
	ret := &intentWrapper{
		router:  i,
		notifyC: make(chan Route, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *IntentRouter) addWrapper(w *intentWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.wrappers = append(i.wrappers, w)
}

func (i *IntentRouter) removeWrapper(w *intentWrapper) error {
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

func (i *IntentRouter) AddIntent(intent Intent) error {
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

func (i *IntentRouter) RemoveIntent(intent Intent) {
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

func (i *IntentRouter) notifyRunner(ctx context.Context, intent Intent) {
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

func (i *IntentRouter) notifyWrappers(ctx context.Context, route Route) error {
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

func (i *IntentRouter) Close() error {
	i.closer()
	i.cancel()
	errarr := make([]error, len(i.intents))
	for i, intent := range i.intents {
		errarr[i] = intent.Close()
	}
	i.wg.Wait()
	return errors.Join(errarr...)
}

func (i *IntentRouter) Route() Route {
	return i.route
}

func (i *IntentRouter) Send(ctx context.Context, msg proto.Message) error {
	if reflect.TypeOf(msg) != i.route.Type() {
		return errors.ErrInvalidType
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	var wg sync.WaitGroup
	errarr := make([]error, len(i.intents))
	for i, intent := range i.intents {
		wg.Add(1)
		go func(i int, intent Intent) {
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