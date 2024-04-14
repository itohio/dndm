package mesh

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	p2ptypes "github.com/itohio/dndm/types/p2p"
)

var _ routers.Transport = (*Mesh)(nil)

const NumDialers = 10

type Mesh struct {
	*routers.Base
	timeout      time.Duration
	pingDuration time.Duration
	dialer       dialers.Dialer
	container    *routers.Container

	peerDialerQueue chan *AddrbookEntry
	mu              sync.Mutex
	peers           map[string]*AddrbookEntry
}

func New(name string, size, numDialers int, timeout, pingDuration time.Duration, node dialers.Dialer, peers []*p2ptypes.AddrbookEntry) (*Mesh, error) {
	pm := make(map[string]*AddrbookEntry, len(peers))
	for _, p := range peers {
		peer := NewAddrbookEntry(p)

		if _, ok := pm[peer.Peer.ID()]; ok {
			continue
		}
		pm[p.Peer] = peer
	}
	return &Mesh{
		Base:            routers.NewBase(name, size),
		timeout:         timeout,
		pingDuration:    pingDuration,
		dialer:          node,
		peers:           pm,
		container:       routers.NewContainer(name, size),
		peerDialerQueue: make(chan *AddrbookEntry, NumDialers),
	}, nil
}

func (t *Mesh) Addrbook() []*p2ptypes.AddrbookEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	book := make([]*p2ptypes.AddrbookEntry, 0, len(t.peers))
	for _, p := range t.peers {
		p.Lock()
		book = append(
			book,
			&p2ptypes.AddrbookEntry{
				Peer:              p.Peer.String(),
				MaxAttempts:       uint32(p.MaxAttempts),
				DefaultBackoff:    uint64(p.DefaultBackoff),
				MaxBackoff:        uint64(p.MaxBackoff),
				BackoffMultiplier: float32(p.BackOffMultiplier),
				Attempts:          uint32(p.Attempts),
				FailedAttempts:    uint32(p.Failed),
				LastSuccess:       uint64(p.LastSuccess.UnixNano()),
				Backoff:           uint64(p.BackOff),
			},
		)
		p.Unlock()
	}
	return book
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

	for i := 0; i < NumDialers; i++ {
		go t.dialerLoop()
	}

	for _, p := range t.peers {
		t.peerDialerQueue <- p
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
	return t.container.Publish(route, opt...)
}

func (t *Mesh) Subscribe(route routers.Route, opt ...routers.SubOpt) (routers.Interest, error) {
	return t.container.Subscribe(route, opt...)
}
