package mesh

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/routers"
)

var _ routers.Transport = (*Mesh)(nil)

type AddrbookEntry struct {
	Peer dialers.Peer
}

type Mesh struct {
	*routers.Base
	dialer    dialers.Dialer
	container *routers.Container

	mu    sync.Mutex
	peers map[string]AddrbookEntry
}

func New(name string, size int, node dialers.Dialer, peers []dialers.Peer) (*Mesh, error) {
	pm := make(map[string]AddrbookEntry, len(peers))
	for _, p := range peers {
		if _, ok := pm[p.ID()]; ok {
			continue
		}
		pm[p.ID()] = AddrbookEntry{Peer: p}
	}
	return &Mesh{
		Base:      routers.NewBase(name, size),
		dialer:    node,
		peers:     pm,
		container: routers.NewContainer(name, size),
	}, nil
}

func (t *Mesh) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if err := t.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}
	if err := t.container.Init(ctx, logger, add, remove); err != nil {
		return err
	}

	if server, ok := t.dialer.(dialers.Server); ok {
		err := server.Serve(ctx, t.onConnect)
		if err != nil {
			return err
		}
	}

	for _, p := range t.peers {
		err := t.dial(p.Peer)
		if err != nil {
			t.Log.Error("init.dial", "err", err, "peer", p)
		}
	}

	return nil
}

func (t *Mesh) Close() error {
	errarr := make([]error, 0, 3)
	errarr = append(errarr, t.Base.Close())
	errarr = append(errarr, t.container.Close())
	if closer, ok := t.dialer.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}
	return errors.Join(errarr...)
}

func (t *Mesh) Publish(route routers.Route, opt ...routers.PubOpt) (routers.Intent, error) {
}

func (t *Mesh) Subscribe(route routers.Route, opt ...routers.SubOpt) (routers.Interest, error) {
}
