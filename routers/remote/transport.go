package remote

import (
	"container/ring"
	"context"
	"io"
	"log/slog"
	sync "sync"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
)

var _ routers.Transport = (*Transport)(nil)

type Transport struct {
	*routers.Base

	wg     sync.WaitGroup
	remote dialers.Remote

	timeout time.Duration
	linker  *routers.Linker

	pingDuration time.Duration
	pingMu       sync.Mutex
	pingRing     *ring.Ring
	pongRing     *ring.Ring

	nonce atomic.Uint64
}

// New creates a transport that communicates with a remote via Remote interface.
func New(name string, remote dialers.Remote, size int, timeout, pingDuration time.Duration) *Transport {
	return &Transport{
		Base:         routers.NewBase(name, size),
		remote:       remote,
		pingDuration: pingDuration,
		timeout:      timeout,
		pingRing:     ring.New(3),
		pongRing:     ring.New(3),
	}
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}

	t.linker = routers.NewLinker(
		ctx, logger, t.Size,
		func(interest routers.Interest) error {
			err := add(interest, t)
			if err != nil {
				return err
			}
			// Nil Type indicates remote interest
			r := interest.Route()
			if r.Type() != nil {
				t.remote.AddRoute(r)
			}
			return nil
		},
		func(interest routers.Interest) error {
			err := remove(interest, t)
			if err != nil {
				return err
			}
			// Nil Type indicates remote interest
			r := interest.Route()
			if r.Type() != nil {
				t.remote.DelRoute(r)
			}
			return nil
		},
		nil,
	)

	t.wg.Add(1)
	go t.messageHandler()
	if t.pingDuration >= time.Millisecond*100 {
		t.wg.Add(1)
		go t.messageSender(t.pingDuration)
	}
	return nil
}

func (t *Transport) Close() error {
	t.Log.Info("Remote.Close")
	t.Base.Close()
	t.wg.Wait()
	if closer, ok := t.remote.(io.Closer); ok {
		closer.Close()
	}
	return t.linker.Close()
}

func (t *Transport) Publish(route routers.Route, opt ...routers.PubOpt) (routers.Intent, error) {
	// TODO: LocalWrapped intent
	intent, err := t.linker.AddIntentWithWrapper(route, wrapLocalIntent(t.Log, t.remote))
	if err != nil {
		return nil, err
	}

	if _, ok := intent.(*RemoteIntent); ok {
		return nil, errors.ErrRemoteIntent
	}

	t.Log.Info("intent registered", "route", route.Route())
	return intent, err
}

func (t *Transport) publish(route routers.Route, m *types.Intent) (routers.Intent, error) {
	intent, err := t.linker.AddIntentWithWrapper(route, wrapRemoteIntent(t.Log, t.remote, m))
	if err != nil {
		return nil, err
	}

	t.Log.Info("remote intent registered", "route", route.Route())
	return intent, nil
}

func (t *Transport) Subscribe(route routers.Route, opt ...routers.SubOpt) (routers.Interest, error) {
	// TODO: LocalWrapped intent
	interest, err := t.linker.AddInterestWithWrapper(route, wrapLocalInterest(t.Log, t.remote))
	if err != nil {
		return nil, err
	}

	if _, ok := interest.(*RemoteInterest); ok {
		return nil, errors.ErrRemoteInterest
	}

	t.AddCallback(interest, t)
	t.Log.Info("interest registered", "route", route.Route())
	return interest, err
}

func (t *Transport) subscribe(route routers.Route, m *types.Interest) (routers.Interest, error) {
	interest, err := t.linker.AddInterestWithWrapper(route, wrapRemoteInterest(t.Log, t.remote, m))
	if err != nil {
		return nil, err
	}
	t.AddCallback(interest, t)
	t.Log.Info("remote interest registered", "route", route.Route())
	return interest, nil
}
