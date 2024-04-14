package routers

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"github.com/itohio/dndm/errors"
)

var _ Transport = (*Container)(nil)

type Container struct {
	ctx            context.Context
	cancel         context.CancelFunc
	log            *slog.Logger
	name           string
	addCallback    func(interest Interest, t Transport) error
	removeCallback func(interest Interest, t Transport) error

	mu              sync.Mutex
	transports      []Transport
	intentRouters   map[string]*IntentRouter
	interestRouters map[string]*InterestRouter
}

func NewTransportContainer(name string) *Container {
	return &Container{
		name:            name,
		transports:      make([]Transport, 0, 8),
		intentRouters:   make(map[string]*IntentRouter),
		interestRouters: make(map[string]*InterestRouter),
	}
}

func (t *Container) Close() error {
	errarr := make([]error, 0, len(t.transports))
	for _, tr := range t.transports {
		err := tr.Close()
		if err != nil {
			errarr = append(errarr, err)
		}
	}
	t.transports = nil
	return errors.Join(errarr...)
}

func (t *Container) Name() string {
	return t.name
}

// Init is used by the Router to initialize this transport.
func (t *Container) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Transport) error) error {
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

func (t *Container) Add(transport Transport) error {
	if transport == nil {
		return errors.ErrInvalidTransport
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	idx := slices.Index(t.transports, transport)
	if idx >= 0 {
		return errors.ErrDuplicate
	}
	// TODO
	err := transport.Init(t.ctx, t.log,
		func(interest Interest, t Transport) error { return nil },
		func(interest Interest, t Transport) error { return nil },
	)
	if err != nil {
		return err
	}
	t.transports = append(t.transports, transport)
	return nil
}

func (t *Container) Remove(transport Transport) error {
	t.mu.Lock() // NOTE: Watchout for unlocks!
	idx := slices.Index(t.transports, transport)
	if idx < 0 {
		t.mu.Unlock()
		return errors.ErrNotFound
	}
	transport = t.transports[idx]
	t.transports = slices.Delete(t.transports, idx, idx+1)
	t.mu.Unlock()
	return transport.Close()
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

func (t *Container) Transport(compare func(Transport) bool) []Transport {
	t.mu.Lock()
	defer t.mu.Unlock()
	return finderFunc(t.transports, compare)
}

func (t *Container) Intent(compare func(Intent) bool) []Intent {
	return nil
}

func (t *Container) Interest(compare func(Interest) bool) []Interest {
	return nil
}

// Publish will advertise an intent to publish named and typed data.
func (t *Container) Publish(route Route) (Intent, error) {
	return nil, nil
}

// Subscribe will advertise an interest in named and typed data.
func (t *Container) Subscribe(route Route) (Interest, error) {
	return nil, nil
}
