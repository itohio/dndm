package mesh

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	"github.com/itohio/dndm/router"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"golang.org/x/sync/errgroup"
)

var _ router.Endpoint = (*Mesh)(nil)

const NumDialers = 10

type Mesh struct {
	*router.Base
	timeout      time.Duration
	pingDuration time.Duration
	dialer       network.Dialer
	container    *router.Container
	localPeer    network.Peer

	addrbook *Addrbook
}

func New(localPeer network.Peer, size, numDialers int, timeout, pingDuration time.Duration, node network.Dialer, peers []*p2ptypes.AddrbookEntry) (*Mesh, error) {
	ret := &Mesh{
		Base:         router.NewBase(localPeer.String(), size),
		localPeer:    localPeer,
		timeout:      timeout,
		pingDuration: pingDuration,
		dialer:       node,
		container:    router.NewContainer(localPeer.String(), size),
	}
	ret.addrbook = NewAddrbook(localPeer, peers)

	return ret, nil
}

func (t *Mesh) Addrbook() []*p2ptypes.AddrbookEntry {
	return t.addrbook.Peers()
}

func (t *Mesh) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest router.Interest, t router.Endpoint) error) error {
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

	t.addrbook.Init(ctx)

	return nil
}

func (t *Mesh) Close() error {
	t.Log.Info("Mesh.Close")
	errarr := make([]error, 0, 3)
	errarr = append(errarr, t.Base.Close())
	errarr = append(errarr, t.container.Close())
	if closer, ok := t.dialer.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}
	return errors.Join(errarr...)
}

func (t *Mesh) Publish(route router.Route, opt ...router.PubOpt) (router.Intent, error) {
	return t.container.Publish(route, opt...)
}

func (t *Mesh) Subscribe(route router.Route, opt ...router.SubOpt) (router.Interest, error) {
	return t.container.Subscribe(route, opt...)
}
