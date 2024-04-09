package p2p

import (
	"container/ring"
	"context"
	"io"
	"log/slog"
	sync "sync"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

var _ routers.Transport = (*Transport)(nil)

type Remote interface {
	io.Closer
	Id() string
	Read(ctx context.Context) (*types.Header, proto.Message, error)
	Write(ctx context.Context, route routers.Route, msg proto.Message) error
}

type Transport struct {
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	log            *slog.Logger
	name           string
	dialer         Remote
	addCallback    func(interest routers.Interest, t routers.Transport) error
	removeCallback func(interest routers.Interest, t routers.Transport) error
	size           int

	mu        sync.Mutex
	timeout   time.Duration
	intents   map[string]routers.IntentInternal
	interests map[string]routers.InterestInternal
	links     map[string]*routers.Link

	pingDuration time.Duration
	pingMu       sync.Mutex
	pingRing     *ring.Ring
	pongRing     *ring.Ring

	nonce atomic.Uint64
}

// New creates a transport that communicates with a remote via Remote interface.
func New(name string, remote Remote, size int, timeout time.Duration) *Transport {
	return &Transport{
		name:         name,
		size:         size,
		dialer:       remote,
		pingDuration: time.Second * 3,
		timeout:      timeout,
		intents:      make(map[string]routers.IntentInternal),
		interests:    make(map[string]routers.InterestInternal),
		pingRing:     ring.New(3),
		pongRing:     ring.New(3),
	}
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.log = logger
	t.addCallback = add
	t.removeCallback = remove
	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel

	t.wg.Add(2)
	go t.messageHandler()
	go t.messageSender(t.pingDuration)
	return nil
}

func (t *Transport) Close() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Transport) Name() string {
	return t.name
}

func (t *Transport) Publish(route routers.Route) (routers.Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.publish(route, nil)
}

func (t *Transport) publish(route routers.Route, remote *types.Intent) (routers.Intent, error) {
	intent, err := t.setIntent(route, remote)
	if err != nil {
		return nil, err
	}

	if interest, ok := t.interests[route.String()]; ok {
		t.link(route, intent, interest)
	}

	return intent, nil
}

func (t *Transport) setIntent(route routers.Route, remote *types.Intent) (routers.IntentInternal, error) {
	if intent, ok := t.intents[route.String()]; ok {
		return intent, nil
	}

	// Do not allow local interests
	// FIXME: do not allow remote intents and remote interests
	if interest, ok := t.interests[route.String()]; ok {
		if _, ok := interest.(*RemoteInterest); remote == nil && !ok {
			return nil, errors.ErrLocalIntent
		}
	}

	pi := routers.NewIntent(t.ctx, route, t.size, func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.unlink(route)
		delete(t.intents, route.String())
		return nil
	})

	if remote == nil {
		t.intents[route.String()] = pi
		return pi, nil
	}

	ri := wrapIntent(pi, remote)
	t.intents[route.String()] = ri
	return ri, nil
}

func (t *Transport) setInterest(route routers.Route, remote *types.Interest) (routers.InterestInternal, error) {
	if interest, ok := t.interests[route.String()]; ok {
		if link, ok := t.links[route.String()]; ok {
			link.Notify()
		}
		return interest, nil
	}

	// Do not allow local intents and local interests
	// FIXME: do not allow remote intents and remote interests
	if intent, ok := t.intents[route.String()]; ok {
		if _, ok := intent.(*RemoteIntent); remote == nil && !ok {
			return nil, errors.ErrLocalIntent
		}
	}

	var interest routers.Interest
	pi := routers.NewInterest(t.ctx, route, t.size, func() error {
		t.mu.Lock()
		t.unlink(route)
		delete(t.interests, route.String())
		t.mu.Unlock()
		return t.removeCallback(interest, t)
	})
	interest = pi

	if remote == nil {
		t.interests[route.String()] = pi
		return pi, nil
	}

	ri := wrapInterest(pi, remote)
	interest = ri
	t.interests[route.String()] = ri
	return ri, nil
}

func (t *Transport) link(route routers.Route, intent routers.IntentInternal, interest routers.InterestInternal) {
	_, localIntent := intent.(*routers.LocalIntent)
	_, localInterest := interest.(*routers.LocalInterest)

	// Do not link local&local and remote&remote
	if localIntent == localInterest {
		return
	}
	link := routers.NewLink(t.ctx, intent, interest, func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.unlink(route)
		return nil
	})
	t.links[route.String()] = link
	link.Link()
}

func (t *Transport) unlink(route routers.Route) {
	link, ok := t.links[route.String()]
	if !ok {
		return
	}
	link.Unlink()
	delete(t.links, route.String())
}

func (t *Transport) Subscribe(route routers.Route) (routers.Interest, error) {
	interest, err := t.subscribe(route, nil)
	if err != nil {
		return nil, err
	}
	t.addCallback(interest, t)
	return interest, nil
}

func (t *Transport) subscribe(route routers.Route, remote *types.Interest) (routers.Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	interest, err := t.setInterest(route, remote)
	if err != nil {
		return nil, err
	}

	if intent, ok := t.intents[route.String()]; ok {
		t.link(route, intent, interest)
	}

	return interest, nil
}
