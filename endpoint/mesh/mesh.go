package mesh

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"golang.org/x/sync/errgroup"
)

var _ dndm.Endpoint = (*Endpoint)(nil)

const NumDialers = 10

type Endpoint struct {
	*dndm.Container
	timeout      time.Duration
	pingDuration time.Duration
	dialer       network.Dialer
	localPeer    dndm.Peer

	addrbook *Addrbook
}

func New(localPeer dndm.Peer, size, numDialers int, timeout, pingDuration time.Duration, node network.Dialer, peers []*p2ptypes.AddrbookEntry) (*Endpoint, error) {
	ret := &Endpoint{
		Container:    dndm.NewContainer(localPeer.String(), size),
		localPeer:    localPeer,
		timeout:      timeout,
		pingDuration: pingDuration,
		dialer:       node,
	}
	ret.addrbook = NewAddrbook(localPeer, peers, 3)

	return ret, nil
}

func (t *Endpoint) Addrbook() []*p2ptypes.AddrbookEntry {
	return t.addrbook.Addrbook()
}

func (t *Endpoint) Init(ctx context.Context, logger *slog.Logger, addIntent dndm.IntentCallback, addInterest dndm.InterestCallback) error {
	eg, ctx := errgroup.WithContext(ctx)
	if err := t.Container.Init(ctx, logger, addIntent, addInterest); err != nil {
		return err
	}

	if server, ok := t.dialer.(network.Server); ok {
		eg.Go(func() error {
			err := server.Serve(ctx, t.onConnect)
			return err
		})
	}

	for i := 0; i < NumDialers; i++ {
		go t.dialerLoop()
	}

	return t.addrbook.Init(ctx, logger)
}

func (t *Endpoint) Close() error {
	t.Log.Info("Mesh.Close")
	errarr := make([]error, 0, 3)
	errarr = append(errarr, t.Container.Close())
	if closer, ok := t.dialer.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}
	return errors.Join(errarr...)
}

func (t *Endpoint) Publish(route dndm.Route, opt ...dndm.PubOpt) (dndm.Intent, error) {
	return t.Container.Publish(route, opt...)
}

func (t *Endpoint) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	return t.Container.Subscribe(route, opt...)
}
