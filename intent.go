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

// IntentCallback is a function type used for callbacks upon adding an Intent.
type IntentCallback func(intent Intent, ep Endpoint) error

// Intent is an interface that defines methods to manage data provision requests.
// It encapsulates behaviors to close the intent, listen for interest on a route,
// send messages, and execute closure callbacks.
type Intent interface {
	io.Closer
	OnClose(func()) Intent
	Route() Route
	// Interest returns a channel that contains Routes that are interested in the data indicated by the intent.
	// Users should start sending the data once an event is received on this channel.
	Interest() <-chan Route
	// Send will send a message to any recepient that indicated an interest.
	//
	// Ownership (DND-002): the message is immutable once Send is called. Neither the
	// sender nor anyone else may modify it, or anything it points to, after that point,
	// even after Send returns: local subscribers receive this same pointer and keep it as
	// long as they like, and remote endpoints may marshal it concurrently. To change a
	// message, send a new one (or proto.Clone and modify the clone).
	Send(context.Context, proto.Message) error
}

// RemoteIntent extends Intent with a method to retrieve associated peer information.
type RemoteIntent interface {
	Intent
	Peer() Peer
}

// IntentInternal extends Intent with functionalities for linking and notifications.
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
	closed  bool
	link    *linkState
}

// linkState is the current link of a LocalIntent: the channel Send delivers to, and a
// signal that the link is being removed, which releases a Send blocked on a full channel.
type linkState struct {
	c    chan<- proto.Message
	done chan struct{}
	once sync.Once
}

func (l *linkState) stop() { l.once.Do(func() { close(l.done) }) }

// NewIntent initializes a new LocalIntent with specified context, route, and buffer size for the notifications channel.
func NewIntent(ctx context.Context, route Route, size int) *LocalIntent {
	intent := &LocalIntent{
		Base:    NewBaseWithCtx(ctx),
		route:   route,
		notifyC: make(chan Route, size),
	}
	intent.AddOnClose(func() {
		// The context is already cancelled, so a blocked Send returns and releases mu.
		intent.mu.Lock()
		intent.closed = true
		intent.link = nil
		close(intent.notifyC)
		intent.mu.Unlock()
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
	defer i.mu.RUnlock()
	if i.link == nil {
		return nil
	}
	return i.link.c
}

// Send delivers msg to the linked interest's channel, waiting while it is full.
// It holds a read lock for the whole delivery, so once Link(nil) returns no Send is
// still using the old channel and its owner may close it. A Send that is waiting when
// the link is removed returns ErrNoInterest.
func (i *LocalIntent) Send(ctx context.Context, msg proto.Message) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	l := i.link
	if l == nil {
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
	case <-l.done:
		return errors.ErrNoInterest
	case l.c <- msg:
	}

	return nil
}

// Link sets the channel Send delivers to; nil unlinks. A closed intent stays unlinked.
// Link(nil) returns after every Send on the old channel has returned.
func (i *LocalIntent) Link(c chan<- proto.Message) {
	if c == nil {
		i.unlink()
		return
	}
	// Re-linking the current channel is a no-op. Check it under the read lock: Sends
	// blocked on a full channel hold that lock, and the write lock would wait for them
	// (review R-DND-R5 F8).
	i.mu.RLock()
	same := i.link != nil && i.link.c == c
	i.mu.RUnlock()
	if same {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed {
		return
	}
	if i.link != nil {
		if i.link.c != c {
			panic("Link called multiple times illegally")
		}
		return
	}
	i.link = &linkState{c: c, done: make(chan struct{})}
}

func (i *LocalIntent) unlink() {
	i.mu.RLock()
	l := i.link
	i.mu.RUnlock()
	if l == nil {
		return
	}
	l.stop() // release Sends waiting on a full channel; they hold the read lock
	i.mu.Lock()
	if i.link == l {
		i.link = nil
	}
	i.mu.Unlock()
}

// Notify queues an interest notification without blocking; it is a no-op once the intent is closed.
func (i *LocalIntent) Notify() {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.closed {
		return
	}
	select {
	case i.notifyC <- i.route:
	default:
	}
}

// FanOutIntent represents an Intent that manages multiple underlying intents.
// It distributes incoming messages across all registered intents and synchronizes their lifecycle.
type FanOutIntent struct {
	li       *LocalIntent
	mu       sync.RWMutex
	intents  []Intent
	cancels  []context.CancelFunc
	wg       sync.WaitGroup
	onNotify func()
}

// NewFanOutIntent creates a FanOutIntent with given context, route, size, and optional initial intents.
func NewFanOutIntent(ctx context.Context, route Route, size int, intents ...Intent) (*FanOutIntent, error) {
	ret := newFanOutIntent(ctx, route, size)
	if err := ret.addIntents(intents); err != nil {
		return nil, err
	}
	return ret, nil
}

// newFanOutIntent builds the fan-out without intents, so callers can set onNotify
// before AddIntent starts the notify runners that read it.
func newFanOutIntent(ctx context.Context, route Route, size int) *FanOutIntent {
	ret := &FanOutIntent{
		li: NewIntent(ctx, route, size),
	}
	ret.li.AddOnClose(ret.waitRunners)
	return ret
}

func (i *FanOutIntent) addIntents(intents []Intent) error {
	for _, intent := range intents {
		if err := i.AddIntent(intent); err != nil {
			i.Close()
			return err
		}
	}
	return nil
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
	i.waitRunners()
	return err
}

// waitRunners waits for the notify runners. The context is already cancelled, so
// AddIntent refuses new runners; taking i.mu orders any AddIntent that passed its
// check before Wait (WaitGroup.Add must not race Wait).
func (i *FanOutIntent) waitRunners() {
	i.mu.Lock()
	i.mu.Unlock() //nolint:staticcheck // empty critical section is the ordering point
	i.wg.Wait()
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
// Adding an intent that is already present is a no-op; adding to a closed fan-out returns ErrClosed.
func (i *FanOutIntent) AddIntent(intent Intent) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.Route().Equal(intent.Route()) {
		return errors.ErrInvalidRoute
	}
	if i.li.Ctx().Err() != nil {
		return errors.ErrClosed
	}
	if slices.Contains(i.intents, intent) {
		return nil
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
	idx := slices.Index(i.intents, intent)
	if idx < 0 {
		i.mu.Unlock()
		return
	}
	i.cancels[idx]()
	i.intents = slices.Delete(i.intents, idx, idx+1)
	i.cancels = slices.Delete(i.cancels, idx, idx+1)
	last := len(i.intents) == 0
	i.mu.Unlock()

	if last {
		i.Close()
	}
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

// Send forwards to the router. A closed wrapper returns ErrClosed.
func (w *intentWrapper) Send(ctx context.Context, msg proto.Message) error {
	if w.Ctx().Err() != nil {
		return errors.ErrClosed
	}
	return w.router.Send(ctx, msg)
}

// IntentRouter manages a collection of intents, routing messages and notifications among them.
type IntentRouter struct {
	*FanOutIntent
	size     int
	mu       sync.RWMutex
	wrappers []*intentWrapper
	notified bool // an interest notification arrived; later wrappers start with one
}

// NewIntentRouter creates a new IntentRouter with a given context, route, size, and optionally pre-registered intents.
func NewIntentRouter(ctx context.Context, route Route, size int, intents ...Intent) (*IntentRouter, error) {
	ret := &IntentRouter{
		FanOutIntent: newFanOutIntent(ctx, route, size),
		size:         size,
	}
	ret.onNotify = ret.Notify
	if err := ret.addIntents(intents); err != nil {
		return nil, err
	}
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
	if i.notified {
		// The router already saw interest before this wrapper existed; without this the
		// wrapper would wait for a notification that was delivered to nobody.
		select {
		case w.notifyC <- i.Route():
		default:
		}
	}
}

func (i *IntentRouter) removeWrapper(w *intentWrapper) error {
	i.mu.Lock()
	idx := slices.Index(i.wrappers, w)
	if idx >= 0 {
		i.wrappers = slices.Delete(i.wrappers, idx, idx+1)
	}
	last := len(i.wrappers) == 0
	i.mu.Unlock()
	if !last {
		return nil
	}
	// Close outside i.mu: closing waits for notify runners, which take i.mu in Notify.
	return i.Close()
}

func (i *IntentRouter) Notify() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.notified = true
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
