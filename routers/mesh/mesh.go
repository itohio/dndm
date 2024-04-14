package mesh

import (
	"context"
	"log/slog"
	"sync"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/routers"
)

var _ routers.Transport = (*Mesh)(nil)

type Mesh struct {
	name           string
	log            *slog.Logger
	addCallback    func(interest routers.Interest, t routers.Transport) error
	removeCallback func(interest routers.Interest, t routers.Transport) error
	size           int

	mu    sync.Mutex
	peers []dialers.Peer
}

func New(name string, Node dialers.Node, peers []dialers.Peer) (*Mesh, error) {

}

func (t *Mesh) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
}

func (t *Mesh) Close() error {
}

func (t *Mesh) Name() string {
}

func (t *Mesh) Publish(route routers.Route) (routers.Intent, error) {
}

func (t *Mesh) Subscribe(route routers.Route) (routers.Interest, error) {
}
