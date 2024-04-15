package direct

import (
	"context"
	"log/slog"

	"github.com/itohio/dndm"
)

var _ dndm.Endpoint = (*Endpoint)(nil)

type Endpoint struct {
	*dndm.Base
	linker *dndm.Linker
}

func New(size int) *Endpoint {
	return &Endpoint{
		Base: dndm.NewBase("direct", size),
	}
}

func (t *Endpoint) Close() error {
	t.Base.Close()
	if t.linker == nil {
		return nil
	}
	return t.linker.Close()
}

func (t *Endpoint) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest dndm.Interest, t dndm.Endpoint) error) error {
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}

	t.linker = dndm.NewLinker(
		ctx, logger, t.Size,
		func(interest dndm.Interest) error {
			return add(interest, t)
		},
		func(interest dndm.Interest) error {
			return remove(interest, t)
		},
		nil,
	)
	return nil
}

func (t *Endpoint) Publish(route dndm.Route, opt ...dndm.PubOpt) (dndm.Intent, error) {
	intent, err := t.linker.AddIntent(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("intent registered", "intent", route.Route())
	return intent, nil
}

func (t *Endpoint) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	interest, err := t.linker.AddInterest(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("registered", "interest", route.Route())
	return interest, nil
}
