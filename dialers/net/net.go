package net

import (
	"context"
	"io"
	"net"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
)

var _ dialers.Node = (*Node)(nil)

type Node struct {
	peer dialers.Peer
}

func New(peer dialers.Peer) (*Node, error) {
	return &Node{
		peer: peer,
	}, nil
}

func (f *Node) Scheme() string {
	return f.peer.Scheme()
}

func (f *Node) Dial(ctx context.Context, peer dialers.Peer) (io.ReadWriteCloser, error) {
	if f.peer.Scheme() != peer.Scheme() {
		return nil, errors.ErrBadArgument
	}

	conn, err := net.Dial(peer.Scheme(), peer.Address())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (f *Node) Serve(ctx context.Context, onConnect func(r io.ReadWriteCloser) error) error {
	listener, err := net.Listen(f.peer.Scheme(), f.peer.Address())
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
			default:
			}

			listener.Accept()
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				listener.Close()
			}
		}
	}()

	return nil
}
