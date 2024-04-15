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
	*dndm.Base
	timeout      time.Duration
	pingDuration time.Duration
	dialer       network.Dialer
	container    *dndm.Container
	localPeer    network.Peer

	addrbook *Addrbook
}

func New(localPeer network.Peer, size, numDialers int, timeout, pingDuration time.Duration, node network.Dialer, peers []*p2ptypes.AddrbookEntry) (*Endpoint, error) {
	ret := &Endpoint{
		Base:         dndm.NewBase(localPeer.String(), size),
		localPeer:    localPeer,
		timeout:      timeout,
		pingDuration: pingDuration,
		dialer:       node,
		container:    dndm.NewContainer(localPeer.String(), size),
	}
	ret.addrbook = NewAddrbook(localPeer, peers, 3)

	return ret, nil
}

func (t *Endpoint) Addrbook() []*p2ptypes.AddrbookEntry {
	return t.addrbook.Peers()
}

func (t *Endpoint) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest dndm.Interest, t dndm.Endpoint) error) error {
	eg, ctx := errgroup.WithContext(ctx)
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}
	if err := t.container.Init(t.Ctx, logger, add, remove); err != nil {
		return err
	}

	if server, ok := t.dialer.(network.Server); ok {
		eg.Go(func() error {
			err := server.Serve(t.Ctx, t.onConnect)
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
	errarr = append(errarr, t.Base.Close())
	errarr = append(errarr, t.container.Close())
	if closer, ok := t.dialer.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}
	return errors.Join(errarr...)
}

func (t *Endpoint) Publish(route dndm.Route, opt ...dndm.PubOpt) (dndm.Intent, error) {
	return t.container.Publish(route, opt...)
}

func (t *Endpoint) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	return t.container.Subscribe(route, opt...)
}
