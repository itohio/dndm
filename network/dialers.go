package network

import (
	"context"
	"io"

	"github.com/itohio/dndm/router"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

type MessageHandler func(hdr *types.Header, msg proto.Message, remote Remote) (pass bool, err error)

type Remote interface {
	io.Closer
	Peer() Peer
	Read(ctx context.Context) (*types.Header, proto.Message, error)
	Write(ctx context.Context, route router.Route, msg proto.Message) error

	AddRoute(...router.Route)
	DelRoute(...router.Route)
}

type Dialer interface {
	Scheme() string
	Dial(ctx context.Context, peer Peer, o ...DialOpt) (io.ReadWriteCloser, error)
}

type Server interface {
	Serve(ctx context.Context, onConnect func(r io.ReadWriteCloser) error, o ...SrvOpt) error
}

type Node interface {
	Dialer
	Server
}
