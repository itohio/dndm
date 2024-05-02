package dndm

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
)

var (
	_ Endpoint = (*Container)(nil)
)

// Container implements an aggregate Endpoint that stores and manages multiple endpoints,
// and it coordinates the linking of intents and interests across these endpoints.
//
// Actions:
//   - Add/Remove endpoints look for existing intents and interests and registers them to respective routers.
//   - Add calls initialize on new endpoint
//   - Publish/Subscribe look for existing endpoints and registers intents/interests respectively.
type Container struct {
	BaseEndpoint

	mu              sync.Mutex
	endpoints       []Endpoint
	intentRouters   map[string]*IntentRouter
	interestRouters map[string]*InterestRouter
}

// NewContainer creates a new Container with a given name and size.
func NewContainer(name string, size int) *Container {
	return &Container{
		BaseEndpoint:    NewEndpointBase(name, size),
		endpoints:       make([]Endpoint, 0, 8),
		intentRouters:   make(map[string]*IntentRouter),
		interestRouters: make(map[string]*InterestRouter),
	}
}

func (t *Container) Close() error {
	errarr := make([]error, 0, len(t.endpoints))
	for _, tr := range t.endpoints {
		err := tr.Close()
		if err != nil {
			errarr = append(errarr, err)
		}
	}
	t.endpoints = nil
	errarr = append(errarr, t.BaseEndpoint.Close())
	return errors.Join(errarr...)
}

func (t *Container) OnClose(f func()) Endpoint {
	t.AddOnClose(f)
	return t
}

// Init is used by the Router to initialize this endpoint.
func (t *Container) Init(ctx context.Context, logger *slog.Logger, addIntent IntentCallback, addInterest InterestCallback) error {
	if err := t.BaseEndpoint.Init(ctx, logger, addIntent, addInterest); err != nil {
		return err
	}

	return nil
}

func (t *Container) Add(ep Endpoint) error {
	if ep == nil {
		return errors.ErrInvalidEndpoint
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if idx := slices.Index(t.endpoints, ep); idx >= 0 {
		return errors.ErrDuplicate
	}

	if err := t.initializeEndpoint(ep); err != nil {
		return err
	}

	t.endpoints = append(t.endpoints, ep)

	t.registerEndpointWithRouters(ep)

	return nil
}

// initializeEndpoint handles the initialization and setup of an endpoint.
func (t *Container) initializeEndpoint(ep Endpoint) error {
	err := ep.Init(t.Ctx(), t.Log,
		func(intent Intent, ep Endpoint) error { return t.OnAddIntent(intent, ep) },
		func(interest Interest, ep Endpoint) error { return t.OnAddInterest(interest, ep) },
	)
	if err != nil {
		return err
	}

	ep.OnClose(func() {
		t.Log.Info("Container OnClose", "name", ep.Name())
		t.Remove(ep)
	})

	return nil
}

// registerEndpointWithRouters links the endpoint with all existing routers.
func (t *Container) registerEndpointWithRouters(ep Endpoint) {
	canPublish := func(Intent) bool { return true }
	canSubscribe := func(Interest) bool { return true }
	if rep, ok := ep.(RemoteEndpoint); ok {
		// Local Intent (created via API)
		//  - Only if route prefix matches LocalPeer path
		// Remote Intent (created via remote)
		//  - always false, because remote intent is local to the endpoint that created it
		canPublish = func(i Intent) bool {
			if _, ok := i.(RemoteIntent); ok {
				return false
			}
			return rep.Local().HasPrefix(i.Route())
		}

		// Local Interest (created via API)
		//  - Only if route prefix matches RemotePeer path
		// Remote Interest (created via remote)
		//  - always false, because remote interest is local to the endpoint that created it
		canSubscribe = func(i Interest) bool {
			if _, ok := i.(RemoteInterest); ok {
				return false
			}
			return rep.Remote().HasPrefix(i.Route())
		}
	}

	// Add endpoint to intents
	for _, ir := range t.intentRouters {
		if !canPublish(ir) {
			continue
		}

		intent, err := ep.Publish(ir.Route())
		if err != nil {
			t.Log.Warn("AddIntent.Publish", "err", err, "route", ir.Route())
			continue
		}
		err = ir.AddIntent(intent)
		if err != nil {
			t.Log.Warn("AddIntent.AddIntent", "err", err, "route", ir.Route())
		}
	}

	// Add enpoint to interests
	for _, ir := range t.interestRouters {
		if !canSubscribe(ir) {
			continue
		}

		interest, err := ep.Subscribe(ir.Route())
		if err != nil {
			t.Log.Warn("AddIntent.Subscribe", "err", err, "route", ir.Route())
			continue
		}
		err = ir.AddInterest(interest)
		if err != nil {
			t.Log.Warn("AddIntent.AddInterest", "err", err, "route", ir.Route())
		}
	}
}

func (t *Container) Remove(ep Endpoint) error {
	t.mu.Lock() // NOTE: Watchout for unlocks!
	idx := slices.Index(t.endpoints, ep)
	if idx < 0 {
		t.mu.Unlock()
		return errors.ErrNotFound
	}
	ep = t.endpoints[idx]
	t.endpoints = slices.Delete(t.endpoints, idx, idx+1)
	t.mu.Unlock()
	return ep.Close()
}

func finderFunc[T any](arr []T, compare func(T) bool) []T {
	res := make([]T, 0, 8)
	for _, item := range arr {
		if compare(item) {
			res = append(res, item)
		}
	}
	return res
}

func (t *Container) Endpoint(compare func(Endpoint) bool) []Endpoint {
	t.mu.Lock()
	defer t.mu.Unlock()
	return finderFunc(t.endpoints, compare)
}

func (t *Container) Intent(compare func(Intent) bool) []Intent {
	t.mu.Lock()
	r := make([]Intent, 0, len(t.intentRouters))
	for _, v := range t.intentRouters {
		r = append(r, v.intents...)
	}
	t.mu.Unlock()
	return finderFunc(r, compare)
}

func (t *Container) Interest(compare func(Interest) bool) []Interest {
	t.mu.Lock()
	r := make([]Interest, 0, len(t.interestRouters))
	for _, v := range t.interestRouters {
		r = append(r, v.interests...)
	}
	t.mu.Unlock()
	return finderFunc(r, compare)
}

func closeAll[T io.Closer](closers ...T) error {
	errarr := make([]error, len(closers))
	for i, closer := range closers {
		errarr[i] = closer.Close()
	}
	return errors.Join(errarr...)
}

// Publish will advertise an intent to publish named and typed data.
func (t *Container) Publish(route Route, opt ...PubOpt) (Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.publish(route, opt...)
}

func (t *Container) publish(route Route, opt ...PubOpt) (Intent, error) {
	intents := make([]Intent, 0, len(t.endpoints))
	// Advertise intents even if we are already publishing
	for _, ep := range t.endpoints {
		if rep, ok := ep.(RemoteEndpoint); ok {
			// We can only publish as part of local peer prefix
			if !rep.Local().HasPrefix(route) {
				continue
			}
		}

		intent, err := ep.Publish(route)
		if err != nil {
			closeAll(intents...)
			return nil, err
		}
		intents = append(intents, intent)
	}

	ir, ok := t.intentRouters[route.ID()]
	if ok {
		return ir.Wrap(), nil
	}

	ir, err := NewIntentRouter(t.Ctx(), route,
		t.Size,
		intents...,
	)
	if err != nil {
		return nil, err
	}
	ir.OnClose(func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.intentRouters, route.ID())
	})

	t.intentRouters[route.ID()] = ir
	return ir.Wrap(), nil
}

// Subscribe will advertise an interest in named and typed data.
func (t *Container) Subscribe(route Route, opt ...SubOpt) (Interest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.subscribe(route, opt...)
}

func (t *Container) subscribe(route Route, opt ...SubOpt) (Interest, error) {
	// Advertise interests anyway (even if we are already subscribed)
	interests := make([]Interest, 0, len(t.endpoints))
	for _, ep := range t.endpoints {
		if rep, ok := ep.(RemoteEndpoint); ok {
			// We can only subscribe to remote peers as part of remote peer prefix
			if !rep.Remote().HasPrefix(route) {
				continue
			}
		}

		interest, err := ep.Subscribe(route)
		if err != nil {
			closeAll(interests...)
			return nil, err
		}
		interests = append(interests, interest)
	}

	ir, ok := t.interestRouters[route.ID()]
	if ok {
		return ir.Wrap(), nil
	}

	ir, err := NewInterestRouter(t.Ctx(), route,
		t.Size,
		interests...,
	)
	if err != nil {
		return nil, err
	}
	ir.OnClose(func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.interestRouters, route.ID())
	})

	t.interestRouters[route.ID()] = ir
	return ir.Wrap(), nil
}
