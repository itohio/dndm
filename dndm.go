package dndm

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	"google.golang.org/protobuf/proto"
)

type Router struct {
	mu              sync.Mutex
	log             *slog.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	size            int
	transports      []routers.Transport
	intentRouters   map[string]*routers.IntentRouter
	interestRouters map[string]*routers.InterestRouter
}

func New(opts ...Option) (*Router, error) {
	opt := defaultOptions()
	if err := opt.Config(opts...); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(opt.ctx)
	ret := &Router{
		ctx:             ctx,
		cancel:          cancel,
		log:             opt.logger.With("module", "router"),
		transports:      make([]routers.Transport, len(opt.transports)),
		intentRouters:   make(map[string]*routers.IntentRouter),
		interestRouters: make(map[string]*routers.InterestRouter),
		size:            opt.size,
	}

	for i, t := range opt.transports {
		log := opt.logger.With("transport", t.Name())
		if err := t.Init(ret.ctx, log, ret.addInterest, ret.removeInterest); err != nil {
			return nil, err
		}
		ret.transports[i] = t
	}

	return ret, nil
}

func (d *Router) addInterest(interest routers.Interest, t routers.Transport) error {
	route := interest.Route()
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		ir, ok := d.interestRouters[route.ID()]
		if ok {
			ir.AddInterest(interest)
			return
		}

		ir = routers.NewInterestRouter(d.ctx, route,
			func() error {
				d.mu.Lock()
				defer d.mu.Unlock()
				delete(d.interestRouters, route.ID())
				return nil
			},
			d.size,
			interest,
		)
		d.interestRouters[route.ID()] = ir
	}()

	return nil
}

func (d *Router) removeInterest(interest routers.Interest, t routers.Transport) error {
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

func closeAll[T io.Closer](closers ...T) error {
	errarr := make([]error, len(closers))
	for i, closer := range closers {
		errarr[i] = closer.Close()
	}
	return errors.Join(errarr...)
}

// Publish delivers data to an interested party. It may advertise the availability of the data if no interest is found.
func (d *Router) Publish(path string, msg proto.Message, opt ...routers.PubOpt) (routers.Intent, error) {
	route, err := routers.NewRoute(path, msg)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	intents := make([]routers.Intent, 0, len(d.transports))
	// Advertise intents even if we are already publishing
	for _, t := range d.transports {
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

	ir = routers.NewIntentRouter(d.ctx, route,
		func() error {
			d.mu.Lock()
			defer d.mu.Unlock()
			delete(d.intentRouters, route.ID())
			return nil
		},
		d.size,
		intents...,
	)
	d.intentRouters[route.ID()] = ir
	return ir.Wrap(), nil
}

// Subscribe advertises an interest in a specific message type on particular path.
func (d *Router) Subscribe(path string, msg proto.Message, opt ...routers.SubOpt) (routers.Interest, error) {
	route, err := routers.NewRoute(path, msg)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	// Advertise interests anyway (even if we are already subscribed)
	interests := make([]routers.Interest, 0, len(d.transports))
	for _, t := range d.transports {
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

	ir = routers.NewInterestRouter(d.ctx, route,
		func() error {
			d.mu.Lock()
			defer d.mu.Unlock()
			delete(d.interestRouters, route.ID())
			return nil
		},
		d.size,
		interests...,
	)
	d.interestRouters[route.ID()] = ir
	return ir.Wrap(), nil
}
