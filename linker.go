package dndm

import (
	"context"
	"log/slog"
	"reflect"
	"sync"

	"github.com/itohio/dndm/errors"
)

type InterestCallback func(interest Interest) error
type IntentWrapperFunc func(IntentInternal) (IntentInternal, error)
type InterestWrapperFunc func(InterestInternal) (InterestInternal, error)

type Linker struct {
	ctx            context.Context
	log            *slog.Logger
	size           int
	mu             sync.Mutex
	intents        map[string]IntentInternal
	interests      map[string]InterestInternal
	addCallback    InterestCallback
	removeCallback InterestCallback
	beforeLink     func(Intent, Interest) error
	links          map[string]*Link
}

func NewLinker(ctx context.Context, log *slog.Logger, size int, add, remove InterestCallback, beforeLink func(Intent, Interest) error) *Linker {
	if beforeLink == nil {
		beforeLink = func(i1 Intent, i2 Interest) error {
			return nil
		}
	}
	return &Linker{
		ctx:            ctx,
		log:            log,
		size:           size,
		intents:        make(map[string]IntentInternal),
		interests:      make(map[string]InterestInternal),
		links:          make(map[string]*Link),
		addCallback:    add,
		removeCallback: remove,
		beforeLink:     beforeLink,
	}
}

func (t *Linker) Close() error {
	errarr := make([]error, 0, len(t.links))
	for _, i := range t.interests {
		err := i.Close()
		if err != nil {
			errarr = append(errarr, err)
		}
	}
	for _, i := range t.intents {
		err := i.Close()
		if err != nil {
			errarr = append(errarr, err)
		}
	}

	t.mu.Lock()
	t.intents = nil
	t.interests = nil
	t.links = nil
	t.mu.Unlock()

	return errors.Join(errarr...)
}

func (t *Linker) Intent(route Route) (Intent, bool) {
	t.mu.Lock()
	intent, ok := t.intents[route.ID()]
	t.mu.Unlock()
	return intent, ok
}

func (t *Linker) AddIntent(route Route) (Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addIntentLocked(route, func(ii IntentInternal) (IntentInternal, error) { return ii, nil })
}

func (t *Linker) AddIntentWithWrapper(route Route, wrapper IntentWrapperFunc) (Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addIntentLocked(route, wrapper)
}

func (t *Linker) addIntentLocked(route Route, wrapper IntentWrapperFunc) (Intent, error) {
	id := route.ID()
	intent, ok := t.intents[id]
	if ok {
		return intent, nil
	}

	intent = NewIntent(t.ctx, route, t.size, func() error {
		return t.RemoveIntent(route)
	})
	intent, err := wrapper(intent)
	if err != nil {
		return nil, err
	}

	t.intents[id] = intent
	if interest, ok := t.interests[id]; ok {
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

func (t *Linker) Interest(route Route) (Interest, bool) {
	t.mu.Lock()
	interest, ok := t.interests[route.ID()]
	t.mu.Unlock()
	return interest, ok
}

func (t *Linker) AddInterest(route Route) (Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addInterestLocked(route, func(ii InterestInternal) (InterestInternal, error) { return ii, nil })
}

func (t *Linker) AddInterestWithWrapper(route Route, wrapper InterestWrapperFunc) (Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addInterestLocked(route, wrapper)
}

func (t *Linker) addInterestLocked(route Route, wrapper InterestWrapperFunc) (Interest, error) {
	id := route.ID()
	interest, ok := t.interests[id]
	if ok {
		if link, ok := t.links[id]; ok {
			link.Notify()
		}
		return interest, nil
	}

	interest = NewInterest(t.ctx, route, t.size, func() error {
		riErr := t.RemoveInterest(route)
		rcErr := t.removeCallback(interest)
		return errors.Join(riErr, rcErr)
	})
	interest, err := wrapper(interest)
	if err != nil {
		return nil, err
	}

	if err := t.addCallback(interest); err != nil {
		return nil, err
	}

	t.interests[id] = interest
	if intent, ok := t.intents[id]; ok {
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
		return errors.ErrInvalidRoute
	}

	if _, ok := t.links[route.ID()]; ok {
		return nil
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
	t.log.Info("linked", "route", intent.Route(), "intent", reflect.TypeOf(intent), "interest", reflect.TypeOf(interest), "C", interest.C())
	return nil
}

func (t *Linker) unlink(route Route) {
	link, ok := t.links[route.ID()]
	if !ok {
		return
	}
	link.Unlink()
	delete(t.links, route.ID())
	t.log.Info("unlinked", "route", route)
}
