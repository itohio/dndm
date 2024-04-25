package dndm

import (
	"log/slog"
	"sync"

	"google.golang.org/protobuf/proto"
)

// CauseCloser interface for objects that accepts closure reason
type CauseCloser interface {
	CloseCause(e error) error
}

type Router struct {
	BaseCtx
	mu              sync.Mutex
	log             *slog.Logger
	size            int
	endpoints       []Endpoint
	intentRouters   map[string]*IntentRouter
	interestRouters map[string]*InterestRouter
}

func New(opts ...Option) (*Router, error) {
	opt := defaultOptions()
	if err := opt.Config(opts...); err != nil {
		return nil, err
	}

	ret := &Router{
		BaseCtx:         NewBaseCtxWithCtx(opt.ctx),
		log:             opt.logger.With("module", "router"),
		endpoints:       make([]Endpoint, len(opt.endpoints)),
		intentRouters:   make(map[string]*IntentRouter),
		interestRouters: make(map[string]*InterestRouter),
		size:            opt.size,
	}

	for i, t := range opt.endpoints {
		log := opt.logger.With("endpoint", t.Name())
		if err := t.Init(ret.ctx, log, func(intent Intent, ep Endpoint) error { return nil }, ret.addInterest); err != nil {
			return nil, err
		}
		ret.endpoints[i] = t
	}

	return ret, nil
}

func (d *Router) addInterest(interest Interest, t Endpoint) error {
	route := interest.Route()
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		ir, ok := d.interestRouters[route.ID()]
		if ok {
			ir.AddInterest(interest)
			return
		}

		ir, err := NewInterestRouter(d.ctx, route,
			d.size,
			interest,
		)
		ir.OnClose(func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			delete(d.interestRouters, route.ID())
		})
		if err != nil {
			panic(err)
		}
		d.interestRouters[route.ID()] = ir
	}()

	return nil
}

func (d *Router) removeInterest(interest Interest, t Endpoint) error {
	route := interest.Route()
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		ir, ok := d.interestRouters[route.ID()]
		if !ok {
			return
		}
		ir.RemoveInterest(interest)
	}()
	return nil
}

// Publish delivers data to an interested party. It may advertise the availability of the data if no interest is found.
func (d *Router) Publish(path string, msg proto.Message, opt ...PubOpt) (Intent, error) {
	route, err := NewRoute(path, msg)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	intents := make([]Intent, 0, len(d.endpoints))
	// Advertise intents even if we are already publishing
	for _, t := range d.endpoints {
		intent, err := t.Publish(route)
		if err != nil {
			closeAll(intents...)
			return nil, err
		}
		intents = append(intents, intent)
	}

	ir, ok := d.intentRouters[route.ID()]
	if ok {
		return ir.Wrap(), nil
	}

	ir, err = NewIntentRouter(d.ctx, route,
		d.size,
		intents...,
	)
	ir.OnClose(func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		delete(d.intentRouters, route.ID())
	})
	if err != nil {
		return nil, err
	}
	d.intentRouters[route.ID()] = ir
	return ir.Wrap(), nil
}

// Subscribe advertises an interest in a specific message type on particular path.
func (d *Router) Subscribe(path string, msg proto.Message, opt ...SubOpt) (Interest, error) {
	route, err := NewRoute(path, msg)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	// Advertise interests anyway (even if we are already subscribed)
	interests := make([]Interest, 0, len(d.endpoints))
	for _, t := range d.endpoints {
		interest, err := t.Subscribe(route)
		if err != nil {
			closeAll(interests...)
			return nil, err
		}
		interests = append(interests, interest)
	}

	ir, ok := d.interestRouters[route.ID()]
	if ok {
		return ir.Wrap(), nil
	}

	ir, err = NewInterestRouter(d.ctx, route,
		d.size,
		interests...,
	)
	ir.OnClose(func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		delete(d.interestRouters, route.ID())
	})
	if err != nil {
		return nil, err
	}

	d.interestRouters[route.ID()] = ir
	return ir.Wrap(), nil
}
