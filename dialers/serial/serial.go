package serial

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/tarm/serial"

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

func defaultConfig() *serial.Config {
	return &serial.Config{Baud: 115200}
}

func (f *Node) Dial(ctx context.Context, peer dialers.Peer) (io.ReadWriteCloser, error) {
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

func (f *Node) Serve(ctx context.Context, onConnect func(r io.ReadWriteCloser) error) error {
	rwc, err := f.Dial(ctx, f.peer)
	if err != nil {
		return err
	}

	return onConnect(rwc)
}
