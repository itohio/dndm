package stream

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sync"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

var _ network.Conn = (*StreamContext)(nil)
var _ network.Conn = (*Stream)(nil)

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
	once   sync.Once
}

// NewWithContext will use ReadWriter and allow for context cancellation in Read and Write methods.
func NewWithContext(ctx context.Context, localPeer, remotePeer network.Peer, rw io.ReadWriter, handlers map[types.Type]network.MessageHandler) *StreamContext {
	ret := &StreamContext{
		Stream: New(localPeer, remotePeer, rw, handlers),
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
	go func() {
		<-ctx.Done()
		c.Close()
	}()
}

func (c *StreamContext) Reader(ctx context.Context) io.Reader {
	return &reader{
		c:   &c.read,
		ctx: ctx,
	}
}

func (w *StreamContext) Close() error {
	w.once.Do(func() {
		w.cancel()
		close(w.read.request)
		close(w.write.request)
		close(w.read.result)
		close(w.write.result)
	})
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

func (w *StreamContext) Write(ctx context.Context, route dndm.Route, msg proto.Message) error {
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
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("TypeOf", err, reflect.TypeOf(err))
		}
	}()

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
	handlers map[types.Type]network.MessageHandler
	rw       io.ReadWriter

	mu         sync.Mutex
	localPeer  network.Peer
	remotePeer network.Peer
	routes     map[string]dndm.Route
	once       sync.Once
	done       chan struct{}
}

type readWriter struct {
	r io.Reader
	w io.Writer
}

func (rw readWriter) Read(buf []byte) (int, error)  { return rw.r.Read(buf) }
func (rw readWriter) Write(buf []byte) (int, error) { return rw.w.Write(buf) }

// NewIO creates a Remote using io.Reader and io.Writer.
func NewIO(localPeer, remotePeer network.Peer, r io.Reader, w io.Writer, handlers map[types.Type]network.MessageHandler) *Stream {
	rw := readWriter{r: r, w: w}
	return New(localPeer, remotePeer, rw, handlers)
}

// New creates a Remote using provided ReaderWriter.
func New(localPeer, remotePeer network.Peer, rw io.ReadWriter, handlers map[types.Type]network.MessageHandler) *Stream {
	return &Stream{
		localPeer:  localPeer,
		remotePeer: remotePeer,
		rw:         rw,
		handlers:   handlers,
		done:       make(chan struct{}),
	}
}

func (w *Stream) Local() network.Peer {
	return w.localPeer
}

func (w *Stream) Remote() network.Peer {
	w.mu.Lock()
	p := w.remotePeer
	w.mu.Unlock()
	return p
}

func (w *Stream) UpdateRemotePeer(p network.Peer) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p.Scheme() != w.remotePeer.Scheme() {
		return errors.ErrBadArgument
	}
	w.remotePeer = p
	return nil
}

func (w *Stream) Close() error {
	slog.Info("Stream.Close")
	var err error
	if closer, ok := w.rw.(io.Closer); ok {
		err = closer.Close()
	}
	w.once.Do(func() { close(w.done) })
	return err
}

func (w *Stream) OnClose(f func()) {
	if notifier, ok := w.rw.(dndm.CloseNotifier); ok {
		notifier.OnClose(f)
		return
	}

	if f == nil {
		return
	}

	go func() {
		<-w.done
		f()
	}()
}

func (w *Stream) AddRoute(routes ...dndm.Route) {
	w.mu.Lock()
	for _, r := range routes {
		w.routes[r.ID()] = r
	}
	w.mu.Unlock()
}

func (w *Stream) DelRoute(routes ...dndm.Route) {
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

func (w *Stream) Write(ctx context.Context, route dndm.Route, msg proto.Message) error {
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
