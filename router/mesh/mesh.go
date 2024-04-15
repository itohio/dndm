package mesh

import (
	"context"
	"io"
	"log/slog"
	"sync"
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

	peerDialerQueue chan *AddrbookEntry
	mu              sync.Mutex
	peers           map[string]*AddrbookEntry
}

func New(name string, size, numDialers int, timeout, pingDuration time.Duration, node network.Dialer, peers []*p2ptypes.AddrbookEntry) (*Mesh, error) {
	pm := make(map[string]*AddrbookEntry, len(peers))
	for _, p := range peers {
		peer := NewAddrbookEntry(p)
		pm[p.Peer] = peer
	}
	return &Mesh{
		Base:            router.NewBase(name, size),
		timeout:         timeout,
		pingDuration:    pingDuration,
		dialer:          node,
		peers:           pm,
		container:       router.NewContainer(name, size),
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
				BackoffMultiplier: float32(p.BackoffMultiplier),
				Attempts:          uint32(p.Attempts),
				FailedAttempts:    uint32(p.Failed),
				LastSuccess:       uint64(p.LastSuccess.UnixNano()),
				Backoff:           uint64(p.Backoff),
			},
		)
		p.Unlock()
	}
	return book
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

	for _, p := range t.peers {
		t.peerDialerQueue <- p
	}

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
