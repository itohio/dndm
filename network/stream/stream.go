package stream

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

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
	read  contextRW
	write contextRW
}

// NewWithContext will use ReadWriter and allow for context cancellation in Read and Write methods.
func NewWithContext(ctx context.Context, localPeer, remotePeer network.Peer, rw io.ReadWriter, handlers map[types.Type]network.MessageHandler) *StreamContext {
	ret := &StreamContext{
		Stream: New(ctx, localPeer, remotePeer, rw, handlers),
	}
	ret.read = contextRW{
		Base:    dndm.NewBaseWithCtx(ctx),
		request: make(chan contextRWRequest),
	}
	ret.write = contextRW{
		Base:    dndm.NewBaseWithCtx(ctx),
		request: make(chan contextRWRequest),
	}
	ret.AddOnClose(func() {
		ret.read.Close()
		ret.write.Close()
	})

	ret.run()
	return ret
}

func (c *StreamContext) run() {
	go c.read.Run(c.Ctx(), c.rw.Read)
	go c.write.Run(c.Ctx(), c.rw.Write)
	go func() {
		<-c.Ctx().Done()
		c.Close()
	}()
}

func (c *StreamContext) Reader(ctx context.Context) io.Reader {
	return &reader{
		c:   c.read,
		ctx: ctx,
	}
}

func (w *StreamContext) Close() error {
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

type contextRWRequest struct {
	ctx      context.Context
	data     []byte
	resultCh chan<- contextRWResult
}
type contextRWResult struct {
	n   int
	err error
}

type contextRW struct {
	dndm.Base
	request chan contextRWRequest
}

func (c *contextRW) Close() error {
	c.Base.Close()
	time.Sleep(time.Microsecond)
	for {
		select {
		case <-c.request:
		default:
			close(c.request)
			return nil
		}
	}
}

func (c *contextRW) Run(ctx context.Context, f func([]byte) (int, error)) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.Ctx().Done():
			return
		case r, ok := <-c.request:
			if !ok {
				continue
			}

			n, err := f(r.data)
			select {
			case <-ctx.Done():
				close(r.resultCh)
				return
			case <-c.Ctx().Done():
				close(r.resultCh)
				return
			case <-r.ctx.Done():
			case r.resultCh <- contextRWResult{n: n, err: err}:
			}
			close(r.resultCh)
		}
	}
}

func (c *contextRW) Request(ctx context.Context, buf []byte) (n int, err error) {
	defer func() {
		if e := recover(); e != nil {
			// NOTE: This panic is unavoidable sometimes: we close the channel onClose, however, someone might have
			// pending request on this channel due to previous call not returning yet.
			// Since onClosed is sometimes called before context cancel, there is no way to safely
			// unblock the sending to a channel that is about to be closed.
			if e, ok := e.(error); !ok || e.Error() != "send on closed channel" {
				panic(e)
			}
			err = context.Canceled
		}
	}()
	ch := make(chan contextRWResult)
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-c.Ctx().Done():
		return 0, c.Ctx().Err()
	case c.request <- contextRWRequest{data: buf, resultCh: ch, ctx: ctx}:
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-c.Ctx().Done():
			return 0, c.Ctx().Err()
		case result, ok := <-ch:
			if !ok {
				return 0, context.Canceled
			}
			return result.n, result.err
		}
	}
}

type reader struct {
	c   contextRW
	ctx context.Context
}

func (r reader) Read(buf []byte) (int, error) {
	return r.c.Request(r.ctx, buf)
}

// Stream converts regular ReaderWriter into a Remote
type Stream struct {
	dndm.Base
	localPeer network.Peer
	handlers  map[types.Type]network.MessageHandler
	rw        io.ReadWriter

	mu         sync.Mutex
	remotePeer network.Peer
	routes     map[string]dndm.Route
}

type readWriter struct {
	r io.Reader
	w io.Writer
}

func (rw readWriter) Read(buf []byte) (int, error)  { return rw.r.Read(buf) }
func (rw readWriter) Write(buf []byte) (int, error) { return rw.w.Write(buf) }

// NewIO creates a Remote using io.Reader and io.Writer.
func NewIO(ctx context.Context, localPeer, remotePeer network.Peer, r io.Reader, w io.Writer, handlers map[types.Type]network.MessageHandler) *Stream {
	rw := readWriter{r: r, w: w}
	return New(ctx, localPeer, remotePeer, rw, handlers)
}

// New creates a Remote using provided ReaderWriter.
func New(ctx context.Context, localPeer, remotePeer network.Peer, rw io.ReadWriter, handlers map[types.Type]network.MessageHandler) *Stream {
	return &Stream{
		Base:       dndm.NewBaseWithCtx(ctx),
		localPeer:  localPeer,
		remotePeer: remotePeer,
		rw:         rw,
		handlers:   handlers,
		routes:     make(map[string]dndm.Route),
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
	w.Base.AddOnClose(func() {
		if closer, ok := w.rw.(io.Closer); ok {
			err = closer.Close()
		}
	})
	w.Base.Close()
	return err
}

func (w *Stream) OnClose(f func()) network.Conn {
	w.AddOnClose(f)
	return w
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

func (w *Stream) read(ctx context.Context) (*types.Header, proto.Message, error) {
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
	return hdr, msg, nil
}

func (w *Stream) Read(ctx context.Context) (*types.Header, proto.Message, error) {
	for {
		hdr, msg, err := w.read(ctx)
		if err != nil {
			return nil, nil, err
		}

		if w.handlers == nil {
			return hdr, msg, err
		}

		pass := true
		if handler, found := w.handlers[hdr.Type]; found {
			_pass, err := handler(hdr, msg, w)
			if err != nil {
				return hdr, msg, err
			}

			if !_pass {
				pass = false
			}
		}
		if pass {
			return hdr, msg, err
		}
	}
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
