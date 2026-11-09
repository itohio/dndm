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
// Send must not keep frame after it returns and must not call back into the Link.
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
	session     uint64 // ours; 0 until Start
	helloSent   bool
	peerSession uint64 // 0 until the peer's Hello
	peerRoutes  int    // the peer's max_routes

	ids       idTable
	tx        map[routeKey]*txRoute
	unbinding map[uint32]struct{}
	pending   map[uint32]routeKey // Bind sent, BindAck not yet received
	rx        map[uint32]routeKey // the peer's ids bound on this side

	buf   []byte
	stats Stats
}

// New returns an idle link. Call Start to send the Hello.
func New(face Face, cfg Config, deliver DeliverFunc) *Link {
	if cfg.MaxRoutes <= 0 {
		cfg.MaxRoutes = 64
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

func (l *Link) resetLocked() {
	l.state = stateIdle
	l.session, l.helloSent, l.peerSession, l.peerRoutes = 0, false, 0, 0
	l.ids = newIDTable()
	l.tx = make(map[routeKey]*txRoute)
	l.unbinding = make(map[uint32]struct{})
	l.pending = make(map[uint32]routeKey)
	l.rx = make(map[uint32]routeKey)
}

// Start begins a session: a fresh random session id and a Hello. The link opens once the
// peer's Hello has arrived as well.
func (l *Link) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cfg.Profile != linktypes.Profile_PROFILE_ONBOARD {
		return fmt.Errorf("%w: %v", ErrUnsupportedProfile, l.cfg.Profile)
	}
	if l.state == stateClosed {
		return ErrClosed
	}
	return l.sendHelloLocked()
}

func (l *Link) sendHelloLocked() error {
	l.session = newSession()
	l.helloSent = true
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

// Restart forgets the session (a reconnect): every id, binding and route is dropped. Call
// Start to begin the next session.
func (l *Link) Restart() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.resetLocked()
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
		// Accepted in any state: ONBOARD has no authentication to check. Secured profiles
		// must drop an unauthenticated Close before the handshake (DND-009 acceptance).
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
	if h.GetSession() == 0 || h.GetSession() == l.peerSession {
		return nil // a repeated Hello of the current session
	}
	if l.peerSession != 0 {
		// The peer started a new session (it reconnected): its ids and ours are gone.
		l.resetLocked()
		l.peerSession, l.peerRoutes = h.GetSession(), int(h.GetMaxRoutes())
		return l.sendHelloLocked()
	}
	l.peerSession, l.peerRoutes = h.GetSession(), int(h.GetMaxRoutes())
	l.maybeOpenLocked()
	return nil
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
