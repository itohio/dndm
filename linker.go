package dndm

import (
	"context"
	"log/slog"
	"reflect"
	"sync"

	"github.com/itohio/dndm/errors"
)

// IntentWrapperFunc is a type of function intended to wrap or modify an IntentInternal object.
// It accepts an IntentInternal as input and returns a possibly modified IntentInternal and an error.
// The primary use case for this function is to provide a mechanism to alter or augment the behavior
// of an Intent object at runtime, such as adding logging, validation, or other cross-cutting concerns.
//
// Parameters:
//
//	intent - The IntentInternal to wrap or modify.
//
// Returns:
//
//	IntentInternal - The wrapped or modified IntentInternal.
//	error - An error if something goes wrong during the wrapping/modification process.
type IntentWrapperFunc func(IntentInternal) (IntentInternal, error)

// InterestWrapperFunc is a type of function designed to wrap or modify an InterestInternal object.
// Similar to IntentWrapperFunc, it takes an InterestInternal as input and returns a potentially
// modified InterestInternal and an error. This function type facilitates dynamic alterations to
// the behavior of Interest objects, enabling enhancements such as security checks, data enrichment,
// or custom event handling to be injected transparently.
//
// Parameters:
//
//	interest - The InterestInternal to wrap or modify.
//
// Returns:
//
//	InterestInternal - The wrapped or modified InterestInternal.
//	error - An error if there is a failure in the wrapping/modification process.
type InterestWrapperFunc func(InterestInternal) (InterestInternal, error)

// Link represents a dynamic connection between an Intent and an Interest.
// It manages the lifecycle and interactions between linked entities, ensuring
// that actions on one entity are reflected on the other. For example, closing
// an Intent should also close the linked Interest.
type Linker struct {
	Base
	log           *slog.Logger
	size          int
	mu            sync.Mutex
	intents       map[string]IntentInternal
	interests     map[string]InterestInternal
	onAddIntent   func(intent Intent) error
	onAddInterest func(interest Interest) error
	beforeLink    func(Intent, Interest) error
	links         map[string]*Link
}

// NewLinker creates a new Linker with provided context, logger, size, and callback functions.
// It initializes the Linker with empty maps for intents and interests and sets up a beforeLink function
// if not provided.
func NewLinker(ctx context.Context, log *slog.Logger, size int, addIntent func(intent Intent) error, addInterest func(interest Interest) error, beforeLink func(Intent, Interest) error) *Linker {
	if beforeLink == nil {
		beforeLink = func(i1 Intent, i2 Interest) error {
			return nil
		}
	}
	return &Linker{
		Base:          NewBaseWithCtx(ctx),
		log:           log,
		size:          size,
		intents:       make(map[string]IntentInternal),
		interests:     make(map[string]InterestInternal),
		links:         make(map[string]*Link),
		onAddIntent:   addIntent,
		onAddInterest: addInterest,
		beforeLink:    beforeLink,
	}
}

// Close shuts down the Linker and cleans up all resources associated with it.
// It iterates through all intents and interests, closes them, and finally clears the collections.
func (t *Linker) Close() error {
	errarr := make([]error, 0, len(t.links))

	t.AddOnClose(func() {
		for _, i := range t.intents {
			err := i.Close()
			if err != nil {
				errarr = append(errarr, err)
			}
		}
		for _, i := range t.interests {
			err := i.Close()
			if err != nil {
				errarr = append(errarr, err)
			}
		}
		t.intents = nil
		t.interests = nil
	})
	t.Base.Close()

	return errors.Join(errarr...)
}

// Intent returns an intent identified by a route if found.
func (t *Linker) Intent(route Route) (Intent, bool) {
	t.mu.Lock()
	intent, ok := t.intents[route.ID()]
	t.mu.Unlock()
	return intent, ok
}

// AddIntent registers a new intent by its route. If a matching intent is found, it attempts to link it
// with a corresponding interest if available.
func (t *Linker) AddIntent(route Route) (Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addIntentLocked(route, func(ii IntentInternal) (IntentInternal, error) { return ii, nil })
}

// AddIntentWithWrapper acts like AddIntent but allows the intent to be modified or wrapped by a provided function
// before being added to the Linker.
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

	intent = NewIntent(t.ctx, route, t.size)
	intent.OnClose(func() {
		t.RemoveIntent(route)
	})
	intent, err := wrapper(intent)
	if err != nil {
		return nil, err
	}

	if err := t.onAddIntent(intent); err != nil {
		return nil, err
	}

	t.intents[id] = intent
	if interest, ok := t.interests[id]; ok {
		t.link(route, intent, interest)
	}

	return intent, nil
}

// RemoveIntent removes an intent by its route and cleans up any associated links.
func (t *Linker) RemoveIntent(route Route) error {
	t.mu.Lock()
	t.unlink(route)
	delete(t.intents, route.ID())
	t.mu.Unlock()
	return nil
}

// Interest retrieves an interest by its route if it exists within the Linker.
func (t *Linker) Interest(route Route) (Interest, bool) {
	t.mu.Lock()
	interest, ok := t.interests[route.ID()]
	t.mu.Unlock()
	return interest, ok
}

// AddInterest registers a new interest by its route. If a matching interest is found, it attempts to link it
// with a corresponding intent if available.
func (t *Linker) AddInterest(route Route) (Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addInterestLocked(route, func(ii InterestInternal) (InterestInternal, error) { return ii, nil })
}

// AddInterestWithWrapper acts like AddInterest but allows the interest to be modified or wrapped by a provided function
// before being added to the Linker.
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

	interest = NewInterest(t.Ctx(), route, t.size)
	interest.OnClose(func() {
		t.RemoveInterest(route)
	})
	interest, err := wrapper(interest)
	if err != nil {
		return nil, err
	}

	if err := t.onAddInterest(interest); err != nil {
		return nil, err
	}

	t.interests[id] = interest
	if intent, ok := t.intents[id]; ok {
		t.link(route, intent, interest)
	}

	return interest, nil
}

// RemoveInterest removes an interest by its route and cleans up any associated links.
func (t *Linker) RemoveInterest(route Route) error {
	t.mu.Lock()
	_, ok := t.interests[route.ID()]
	var link *Link
	if ok {
		link = t.unlink(route)
		delete(t.interests, route.ID())
	}
	t.mu.Unlock()
	if link != nil {
		link.Close()
	}
	return nil
}

// link establishes a link between an intent and an interest if they share the same route and no existing link is found.
// It logs the linking process and handles errors in linking, including invalid routes.
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

	link := NewLink(t.ctx, intent, interest)
	link.AddOnClose(func() {
		t.mu.Lock()
		_ = t.unlink(route)
		t.mu.Unlock()
	})
	t.links[route.ID()] = link
	link.Link()
	t.log.Info("linked", "route", intent.Route(), "intent", reflect.TypeOf(intent), "interest", reflect.TypeOf(interest), "C", interest.C())
	return nil
}

// unlink removes a link associated with a given route, logs the unlinking process, and returns the removed link.
func (t *Linker) unlink(route Route) *Link {
	link, ok := t.links[route.ID()]
	if !ok {
		return nil
	}
	delete(t.links, route.ID())
	t.log.Info("unlinked", "route", route)
	return link
}
