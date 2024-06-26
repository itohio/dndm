package remote

import (
	"context"
	"io"
	"log/slog"
	sync "sync"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	types "github.com/itohio/dndm/types/core"
)

var _ dndm.RemoteEndpoint = (*Endpoint)(nil)

type Endpoint struct {
	dndm.BaseEndpoint

	wg   sync.WaitGroup
	conn network.Conn

	timeout time.Duration
	linker  *dndm.Linker

	pingDuration time.Duration

	nonce atomic.Uint64

	latency *LatencyTracker
}

// New creates a endpoint that communicates with a remote via Remote interface.
func New(self dndm.Peer, conn network.Conn, size int, timeout, pingDuration time.Duration) *Endpoint {
	return &Endpoint{
		BaseEndpoint: dndm.NewEndpointBase(self.String(), size),
		conn:         conn,
		pingDuration: pingDuration,
		timeout:      timeout,
		latency:      NewLatencyTracker(10),
	}
}

func (t *Endpoint) Local() dndm.Peer {
	return t.conn.Local()
}

func (t *Endpoint) Remote() dndm.Peer {
	return t.conn.Remote()
}

func (t *Endpoint) Init(ctx context.Context, logger *slog.Logger, addIntent dndm.IntentCallback, addInterest dndm.InterestCallback) error {
	if err := t.BaseEndpoint.Init(ctx, logger, addIntent, addInterest); err != nil {
		return err
	}

	t.conn.OnClose(func() { t.Close() })

	t.linker = dndm.NewLinker(
		ctx, logger, t.Size,
		func(intent dndm.Intent) error {
			err := addIntent(intent, t)
			if err != nil {
				return err
			}
			return nil
		},
		func(interest dndm.Interest) error {
			err := addInterest(interest, t)
			if err != nil {
				return err
			}
			// Nil Type indicates remote interest
			r := interest.Route()
			if r.Type() != nil {
				t.conn.AddRoute(r)
			}

			interest.OnClose(func() {
				r := interest.Route()
				if r.Type() != nil {
					t.conn.DelRoute(r)
				}
			})
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

func (t *Endpoint) OnClose(f func()) dndm.Endpoint {
	t.AddOnClose(f)
	return t
}

func (t *Endpoint) Close() error {
	t.Log.Info("Remote.Close")
	errarr := make([]error, 0, 3)
	if closer, ok := t.conn.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}
	errarr = append(errarr, t.BaseEndpoint.Close(), t.linker.Close())
	t.wg.Wait()
	return errors.Join(errarr...)
}

func (t *Endpoint) Publish(route dndm.Route, opt ...dndm.PubOpt) (dndm.Intent, error) {
	// TODO: LocalWrapped intent
	intent, err := t.linker.AddIntentWithWrapper(route, wrapLocalIntent(t.Log, t.conn))
	if err != nil {
		return nil, err
	}

	if _, ok := intent.(*RemoteIntent); ok {
		return nil, errors.ErrRemoteIntent
	}

	t.Log.Info("intent registered", "route", route)
	return intent, err
}

func (t *Endpoint) publish(route dndm.Route, m *types.Intent) (dndm.Intent, error) {
	intent, err := t.linker.AddIntentWithWrapper(route, wrapRemoteIntent(t.Log, t.conn, m))
	if err != nil {
		return nil, err
	}

	t.Log.Info("remote intent registered", "route", route)
	return intent, nil
}

func (t *Endpoint) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	// TODO: LocalWrapped intent
	interest, err := t.linker.AddInterestWithWrapper(route, wrapLocalInterest(t.Log, t.conn))
	if err != nil {
		return nil, err
	}

	if _, ok := interest.(*RemoteInterest); ok {
		return nil, errors.ErrRemoteInterest
	}

	t.Log.Info("interest registered", "route", route)
	return interest, err
}

func (t *Endpoint) subscribe(route dndm.Route, m *types.Interest) (dndm.Interest, error) {
	interest, err := t.linker.AddInterestWithWrapper(route, wrapRemoteInterest(t.Log, t.conn, m))
	if err != nil {
		return nil, err
	}
	t.Log.Info("remote interest registered", "route", route)
	return interest, nil
}

func (t *Endpoint) Latency() *LatencyTracker {
	return t.latency
}
