package direct

import (
	"context"
	"log/slog"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
)

var _ routers.Transport = (*Transport)(nil)

type Transport struct {
	ctx            context.Context
	log            *slog.Logger
	addCallback    func(interest routers.Interest, t routers.Transport) error
	removeCallback func(interest routers.Interest, t routers.Transport) error
	size           int
	linker         *routers.Linker
}

func New(size int) *Transport {
	return &Transport{
		size: size,
	}
}

func (t *Transport) Close() error {
	if t.linker == nil {
		return nil
	}
	return t.linker.Close()
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.log = logger
	t.addCallback = add
	t.removeCallback = remove
	t.ctx = ctx

	t.linker = routers.NewLinker(
		ctx, logger, t.size,
		func(interest routers.Interest) error {
			return add(interest, t)
		},
		func(interest routers.Interest) error {
			return remove(interest, t)
		},
		nil,
	)
	return nil
}

func (t *Transport) Name() string {
	return "direct"
}

func (t *Transport) Publish(route routers.Route) (routers.Intent, error) {
	intent, err := t.linker.AddIntent(route)
	if err != nil {
		return nil, err
	}
	t.log.Info("intent registered", "intent", route.Route())
	return intent, nil
}

func (t *Transport) Subscribe(route routers.Route) (routers.Interest, error) {
	interest, err := t.linker.AddInterest(route)
	if err != nil {
		return nil, err
	}
	t.log.Info("registered", "interest", route.Route())
	return interest, nil
}
