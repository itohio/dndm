package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/mesh"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/examples"
	"github.com/itohio/dndm/network"
	"github.com/itohio/dndm/network/net"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	types "github.com/itohio/dndm/types/test"
)

var _ flag.Value = (*PeerArr)(nil)

type PeerArr []string

func (p *PeerArr) String() string { return strings.Join([]string(*p), ",") }
func (p *PeerArr) Set(s string) error {
	*p = append(*p, s)
	return nil
}

func (p PeerArr) Peers() []*p2ptypes.AddrbookEntry {
	return p2ptypes.AddrbookFromPeers(p)
}

func main() {
	var peers PeerArr
	self := flag.String("n", "tcp://localhost:1234/example", "self peer address")
	flag.Var(&peers, "P", "multiple peer multi-addresses to connect to")
	consume := flag.String("c", "", "Consume Foo from name")
	produce := flag.String("p", "", "Produce Foo to name")
	reproduce := flag.String("r", "", "Reproduce Boo to name")

	flag.Parse()

	selfPeer := errors.Must(dndm.PeerFromString(*self))

	d := errors.Must(network.New(
		errors.Must(net.New(slog.Default(), selfPeer)),
	))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	node := errors.Must(dndm.New(
		dndm.WithContext(ctx),
		dndm.WithEndpoint(errors.Must(mesh.New(
			selfPeer,
			3,
			runtime.NumCPU()*5,
			time.Second*10,
			time.Second*3,
			d,
			peers.Peers(),
		))),
	))

	go examples.Consume[*types.Foo](ctx, node, *consume)
	go examples.GenerateFoo(ctx, node, *produce)
	_ = reproduce

	<-ctx.Done()
	slog.Info("Stopped", "reason", ctx.Err())
}
