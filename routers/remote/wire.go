package remote

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/errors"
	routers "github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

var _ Remote = (*wireRemote)(nil)
var _ routers.Transport = (*Wire)(nil)

type ReadWriter interface {
	io.Reader
	io.Writer
}

type MessageHandler func(hdr *types.Header, msg proto.Message, remote Remote) (pass bool, err error)

// Wire is a Transport that communicates with another remote Transport connected by bufio.ReaderWriter.
type Wire struct {
	*Transport
	remote *wireRemote
}

// NewWire will create a Wire transport wrapper around pipe.
//
// handlers is a map of message handlers identified by message type.
// Handlers are provided with a pointer to Remote that may reuse Read and Write thus creating communication loops (e.g. handshake).
// However, handlers must be careful not to introduce infinite loops by e.g. capturing Pong message and sending Ping back.
//
// Handlers are invoked inside Read message and if it returns true the message will be passed to the original Read caller, otherwise it will attempt to read more messages recursively.
func NewWire(id, name string, size int, timeout, pingDuration time.Duration, rw ReadWriter, handlers map[types.Type]MessageHandler) *Wire {
	wireRemote := &wireRemote{
		id:       id,
		rw:       rw,
		handlers: handlers,
		routes:   make(map[string]routers.Route),
		read: contextRW{
			request: make(chan []byte),
			result:  make(chan contextRWResult),
		},
		write: contextRW{
			request: make(chan []byte),
			result:  make(chan contextRWResult),
		},
	}

	transport := New(name, wireRemote, size, timeout, pingDuration)
	return &Wire{
		Transport: transport,
		remote:    wireRemote,
	}
}

func (w *Wire) Init(ctx context.Context, log *slog.Logger, add, remove func(routers.Interest, routers.Transport) error) error {
	if add == nil || remove == nil {
		return errors.ErrBadArgument
	}
	addW := func(i routers.Interest, t routers.Transport) error {
		err := add(i, t)
		if err != nil {
			return err
		}
		// Nil Type indicates remote interest
		if i.Route().Type() != nil {
			w.remote.routes[i.Route().ID()] = i.Route()
		}
		return nil
	}
	remW := func(i routers.Interest, t routers.Transport) error {
		err := remove(i, t)
		if err != nil {
			return err
		}
		// Nil Type indicates remote interest
		if i.Route().Type() != nil {
			delete(w.remote.routes, i.Route().ID())
		}
		return nil
	}

	w.remote.run(ctx)
	return w.Transport.Init(ctx, log, addW, remW)
}

// wireRemote is a wrapper over ReadWriter that will read and decode messages from Reader as well as encode and write them to the Writer. It allows
// using regular Reader/Writer interfaces with a context, however, it must be noted that the read/write loop will be leaked if Read/Write blocks.
//
// - Read will read a message and decode it. Then call any registered handler for that message. Then it returns the message for further processing.
// - Read will also modify received header ReceivedAt with the timestamp of local reception of the packet before decoding it
// - Write will encode the message
// - Write will set header Timestamp to the time when the header is constructed, so it will include the overhead of marshaling the message
type wireRemote struct {
	id       string
	handlers map[types.Type]MessageHandler
	routes   map[string]routers.Route
	rw       ReadWriter
	cancel   context.CancelFunc
	read     contextRW
	write    contextRW
}

func (c *wireRemote) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go c.read.Run(ctx, func(b []byte) (int, error) {
		return c.rw.Read(b)
	})
	go c.write.Run(ctx, func(b []byte) (int, error) {
		return c.rw.Write(b)
	})
}

func (c *wireRemote) Reader(ctx context.Context) io.Reader {
	return &reader{
		c:   &c.read,
		ctx: ctx,
	}
}

func (w *wireRemote) Id() string {
	return w.id
}

func (w *wireRemote) Close() error {
	w.cancel()
	close(w.read.request)
	close(w.write.request)
	close(w.read.result)
	close(w.write.result)
	return nil
}

func (w *wireRemote) Read(ctx context.Context) (*types.Header, proto.Message, error) {
	buf, ts, err := codec.ReadMessage(w.Reader(ctx))
	if err != nil {
		return nil, nil, err
	}
	hdr, msg, err := codec.DecodeMessage(buf, w.routes)
	if err != nil {
		codec.Release(buf)
		return hdr, msg, err
	}
	codec.Release(buf)
	hdr.ReceiveTimestamp = ts

	if w.handlers == nil {
		return hdr, msg, err
	}

	if handler, found := w.handlers[hdr.Type]; found {
		pass, err := handler(hdr, msg, w)
		if err != nil {
			return hdr, msg, err
		}
		if !pass {
			return w.Read(ctx)
		}
	}

	return hdr, msg, err
}

func (w *wireRemote) Write(ctx context.Context, route routers.Route, msg proto.Message) error {
	buf, err := codec.EncodeMessage(msg, route)
	if err != nil {
		return err
	}
	n, err := w.write.Request(ctx, buf)
	if n != len(buf) {
		err = errors.ErrNotEnoughBytes
	}
	codec.Release(buf)
	return err
}

type contextRWResult struct {
	n   int
	err error
}

type contextRW struct {
	request chan []byte
	result  chan contextRWResult
}

func (c *contextRW) Run(ctx context.Context, f func([]byte) (int, error)) {
	for r := range c.request {
		n, err := f(r)
		select {
		case <-ctx.Done():
			return
		case c.result <- contextRWResult{
			n:   n,
			err: err,
		}:
		}
	}
}

func (c *contextRW) Request(ctx context.Context, buf []byte) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case c.request <- buf:
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case result := <-c.result:
		return result.n, result.err
	}
}

type reader struct {
	c   *contextRW
	ctx context.Context
}

func (r reader) Read(buf []byte) (int, error) {
	return r.c.Request(r.ctx, buf)
}
