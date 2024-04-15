package router

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
)

type Container struct {
	ctx            context.Context
	cancel         context.CancelFunc
	log            *slog.Logger
	size           int
	addCallback    func(interest Interest, t Endpoint) error
	removeCallback func(interest Interest, t Endpoint) error

	mu              sync.Mutex
	endpoints       []Endpoint
	intentRouters   map[string]*IntentRouter
	interestRouters map[string]*InterestRouter
}

func NewContainer(name string, size int) *Container {
	return &Container{
		size:            size,
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
	return errors.Join(errarr...)
}

// Init is used by the Router to initialize this endpoint.
func (t *Container) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Endpoint) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.log = logger
	t.addCallback = add
	t.removeCallback = remove
	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel

	return nil
}

func (t *Container) Add(ep Endpoint) error {
	if ep == nil {
		return errors.ErrInvalidEndpoint
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	idx := slices.Index(t.endpoints, ep)
	if idx >= 0 {
		return errors.ErrDuplicate
	}
	// TODO
	err := ep.Init(t.ctx, t.log,
		func(interest Interest, t Endpoint) error { return nil },
		func(interest Interest, t Endpoint) error { return nil },
	)
	if err != nil {
		return err
	}
	t.endpoints = append(t.endpoints, ep)
	return nil
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
	return nil
}

func (t *Container) Interest(compare func(Interest) bool) []Interest {
	return nil
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
	for _, t := range t.endpoints {
		intent, err := t.Publish(route)
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

	ir = NewIntentRouter(t.ctx, route,
		func() error {
			t.mu.Lock()
			defer t.mu.Unlock()
			delete(t.intentRouters, route.ID())
			return nil
		},
		t.size,
		intents...,
	)
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
	for _, t := range t.endpoints {
		interest, err := t.Subscribe(route)
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

	ir = NewInterestRouter(t.ctx, route,
		func() error {
			t.mu.Lock()
			defer t.mu.Unlock()
			delete(t.interestRouters, route.ID())
			return nil
		},
		t.size,
		interests...,
	)
	t.interestRouters[route.ID()] = ir
	return ir.Wrap(), nil
}
