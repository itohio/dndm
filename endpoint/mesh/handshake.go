package mesh

import (
	"context"
	"io"
	"log/slog"
	"reflect"
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

var _ dndm.Endpoint = (*Handshaker)(nil)

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

type Handshaker struct {
	*dndm.Base

	state        HandshakeState
	hsCount      int
	rw           io.ReadWriter
	timeout      time.Duration
	pingDuration time.Duration
	conn         network.Conn
	remote       atomic.Pointer[remote.Endpoint]
	addrbook     *Addrbook
	remotePeer   network.Peer
}

func NewHandshaker(addrbook *Addrbook, remotePeer network.Peer, size int, timeout, pingDuration time.Duration, rw io.ReadWriter, state HandshakeState) *Handshaker {
	ret := &Handshaker{
		Base:         dndm.NewBase(remotePeer.String(), size),
		state:        state,
		rw:           rw,
		timeout:      timeout,
		pingDuration: pingDuration,
		addrbook:     addrbook,
		remotePeer:   remotePeer,
	}

	return ret
}

func (h *Handshaker) Close() error {
	h.Log.Info("Handshaker.Close")
	errarr := make([]error, 0, 3)

	if h.conn != nil {
		errarr = append(errarr, h.conn.Close())
		h.conn = nil
	}
	if h.rw != nil {
		if closer, ok := h.rw.(io.Closer); ok {
			errarr = append(errarr, closer.Close())
		}
		h.rw = nil
	}

	errarr = append(errarr, h.Base.Close())

	return errors.Join(errarr...)
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
	if h.state != HS_DONE || tr == nil {
		return nil, errors.ErrForbidden
	}

	return tr.Publish(route, opt...)
}

// Subscribe will advertise an interest in named and typed data.
func (h *Handshaker) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	tr := h.remote.Load()
	if h.state != HS_DONE || tr == nil {
		return nil, errors.ErrForbidden
	}

	return tr.Subscribe(route, opt...)
}

// Init is used by the Router to initialize this transport.
func (h *Handshaker) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest dndm.Interest, t dndm.Endpoint) error) error {
	if h.remote.Load() != nil {
		panic("h.endpoint != nil")
	}

	if err := h.Base.Init(ctx, logger, add, remove); err != nil {
		return err
	}

	h.conn = stream.NewWithContext(h.Ctx,
		h.addrbook.Self(), h.remotePeer,
		h.rw, map[types.Type]network.MessageHandler{
			types.Type_HANDSHAKE: h.handshakeMsg,
			types.Type_ADDRBOOK:  h.addrbookMsg,
			types.Type_PEERS:     h.peersMsg,
			types.Type_RESULT:    h.resultMsg,
		})

	tr := remote.New(h.remotePeer, h.conn, h.Size, h.timeout, h.pingDuration)

	err := tr.Init(h.Ctx, logger, add, remove)
	if err != nil {
		return err
	}
	h.remote.Store(tr)

	tr.OnClose(func() {
		h.Log.Info("Handshaker Remote.OnClose", "name", tr.Name())
		h.Close()
	})

	if h.state == HS_INIT {
		h.state = HS_WAIT
		h.Log.Info("Sending Handshake", "state", h.state, "local", h.addrbook.Self(), "peer", h.remotePeer)
		h.conn.Write(h.Ctx, dndm.Route{}, &p2ptypes.Handshake{
			Me:    h.addrbook.Self().String(),
			You:   h.remotePeer.String(),
			Stage: p2ptypes.HandshakeStage_INITIAL,
		})
	}

	go func() {
		<-h.Ctx.Done()
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
		peer, err := network.PeerFromString(hs.Me)
		if err != nil {
			return false, err
		}
		err = h.conn.UpdateRemotePeer(peer)
		if err != nil {
			return false, err
		}
		h.Log.Info("Handshaker.UpdatePeer", "them", hs.Me, "us", hs.You, "prevPeer", h.remotePeer)
		h.remotePeer = peer

		return false, nil
	case HS_DONE:
		h.Log.Info("Handshaker DONE", "remote", h.remotePeer)
		h.hsCount = 0
		return false, nil
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
