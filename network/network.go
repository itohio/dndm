package network

import (
	"context"
	"io"

	"github.com/itohio/dndm"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

type MessageHandler func(hdr *types.Header, msg proto.Message, remote Conn) (pass bool, err error)

// Conn interface represents a communication channel with the remote peer
type Conn interface {
	io.Closer
	OnClose(func()) Conn
	// Local returns the name of the local peer
	Local() dndm.Peer
	// Remote returns the name of the remote peer
	Remote() dndm.Peer
	// UpdateRemotePeer sets the remote peer name. Peer scheme and address must match to take effect.
	UpdateRemotePeer(dndm.Peer) error
	// Read reads a message sent by the peer
	Read(ctx context.Context) (*types.Header, proto.Message, error)
	// Write sends a message to the peer
	Write(ctx context.Context, route dndm.Route, msg proto.Message) error

	// AddRoute registers a route type. NOTE: Route must have a valid Type
	AddRoute(...dndm.Route)
	// DelRoute unregisters a route type.
	DelRoute(...dndm.Route)
}

// Dialer interface describes objects that can dial a remote peer.
type Dialer interface {
	// Scheme returns the scheme this dialer handles
	Scheme() string
	// Dial dials the remote peer and returns a ReadeWriteCloser object
	Dial(ctx context.Context, peer dndm.Peer, o ...DialOpt) (io.ReadWriteCloser, error)
}

// Server interface describes objects that can listen for connections.
type Server interface {
	Serve(ctx context.Context, onConnect func(peer dndm.Peer, r io.ReadWriteCloser) error, o ...SrvOpt) error
}

// Node interface describes both a dialer and a server.
type Node interface {
	Dialer
	Server
}
