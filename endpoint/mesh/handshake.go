package mesh

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/remote"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	"github.com/itohio/dndm/network/stream"
	types "github.com/itohio/dndm/types/core"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"google.golang.org/protobuf/proto"
)

var (
	_ dndm.Endpoint      = (*Handshaker)(nil)
	_ dndm.ReadyEndpoint = (*Handshaker)(nil)
)

type HandshakeState int

const (
	HS_INIT HandshakeState = iota
	HS_WAIT
	HS_PEERS
	HS_DONE
)

type Container interface {
	Add(dndm.Endpoint) error
	Remove(dndm.Endpoint) error
}

// Handshaker wraps a remote endpoint and gates Publish/Subscribe on handshake completion.
// state is only touched by Init (before the message handler starts) and by the handlers
// that run on the remote endpoint's message goroutine; Publish/Subscribe use ready.
type Handshaker struct {
	dndm.BaseEndpoint

	state        HandshakeState
	hsCount      int
	rw           io.ReadWriter
	timeout      time.Duration
	pingDuration time.Duration
	conn         network.Conn
	remote       atomic.Pointer[remote.Endpoint]
	addrbook     *Addrbook
	remotePeer   dndm.Peer
	isSeed       bool
	dialer       bool // this node dialled the connection; set in Init before the handler starts

	readyMu sync.Mutex
	ready   bool
	closing bool // set under readyMu before Close waits; no callback starts after that
	onReady []func()
	readyWG sync.WaitGroup // OnReady callbacks; Close waits for them
	closed  atomic.Bool
}

// NewHandshake creates a Endpoint wrapper over Remote endpoint that acts as a handshake middleware.
//
// initial state transitions:
//
//	Connect to remote:
//	1. HS_INIT
//	2. HS_WAIT
//	3. HS_DONE
//
//	Remote connects to us:
//	1. HS_WAIT
//	2. HS_DONE
func NewHandshaker(addrbook *Addrbook, remotePeer dndm.Peer, size int, timeout, pingDuration time.Duration, rw io.ReadWriter, state HandshakeState) *Handshaker {
	ret := &Handshaker{
		BaseEndpoint: dndm.NewEndpointBase(remotePeer.String(), size),
		state:        state,
		rw:           rw,
		timeout:      timeout,
		pingDuration: pingDuration,
		addrbook:     addrbook,
		remotePeer:   remotePeer,
		isSeed:       true,
	}

	return ret
}

func (h *Handshaker) OnClose(f func()) dndm.Endpoint {
	h.AddOnClose(f)
	return h
}

// Close closes the connection and the endpoint once, then waits for running OnReady
// callbacks. Later calls return nil at once; several close paths (context, remote close
// callback, container) call it.
func (h *Handshaker) Close() error {
	if !h.closed.CompareAndSwap(false, true) {
		return nil
	}
	h.Log.Info("Handshaker.Close")
	// From here on OnReady and markReady start nothing, so readyWG.Add never races Wait.
	h.readyMu.Lock()
	h.closing = true
	h.onReady = nil
	h.readyMu.Unlock()

	errarr := make([]error, 0, 3)
	if h.conn != nil {
		errarr = append(errarr, h.conn.Close())
	}
	if closer, ok := h.rw.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}

	errarr = append(errarr, h.BaseEndpoint.Close())
	h.readyWG.Wait()

	return errors.Join(errarr...)
}

// Ready reports whether the handshake has completed. Once true it stays true.
func (h *Handshaker) Ready() bool {
	h.readyMu.Lock()
	defer h.readyMu.Unlock()
	return h.ready
}

// OnReady runs f once, on its own goroutine, after the handshake completes (at once if it
// already has). Running it inline in the message handler could deadlock: f may publish,
// which writes to the peer while this side is not reading. After Close has begun, f is
// dropped and never runs.
func (h *Handshaker) OnReady(f func()) {
	h.readyMu.Lock()
	defer h.readyMu.Unlock()
	switch {
	case h.closing:
	case !h.ready:
		h.onReady = append(h.onReady, f)
	default:
		h.startReadyLocked(f)
	}
}

// startReadyLocked starts f on its own goroutine. readyMu must be held, so the
// WaitGroup Add is ordered before Close sets closing and waits.
func (h *Handshaker) startReadyLocked(f func()) {
	h.readyWG.Add(1)
	go func() {
		defer h.readyWG.Done()
		f()
	}()
}

// markReady flips ready once and starts the queued OnReady callbacks, unless Close has begun.
func (h *Handshaker) markReady() {
	h.readyMu.Lock()
	defer h.readyMu.Unlock()
	if h.ready || h.closing {
		return
	}
	h.ready = true
	callbacks := h.onReady
	h.onReady = nil
	for _, f := range callbacks {
		h.startReadyLocked(f)
	}
}

func (h *Handshaker) Name() string {
	return h.remotePeer.String()
}

func (h *Handshaker) SetName(name string) {
	if tr := h.remote.Load(); tr != nil {
		tr.SetName(name)
	}
}

// Publish will advertise an intent to publish named and typed data.
func (h *Handshaker) Publish(route dndm.Route, opt ...dndm.PubOpt) (dndm.Intent, error) {
	tr := h.remote.Load()
	if !h.Ready() || tr == nil {
		return nil, errors.ErrForbidden
	}

	return tr.Publish(route, opt...)
}

// Subscribe will advertise an interest in named and typed data.
func (h *Handshaker) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	tr := h.remote.Load()
	if !h.Ready() || tr == nil {
		return nil, errors.ErrForbidden
	}

	return tr.Subscribe(route, opt...)
}

// Init is used by the Router to initialize this transport.
func (h *Handshaker) Init(ctx context.Context, logger *slog.Logger, addIntent dndm.IntentCallback, addInterest dndm.InterestCallback) error {
	if h.remote.Load() != nil {
		panic("h.endpoint != nil")
	}

	if err := h.BaseEndpoint.Init(ctx, logger, addIntent, addInterest); err != nil {
		return err
	}

	h.conn = stream.NewWithContext(h.Ctx(),
		h.addrbook.Self(), h.remotePeer,
		h.rw, map[types.Type]network.MessageHandler{
			types.Type_HANDSHAKE: h.handshakeMsg,
			types.Type_ADDRBOOK:  h.addrbookMsg,
			types.Type_PEERS:     h.peersMsg,
			types.Type_RESULT:    h.resultMsg,
		})

	tr := remote.New(h.remotePeer, h.conn, h.Size, h.timeout, h.pingDuration)

	// Decide the state before tr.Init starts the message handler, which reads it.
	dialing := h.state == HS_INIT
	if dialing {
		h.state = HS_WAIT
	}
	h.dialer = dialing

	err := tr.Init(h.Ctx(), logger, addIntent, addInterest)
	if err != nil {
		return err
	}
	h.remote.Store(tr)

	tr.OnClose(func() {
		h.Log.Info("Handshaker Remote.OnClose", "name", tr.Name())
		h.Close()
	})

	if dialing {
		// Record the dial for the address book (IsActive) without deciding anything: the
		// peer's declared identity is not known yet. Simultaneous connections are settled
		// when the handshake completes (close-reconnect.md §3.1, review R-DND-R8 N2).
		h.addrbook.NoteDial(h.remotePeer, h.conn)
		h.Log.Info("Sending Handshake", "state", HS_WAIT, "local", h.addrbook.Self(), "peer", h.remotePeer)
		h.conn.Write(h.Ctx(), dndm.EmptyRoute(), &p2ptypes.Handshake{
			Me:        h.addrbook.Self().String(),
			You:       h.remotePeer.String(),
			Stage:     p2ptypes.HandshakeStage_INITIAL,
			Intents:   nil, // FIXME
			Interests: nil,
		})
	}

	go func() {
		<-h.Ctx().Done()
		h.Close()
	}()
	return nil
}

// FIXME: This is a very rudimentary handshake. Probably a proper handshake middleware should be implemented
func (h *Handshaker) handshakeMsg(hdr *types.Header, msg proto.Message, remote network.Conn) (pass bool, err error) {
	h.hsCount++
	if h.hsCount > 5 {
		h.Close()
		h.Log.Error("Handshake", "state", h.state, "peer", h.remotePeer, "count", h.hsCount)
		return false, errors.ErrForbidden
	}

	hs, ok := msg.(*p2ptypes.Handshake)
	if !ok {
		h.Log.Error("Handshake", "state", h.state, "peer", h.remotePeer, "type", reflect.TypeOf(msg))
		return true, errors.ErrBadArgument
	}

	h.Log.Info("Got Handshake", "state", h.state, "peer", h.remotePeer, "them", hs.Me, "us", hs.You, "stage", hs.Stage)

	switch h.state {
	case HS_WAIT:
		h.hsCount = 0
		h.state = HS_DONE
		peer, err := dndm.PeerFromString(hs.Me)
		if err != nil {
			return false, err
		}
		err = h.conn.UpdateRemotePeer(peer)
		if err != nil {
			return false, err
		}
		h.Log.Info("Handshaker.UpdatePeer", "them", hs.Me, "us", hs.You, "prevPeer", h.remotePeer)
		h.remotePeer = peer
		// Both roles pass here: the acceptor on INITIAL, the dialer on FINAL. Now both
		// declared identities are known, so the tie-break gives the same answer on both
		// sides. A losing connection closes before it becomes ready (or, on the dialer,
		// before it replies), and the peer's side of it then closes too.
		if err := h.addrbook.AddConn(h.remotePeer, h.dialer, h.conn); err != nil {
			h.Log.Info("Handshaker: dropping duplicate connection", "peer", h.remotePeer, "err", err)
			h.Close()
			return false, err
		}

		err = h.conn.Write(h.Ctx(), dndm.EmptyRoute(), &p2ptypes.Handshake{
			Me:        h.addrbook.Self().String(),
			You:       peer.String(),
			Stage:     p2ptypes.HandshakeStage_FINAL,
			Intents:   nil, // FIXME
			Interests: nil,
		})
		if err != nil {
			return false, err
		}
		h.markReady()

		if !h.isSeed {
			return true, nil
		}

		activePeers := h.addrbook.ActivePeers(peer)
		if activePeers == nil {
			return true, nil
		}

		err = h.conn.Write(h.Ctx(), dndm.EmptyRoute(), &p2ptypes.Peers{
			Remove: false,
			Ids:    activePeers,
		})
		if err != nil {
			return false, err
		}
		h.addrbook.SetSharedPeers(peer, activePeers)

		return true, nil
	case HS_DONE:
		h.Log.Info("Handshaker DONE", "remote", h.remotePeer)
		h.hsCount = 0
		return true, nil
	}

	return false, nil
}

func (h *Handshaker) peersMsg(hdr *types.Header, msg proto.Message, remote network.Conn) (pass bool, err error) {
	if h.state != HS_DONE {
		h.Close()
		return false, errors.ErrForbidden
	}

	peers, ok := msg.(*p2ptypes.Peers)
	if !ok {
		h.Log.Error("peers", "state", h.state, "peer", h.remotePeer, "type", reflect.TypeOf(msg))
		return true, errors.ErrBadArgument
	}

	h.Log.Info("Got Peers", "state", h.state, "peer", h.remotePeer, "peers", peers)

	for _, p := range peers.Ids {
		peer, err := dndm.PeerFromString(p)
		if err != nil {
			h.Log.Error("Parsing peer", "err", err, "peer", p)
			return false, err
		}
		h.addrbook.AddConn(peer, true, nil)
	}

	return false, nil
}

func (h *Handshaker) addrbookMsg(hdr *types.Header, msg proto.Message, remote network.Conn) (pass bool, err error) {
	if h.state != HS_DONE {
		h.Close()
		return false, errors.ErrForbidden
	}

	book, ok := msg.(*p2ptypes.Addrbook)
	if !ok {
		h.Log.Error("addrbook", "state", h.state, "peer", h.remotePeer, "type", reflect.TypeOf(msg))
		return true, errors.ErrBadArgument
	}

	h.Log.Info("Got Peers", "state", h.state, "peer", h.remotePeer, "peers", book)

	return false, nil
}

func (h *Handshaker) messageMsg(hdr *types.Header, msg proto.Message, remote network.Conn) (pass bool, err error) {
	if h.state != HS_DONE {
		h.Close()
		return false, errors.ErrForbidden
	}
	return true, nil
}

func (h *Handshaker) resultMsg(hdr *types.Header, msg proto.Message, remote network.Conn) (pass bool, err error) {
	if h.state != HS_DONE {
		h.Close()
		return false, errors.ErrForbidden
	}

	return true, nil
}
