package direct

import (
	"context"
	"log/slog"

	"github.com/itohio/dndm/router"
)

var _ router.Endpoint = (*Direct)(nil)

type Direct struct {
	*router.Base
	linker *router.Linker
}

func New(size int) *Direct {
	return &Direct{
		Base: router.NewBase("direct", size),
	}
}

func (t *Direct) Close() error {
	t.Base.Close()
	if t.linker == nil {
		return nil
	}
	return t.linker.Close()
}

func (t *Direct) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest router.Interest, t router.Endpoint) error) error {
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}

	t.linker = router.NewLinker(
		ctx, logger, t.Size,
		func(interest router.Interest) error {
			return add(interest, t)
		},
		func(interest router.Interest) error {
			return remove(interest, t)
		},
		nil,
	)
	return nil
}

func (t *Direct) Publish(route router.Route, opt ...router.PubOpt) (router.Intent, error) {
	intent, err := t.linker.AddIntent(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("intent registered", "intent", route.Route())
	return intent, nil
}

func (t *Direct) Subscribe(route router.Route, opt ...router.SubOpt) (router.Interest, error) {
	interest, err := t.linker.AddInterest(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("registered", "interest", route.Route())
	return interest, nil
}
