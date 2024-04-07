package direct

import (
	"context"
	"log/slog"
	"sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
)

var _ router.Transport = (*Transport)(nil)

type Transport struct {
	mu             sync.Mutex
	ctx            context.Context
	log            *slog.Logger
	addCallback    func(interest router.Interest, t router.Transport) error
	removeCallback func(interest router.Interest, t router.Transport) error
	intents        map[string]*Intent
	interests      map[string]*Interest
	links          map[string]*Link
	size           int
}

func New(size int) *Transport {
	return &Transport{
		size:      size,
		intents:   make(map[string]*Intent),
		interests: make(map[string]*Interest),
	}
}

func (t *Transport) Close() error {
	return nil
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest router.Interest, t router.Transport) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.log = logger
	t.addCallback = add
	t.removeCallback = remove
	t.ctx = ctx
	return nil
}

func (t *Transport) Name() string {
	return "pass-through"
}

func (t *Transport) Publish(route router.Route) (router.Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	intent, err := t.setIntent(route)
	if err != nil {
		return nil, err
	}

	if interest, ok := t.interests[route.String()]; ok {
		t.link(route, intent, interest)
	}

	return intent, nil
}

func (t *Transport) setIntent(route router.Route) (*Intent, error) {
	intent, ok := t.intents[route.String()]
	if ok {
		return intent, nil
	}

	intent = NewIntent(t.ctx, route, t.size, func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.unlink(route)
		delete(t.intents, route.String())
		return nil
	})

	t.intents[route.String()] = intent
	return intent, nil
}

func (t *Transport) setInterest(route router.Route) (*Interest, error) {
	interest, ok := t.interests[route.String()]
	if ok {
		if link, ok := t.links[route.String()]; ok {
			link.Notify()
		}
		return interest, nil
	}

	interest = NewInterest(t.ctx, route, t.size, func() error {
		t.mu.Lock()
		t.unlink(route)
		delete(t.interests, route.String())
		t.mu.Unlock()
		return t.removeCallback(interest, t)
	})

	t.interests[route.String()] = interest
	return interest, nil
}

func (t *Transport) link(route router.Route, intent *Intent, interest *Interest) {
	link := NewLink(t.ctx, intent, interest, func() error {
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

	interest, err := t.setInterest(route)
	if err != nil {
		return nil, err
	}

	if intent, ok := t.intents[route.String()]; ok {
		t.link(route, intent, interest)
	}

	return interest, nil
}
