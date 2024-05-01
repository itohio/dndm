package serial

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/tarm/serial"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
)

var _ network.Node = (*Node)(nil)

// var _ dndm.CloseNotifier = (*Node)(nil)

type Node struct {
	peer dndm.Peer
}

func New(peer dndm.Peer) (*Node, error) {
	return &Node{
		peer: peer,
	}, nil
}

func (f *Node) Scheme() string {
	return f.peer.Scheme()
}

func defaultConfig() *serial.Config {
	return &serial.Config{Baud: 115200}
}

func (f *Node) Dial(ctx context.Context, peer dndm.Peer, o ...network.DialOpt) (io.ReadWriteCloser, error) {
	if f.peer.Scheme() != peer.Scheme() {
		return nil, errors.ErrBadArgument
	}

	cfg := defaultConfig()

	cfg.Name = strings.ReplaceAll(peer.Address(), ".", "/")
	if baudS := peer.Values().Get("baud"); baudS != "" {
		baud, err := strconv.ParseInt(baudS, 10, 32)
		if err != nil {
			return nil, err
		}
		cfg.Baud = int(baud)
	}
	if parity := peer.Values().Get("parity"); parity != "" {
		cfg.Parity = serial.Parity(parity[0])
	}
	if stop := peer.Values().Get("stop"); stop != "" {
		switch stop {
		case "1":
			cfg.StopBits = serial.Stop1
		case "2":
			cfg.StopBits = serial.Stop2
		case "1.5":
			cfg.StopBits = serial.Stop1Half
		default:
			return nil, errors.ErrStopBits
		}
	}
	if bitsS := peer.Values().Get("bits"); bitsS != "" {
		bits, err := strconv.ParseInt(bitsS, 10, 8)
		if err != nil {
			return nil, err
		}
		cfg.Size = byte(bits)
	}

	port, err := serial.OpenPort(cfg)

	return port, err
}

func (f *Node) Serve(ctx context.Context, onConnect func(peer dndm.Peer, r io.ReadWriteCloser) error, o ...network.SrvOpt) error {
	rwc, err := f.Dial(ctx, f.peer)
	if err != nil {
		return err
	}

	return onConnect(f.peer, rwc)
}
