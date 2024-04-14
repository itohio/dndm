package mesh

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	"github.com/itohio/dndm/routers/remote"
	"github.com/itohio/dndm/stream"
	types "github.com/itohio/dndm/types/core"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"google.golang.org/protobuf/proto"
)

var _ routers.Transport = (*Handshaker)(nil)

type HandshakeState int

const (
	HS_INIT HandshakeState = iota
	HS_WAIT
	HS_PEERS
	HS_DONE
)

type Container interface {
	Add(routers.Transport) error
	Remove(routers.Transport) error
}

type Handshaker struct {
	ctx          context.Context
	log          *slog.Logger
	cancel       context.CancelFunc
	state        HandshakeState
	hsCount      int
	rw           io.ReadWriter
	size         int
	timeout      time.Duration
	pingDuration time.Duration
	remote       dialers.Remote
	transport    atomic.Pointer[remote.Transport]
	container    Container
	peer         dialers.Peer
}

func NewHandshaker(size int, timeout, pingDuration time.Duration, c Container, rw io.ReadWriter, p dialers.Peer, state HandshakeState) *Handshaker {
	ret := &Handshaker{
		state:        state,
		rw:           rw,
		size:         size,
		timeout:      timeout,
		pingDuration: pingDuration,
		container:    c,
		peer:         p,
	}

	return ret
}

func (h *Handshaker) Close() error {
	errarr := []error{h.remote.Close()}
	if closer, ok := h.rw.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}

	if tr := h.transport.Swap(nil); tr != nil {
		h.container.Remove(tr)
	}
	h.remote = nil
	h.rw = nil

	return errors.Join(errarr...)
}

func (h *Handshaker) Name() string {
	return h.peer.String()
}

func (h *Handshaker) SetName(name string) {
	if tr := h.transport.Load(); tr != nil {
		tr.SetName(name)
	}
}

// Publish will advertise an intent to publish named and typed data.
func (h *Handshaker) Publish(route routers.Route, opt ...routers.PubOpt) (routers.Intent, error) {
	tr := h.transport.Load()
	if h.state != HS_DONE || tr == nil {
		return nil, errors.ErrForbidden
	}

	return tr.Publish(route, opt...)
}

// Subscribe will advertise an interest in named and typed data.
func (h *Handshaker) Subscribe(route routers.Route, opt ...routers.SubOpt) (routers.Interest, error) {
	tr := h.transport.Load()
	if h.state != HS_DONE || tr == nil {
		return nil, errors.ErrForbidden
	}

	return tr.Subscribe(route, opt...)
}

// Init is used by the Router to initialize this transport.
func (h *Handshaker) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if h.transport.Load() != nil {
		panic("h.transport != nil")
	}

	h.remote = stream.NewWithContext(ctx, h.peer, h.rw, map[types.Type]dialers.MessageHandler{
		types.Type_HANDSHAKE: h.handshake,
		types.Type_ADDRBOOK:  h.addrbook,
		types.Type_PEERS:     h.peers,
		types.Type_RESULT:    h.result,
	})

	tr := remote.New(h.peer.String(), h.remote, h.size, h.timeout, h.pingDuration)

	err := tr.Init(ctx, logger, add, remove)
	if err != nil {
		return err
	}
	h.transport.Store(tr)

	if h.state == HS_INIT {
		h.state = HS_WAIT
		h.log.Info("Sending Handshake", "state", h.state, "peer", h.peer)
		h.remote.Write(ctx, routers.Route{}, &p2ptypes.Handshake{
			Id:    h.peer.String(),
			Stage: p2ptypes.HandshakeStage_INITIAL,
		})
	}

	go func() {
		<-ctx.Done()
		h.log.Info("Handshake.Close", "state", h.state, "peer", h.peer)
		h.Close()
	}()
	return nil
}

func (h *Handshaker) handshake(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {
	h.hsCount++
	if h.hsCount > 5 {
		h.cancel()
		h.log.Error("Handshake", "state", h.state, "peer", h.peer, "count", h.hsCount)
		return false, errors.ErrForbidden
	}

	hs, ok := msg.(*p2ptypes.Handshake)
	if !ok {
		h.log.Error("Handshake", "state", h.state, "peer", h.peer, "type", reflect.TypeOf(msg))
		return true, errors.ErrBadArgument
	}

	h.log.Info("Got Handshake", "state", h.state, "peer", h.peer, "id", hs.Id, "stage", hs.Stage)

	switch h.state {
	case HS_WAIT:
		h.hsCount = 0
		h.state = HS_DONE
		peer, err := dialers.PeerFromString(hs.Id)
		if err != nil {
			return false, err
		}
		h.peer = peer
		h.SetName(peer.String())
		return false, nil
	case HS_DONE:
		h.hsCount = 0
		return false, nil
	}

	return false, nil
}

func (h *Handshaker) peers(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {
	if h.state != HS_DONE {
		return false, errors.ErrForbidden
	}

	peers, ok := msg.(*p2ptypes.Peers)
	if !ok {
		h.log.Error("peers", "state", h.state, "peer", h.peer, "type", reflect.TypeOf(msg))
		return true, errors.ErrBadArgument
	}

	h.log.Info("Got Peers", "state", h.state, "peer", h.peer, "peers", peers)

	return false, nil
}

func (h *Handshaker) addrbook(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {
	if h.state != HS_DONE {
		return false, errors.ErrForbidden
	}

	book, ok := msg.(*p2ptypes.Addrbook)
	if !ok {
		h.log.Error("addrbook", "state", h.state, "peer", h.peer, "type", reflect.TypeOf(msg))
		return true, errors.ErrBadArgument
	}

	h.log.Info("Got Peers", "state", h.state, "peer", h.peer, "peers", book)

	return false, nil
}

func (h *Handshaker) message(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {
	if h.state != HS_DONE {
		return false, errors.ErrForbidden
	}
	return true, nil
}

func (h *Handshaker) result(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {
	if h.state != HS_DONE {
		return false, errors.ErrForbidden
	}

	return true, nil
}
