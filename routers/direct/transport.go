package direct

import (
	"context"
	"log/slog"

	"github.com/itohio/dndm/routers"
)

var _ routers.Transport = (*Transport)(nil)

type Transport struct {
	*routers.Base
	linker *routers.Linker
}

func New(size int) *Transport {
	return &Transport{
		Base: routers.NewBase("direct", size),
	}
}

func (t *Transport) Close() error {
	t.Base.Close()
	if t.linker == nil {
		return nil
	}
	return t.linker.Close()
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}

	t.linker = routers.NewLinker(
		ctx, logger, t.Size,
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

func (t *Transport) Publish(route routers.Route, opt ...routers.PubOpt) (routers.Intent, error) {
	intent, err := t.linker.AddIntent(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("intent registered", "intent", route.Route())
	return intent, nil
}

func (t *Transport) Subscribe(route routers.Route, opt ...routers.SubOpt) (routers.Interest, error) {
	interest, err := t.linker.AddInterest(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("registered", "interest", route.Route())
	return interest, nil
}
