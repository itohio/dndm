package mesh

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	p2ptypes "github.com/itohio/dndm/types/p2p"
)

type Addrbook struct {
	ctx             context.Context
	mu              sync.Mutex
	self            network.Peer
	peers           map[string]*AddrbookEntry
	peerDialerQueue chan *AddrbookEntry
}

func NewAddrbook(self network.Peer, peers []*p2ptypes.AddrbookEntry) *Addrbook {
	pm := make(map[string]*AddrbookEntry, len(peers))
	for _, p := range peers {
		peer := NewAddrbookEntry(p)
		pm[p.Peer] = peer
	}

	ret := &Addrbook{
		self:            self,
		peers:           pm,
		peerDialerQueue: make(chan *AddrbookEntry, NumDialers),
	}

	return ret
}

func (b *Addrbook) Self() network.Peer {
	return b.self
}

func (b *Addrbook) Dials() chan *AddrbookEntry {
	return b.peerDialerQueue
}

func (b *Addrbook) Init(ctx context.Context) {
	b.ctx = ctx

	for _, p := range b.peers {
		b.peerDialerQueue <- p
	}
}

func (b *Addrbook) AddPeer(p network.Peer) {
}

func (b *Addrbook) BanPeer(p network.Peer) {
}

func (b *Addrbook) Peers() []*p2ptypes.AddrbookEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	book := make([]*p2ptypes.AddrbookEntry, 0, len(b.peers))
	for _, p := range b.peers {
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

type AddrbookEntry struct {
	sync.Mutex

	Peer              network.Peer
	MaxAttempts       int
	DefaultBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64

	Attempts    int
	Failed      int
	LastSuccess time.Time
	Backoff     time.Duration
}

func NewAddrbookEntry(p *p2ptypes.AddrbookEntry) *AddrbookEntry {
	ret := &AddrbookEntry{
		Peer:              errors.Must(network.PeerFromString(p.Peer)),
		MaxAttempts:       int(p.MaxAttempts),
		DefaultBackoff:    time.Duration(p.DefaultBackoff),
		MaxBackoff:        time.Duration(p.MaxBackoff),
		BackoffMultiplier: float64(p.BackoffMultiplier),
		Attempts:          int(p.Attempts),
		Failed:            int(p.FailedAttempts),
		LastSuccess:       time.Unix(0, int64(p.LastSuccess)),
		Backoff:           time.Duration(p.Backoff),
	}
	if ret.BackoffMultiplier < .1 {
		ret.BackoffMultiplier = .1
	}
	if ret.DefaultBackoff < time.Millisecond {
		ret.DefaultBackoff = time.Millisecond
	}
	if ret.MaxBackoff < time.Second {
		ret.MaxBackoff = time.Second
	}
	if ret.Backoff < ret.DefaultBackoff {
		ret.Backoff = ret.DefaultBackoff
	}
	return ret
}

func (a *AddrbookEntry) Dial(ctx context.Context, log *slog.Logger, dialer network.Dialer, q chan<- *AddrbookEntry) (io.ReadWriter, error) {
	rw, err := dialer.Dial(ctx, a.Peer)
	a.Lock()
	defer a.Unlock()
	if err == nil {
		a.Attempts++
		a.Failed = 0
		a.LastSuccess = time.Now()
		a.Backoff = a.DefaultBackoff
		return rw, nil
	}

	a.Failed++
	if a.Failed > a.MaxAttempts {
		log.Info("Max attempts", "num", a.Failed, "peer", a.Peer)
		return nil, errors.ErrNotFound
	}
	if a.Backoff < a.DefaultBackoff {
		a.Backoff = a.DefaultBackoff
	}

	a.Backoff += time.Duration(float64(time.Second) * (a.Backoff.Seconds() * a.BackoffMultiplier))
	if a.Backoff > a.MaxBackoff {
		a.Backoff = a.MaxBackoff
	}

	t := time.NewTimer(a.Backoff)
	go func() {
		select {
		case <-ctx.Done():
		case <-t.C:
			log.Info("Retry", "num", a.Failed, "peer", a.Peer, "backoff", a.Backoff)
		}

		select {
		case <-ctx.Done():
		case q <- a:
		}
	}()

	return nil, errors.ErrRetry
}
