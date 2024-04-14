package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/dialers/net"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/examples"
	"github.com/itohio/dndm/routers/mesh"
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
	self := flag.String("n", "tcp://0.0.0.0:1234/example", "self peer address")
	flag.Var(&peers, "p", "multiple peer multi-addresses to connect to")
	consume := flag.String("c", "", "Consume Foo from name")
	produce := flag.String("p", "", "Produce Foo to name")
	reproduce := flag.String("r", "", "Reproduce Boo to name")

	flag.Parse()

	selfPeer := errors.Must(dialers.PeerFromString(*self))

	d := errors.Must(dialers.New(
		errors.Must(net.New(slog.Default(), selfPeer)),
	))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	node := errors.Must(dndm.New(
		dndm.WithContext(ctx),
		dndm.WithTransport(errors.Must(mesh.New(selfPeer.ID(), 3, d, peers.Peers()))),
	))

	go examples.Consume[*types.Foo](ctx, node, *consume)
	go examples.GenerateFoo(ctx, node, *produce)
	_ = reproduce

	<-ctx.Done()
	slog.Info("Stopped", "reason", ctx.Err())
}
