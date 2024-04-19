package mesh

import (
	"context"
	"io"
	"log/slog"
	"maps"
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
		peer := NewAddrbookEntry(slog.Default(), p)
		peer.outbound = true
		pm[peer.Peer.Address()] = peer
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

	b.mu.Lock()
	for _, p := range b.peers {
		p.log = log
	}
	b.mu.Unlock()

	go func() {
		t := time.NewTicker(time.Second * 10)
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
	// simplest algorithm ever
	b.mu.Lock()
	defer b.mu.Unlock()
	// b.log.Info("Monitor", "peers", len(b.peers))
	for k, p := range b.peers {
		b.log.Info("Monitor", "addr", k, "p", p.Peer.String(), "out", p.outbound, "active", p.IsActive(), "failed", p.Failed, "persistent", p.Persistent)
		if !p.outbound {
			continue
		}
		if p.IsActive() {
			continue
		}
		if p.Failed > p.MaxAttempts && !p.Persistent {
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

func (b *Addrbook) AddPeers(outbound bool, peers ...network.Peer) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, p := range peers {
		if b.self.Equal(p) {
			continue
		}
		if _, ok := b.peers[p.Address()]; ok {
			continue
		}

		abe := p2ptypes.AddrbookEntryFromPeer(p.String())
		e := NewAddrbookEntry(b.log, abe)
		e.outbound = outbound
		b.peers[p.Address()] = e
		b.log.Info("Addrbook.AddPeers", "peer", p)
	}
	return nil
}

// SetSharedPeers sets the list of peers that was shared with the target peer.
// The peers returned by ActivePeers will be excluded from the returned list
func (b *Addrbook) SetSharedPeers(remote network.Peer, activePeers []string) {
	entry, ok := b.Entry(remote)
	if !ok {
		return
	}

	entry.SetSharedPeers(activePeers)
}

// ActivePeers returns a list of active peers to be shared to the remote peer.
// The remote peer itself will be excluded as well as any peers that were already shared with the remote.
func (b *Addrbook) ActivePeers(remote network.Peer) []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	sharedPeers := make(map[string]struct{})
	if e, remoteFound := b.peers[remote.Address()]; remoteFound {
		sharedPeers = e.SharedPeers()
	}

	peers := make([]string, 0, len(b.peers))
	for _, p := range b.peers {
		p.Lock()
		peer := p.Peer.String()
		p.Unlock()

		b.log.Info("Addrbook ACTIVE PEERS", "peer", p.Peer, "active", p.IsActive())

		if !p.IsActive() {
			continue
		}

		if p.Peer.Equal(remote) {
			continue
		}

		if _, ok := sharedPeers[peer]; ok {
			continue
		}

		peers = append(peers, peer)
	}
	return peers
}

func (b *Addrbook) Addrbook() []*p2ptypes.AddrbookEntry {
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
	e, ok := b.peers[p.Address()]
	return e, ok
}

func (b *Addrbook) AddConn(p network.Peer, outbound bool, c network.Conn) error {
	if b.self.Equal(p) {
		return nil
	}

	e, ok := b.Entry(p)
	if !ok {
		abe := p2ptypes.AddrbookEntryFromPeer(p.String())
		e = NewAddrbookEntry(b.log, abe)
		e.outbound = outbound
		b.peers[p.Address()] = e
		b.log.Info("Addrbook.AddConn", "peer", p)
	}

	if c == nil {
		return nil
	}

	return e.SetConn(c)
}

func (b *Addrbook) DelConn(p network.Peer, c network.Conn) {
	if e, ok := b.Entry(p); ok {
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

	sharedPeers map[string]struct{}

	log *slog.Logger
}

func NewAddrbookEntry(log *slog.Logger, p *p2ptypes.AddrbookEntry) *AddrbookEntry {
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
		log:               log,
		sharedPeers:       make(map[string]struct{}),
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

func (a *AddrbookEntry) WasPeerShared(peer string) bool {
	a.Lock()
	_, ok := a.sharedPeers[peer]
	a.Unlock()
	return ok
}

func (a *AddrbookEntry) SetSharedPeers(peers []string) {
	a.Lock()
	defer a.Unlock()

	for _, p := range peers {
		a.sharedPeers[p] = struct{}{}
	}
}

func (a *AddrbookEntry) SharedPeers() map[string]struct{} {
	a.Lock()
	sp := maps.Clone(a.sharedPeers)
	a.Unlock()
	return sp
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
		a.log.Error("Setting Connection on Active Peer", "peer", a.Peer, "conn.remote", c.Remote(), "conn.local", c.Local())
		return errors.ErrDuplicate
	}

	a.sharedPeers = make(map[string]struct{})
	a.conn = c
	if c == nil {
		return nil
	}

	c.OnClose(func() {
		a.SetConn(nil)
		a.log.Info("Addrbook Peer closed", "peer", a.Peer)
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
