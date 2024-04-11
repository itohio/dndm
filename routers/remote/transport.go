package remote

import (
	"container/ring"
	"context"
	"io"
	"log/slog"
	sync "sync"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

var _ routers.Transport = (*Transport)(nil)

type Remote interface {
	io.Closer
	Id() string
	Read(ctx context.Context) (*types.Header, proto.Message, error)
	Write(ctx context.Context, route routers.Route, msg proto.Message) error
}

type Transport struct {
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	log            *slog.Logger
	name           string
	remote         Remote
	addCallback    func(interest routers.Interest, t routers.Transport) error
	removeCallback func(interest routers.Interest, t routers.Transport) error
	size           int

	mu      sync.Mutex
	timeout time.Duration
	linker  *routers.Linker

	pingDuration time.Duration
	pingMu       sync.Mutex
	pingRing     *ring.Ring
	pongRing     *ring.Ring

	nonce atomic.Uint64
}

// New creates a transport that communicates with a remote via Remote interface.
func New(name string, remote Remote, size int, timeout, pingDuration time.Duration) *Transport {
	return &Transport{
		name:         name,
		size:         size,
		remote:       remote,
		pingDuration: pingDuration,
		timeout:      timeout,
		pingRing:     ring.New(3),
		pongRing:     ring.New(3),
	}
}

func (t *Transport) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest routers.Interest, t routers.Transport) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.log = logger
	t.addCallback = add
	t.removeCallback = remove
	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel

	t.linker = routers.NewLinker(
		ctx, t.size,
		func(interest routers.Interest) error {
			return add(interest, t)
		},
		func(interest routers.Interest) error {
			return remove(interest, t)
		},
		nil,
	)

	t.wg.Add(1)
	go t.messageHandler()
	if t.pingDuration >= time.Millisecond*100 {
		t.wg.Add(1)
		go t.messageSender(t.pingDuration)
	}
	return nil
}

func (t *Transport) Close() error {
	t.cancel()
	t.wg.Wait()
	return t.linker.Close()
}

func (t *Transport) Name() string {
	return t.name
}

func (t *Transport) Publish(route routers.Route) (routers.Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	intent, err := t.linker.AddIntent(route)
	if err != nil {
		return nil, err
	}

	t.log.Info("intent registered", "route", route.Route())
	return intent, nil
}

func (t *Transport) publish(route routers.Route, m *types.Intent) (routers.Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	intent, err := t.linker.AddIntentWithWrapper(route, wrapIntent(m))
	if err != nil {
		return nil, err
	}

	t.log.Info("remote intent registered", "route", route.Route())
	return intent, nil
}

func (t *Transport) Subscribe(route routers.Route) (routers.Interest, error) {
	interest, err := t.linker.AddInterest(route)
	if err != nil {
		return nil, err
	}
	t.addCallback(interest, t)
	t.log.Info("interest registered", "route", route.Route())
	return interest, nil
}

func (t *Transport) subscribe(route routers.Route, m *types.Interest) (routers.Interest, error) {
	interest, err := t.linker.AddInterestWithWrapper(route, wrapInterest(m))
	if err != nil {
		return nil, err
	}
	t.addCallback(interest, t)
	t.log.Info("remote interest registered", "route", route.Route())
	return interest, nil
}
