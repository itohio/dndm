package pipe

import (
	"container/ring"
	"context"
	"io"
	"log/slog"
	sync "sync"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
	"github.com/itohio/dndm/router/direct"
	"github.com/itohio/dndm/router/pipe/types"
)

var _ router.Transport = (*Transport)(nil)

type Dialer interface {
	io.Closer
	Dial(id ...string) error
	Undial(id ...string) error
}

type RemoteIntent struct {
	*direct.Intent
	remoteId string
	reader   io.Reader
}

func (r *RemoteIntent) RemoteID() string { return r.remoteId }

type RemoteInterest struct {
	*direct.Interest
	remoteId string
	writer   io.Writer
}

func (r *RemoteInterest) RemoteID() string { return r.remoteId }

type RemoteId interface {
	RemoteID() string
}

type Transport struct {
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	log            *slog.Logger
	name           string
	reader         io.Reader
	writer         io.Writer
	dialer         Dialer
	addCallback    func(interest router.Interest, t router.Transport) error
	removeCallback func(interest router.Interest, t router.Transport) error
	size           int

	mu        sync.Mutex
	intents   map[string]router.IntentInternal
	interests map[string]router.InterestInternal
	links     map[string]*direct.Link

	pingDuration time.Duration
	pingNonce    atomic.Uint64
	pingMu       sync.Mutex
	pingRing     *ring.Ring
	pongRing     *ring.Ring

	messageQueue chan []byte
	remote       *types.Handshake
}

func New(name string, r io.Reader, w io.Writer, d Dialer, size int) *Transport {
	return &Transport{
		name:         name,
		size:         size,
		reader:       r,
		writer:       w,
		dialer:       d,
		pingDuration: time.Second * 3,
		intents:      make(map[string]router.IntentInternal),
		interests:    make(map[string]router.InterestInternal),
		messageQueue: make(chan []byte, 1024),
		pingRing:     ring.New(3),
		pongRing:     ring.New(3),
	}
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest router.Interest, t router.Transport) error) error {
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

func (t *Transport) Publish(route router.Route) (router.Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	intent, err := t.setIntent(route, "")
	if err != nil {
		return nil, err
	}

	if interest, ok := t.interests[route.String()]; ok {
		t.link(route, intent, interest)
	}

	return intent, nil
}

func (t *Transport) setIntent(route router.Route, remoteId string) (router.IntentInternal, error) {
	if intent, ok := t.intents[route.String()]; ok {
		return intent, nil
	}

	// Do not allow local interests
	// FIXME: do not allow remote intents and remote interests
	if interest, ok := t.interests[route.String()]; ok {
		if _, ok := interest.(*RemoteInterest); remoteId == "" && !ok {
			return nil, errors.ErrLocalInterest
		}
	}

	pi := direct.NewIntent(t.ctx, route, t.size, func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.unlink(route)
		delete(t.intents, route.String())
		return nil
	})

	if remoteId == "" {
		t.intents[route.String()] = pi
		return pi, nil
	}

	ri := RemoteIntent{
		Intent:   pi,
		remoteId: remoteId,
		reader:   t.reader,
	}
	t.intents[route.String()] = ri
	return ri, nil
}

func (t *Transport) setInterest(route router.Route, remoteId string) (router.InterestInternal, error) {
	if interest, ok := t.interests[route.String()]; ok {
		if link, ok := t.links[route.String()]; ok {
			link.Notify()
		}
		return interest, nil
	}

	// Do not allow local intents and local interests
	// FIXME: do not allow remote intents and remote interests
	if intent, ok := t.intents[route.String()]; ok {
		if _, ok := intent.(*RemoteIntent); remoteId == "" && !ok {
			return nil, errors.ErrLocalIntent
		}
	}

	var interest router.Interest
	pi := direct.NewInterest(t.ctx, route, t.size, func() error {
		t.mu.Lock()
		t.unlink(route)
		delete(t.interests, route.String())
		t.mu.Unlock()
		return t.removeCallback(interest, t)
	})
	interest = pi

	if remoteId == "" {
		t.interests[route.String()] = pi
		return pi, nil
	}

	ri := RemoteInterest{
		Interest: pi,
		remoteId: remoteId,
		writer:   t.writer,
	}
	interest = ri
	t.interests[route.String()] = ri
	return ri, nil
}

func (t *Transport) link(route router.Route, intent router.IntentInternal, interest router.InterestInternal) {
	_, localIntent := intent.(*direct.Intent)
	_, localInterest := interest.(*direct.Interest)

	// Do not link local&local and remote&remote
	if localIntent == localInterest {
		return
	}
	link := direct.NewLink(t.ctx, intent, interest, func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.unlink(route)
		return nil
	})
	t.links[route.String()] = link
	link.Link()
}

func (t *Transport) unlink(route router.Route) {
	link, ok := t.links[route.String()]
	if !ok {
		return
	}
	link.Unlink()
	delete(t.links, route.String())
}

func (t *Transport) Subscribe(route router.Route) (router.Interest, error) {
	interest, err := t.subscribe(route)
	if err != nil {
		return nil, err
	}
	t.addCallback(interest, t)
	return interest, nil
}

func (t *Transport) subscribe(route router.Route) (router.Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	interest, err := t.setInterest(route, "")
	if err != nil {
		return nil, err
	}

	if intent, ok := t.intents[route.String()]; ok {
		t.link(route, intent, interest)
	}

	return interest, nil
}
