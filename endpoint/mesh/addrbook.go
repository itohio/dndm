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
	log             *slog.Logger
	self            network.Peer
	minConnected    int
	mu              sync.Mutex
	peers           map[string]*AddrbookEntry
	peerDialerQueue chan *AddrbookEntry
}

func NewAddrbook(self network.Peer, peers []*p2ptypes.AddrbookEntry, minConnected int) *Addrbook {
	pm := make(map[string]*AddrbookEntry, len(peers))
	for _, p := range peers {
		peer := NewAddrbookEntry(p)
		peer.outbound = true
		pm[peer.Peer.ID()] = peer
	}

	ret := &Addrbook{
		self:            self,
		peers:           pm,
		peerDialerQueue: make(chan *AddrbookEntry, NumDialers),
		minConnected:    minConnected,
	}

	return ret
}

func (b *Addrbook) Self() network.Peer {
	return b.self
}

func (b *Addrbook) Dials() chan *AddrbookEntry {
	return b.peerDialerQueue
}

func (b *Addrbook) Init(ctx context.Context, log *slog.Logger) error {
	b.ctx = ctx
	b.log = log

	go func() {
		t := time.NewTicker(time.Second * 30)
		for {
			b.monitor()
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	return nil
}

func (b *Addrbook) monitor() {
	n := b.NumConnectedPeers()
	b.log.Info("Addrbook", "active", n)

	if n >= b.minConnected {
		return
	}

	// simplest algorithm ever
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, p := range b.peers {
		if !p.outbound {
			continue
		}
		if p.IsActive() {
			continue
		}
		if p.Failed > p.MaxAttempts {
			continue
		}

		select {
		case <-b.ctx.Done():
			return
		case b.peerDialerQueue <- p:
		default:
		}
	}
}

func (b *Addrbook) NumConnectedPeers() int {
	b.mu.Lock()
	n := 0
	for _, p := range b.peers {
		if p.IsActive() {
			n++
		}
	}
	b.mu.Unlock()
	return n
}

func (b *Addrbook) AddPeers(peers ...network.Peer) {
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
				Persistent:        p.Persistent,
			},
		)
		p.Unlock()
	}
	return book

}

func (b *Addrbook) Entry(p network.Peer) (*AddrbookEntry, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.peers[p.ID()]
	return e, ok
}

func (b *Addrbook) AddConn(c network.Conn) error {
	e, ok := b.Entry(c.Remote())
	if !ok {
		abe := p2ptypes.AddrbookEntryFromPeer(c.Remote().String())
		e = NewAddrbookEntry(abe)
	}

	return e.SetConn(c)
}

func (b *Addrbook) DelConn(c network.Conn) {
	if e, ok := b.Entry(c.Remote()); ok {
		e.SetConn(nil)
	}
}

type AddrbookEntry struct {
	sync.Mutex

	outbound bool

	Peer              network.Peer
	Persistent        bool
	MaxAttempts       int
	DefaultBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64

	Attempts    int
	Failed      int
	LastSuccess time.Time
	Backoff     time.Duration

	conn    network.Conn
	dialing bool
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

func (a *AddrbookEntry) IsActive() bool {
	a.Lock()
	b := a.conn != nil
	a.Unlock()
	return b || a.dialing
}

func (a *AddrbookEntry) SetConn(c network.Conn) error {
	a.Lock()
	defer a.Unlock()

	if a.conn != nil && c != nil {
		return errors.ErrDuplicate
	}

	a.conn = c
	c.OnClose(func() {
		a.SetConn(nil)
	})

	return nil
}

func (a *AddrbookEntry) Dial(ctx context.Context, log *slog.Logger, dialer network.Dialer, q chan<- *AddrbookEntry) (io.ReadWriter, error) {
	a.Lock()
	a.dialing = true
	defer func() { a.dialing = false }()
	if a.conn != nil {
		a.Unlock()
		log.Error("AddrbookEntry.Dial", "err", "connection already set")
		return nil, errors.ErrDuplicate
	}
	a.Unlock()

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
