package dndm

import (
	"context"
	"io"
	"log/slog"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

// Interest is an interface to describe an interest in named data.
// User should consume C of the interest until it is closed or no longer needed.
// Messages will be delivered only when a corresponding Intent is discovered.
type Interest interface {
	io.Closer
	Route() Route
	// C returns a channel that contains messages. Users should typecast to specific message type that
	// was registered with the interest.
	C() <-chan proto.Message
}

// InterestInternal is an interface to describe an internal interest data structure. Should not be used by users.
type InterestInternal interface {
	Interest
	Ctx() context.Context
	MsgC() chan<- proto.Message
}

// Intent is an interface to describe an intent to provide named data.
// Users can consume Interest channel to determine if it is worthwhile to send any data.
type Intent interface {
	io.Closer
	Route() Route
	// Interest returns a channel that contains Routes that are interested in the data indicated by the intent.
	// Users should start sending the data once an event is received on this channel.
	Interest() <-chan Route
	// Send will send a message to any recepient that indicated an interest.
	Send(context.Context, proto.Message) error
}
type IntentInternal interface {
	Intent
	Link(chan<- proto.Message)
	Notify()
	Ctx() context.Context
	// MsgC() <-chan proto.Message
}

// Endpoint is the interface that describes a End To End route.
// Endpoint registers Interests and Intents and links them together when they match.
type Endpoint interface {
	io.Closer
	Name() string
	// Publish will advertise an intent to publish named and typed data.
	Publish(route Route, opt ...PubOpt) (Intent, error)
	// Subscribe will advertise an interest in named and typed data.
	Subscribe(route Route, opt ...SubOpt) (Interest, error)
	// Init is used by the Router to initialize this endpoint.
	Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Endpoint) error) error
}

type Base struct {
	Ctx            context.Context
	cancel         context.CancelFunc
	name           string
	Log            *slog.Logger
	AddCallback    func(interest Interest, t Endpoint) error
	RemoveCallback func(interest Interest, t Endpoint) error
	Size           int
}

func NewBase(name string, size int) *Base {
	return &Base{
		name: name,
		Size: size,
	}
}

func (t *Base) Init(ctx context.Context, logger *slog.Logger, add, remove func(interest Interest, t Endpoint) error) error {
	if logger == nil || add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	t.Log = logger
	t.AddCallback = add
	t.RemoveCallback = remove
	ctx, cancel := context.WithCancel(ctx)
	t.Ctx = ctx
	t.cancel = cancel

	return nil
}

func (t *Base) Name() string {
	return t.name
}

func (t *Base) SetName(name string) {
	t.name = name
}

func (t *Base) Close() error {
	t.cancel()
	return nil
}
