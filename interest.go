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
	_ Interest         = (*FanInInterest)(nil)
	_ Interest         = (*InterestRouter)(nil)
	_ Interest         = (*interestWrapper)(nil)
)

// InterestCallback is a function type used for callbacks upon adding an Interest.
type InterestCallback func(interest Interest, ep Endpoint) error

// Interest defines the behavior for components interested in receiving named data.
// It allows closing of the interest, setting closure callbacks, and accessing routes and message channels.
//
// User should consume C of the interest until it is closed or no longer needed.
// Messages will be delivered only when a corresponding Intent is discovered.
type Interest interface {
	io.Closer
	OnClose(func()) Interest
	Route() Route
	// C returns a channel that contains messages. Users should typecast to specific message type that
	// was registered with the interest.
	C() <-chan proto.Message
}

// RemoteInterest extends Interest with the ability to retrieve the peer involved in the interest.
type RemoteInterest interface {
	Interest
	Peer() Peer
}

// InterestInternal extends Interest with additional internal management capabilities.
type InterestInternal interface {
	Interest
	Ctx() context.Context
	MsgC() chan<- proto.Message
}

// LocalInterest manages a local interest for receiving data based on a specific route.
type LocalInterest struct {
	Base
	route Route
	msgC  chan proto.Message
}

// NewInterest creates a new LocalInterest with a specified context, route, and buffer size.
func NewInterest(ctx context.Context, route Route, size int) *LocalInterest {
	ret := &LocalInterest{
		Base:  NewBase(),
		route: route,
		msgC:  make(chan proto.Message, size),
	}
	ret.Init(ctx)
	ret.OnClose(func() {
		ret.cancel(nil)
		close(ret.msgC)
	})
	return ret
}

func (t *LocalInterest) OnClose(f func()) Interest {
	t.AddOnClose(f)
	return t
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

// FanInInterest aggregates multiple interests, routing incoming messages to a single channel.
type FanInInterest struct {
	li        *LocalInterest
	mu        sync.RWMutex
	wg        sync.WaitGroup
	interests []Interest
	cancels   []context.CancelFunc
	onRecv    func(ctx context.Context, msg proto.Message) error
}

// NewFanInInterest creates a new FanInInterest with specified context, route, size, and initial interests.
func NewFanInInterest(ctx context.Context, route Route, size int, interests ...Interest) (*FanInInterest, error) {
	ret := &FanInInterest{
		li: NewInterest(ctx, route, size),
	}
	for _, i := range interests {
		if err := ret.AddInterest(i); err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (i *FanInInterest) Close() error {
	err := i.li.Close()
	i.wg.Wait()
	return err
}

func (i *FanInInterest) OnClose(f func()) Interest {
	i.li.AddOnClose(f)
	return i
}

func (i *FanInInterest) Route() Route {
	return i.li.Route()
}

func (i *FanInInterest) C() <-chan proto.Message {
	return i.li.C()
}

func (i *FanInInterest) Ctx() context.Context {
	return i.li.Ctx()
}

// AddInterest registers an interest and sets up the routing.
func (i *FanInInterest) AddInterest(interest Interest) error {
	if !i.Route().Equal(interest.Route()) {
		return errors.ErrInvalidRoute
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if idx := slices.Index(i.interests, interest); idx >= 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(i.Ctx())
	i.interests = append(i.interests, interest)
	i.cancels = append(i.cancels, cancel)
	i.wg.Add(1)
	interest.OnClose(cancel)
	go i.recvRunner(ctx, interest)
	return nil
}

func (i *FanInInterest) RemoveInterest(interest Interest) {
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

func (i *FanInInterest) recvRunner(ctx context.Context, interest Interest) {
	defer i.wg.Done()
	for {
		select {
		case <-i.Ctx().Done():
			return
		case <-ctx.Done():
			return
		case msg := <-interest.C():

			if i.onRecv != nil {
				err := i.onRecv(ctx, msg)
				if err != nil {
					i.RemoveInterest(interest)
					return
				}
				continue
			}

			select {
			case <-i.Ctx().Done():
				return
			case <-ctx.Done():
				return
			case i.li.MsgC() <- msg:
			}
		}
	}
}

type interestWrapper struct {
	Base
	router *InterestRouter
	c      chan proto.Message
}

func (w *interestWrapper) OnClose(f func()) Interest {
	w.Base.AddOnClose(f)
	return w
}
func (w *interestWrapper) Route() Route {
	return w.router.Route()
}
func (w *interestWrapper) C() <-chan proto.Message {
	return w.c
}

// InterestRouter manages a collection of interests, directing incoming messages to multiple subscribers.
type InterestRouter struct {
	*FanInInterest
	size     int
	mu       sync.RWMutex
	wrappers []*interestWrapper
}

// NewInterestRouter initializes a new InterestRouter with a context, route, size, and initial interests.
func NewInterestRouter(ctx context.Context, route Route, size int, interests ...Interest) (*InterestRouter, error) {
	fii, err := NewFanInInterest(ctx, route, size, interests...)
	if err != nil {
		return nil, err
	}
	ret := &InterestRouter{
		FanInInterest: fii,
		size:          size,
	}
	fii.onRecv = ret.routeMsg

	return ret, nil
}

// Wrap returns a wrapped interest that collects messages from all registered interests.
func (i *InterestRouter) Wrap() *interestWrapper {
	ret := &interestWrapper{
		Base:   NewBaseWithCtx(i.Ctx()),
		router: i,
		c:      make(chan proto.Message, i.size),
	}

	i.addWrapper(ret)

	return ret
}

func (i *InterestRouter) addWrapper(w *interestWrapper) {
	i.mu.Lock()
	defer i.mu.Unlock()
	w.OnClose(func() {
		i.removeWrapper(w)
		close(w.c)
	})
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

	if len(i.wrappers) > 0 {
		return nil
	}

	return i.Close()
}

// routeMsg routes message received by some interest to all wrappers.
func (i *InterestRouter) routeMsg(ctx context.Context, msg proto.Message) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, w := range i.wrappers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-i.Ctx().Done():
			return i.Ctx().Err()
		case w.c <- msg:
		default:
		}
	}
	return nil
}
