package mesh

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	p2ptypes "github.com/itohio/dndm/types/p2p"
)

type Addrbook struct {
	ctx             context.Context
	log             *slog.Logger
	self            dndm.Peer
	minConnected    int
	mu              sync.Mutex
	peers           map[string]*AddrbookEntry
	peerDialerQueue chan *AddrbookEntry
}

func NewAddrbook(self dndm.Peer, peers []*p2ptypes.AddrbookEntry, minConnected int) *Addrbook {
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

func (b *Addrbook) Self() dndm.Peer {
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

func (b *Addrbook) AddPeers(outbound bool, peers ...dndm.Peer) error {
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
func (b *Addrbook) SetSharedPeers(remote dndm.Peer, activePeers []string) {
	entry, ok := b.Entry(remote)
	if !ok {
		return
	}

	entry.SetSharedPeers(activePeers)
}

// ActivePeers returns a list of active peers to be shared to the remote peer.
// The remote peer itself will be excluded as well as any peers that were already shared with the remote.
func (b *Addrbook) ActivePeers(remote dndm.Peer) []string {
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

func (b *Addrbook) Entry(p dndm.Peer) (*AddrbookEntry, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.peers[p.Address()]
	return e, ok
}

// AddConn records c for peer p, whose identity p is as the peer declared it in the
// handshake. outbound reports that this node dialled c. See AddrbookEntry.SetConn.
func (b *Addrbook) AddConn(p dndm.Peer, outbound bool, c network.Conn) error {
	return b.addConn(p, outbound, c, p)
}

// NoteDial records a connection this node dialled to target before the handshake has
// told it the peer's declared identity. It never displaces or refuses a live connection;
// the handshake's AddConn decides.
func (b *Addrbook) NoteDial(target dndm.Peer, c network.Conn) error {
	return b.addConn(target, true, c, nil)
}

func (b *Addrbook) addConn(p dndm.Peer, outbound bool, c network.Conn, remote dndm.Peer) error {
	if b.self.Equal(p) {
		return nil
	}

	// Look up and insert under b.mu: monitor and ActivePeers iterate b.peers concurrently.
	b.mu.Lock()
	e, ok := b.peers[p.Address()]
	if !ok {
		abe := p2ptypes.AddrbookEntryFromPeer(p.String())
		e = NewAddrbookEntry(b.log, abe)
		e.outbound = outbound
		b.peers[p.Address()] = e
	}
	b.mu.Unlock()
	if !ok {
		b.log.Info("Addrbook.AddConn", "peer", p)
	}

	if c == nil {
		return nil
	}

	return e.SetConn(c, outbound, b.self, remote)
}

// DelConn forgets c if it is the entry's current connection.
func (b *Addrbook) DelConn(p dndm.Peer, c network.Conn) {
	e, ok := b.Entry(p)
	if !ok {
		return
	}
	e.Lock()
	gen, current := e.gen, e.conn
	e.Unlock()
	if current == c {
		e.clearConn(gen)
	}
}

type AddrbookEntry struct {
	sync.Mutex

	outbound bool

	Peer              dndm.Peer
	Persistent        bool
	MaxAttempts       int
	DefaultBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64

	Attempts    int
	Failed      int
	LastSuccess time.Time
	Backoff     time.Duration

	conn         network.Conn
	connOutbound bool   // conn was dialled by this node
	gen          uint64 // increments on every recorded connection
	dialing      bool

	sharedPeers map[string]struct{}

	log *slog.Logger
}

func NewAddrbookEntry(log *slog.Logger, p *p2ptypes.AddrbookEntry) *AddrbookEntry {
	ret := &AddrbookEntry{
		Peer:              errors.Must(dndm.PeerFromString(p.Peer)),
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
	defer a.Unlock()
	return a.conn != nil || a.dialing
}

// SetConn records c as the entry's connection; outbound reports that this node dialled
// it, self is this node's own peer and remote the peer's declared identity. Each recorded
// connection gets a new generation, and its close callback clears the entry only if that
// generation is still current, so a late callback of an old connection never wipes a
// newer one (DND-008, close-reconnect.md).
//
// If a live connection is already recorded, both sides must keep the same one: the
// connection dialled by the peer with the lower declared identity (Peer.String()) wins.
// If both were dialled by the same side, the newer one wins. A losing c is refused with
// ErrDuplicate (its owner closes it); a losing current connection is replaced and closed.
// A current connection that is already closed is always replaced.
//
// A nil remote means the peer's identity is not known yet (a dial whose handshake has not
// finished): c is then recorded only if no live connection is, and never displaces or
// refuses one. The decision is made when the handshake completes (review R-DND-R8 N2).
// A nil c is ignored.
func (a *AddrbookEntry) SetConn(c network.Conn, outbound bool, self, remote dndm.Peer) error {
	if c == nil {
		return nil
	}
	if connClosed(c) {
		return errors.ErrClosed // never record a connection that is already gone
	}
	a.Lock()
	old := a.conn
	if old == c {
		a.Unlock()
		return nil
	}
	if old != nil && !connClosed(old) {
		if remote == nil {
			a.Unlock()
			return nil // identity unknown: leave the decision to the handshake
		}
		if !a.newWinsLocked(outbound, self, remote) {
			a.Unlock()
			a.log.Info("Addrbook: refusing duplicate connection", "peer", remote, "outbound", outbound)
			return errors.ErrDuplicate
		}
	}
	a.gen++
	gen := a.gen
	a.conn = c
	a.connOutbound = outbound
	a.sharedPeers = make(map[string]struct{})
	a.Unlock()

	c.OnClose(func() {
		if a.clearConn(gen) {
			a.log.Info("Addrbook Peer closed", "peer", a.Peer)
		}
	})
	if old != nil {
		a.log.Info("Addrbook: replacing connection", "peer", a.Peer, "outbound", outbound)
		old.Close() // outside the lock: its close callback takes it
	}
	return nil
}

// newWinsLocked applies the simultaneous-connection rule with declared identities, which
// both sides know once the handshake has run. The caller holds a's lock.
func (a *AddrbookEntry) newWinsLocked(newOutbound bool, self, remote dndm.Peer) bool {
	if newOutbound == a.connOutbound {
		return true // same dialer: the peer redialled, so the old connection is stale
	}
	selfLower := self.String() < remote.String()
	// The new connection was dialled by us if newOutbound, else by the peer.
	return newOutbound == selfLower
}

// clearConn forgets the connection of generation gen, if it is still current.
func (a *AddrbookEntry) clearConn(gen uint64) bool {
	a.Lock()
	defer a.Unlock()
	if a.gen != gen || a.conn == nil {
		return false
	}
	a.conn = nil
	a.sharedPeers = make(map[string]struct{})
	return true
}

// connClosed reports whether c has already been closed, for connections that expose
// their context (stream connections do, through dndm.Base).
func connClosed(c network.Conn) bool {
	if cc, ok := c.(interface{ Ctx() context.Context }); ok {
		return cc.Ctx().Err() != nil
	}
	return false
}

func (a *AddrbookEntry) Dial(ctx context.Context, log *slog.Logger, dialer network.Dialer, q chan<- *AddrbookEntry) (io.ReadWriter, error) {
	a.Lock()
	if a.conn != nil {
		a.Unlock()
		log.Error("AddrbookEntry.Dial", "err", "connection already set")
		return nil, errors.ErrDuplicate
	}
	a.dialing = true
	a.Unlock()

	rw, err := dialer.Dial(ctx, a.Peer)
	a.Lock()
	defer a.Unlock()
	a.dialing = false
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
	failed, backoff := a.Failed, a.Backoff // read under the lock; the goroutine runs without it
	go func() {
		select {
		case <-ctx.Done():
		case <-t.C:
			log.Info("Retry", "num", failed, "peer", a.Peer, "backoff", backoff)
		}

		select {
		case <-ctx.Done():
		case q <- a:
		}
	}()

	return nil, errors.ErrRetry
}
