package direct

import (
	"context"
	"log/slog"

	"github.com/itohio/dndm"
)

var _ dndm.Endpoint = (*Endpoint)(nil)

type Endpoint struct {
	dndm.BaseEndpoint
	linker *dndm.Linker
}

func New(size int) *Endpoint {
	return &Endpoint{
		BaseEndpoint: dndm.NewEndpointBase("direct", size),
	}
}

func (t *Endpoint) OnClose(f func()) dndm.Endpoint {
	t.AddOnClose(f)
	return t
}

func (t *Endpoint) Close() error {
	t.BaseEndpoint.Close()
	if t.linker == nil {
		return nil
	}
	return t.linker.Close()
}

func (t *Endpoint) Init(ctx context.Context, logger *slog.Logger, addIntent dndm.IntentCallback, addInterest dndm.InterestCallback) error {
	if err := t.BaseEndpoint.Init(ctx, logger, addIntent, addInterest); err != nil {
		return err
	}

	t.linker = dndm.NewLinker(
		ctx, logger, t.Size,
		func(intent dndm.Intent) error {
			return addIntent(intent, t)
		},
		func(interest dndm.Interest) error {
			return addInterest(interest, t)
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
