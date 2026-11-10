// Package linkctx is the link-context core (DND-022, docs/design/link-context-wire.md): one
// Link per connection over one face. A link opens with a Hello exchange, binds route names
// to small per-session ids (Bind/BindAck), carries DATA frames of 4 B + payload, and
// releases ids with Unbind/UnbindAck.
//
// Implemented: the ONBOARD profile (the handshake is the Hello exchange), per-session
// lowest-free route ids with a reuse quarantine, the receiver's route table cap, refusing
// a Bind that would retarget a live id, and dropping every frame except Hello/Handshake/
// Close until the link is open. Not yet: secured profiles (keys and tags, DND-009), the
// counter in DATA frames (see counter.go), Bind retries on lossy faces, the Bind rate
// limit, enforcing Hello.max_frame on receive, v1 negotiation and trusted-origin routes
// (DND-022 acceptance A5/A8).
package linkctx

import (
	"crypto/rand"
	"encoding/binary"
	stderrors "errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/itohio/dndm/codec"
	derrors "github.com/itohio/dndm/errors"
	linktypes "github.com/itohio/dndm/types/link"
	"google.golang.org/protobuf/proto"
)

// LinkVersion is the link protocol version this package speaks (Hello.version).
const LinkVersion = 2

var (
	ErrNotOpen            = stderrors.New("linkctx: link not open")
	ErrTableFull          = stderrors.New("linkctx: route table full")
	ErrNotBound           = stderrors.New("linkctx: route not bound")
	ErrUnsupportedProfile = stderrors.New("linkctx: profile not supported yet")
	ErrClosed             = stderrors.New("linkctx: link closed")
)

// Face sends one delimited frame (UDP datagram, COBS on serial, length prefix on TCP).
// Send must not keep frame after it returns, must not call back into the Link, and must
// not block: the link calls it with its lock held, so a blocking Send stalls Receive. A
// face with back-pressure (TCP) queues or drops instead (review R-DND-022 L5).
type Face interface {
	Send(frame []byte) error
}

// DeliverFunc receives the payload of a DATA frame for a bound route. payload aliases the
// received frame and is valid only during the call. It runs with the link locked, so it
// must not call back into the Link.
type DeliverFunc func(path, typ string, payload []byte)

// Config is fixed per link.
type Config struct {
	Profile    linktypes.Profile // only PROFILE_ONBOARD for now
	MaxRoutes  int               // route table cap per direction (Hello.max_routes); default 64
	MaxFrame   int               // largest frame this side accepts (Hello.max_frame); default 1500
	Quarantine time.Duration     // reuse delay for a freed id: 0 on stream faces, 2x the retry timeout on reordering faces
	// Accept resolves a peer's Bind in this link's zone table; nil accepts every route.
	Accept func(path, typ string) linktypes.BindStatus
	Now    func() time.Time // clock; nil means time.Now
}

// Stats counts frames the link refused.
type Stats struct {
	DroppedBeforeOpen uint64 // DATA or control (other than Hello/Handshake/Close) before the handshake (A1)
	DroppedUnbound    uint64 // DATA for an id that is not bound (late, unbound or never bound)
	BindsRefused      uint64 // Binds answered with a status other than OK
	DroppedStale      uint64 // Hello or Close naming a session that is not current (retired, spoofed or unknown)
}

type state int

const (
	stateIdle state = iota
	stateOpen
	stateClosed
)

type routeKey struct{ path, typ string }

type txRoute struct {
	id    uint32
	bound bool // BindAck OK received
}

type Link struct {
	mu      sync.Mutex
	cfg     Config
	face    Face
	deliver DeliverFunc

	state       state
	session     uint64 // ours; 0 until Start, then kept until Restart (also across a peer's reconnect)
	helloSent   bool
	sentHello   bool     // we sent a Hello since the last one we received: do not answer the next
	peerSession uint64   // 0 until the peer's Hello
	peerRoutes  int      // the peer's max_routes
	retired     []uint64 // the peer's previous sessions (last retiredKeep); their Hellos are dropped
	suspect     []uint64 // retired sessions on probation after our Restart (see Restart)

	ids       idTable
	tx        map[routeKey]*txRoute
	unbinding map[uint32]struct{}
	pending   map[uint32]routeKey // Bind sent, BindAck not yet received
	rx        map[uint32]routeKey // the peer's ids bound on this side

	buf   []byte
	stats Stats
}

// DefaultMaxRoutes is the route table cap when none is configured, and what a peer's Hello
// without max_routes (proto3 zero: an older or minimal peer) is taken to mean.
const DefaultMaxRoutes = 64

// retiredKeep is how many of the peer's previous sessions are remembered, so a late
// duplicate Hello from one of them (a reordering face) is dropped instead of resetting
// the link. Every replaced session takes a slot, including spoofed ones and those
// re-retired after a probation (Restart), so 4 was too few: with five reconnects or
// spoofs, the oldest duplicates got through again. A late duplicate of a session older
// than the last 32 replaced ones is still taken for a reconnect.
const retiredKeep = 32

// New returns an idle link. Call Start to send the Hello.
func New(face Face, cfg Config, deliver DeliverFunc) *Link {
	if cfg.MaxRoutes <= 0 {
		cfg.MaxRoutes = DefaultMaxRoutes
	}
	if cfg.MaxFrame <= 0 {
		cfg.MaxFrame = 1500
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	l := &Link{cfg: cfg, face: face, deliver: deliver}
	l.resetLocked()
	return l
}

// resetLocked forgets the session on both sides: our session, the peer's, and every id
// table. The retired peer sessions are kept here; Restart puts them on probation.
func (l *Link) resetLocked() {
	l.state = stateIdle
	l.session, l.helloSent, l.sentHello, l.peerSession, l.peerRoutes = 0, false, false, 0, 0
	l.resetTablesLocked()
}

// resetTablesLocked drops every route id, binding and route of the current session pair.
func (l *Link) resetTablesLocked() {
	l.ids = newIDTable()
	l.tx = make(map[routeKey]*txRoute)
	l.unbinding = make(map[uint32]struct{})
	l.pending = make(map[uint32]routeKey)
	l.rx = make(map[uint32]routeKey)
}

// Start begins a session with a fresh random session id and sends a Hello. On a link that
// has already started, it resends the same Hello (a retry on a lossy face): the session
// changes only after Restart. The link opens once the peer's Hello has arrived as well.
func (l *Link) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cfg.Profile != linktypes.Profile_PROFILE_ONBOARD {
		return fmt.Errorf("%w: %v", ErrUnsupportedProfile, l.cfg.Profile)
	}
	if l.state == stateClosed {
		return ErrClosed
	}
	if l.session == 0 {
		l.session = newSession()
	}
	return l.sendHelloLocked()
}

// sendHelloLocked sends the Hello of the current session.
func (l *Link) sendHelloLocked() error {
	l.helloSent, l.sentHello = true, true
	err := l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Hello{Hello: &linktypes.Hello{
		Version:   LinkVersion,
		Profile:   l.cfg.Profile,
		MaxFrame:  uint32(l.cfg.MaxFrame),
		MaxRoutes: uint32(l.cfg.MaxRoutes),
		Session:   l.session,
	}}})
	l.maybeOpenLocked()
	return err
}

func (l *Link) maybeOpenLocked() {
	if l.state == stateIdle && l.helloSent && l.peerSession != 0 {
		l.state = stateOpen
	}
}

// Restart forgets the session (our reconnect): every id, binding and route is dropped, and
// Start then begins a new session. The peer's session is not retired: a peer that sees our
// new session keeps its own (helloLocked), so its next Hello still carries it.
//
// The retired sessions go on probation. With random session ids, arrival order cannot tell
// a peer's old session from its new one, so we may have retired the peer's live session
// (reordered Hellos, or a spoofed one); keeping it retired would lock the peer out until
// it reconnected (verify DND-022 recheck). Clearing the list instead let late duplicates
// of genuinely old sessions reset the link after our reconnect (review R-DND-022-followups
// D1). So the first peer Hello we accept after Restart, normally the answer to our new
// Hello, fixes the peer's session; the other suspects are retired again. The peer's
// session at the time of Restart is a suspect too: if the peer reconnected meanwhile, it
// is old, and without this it would never be retired.
func (l *Link) Restart() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range append(l.retired, l.peerSession) {
		if s != 0 && !slices.Contains(l.suspect, s) {
			l.suspect = append(l.suspect, s)
		}
	}
	l.retired = l.retired[:0]
	l.resetLocked()
}

// Close sends a Close naming our session (or, before we have one, the peer's) and closes
// the link. Start returns ErrClosed until Restart.
func (l *Link) Close(reason linktypes.CloseReason, text string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state == stateClosed {
		return nil
	}
	session := l.session
	if session == 0 {
		session = l.peerSession
	}
	err := l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Close{Close: &linktypes.Close{
		Reason: reason, Text: text, Session: session,
	}}})
	l.retireLocked(l.peerSession)
	l.resetLocked()
	l.state = stateClosed
	return err
}

// Open reports whether the handshake has completed.
func (l *Link) Open() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state == stateOpen
}

// Routes returns the number of the peer's routes bound on this side.
func (l *Link) Routes() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.rx)
}

// Stats returns a copy of the refusal counters.
func (l *Link) Stats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stats
}

// Bind allocates an id for the route and sends a Bind. The route can carry DATA once the
// peer's BindAck OK arrives. Binding a route that already has an id returns that id.
func (l *Link) Bind(path, typ string) (uint32, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != stateOpen {
		return 0, ErrNotOpen
	}
	k := routeKey{path, typ}
	if r, ok := l.tx[k]; ok {
		return r.id, nil
	}
	limit := min(l.cfg.MaxRoutes, l.peerRoutes)
	id, ok := l.ids.alloc(limit, l.cfg.Now())
	if !ok {
		return 0, ErrTableFull
	}
	l.tx[k] = &txRoute{id: id}
	l.pending[id] = k
	err := l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Bind{Bind: &linktypes.Bind{
		RouteId: id, Path: path, Type: typ,
	}}})
	return id, err
}

// Unbind stops DATA on the route and sends an Unbind. Its id is reusable after the peer's
// UnbindAck plus the quarantine.
func (l *Link) Unbind(path, typ string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != stateOpen {
		return ErrNotOpen
	}
	k := routeKey{path, typ}
	r, ok := l.tx[k]
	if !ok {
		return ErrNotBound
	}
	delete(l.tx, k)
	delete(l.pending, r.id)
	l.unbinding[r.id] = struct{}{}
	return l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Unbind{Unbind: &linktypes.Unbind{RouteId: r.id}}})
}

// Send sends payload as a DATA frame on a bound route.
func (l *Link) Send(path, typ string, payload []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != stateOpen {
		return ErrNotOpen
	}
	r, ok := l.tx[routeKey{path, typ}]
	if !ok || !r.bound {
		return ErrNotBound
	}
	var err error
	l.buf, err = codec.AppendLinkData(l.buf[:0], codec.LinkData{RouteID: r.id, Payload: payload})
	if err != nil {
		return err
	}
	return l.face.Send(l.buf)
}

// Receive handles one frame from the face. Malformed frames return an error wrapping
// errors.ErrBadFrame; refused frames are counted in Stats and return nil.
func (l *Link) Receive(frame []byte) error {
	d, err := codec.DecodeLinkData(frame)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state == stateClosed {
		return ErrClosed
	}
	if d.RouteID != 0 {
		l.dataLocked(d)
		return nil
	}
	var f linktypes.LinkFrame
	if err := proto.Unmarshal(frame, &f); err != nil {
		return fmt.Errorf("%w: %v", derrors.ErrBadFrame, err)
	}
	switch c := f.Control.(type) {
	case *linktypes.LinkFrame_Hello:
		return l.helloLocked(c.Hello)
	case *linktypes.LinkFrame_Handshake:
		return nil // ONBOARD has no handshake steps beyond Hello
	case *linktypes.LinkFrame_Close:
		// A Close names a current session: the sender's own, or the receiver's when the
		// sender has none yet. Any other is stale (an old session's late Close on a
		// reordering face) or forged, and is dropped. On ONBOARD nothing is authenticated;
		// secured profiles must also drop an unauthenticated Close before the handshake
		// (DND-009 acceptance).
		if s := c.Close.GetSession(); s == 0 || (s != l.session && s != l.peerSession) {
			l.stats.DroppedStale++
			return nil
		}
		l.retireLocked(l.peerSession)
		l.resetLocked()
		l.state = stateClosed
		return nil
	}
	if l.state != stateOpen {
		l.stats.DroppedBeforeOpen++
		return nil
	}
	switch c := f.Control.(type) {
	case *linktypes.LinkFrame_Bind:
		return l.bindLocked(c.Bind)
	case *linktypes.LinkFrame_BindAck:
		l.bindAckLocked(c.BindAck)
	case *linktypes.LinkFrame_Unbind:
		delete(l.rx, c.Unbind.GetRouteId())
		return l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_UnbindAck{UnbindAck: &linktypes.UnbindAck{RouteId: c.Unbind.GetRouteId()}}})
	case *linktypes.LinkFrame_UnbindAck:
		id := c.UnbindAck.GetRouteId()
		if _, ok := l.unbinding[id]; ok {
			delete(l.unbinding, id)
			l.ids.release(id, l.cfg.Now(), l.cfg.Quarantine)
		}
	}
	return nil
}

func (l *Link) dataLocked(d codec.LinkData) {
	if l.state != stateOpen {
		l.stats.DroppedBeforeOpen++
		return
	}
	k, ok := l.rx[d.RouteID]
	if !ok {
		l.stats.DroppedUnbound++
		return
	}
	if l.deliver != nil {
		l.deliver(k.path, k.typ, d.Payload)
	}
}

// helloLocked applies the session rules (design link-context-wire §4, review R-DND-022 L1):
//   - another version or profile closes the link with PROFILE_MISMATCH;
//   - a Hello from one of the peer's retired sessions (a late duplicate on a reordering
//     face) is dropped and counted, so it cannot reset the link;
//   - a new peer session while one is known means the peer reconnected: the id tables
//     are reset, and our own session is kept, so the peer sees nothing new to react to;
//   - a Hello is answered with ours only if we have not sent one since the last Hello we
//     received. A retry or a reconnecting peer gets one answer, and two sides can never
//     keep answering each other.
func (l *Link) helloLocked(h *linktypes.Hello) error {
	if h.GetVersion() != LinkVersion || h.GetProfile() != l.cfg.Profile {
		reason := linktypes.CloseReason_CLOSE_REASON_PROFILE_MISMATCH
		err := l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_Close{Close: &linktypes.Close{
			Reason: reason, Text: fmt.Sprintf("version %d profile %v", h.GetVersion(), h.GetProfile()), Session: h.GetSession(),
		}}})
		l.resetLocked()
		l.state = stateClosed
		return err
	}
	s := h.GetSession()
	if s == 0 || slices.Contains(l.retired, s) {
		l.stats.DroppedStale++
		return nil
	}
	answer := l.session != 0 && !l.sentHello
	l.sentHello = false
	switch {
	case s == l.peerSession:
		// a repeat of the current session: a retry, or an answer to ours
	case l.peerSession != 0:
		// the peer reconnected: its ids and ours are gone
		l.retireLocked(l.peerSession)
		l.resetTablesLocked()
		l.peerSession, l.peerRoutes = s, helloRoutes(h)
	default:
		l.peerSession, l.peerRoutes = s, helloRoutes(h)
	}
	l.endProbationLocked(s)
	l.maybeOpenLocked()
	if answer {
		return l.sendHelloLocked()
	}
	return nil
}

// endProbationLocked ends the probation started by Restart: s, the peer session just
// accepted, is the peer's; every other suspect is retired again.
func (l *Link) endProbationLocked(s uint64) {
	for _, x := range l.suspect {
		if x != s {
			l.retireLocked(x)
		}
	}
	l.suspect = l.suspect[:0]
}

// retireLocked remembers one of the peer's sessions as finished.
func (l *Link) retireLocked(s uint64) {
	if s == 0 || slices.Contains(l.retired, s) {
		return
	}
	if len(l.retired) == retiredKeep {
		l.retired = l.retired[1:]
	}
	l.retired = append(l.retired, s)
}

// helloRoutes is the peer's route table cap; 0 (not set) means DefaultMaxRoutes.
func helloRoutes(h *linktypes.Hello) int {
	if r := int(h.GetMaxRoutes()); r > 0 {
		return r
	}
	return DefaultMaxRoutes
}

func (l *Link) bindLocked(b *linktypes.Bind) error {
	id, k := b.GetRouteId(), routeKey{b.GetPath(), b.GetType()}
	status := linktypes.BindStatus_BIND_STATUS_OK
	switch cur, live := l.rx[id]; {
	case id == 0:
		status = linktypes.BindStatus_BIND_STATUS_DENIED
	case live && cur != k:
		status = linktypes.BindStatus_BIND_STATUS_DENIED // never retarget a live id (A3)
	case live:
		// the same Bind again: acknowledge it again
	case len(l.rx) >= l.cfg.MaxRoutes:
		status = linktypes.BindStatus_BIND_STATUS_TABLE_FULL
	case l.cfg.Accept != nil:
		status = l.cfg.Accept(k.path, k.typ)
	}
	if status == linktypes.BindStatus_BIND_STATUS_OK {
		l.rx[id] = k
	} else {
		l.stats.BindsRefused++
	}
	return l.sendControlLocked(&linktypes.LinkFrame{Control: &linktypes.LinkFrame_BindAck{BindAck: &linktypes.BindAck{RouteId: id, Status: status}}})
}

func (l *Link) bindAckLocked(a *linktypes.BindAck) {
	id := a.GetRouteId()
	k, ok := l.pending[id]
	if !ok {
		return // late or duplicate ack
	}
	delete(l.pending, id)
	if a.GetStatus() == linktypes.BindStatus_BIND_STATUS_OK {
		l.tx[k].bound = true
		return
	}
	delete(l.tx, k)
	l.ids.free(id) // never carried DATA: no quarantine
}

func (l *Link) sendControlLocked(f *linktypes.LinkFrame) error {
	var err error
	l.buf, err = proto.MarshalOptions{}.MarshalAppend(l.buf[:0], f)
	if err != nil {
		return err
	}
	return l.face.Send(l.buf)
}

func newSession() uint64 {
	var b [8]byte
	for {
		_, _ = rand.Read(b[:]) // crypto/rand.Read never fails (Go 1.24)
		if s := binary.LittleEndian.Uint64(b[:]); s != 0 {
			return s
		}
	}
}
