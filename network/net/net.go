package net

import (
	"context"
	"io"
	"log/slog"
	"net"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
)

var _ network.Node = (*Node)(nil)

type Node struct {
	log  *slog.Logger
	peer dndm.Peer
}

func New(log *slog.Logger, peer dndm.Peer) (*Node, error) {
	return &Node{
		log:  log,
		peer: peer,
	}, nil
}

func (f *Node) Scheme() string {
	return f.peer.Scheme()
}

func (f *Node) Dial(ctx context.Context, peer dndm.Peer, o ...network.DialOpt) (io.ReadWriteCloser, error) {
	if f.peer.Scheme() != peer.Scheme() {
		return nil, errors.ErrBadArgument
	}

	f.log.Debug("Dialing", "peer", peer)
	conn, err := net.Dial(peer.Scheme(), peer.Address())
	if err != nil {
		f.log.Error("Dial", "peer", peer, "err", err)
		return nil, err
	}

	return conn, nil
}

func (f *Node) Serve(ctx context.Context, onConnect func(peer dndm.Peer, r io.ReadWriteCloser) error, o ...network.SrvOpt) error {
	listener, err := net.Listen(f.peer.Scheme(), f.peer.Address())
	if err != nil {
		return err
	}

	go func() {
		f.log.Info("Listen", "scheme", f.peer.Scheme(), "addr", f.peer.Address(), "path", f.peer.Path(), "peer", f.peer)
		defer listener.Close()
		for {
			select {
			case <-ctx.Done():
			default:
			}

			conn, err := listener.Accept()
			if err != nil {
				f.log.Error("Listen.Accept", "peer", f.peer, "err", err)
				return
			}
			peer, err := dndm.NewPeer(f.Scheme(), conn.RemoteAddr().String(), "", nil)
			if err != nil {
				f.log.Error("Listen.Accept NewPeer", "peer", f.peer, "err", err)
				return
			}
			err = onConnect(peer, conn)
			if err != nil {
				f.log.Error("Listen.onConnect", "peer", f.peer, "err", err)
				return
			}
		}
	}()

	<-ctx.Done()
	f.log.Info("Listen.Close", "peer", f.peer, "err", ctx.Err())
	listener.Close()
	return nil
}
