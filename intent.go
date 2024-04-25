package dndm

import (
	"context"
	"io"
	"reflect"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

var (
	_ Intent         = (*LocalIntent)(nil)
	_ IntentInternal = (*LocalIntent)(nil)
	_ Intent         = (*intentWrapper)(nil)
)

type IntentCallback func(intent Intent, ep Endpoint) error

// Intent is an interface to describe an intent to provide named data.
// Users can consume Interest channel to determine if it is worthwhile to send any data.
type Intent interface {
	io.Closer
	OnClose(func()) Intent
	Route() Route
	// Interest returns a channel that contains Routes that are interested in the data indicated by the intent.
	// Users should start sending the data once an event is received on this channel.
	Interest() <-chan Route
	// Send will send a message to any recepient that indicated an interest.
	Send(context.Context, proto.Message) error
}
type IntentInternal interface {
	Intent
	Link(chan<- proto.Message)
	Notify()
	Ctx() context.Context
}

type LocalIntent struct {
	BaseCtx
	route   Route
	notifyC chan Route
	mu      sync.RWMutex
	linkedC chan<- proto.Message
}

func NewIntent(ctx context.Context, route Route, size int) *LocalIntent {
	intent := &LocalIntent{
		BaseCtx: NewBaseCtxWithCtx(ctx),
		route:   route,
		notifyC: make(chan Route, size),
	}
	intent.AddOnClose(func() {
		close(intent.notifyC)
		intent.linkedC = nil
	})
	return intent
}

func (t *LocalIntent) OnClose(f func()) Intent {
	t.BaseCtx.AddOnClose(f)
	return t
}

func (i *LocalIntent) Route() Route {
	return i.route
}

func (i *LocalIntent) Interest() <-chan Route {
	return i.notifyC
}

// LinkedC is used for internal debugging and race condition hunting
func (i *LocalIntent) LinkedC() chan<- proto.Message {
	i.mu.RLock()
	linkedC := i.linkedC
	i.mu.RUnlock()
	return linkedC
}

func (i *LocalIntent) Send(ctx context.Context, msg proto.Message) error {
	i.mu.RLock()
	linkedC := i.linkedC
	i.mu.RUnlock()
	if linkedC == nil {
		return errors.ErrNoInterest
	}
	if i.route.Type() != nil {
		if reflect.TypeOf(msg) != i.route.Type() {
			return errors.ErrInvalidType
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-i.Ctx().Done():
		return i.Ctx().Err()
	case linkedC <- msg:
	}

	return nil
}

func (i *LocalIntent) Link(c chan<- proto.Message) {
	i.mu.Lock()
	if i.linkedC != nil && c != nil && i.linkedC != c {
		i.mu.Unlock()
		panic("Link called multiple times illegally")
	}
	i.linkedC = c
	i.mu.Unlock()
}

func (i *LocalIntent) Notify() {
	select {
	case <-i.Ctx().Done():
		return
	case i.notifyC <- i.route:
	default:
	}
}

type intentWrapper struct {
	BaseCtx
	router  *IntentRouter
	notifyC chan Route
}

func (w *intentWrapper) Route() Route {
	return w.router.route
}
func (w *intentWrapper) OnClose(f func()) Intent {
	w.BaseCtx.AddOnClose(f)
	return w
}
func (w *intentWrapper) Interest() <-chan Route {
	return w.notifyC
}

func (w *intentWrapper) Send(ctx context.Context, msg proto.Message) error {
	return w.router.Send(ctx, msg)
}

// IntentRouter keeps track of same type intents from different endpoints and multiple publishers.
type IntentRouter struct {
	BaseCtx
	mu       sync.RWMutex
	wg       sync.WaitGroup
	route    Route
	intents  []Intent
	cancels  []context.CancelFunc
	wrappers []*intentWrapper
	size     int
}

func NewIntentRouter(ctx context.Context, route Route, size int, intents ...Intent) (*IntentRouter, error) {
	ret := &IntentRouter{
		BaseCtx: NewBaseCtxWithCtx(ctx),
		route:   route,
		size:    size,
	}
	for _, i := range intents {
		if err := ret.AddIntent(i); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// Wrap returns a wrapped intent. Messages sent to this wrapped intent will be sent to all the registered intents.
func (i *IntentRouter) Wrap() *intentWrapper {
	ret := &intentWrapper{
		BaseCtx: NewBaseCtxWithCtx(i.ctx),
		router:  i,
		notifyC: make(chan Route, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *IntentRouter) addWrapper(w *intentWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.AddOnClose(func() {
		i.removeWrapper(w)
		close(w.notifyC)
	})
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
	if len(i.wrappers) > 0 {
		return nil
	}

	return i.Close()
}

// AddIntent adds a new intent of the same type, creates a notify runner and links wrappers to it.
func (i *IntentRouter) AddIntent(intent Intent) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.route.Equal(intent.Route()) {
		return errors.ErrInvalidRoute
	}
	ctx, cancel := context.WithCancel(i.ctx)
	i.intents = append(i.intents, intent)
	i.cancels = append(i.cancels, cancel)
	i.wg.Add(1)
	go i.notifyRunner(ctx, intent)
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
		case <-i.Ctx().Done():
			return
		case <-ctx.Done():
			return
		case notification := <-intent.Interest():
			_ = notification
			if err := i.notifyWrappers(ctx, notification); err != nil {
				i.RemoveIntent(intent)
				return
			}
		}
	}
}

// notifyWrappers notifies all the registered wrappers
func (i *IntentRouter) notifyWrappers(ctx context.Context, route Route) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, w := range i.wrappers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-i.Ctx().Done():
			return i.Ctx().Err()
		case w.notifyC <- route:
			// slog.Info("SEND notify", "route", route)
		default:
			// slog.Info("SKIP notify", "route", route)
		}
	}
	return nil
}

func (i *IntentRouter) Close() error {
	i.AddOnClose(func() { i.wg.Wait() })
	i.Close()
	return nil
}

func (i *IntentRouter) OnClose(f func()) *IntentRouter {
	i.AddOnClose(f)
	return i
}

func (i *IntentRouter) Route() Route {
	return i.route
}

func (i *IntentRouter) Send(ctx context.Context, msg proto.Message) error {
	if reflect.TypeOf(msg) != i.route.Type() {
		return errors.ErrInvalidType
	}
	select {
	case <-i.Ctx().Done():
		return errors.ErrClosed
	default:
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	var wg sync.WaitGroup
	errarr := make([]error, len(i.intents))
	for i, intent := range i.intents {
		wg.Add(1)
		go func(i int, intent Intent) {
			errarr[i] = intent.Send(ctx, msg)
			wg.Done()
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
