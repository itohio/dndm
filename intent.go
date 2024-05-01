package dndm

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

var (
	_ Intent         = (*LocalIntent)(nil)
	_ IntentInternal = (*LocalIntent)(nil)
	_ Intent         = (*FanOutIntent)(nil)
	_ Intent         = (*IntentRouter)(nil)
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

// RemoteIntent interface represents remote intent.
type RemoteIntent interface {
	Intent
	Peer() Peer
}

// IntentInternal interface extends an intent that can be linked with an interest.
// This interface is used internally by endpoints.
type IntentInternal interface {
	Intent
	Link(chan<- proto.Message)
	Notify()
	Ctx() context.Context
}

// LocalIntent represents a simple intent that is local to the process.
// LocalIntent can be linked with LocalInterest or RemoteInterest.
type LocalIntent struct {
	Base
	route   Route
	notifyC chan Route
	mu      sync.RWMutex
	linkedC chan<- proto.Message
}

func NewIntent(ctx context.Context, route Route, size int) *LocalIntent {
	intent := &LocalIntent{
		Base:    NewBaseWithCtx(ctx),
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
	t.Base.AddOnClose(f)
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

// FanOutIntent is a Intent collection that sends out intents and messages to other intents belonging to different endpoints.
type FanOutIntent struct {
	li       *LocalIntent
	mu       sync.RWMutex
	intents  []Intent
	cancels  []context.CancelFunc
	wg       sync.WaitGroup
	onNotify func()
}

func NewFanOutIntent(ctx context.Context, route Route, size int, intents ...Intent) (*FanOutIntent, error) {
	ret := &FanOutIntent{
		li: NewIntent(ctx, route, size),
	}
	for _, i := range intents {
		if err := ret.AddIntent(i); err != nil {
			return nil, err
		}
	}
	ret.li.AddOnClose(func() {
		ret.wg.Wait()
	})
	return ret, nil
}

func (i *FanOutIntent) OnClose(f func()) Intent {
	i.li.Base.AddOnClose(f)
	return i
}

func (i *FanOutIntent) Route() Route {
	return i.li.Route()
}

func (i *FanOutIntent) Interest() <-chan Route {
	return i.li.Interest()
}

func (i *FanOutIntent) Close() error {
	err := i.li.Close()
	i.wg.Wait()
	return err
}

func (i *FanOutIntent) Ctx() context.Context {
	return i.li.ctx
}

func (i *FanOutIntent) Send(ctx context.Context, msg proto.Message) error {
	if reflect.TypeOf(msg) != i.Route().Type() {
		return errors.ErrInvalidType
	}
	select {
	case <-i.li.Ctx().Done():
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

// AddIntent adds a new intent of the same type, creates a notify runner and links wrappers to it.
func (i *FanOutIntent) AddIntent(intent Intent) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.Route().Equal(intent.Route()) {
		return errors.ErrInvalidRoute
	}
	ctx, cancel := context.WithCancel(i.li.Ctx())
	i.intents = append(i.intents, intent)
	i.cancels = append(i.cancels, cancel)
	i.wg.Add(1)
	intent.OnClose(cancel)
	go i.notifyRunner(ctx, intent)
	return nil
}

func (i *FanOutIntent) RemoveIntent(intent Intent) {
	i.mu.Lock()
	defer i.mu.Unlock()
	idx := slices.Index(i.intents, intent)
	if idx < 0 {
		return
	}
	i.cancels[idx]()
	i.intents = slices.Delete(i.intents, idx, idx+1)
	i.cancels = slices.Delete(i.cancels, idx, idx+1)

	if len(i.intents) > 0 {
		return
	}

	i.Close()
}

func (i *FanOutIntent) notifyRunner(ctx context.Context, intent Intent) {
	defer i.wg.Done()
	for {
		select {
		case <-i.li.Ctx().Done():
			return
		case <-ctx.Done():
			return
		case <-intent.Interest():
			if i.onNotify != nil {
				i.onNotify()
				continue
			}
			i.Notify()
		}
	}
}

func (i *FanOutIntent) Notify() {
	slog.Info("FanOutIntent", "route", i.Route())
	i.li.Notify()
}

// intentWrapper is a wrapper over IntentRouter that acts as multiple-in-multiple-out between multiple user-facing intents
// and e.g. a collection of same intents that belong to different endpoints.
type intentWrapper struct {
	Base
	router  *IntentRouter
	notifyC chan Route
}

func (w *intentWrapper) Route() Route {
	return w.router.Route()
}
func (w *intentWrapper) OnClose(f func()) Intent {
	w.Base.AddOnClose(f)
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
	*FanOutIntent
	size     int
	mu       sync.RWMutex
	wrappers []*intentWrapper
}

func NewIntentRouter(ctx context.Context, route Route, size int, intents ...Intent) (*IntentRouter, error) {
	foi, err := NewFanOutIntent(ctx, route, size, intents...)
	if err != nil {
		return nil, err
	}
	ret := &IntentRouter{
		FanOutIntent: foi,
		size:         size,
	}
	foi.onNotify = ret.Notify
	return ret, nil
}

// Wrap returns a wrapped intent. Messages sent to this wrapped intent will be sent to all the registered intents.
func (i *IntentRouter) Wrap() *intentWrapper {
	ret := &intentWrapper{
		Base:    NewBaseWithCtx(i.li.Ctx()),
		router:  i,
		notifyC: make(chan Route, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *IntentRouter) addWrapper(w *intentWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	w.AddOnClose(func() {
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

func (i *IntentRouter) Notify() {
	i.mu.RLock()
	defer i.mu.RUnlock()
	route := i.Route()
	slog.Info("Notify wrappers", "route", route)
	for _, w := range i.wrappers {
		select {
		case <-i.li.Ctx().Done():
			return
		case w.notifyC <- route:
			// slog.Info("SEND notify", "route", route)
		default:
			// slog.Info("SKIP notify", "route", route)
		}
	}
}
