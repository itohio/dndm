package mesh

import (
	"context"
	"io"
	"log/slog"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	"github.com/itohio/dndm/stream"
	types "github.com/itohio/dndm/types/core"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"google.golang.org/protobuf/proto"
)

var _ dialers.Remote = (*Handshaker)(nil)

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
	ctx       context.Context
	log       *slog.Logger
	cancel    context.CancelFunc
	state     HandshakeState
	rw        io.ReadWriter
	remote    dialers.Remote
	transport routers.Transport
	container Container
}

func NewHandshaker(ctx context.Context, log *slog.Logger, c Container, rw io.ReadWriter, p dialers.Peer, state HandshakeState) *Handshaker {
	ctx, cancel := context.WithCancel(ctx)
	ret := &Handshaker{
		ctx:       ctx,
		cancel:    cancel,
		log:       log,
		state:     state,
		rw:        rw,
		container: c,
	}

	ret.remote = stream.NewWithContext(ctx, p, rw, map[types.Type]dialers.MessageHandler{
		types.Type_HANDSHAKE: ret.handshake,
		types.Type_ADDRBOOK:  ret.addrbook,
		types.Type_PEERS:     ret.peers,
		types.Type_RESULT:    ret.result,
	})

	if state == HS_INIT {
		ret.remote.Write(ctx, routers.Route{}, &p2ptypes.Handshake{
			Id:    p.ID(),
			Stage: p2ptypes.HandshakeStage_INITIAL,
		})
	}

	go func() {
		<-ctx.Done()
		ret.Close()
	}()

	return ret
}

func (h *Handshaker) Close() error {
	errarr := []error{h.remote.Close()}
	if closer, ok := h.rw.(io.Closer); ok {
		errarr = append(errarr, closer.Close())
	}

	if h.transport != nil {
		h.container.Remove(h.transport)
		h.transport = nil
	}
	h.remote = nil
	h.rw = nil

	return errors.Join(errarr...)
}

func (h *Handshaker) Peer() dialers.Peer { return h.remote.Peer() }

func (h *Handshaker) Read(ctx context.Context) (*types.Header, proto.Message, error) {
	return h.remote.Read(ctx)
}

func (h *Handshaker) Write(ctx context.Context, route routers.Route, msg proto.Message) error {
	return h.remote.Write(ctx, route, msg)
}

func (h *Handshaker) AddRoute(r ...routers.Route) { h.remote.AddRoute(r...) }
func (h *Handshaker) DelRoute(r ...routers.Route) { h.remote.DelRoute(r...) }

func (h *Handshaker) handshake(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {

}

func (h *Handshaker) peers(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {

}

func (h *Handshaker) addrbook(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {

}

func (h *Handshaker) result(hdr *types.Header, msg proto.Message, remote dialers.Remote) (pass bool, err error) {

}
