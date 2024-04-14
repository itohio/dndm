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
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	log            *slog.Logger
	name           string
	remote         dialers.Remote
	addCallback    func(interest routers.Interest, t routers.Transport) error
	removeCallback func(interest routers.Interest, t routers.Transport) error
	size           int

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
		name:         name,
		size:         size,
		remote:       remote,
		pingDuration: pingDuration,
		timeout:      timeout,
		pingRing:     ring.New(3),
		pongRing:     ring.New(3),
	}
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.log = logger
	t.addCallback = add
	t.removeCallback = remove
	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel

	t.linker = routers.NewLinker(
		ctx, logger, t.size,
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
	t.cancel()
	t.wg.Wait()
	if closer, ok := t.remote.(io.Closer); ok {
		closer.Close()
	}
	return t.linker.Close()
}

func (t *Transport) Name() string {
	return t.name
}

func (t *Transport) Publish(route routers.Route) (routers.Intent, error) {
	// TODO: LocalWrapped intent
	intent, err := t.linker.AddIntentWithWrapper(route, wrapLocalIntent(t.log, t.remote))
	if err != nil {
		return nil, err
	}

	if _, ok := intent.(*RemoteIntent); ok {
		return nil, errors.ErrRemoteIntent
	}

	t.log.Info("intent registered", "route", route.Route())
	return intent, err
}

func (t *Transport) publish(route routers.Route, m *types.Intent) (routers.Intent, error) {
	intent, err := t.linker.AddIntentWithWrapper(route, wrapRemoteIntent(t.log, t.remote, m))
	if err != nil {
		return nil, err
	}

	t.log.Info("remote intent registered", "route", route.Route())
	return intent, nil
}

func (t *Transport) Subscribe(route routers.Route) (routers.Interest, error) {
	// TODO: LocalWrapped intent
	interest, err := t.linker.AddInterestWithWrapper(route, wrapLocalInterest(t.log, t.remote))
	if err != nil {
		return nil, err
	}

	if _, ok := interest.(*RemoteInterest); ok {
		return nil, errors.ErrRemoteInterest
	}

	t.addCallback(interest, t)
	t.log.Info("interest registered", "route", route.Route())
	return interest, err
}

func (t *Transport) subscribe(route routers.Route, m *types.Interest) (routers.Interest, error) {
	interest, err := t.linker.AddInterestWithWrapper(route, wrapRemoteInterest(t.log, t.remote, m))
	if err != nil {
		return nil, err
	}
	t.addCallback(interest, t)
	t.log.Info("remote interest registered", "route", route.Route())
	return interest, nil
}
