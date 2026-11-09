package dndm

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/itohio/dndm/errors"
)

// Endpoint describes a component that can register, manage, and link intents and interests
// based on data routes. It provides methods for initialization, publishing intents, subscribing interests,
// and managing its lifecycle.
type Endpoint interface {
	io.Closer
	OnClose(func()) Endpoint
	Name() string
	// Publish will advertise an intent to publish named and typed data.
	Publish(route Route, opt ...PubOpt) (Intent, error)
	// Subscribe will advertise an interest in named and typed data.
	Subscribe(route Route, opt ...SubOpt) (Interest, error)
	// Init is used by the Router to initialize this endpoint.
	Init(ctx context.Context, logger *slog.Logger, addIntent IntentCallback, addInterest InterestCallback) error
}

// RemoteEndpoint extends Endpoint with methods to retrieve local and remote peer information.
type RemoteEndpoint interface {
	// Local returns the name of the local peer
	Local() Peer
	// Remote returns the name of the remote peer
	Remote() Peer
}

// ReadyEndpoint is implemented by endpoints that cannot publish or subscribe until some
// setup completes, such as a handshake. A Container skips such an endpoint while it is
// not ready and registers all of its current routes with it once it becomes ready.
type ReadyEndpoint interface {
	// Ready reports whether Publish and Subscribe can succeed now. Once true it stays true.
	Ready() bool
	// OnReady runs f once, on its own goroutine, after the endpoint becomes ready
	// (at once if it already is).
	OnReady(f func())
}

// Base provides basic context management functionalities for components that require
// initialization with a context, cancellation, and cleanup operations.
//
// Close callbacks registered with AddOnClose are owned by the Base: they run once,
// in registration order, when the Base closes. Close runs them in the calling
// goroutine and returns after they finish. Cancellation of the parent context runs
// them in one goroutine started by the context (context.AfterFunc). A Close that
// finds the Base already closing (another Close, a parent cancellation, or a call
// from inside a callback) cancels and returns without waiting, so callbacks may
// close their own Base without deadlocking.
type Base struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	lc     *lifecycle
}

const (
	lifecycleOpen = iota
	lifecycleClosing
	lifecycleClosed
)

// lifecycle holds the close callbacks of a Base. It is shared by copies of the Base value.
type lifecycle struct {
	mu        sync.Mutex
	state     int
	callbacks []func()
	stop      func() bool // stops the context's AfterFunc; set by Init
}

// claim moves the lifecycle from open to closing. Only the claimer runs the callbacks.
// An explicit claim also stops the context's AfterFunc, so cancelling the context does
// not start a goroutine just to lose the claim (review R-DND-R5 F6).
func (l *lifecycle) claim() bool {
	l.mu.Lock()
	if l.state != lifecycleOpen {
		l.mu.Unlock()
		return false
	}
	l.state = lifecycleClosing
	stop := l.stop
	l.stop = nil
	l.mu.Unlock()
	if stop != nil {
		stop()
	}
	return true
}

// drain runs callbacks in order, including ones added while draining, then marks the lifecycle closed.
func (l *lifecycle) drain() {
	l.mu.Lock()
	for len(l.callbacks) > 0 {
		f := l.callbacks[0]
		l.callbacks[0] = nil
		l.callbacks = l.callbacks[1:]
		l.mu.Unlock()
		f()
		l.mu.Lock()
	}
	l.callbacks = nil
	l.state = lifecycleClosed
	l.mu.Unlock()
}

// add registers f. After the lifecycle has fully closed, f runs at once in its own
// goroutine: running it inline could deadlock a caller that holds a lock f needs.
func (l *lifecycle) add(f func()) {
	l.mu.Lock()
	if l.state != lifecycleClosed {
		l.callbacks = append(l.callbacks, f)
		l.mu.Unlock()
		return
	}
	l.mu.Unlock()
	go f()
}

// NewBase initializes a new Base instance without a specific context.
func NewBase() Base {
	return Base{}
}

// NewBaseWithCtx initializes a new Base instance with the provided context.
func NewBaseWithCtx(ctx context.Context) Base {
	ret := Base{}
	ret.Init(ctx)
	return ret
}

// Init sets up the Base instance with a cancellation context.
func (t *Base) Init(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	t.ctx = ctx
	t.cancel = cancel
	if t.lc == nil {
		t.lc = &lifecycle{}
	}
	lc := t.lc
	stop := context.AfterFunc(ctx, func() {
		if lc.claim() {
			lc.drain()
		}
	})
	lc.mu.Lock()
	lc.stop = stop
	lc.mu.Unlock()
	return nil
}

// Close cleans up resources and cancels the context without a specific cause.
func (t *Base) Close() error {
	return t.CloseCause(nil)
}

// CloseCause cancels the context with a specified error cause and runs the close callbacks.
func (t *Base) CloseCause(err error) error {
	claimed := t.lc.claim()
	t.cancel(err)
	if claimed {
		t.lc.drain()
	}
	return nil
}

// Ctx returns the current context associated with the Base instance.
func (t *Base) Ctx() context.Context {
	return t.ctx
}

// AddOnClose registers a function to be called once when the Base closes.
func (t *Base) AddOnClose(f func()) {
	if f == nil {
		return
	}
	if t.lc == nil {
		t.lc = &lifecycle{}
	}
	t.lc.add(f)
}

// BaseEndpoint is a concrete implementation of the Endpoint interface that provides
// methods for endpoint initialization, managing lifecycle, and handling intents and interests.
type BaseEndpoint struct {
	Base
	name          string
	Log           *slog.Logger
	OnAddIntent   IntentCallback
	OnAddInterest InterestCallback
	Size          int
}

// NewEndpointBase creates a new BaseEndpoint with a specified name and size.
func NewEndpointBase(name string, size int) BaseEndpoint {
	return BaseEndpoint{
		Base: NewBase(),
		name: name,
		Size: size,
	}
}

// Init initializes the BaseEndpoint with necessary callbacks and logging capabilities.
func (t *BaseEndpoint) Init(ctx context.Context, logger *slog.Logger, addIntent IntentCallback, addInterest InterestCallback) error {
	if logger == nil || addIntent == nil || addInterest == nil {
		return errors.ErrBadArgument
	}
	t.Base.Init(ctx)
	t.Log = logger
	t.OnAddIntent = addIntent
	t.OnAddInterest = addInterest

	return nil
}

// Name returns the name of the endpoint.
func (t *BaseEndpoint) Name() string {
	return t.name
}

// SetName sets or updates the name of the endpoint.
func (t *BaseEndpoint) SetName(name string) {
	t.name = name
}
