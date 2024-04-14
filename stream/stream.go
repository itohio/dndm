package stream

import (
	"context"
	"io"
	"sync"

	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
	routers "github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

var _ dialers.Remote = (*StreamContext)(nil)
var _ dialers.Remote = (*Stream)(nil)

// StreamContext is a wrapper over ReadWriter that will read and decode messages from Reader as well as encode and write them to the Writer. It allows
// using regular Reader/Writer interfaces with a context, however, it must be noted that the read/write loop will be leaked if Read/Write blocks.
//
// - Read will read a message and decode it. Then call any registered handler for that message. Then it returns the message for further processing.
// - Read will also modify received header ReceivedAt with the timestamp of local reception of the packet before decoding it
// - Write will encode the message
// - Write will set header Timestamp to the time when the header is constructed, so it will include the overhead of marshaling the message
type StreamContext struct {
	*Stream
	cancel context.CancelFunc
	read   contextRW
	write  contextRW
}

// NewWithContext will use ReadWriter and allow for context cancellation in Read and Write methods.
func NewWithContext(ctx context.Context, peer dialers.Peer, rw io.ReadWriter, handlers map[types.Type]dialers.MessageHandler) *StreamContext {
	ret := &StreamContext{
		Stream: New(peer, rw, handlers),
		read: contextRW{
			request: make(chan []byte),
			result:  make(chan contextRWResult),
		},
		write: contextRW{
			request: make(chan []byte),
			result:  make(chan contextRWResult),
		},
	}
	ret.run(ctx)
	return ret
}

func (c *StreamContext) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go c.read.Run(ctx, c.rw.Read)
	go c.write.Run(ctx, c.rw.Write)
}

func (c *StreamContext) Reader(ctx context.Context) io.Reader {
	return &reader{
		c:   &c.read,
		ctx: ctx,
	}
}

func (w *StreamContext) Close() error {
	w.cancel()
	close(w.read.request)
	close(w.write.request)
	close(w.read.result)
	close(w.write.result)
	return w.Stream.Close()
}

func (w *StreamContext) Read(ctx context.Context) (*types.Header, proto.Message, error) {
	buf, ts, err := codec.ReadMessage(w.Reader(ctx))
	if err != nil {
		return nil, nil, err
	}

	w.mu.Lock()
	hdr, msg, err := codec.DecodeMessage(buf, w.routes)
	w.mu.Unlock()

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

func (w *StreamContext) Write(ctx context.Context, route routers.Route, msg proto.Message) error {
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

// Stream converts regular ReaderWriter into a Remote
type Stream struct {
	peer     dialers.Peer
	handlers map[types.Type]dialers.MessageHandler
	rw       io.ReadWriter

	mu     sync.Mutex
	routes map[string]routers.Route
}

type readWriter struct {
	r io.Reader
	w io.Writer
}

func (rw readWriter) Read(buf []byte) (int, error)  { return rw.r.Read(buf) }
func (rw readWriter) Write(buf []byte) (int, error) { return rw.w.Write(buf) }

// NewIO creates a Remote using io.Reader and io.Writer.
func NewIO(peer dialers.Peer, r io.Reader, w io.Writer, handlers map[types.Type]dialers.MessageHandler) *Stream {
	rw := readWriter{r: r, w: w}
	return New(peer, rw, handlers)
}

// New creates a Remote using provided ReaderWriter.
func New(peer dialers.Peer, rw io.ReadWriter, handlers map[types.Type]dialers.MessageHandler) *Stream {
	return &Stream{
		peer:     peer,
		rw:       rw,
		handlers: handlers,
	}
}

func (w *Stream) Peer() dialers.Peer {
	return w.peer
}

func (w *Stream) Close() error {
	if closer, ok := w.rw.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (w *Stream) AddRoute(routes ...routers.Route) {
	w.mu.Lock()
	for _, r := range routes {
		w.routes[r.ID()] = r
	}
	w.mu.Unlock()
}

func (w *Stream) DelRoute(routes ...routers.Route) {
	w.mu.Lock()
	for _, r := range routes {
		delete(w.routes, r.ID())
	}
	w.mu.Unlock()
}

func (w *Stream) Read(ctx context.Context) (*types.Header, proto.Message, error) {
	buf, ts, err := codec.ReadMessage(w.rw)
	if err != nil {
		return nil, nil, err
	}

	w.mu.Lock()
	hdr, msg, err := codec.DecodeMessage(buf, w.routes)
	w.mu.Unlock()

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

func (w *Stream) Write(ctx context.Context, route routers.Route, msg proto.Message) error {
	buf, err := codec.EncodeMessage(msg, route)
	if err != nil {
		return err
	}
	n, err := w.rw.Write(buf)
	if n != len(buf) {
		err = errors.ErrNotEnoughBytes
	}
	codec.Release(buf)
	return err
}
