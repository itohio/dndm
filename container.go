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
	_ Endpoint      = (*Container)(nil)
	_ CloseNotifier = (*Container)(nil)
)

type Container struct {
	*Base

	mu              sync.Mutex
	endpoints       []Endpoint
	intentRouters   map[string]*IntentRouter
	interestRouters map[string]*InterestRouter
}

func NewContainer(name string, size int) *Container {
	return &Container{
		Base:            NewBase(name, size),
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
	errarr = append(errarr, t.Base.Close())
	return errors.Join(errarr...)
}

// Init is used by the Router to initialize this endpoint.
func (t *Container) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Endpoint) error) error {
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
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
	idx := slices.Index(t.endpoints, ep)
	if idx >= 0 {
		return errors.ErrDuplicate
	}
	// TODO
	err := ep.Init(t.Ctx, t.Log,
		func(interest Interest, t Endpoint) error { return nil },
		func(interest Interest, t Endpoint) error { return nil },
	)
	if err != nil {
		return err
	}
	t.endpoints = append(t.endpoints, ep)
	if notifier, ok := ep.(CloseNotifier); ok {
		notifier.OnClose(func() {
			t.Log.Info("Container OnClose", "name", ep.Name())
			t.Remove(ep)
		})
	}
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

	ir, err := NewIntentRouter(t.Ctx, route,
		func() error {
			t.mu.Lock()
			defer t.mu.Unlock()
			delete(t.intentRouters, route.ID())
			return nil
		},
		t.Size,
		intents...,
	)
	if err != nil {
		return nil, err
	}

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

	ir, err := NewInterestRouter(t.Ctx, route,
		func() error {
			t.mu.Lock()
			defer t.mu.Unlock()
			delete(t.interestRouters, route.ID())
			return nil
		},
		t.Size,
		interests...,
	)
	if err != nil {
		return nil, err
	}

	t.interestRouters[route.ID()] = ir
	return ir.Wrap(), nil
}
