package routers

import (
	"context"
	"sync"

	"github.com/itohio/dndm/errors"
)

type InterestCallback func(interest Interest) error

type Linker struct {
	ctx            context.Context
	size           int
	mu             sync.Mutex
	intents        map[string]IntentInternal
	interests      map[string]InterestInternal
	addCallback    InterestCallback
	removeCallback InterestCallback
	beforeLink     func(Intent, Interest) error
	links          map[string]*Link
}

func NewLinker(ctx context.Context, size int, add, remove InterestCallback, beforeLink func(Intent, Interest) error) *Linker {
	return &Linker{
		ctx:            ctx,
		size:           size,
		intents:        make(map[string]IntentInternal),
		interests:      make(map[string]InterestInternal),
		links:          make(map[string]*Link),
		addCallback:    add,
		removeCallback: remove,
		beforeLink:     beforeLink,
	}
}

func (t *Linker) AddIntent(route Route) (Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	intent, ok := t.intents[route.ID()]
	if ok {
		return intent, nil
	}

	intent = NewIntent(t.ctx, route, t.size, func() error {
		return t.RemoveIntent(route)
	})

	t.intents[route.ID()] = intent

	if interest, ok := t.interests[route.ID()]; ok {
		t.link(route, intent, interest)
	}

	return intent, nil
}

// RemoveIntent removes and unlinks an intent. This should be called inside the closer of the intent.
func (t *Linker) RemoveIntent(route Route) error {
	t.mu.Lock()
	t.unlink(route)
	delete(t.intents, route.ID())
	t.mu.Unlock()
	return nil
}

func (t *Linker) NewInterest(route Route) (Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	interest, ok := t.interests[route.ID()]
	if ok {
		if link, ok := t.links[route.ID()]; ok {
			link.Notify()
		}
		return interest, nil
	}

	interest = NewInterest(t.ctx, route, t.size, func() error {
		t.RemoveInterest(route)
		return t.removeCallback(interest)
	})

	t.interests[route.ID()] = interest

	if intent, ok := t.intents[route.ID()]; ok {
		t.link(route, intent, interest)
	}

	return interest, nil
}

func (t *Linker) RemoveInterest(route Route) error {
	t.mu.Lock()
	t.unlink(route)
	delete(t.interests, route.ID())
	t.mu.Unlock()
	return nil
}

func (t *Linker) link(route Route, intent IntentInternal, interest InterestInternal) error {
	if !route.Equal(intent.Route()) || !route.Equal(interest.Route()) {
		// t.log.Error("invalid route", "route", route.Route(), "intent", intent.Route(), "interest", interest.Route())
		return errors.ErrInvalidRoute
	}

	err := t.beforeLink(intent, interest)
	if err != nil {
		return err
	}

	link := NewLink(t.ctx, intent, interest, func() error {
		t.mu.Lock()
		t.unlink(route)
		t.mu.Unlock()
		return nil
	})
	t.links[route.ID()] = link
	link.Link()
	// t.log.Info("link created", "route", route.Route())
	return nil
}

func (t *Linker) unlink(route Route) {
	link, ok := t.links[route.ID()]
	if !ok {
		return
	}
	link.Unlink()
	delete(t.links, route.ID())
}
