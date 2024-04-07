package dndm

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
	"google.golang.org/protobuf/proto"
)

type Router struct {
	mu              sync.Mutex
	log             *slog.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	size            int
	transports      []router.Transport
	intentRouters   map[string]*intentRouter
	interestRouters map[string]*interestRouter
}

func New(opts ...Option) (*Router, error) {
	opt := defaultOptions()
	if err := opt.Config(opts...); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(opt.ctx)
	ret := &Router{
		ctx:        ctx,
		cancel:     cancel,
		log:        opt.logger.With("module", "router"),
		transports: make([]router.Transport, len(opt.transports)),
		size:       opt.size,
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

func (d *Router) addInterest(interest router.Interest, t router.Transport) error {
	route := interest.Route()
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		ir, ok := d.interestRouters[route.String()]
		if ok {
			ir.addInterest(interest)
			return
		}

		ir = makeInterestRouter(d.ctx, route,
			func() error {
				d.mu.Lock()
				defer d.mu.Unlock()
				delete(d.interestRouters, route.String())
				return nil
			},
			d.size,
			interest,
		)
		d.interestRouters[route.String()] = ir
	}()

	return nil
}

func (d *Router) removeInterest(interest router.Interest, t router.Transport) error {
	route := interest.Route()
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		ir, ok := d.interestRouters[route.String()]
		if !ok {
			return
		}
		ir.removeInterest(interest)
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
func (d *Router) Publish(path string, msg proto.Message, opt ...PubOpt) (router.Intent, error) {
	route, err := router.NewRoute(path, msg)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	intents := make([]router.Intent, 0, len(d.transports))
	for _, t := range d.transports {
		intent, err := t.Publish(route)
		if err != nil {
			closeAll(intents...)
			return nil, err
		}
		intents = append(intents, intent)
	}

	ir, ok := d.intentRouters[route.String()]
	if !ok {
		ir = makeIntentRouter(d.ctx, route,
			func() error {
				d.mu.Lock()
				defer d.mu.Unlock()
				delete(d.intentRouters, route.String())
				return nil
			},
			d.size,
			intents...,
		)
		d.intentRouters[route.String()] = ir
	}
	return ir.wrap(), nil
}

// Subscribe advertises an interest in a specific message type on particular path.
func (d *Router) Subscribe(path string, msg proto.Message, opt ...SubOpt) (router.Interest, error) {
	route, err := router.NewRoute(path, msg)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	interests := make([]router.Interest, 0, len(d.transports))
	for _, t := range d.transports {
		interest, err := t.Subscribe(route)
		if err != nil {
			closeAll(interests...)
			return nil, err
		}
		interests = append(interests, interest)
	}

	ir, ok := d.interestRouters[route.String()]
	if !ok {
		ir = makeInterestRouter(d.ctx, route,
			func() error {
				d.mu.Lock()
				defer d.mu.Unlock()
				delete(d.interestRouters, route.String())
				return nil
			},
			d.size,
			interests...,
		)
		d.interestRouters[route.String()] = ir
	}
	return ir.wrap(), nil
}
