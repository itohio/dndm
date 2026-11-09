package stream

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

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
func NewWithContext(ctx context.Context, localPeer, remotePeer dndm.Peer, rw io.ReadWriter, handlers map[types.Type]network.MessageHandler) *StreamContext {
	ret := &StreamContext{
		Stream: New(ctx, localPeer, remotePeer, rw, handlers),
	}
	ret.read = contextRW{
		Base:    dndm.NewBaseWithCtx(ctx),
		request: make(chan contextRWRequest),
		copyOut: true,
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
		c:   &c.read,
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
	n, inUse, err := w.write.Request(ctx, buf)
	if err == nil && n != len(buf) {
		err = errors.ErrNotEnoughBytes
	}
	if !inUse { // a cancelled Write may still be on the wire; leave its buffer to the GC
		codec.Release(buf)
	}
	return err
}

type contextRWRequest struct {
	ctx      context.Context
	data     []byte
	resultCh chan<- contextRWResult
	ack      <-chan struct{} // read side: the caller has copied src
}
type contextRWResult struct {
	n   int
	err error
	src []byte // read side: bytes in the loop's buffer, copied by the caller
}

// contextRW runs blocking Read or Write calls in one loop goroutine so callers can give
// up on a context. A caller's buffer is never touched after its Request returned:
//   - read side (copyOut): the loop reads into its own buffer, and the caller copies the
//     bytes out. Bytes read for a caller that gave up are kept for the next Request.
//   - write side: the loop writes the caller's buffer, and Request reports whether the
//     loop may still be using it.
type contextRW struct {
	dndm.Base
	request chan contextRWRequest
	copyOut bool

	// Owned by the Run goroutine.
	scratch    []byte
	pending    []byte
	pendingErr error
}

// Close cancels the request loop. The request channel is never closed: Run is its only
// receiver and Request selects on the context, so pending senders return instead of
// sending on a closed channel.
func (c *contextRW) Close() error {
	return c.Base.Close()
}

func (c *contextRW) Run(ctx context.Context, f func([]byte) (int, error)) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.Ctx().Done():
			return
		case r := <-c.request:
			if !c.serve(ctx, r, f) {
				return
			}
		}
	}
}

// readScratchSize bounds the read loop's own buffer. Callers such as codec.ReadMessage
// ask for a whole frame body at once; sizing the buffer to the request would keep the
// largest frame (up to codec.DefaultMaxFrameSize) per stream for its whole life.
const readScratchSize = 1024

// serve handles one request and reports whether the loop should go on.
func (c *contextRW) serve(ctx context.Context, r contextRWRequest, f func([]byte) (int, error)) bool {
	var res contextRWResult
	fromPending := false
	switch {
	case !c.copyOut:
		res.n, res.err = f(r.data)
	case len(c.pending) > 0 || c.pendingErr != nil:
		fromPending = true
		res.src = c.pending[:min(len(c.pending), len(r.data))]
		res.n = len(res.src)
		if res.n == len(c.pending) {
			res.err = c.pendingErr
		}
	default:
		if c.scratch == nil {
			c.scratch = make([]byte, readScratchSize)
		}
		// A short read is fine: io.ReadFull and codec.ReadMessage ask again for the rest.
		res.n, res.err = f(c.scratch[:min(len(r.data), len(c.scratch))])
		res.src = c.scratch[:res.n]
	}

	select {
	case <-ctx.Done():
		close(r.resultCh)
		return false
	case <-c.Ctx().Done():
		close(r.resultCh)
		return false
	case <-r.ctx.Done():
		if c.copyOut && !fromPending { // nobody took these bytes; keep them for the next Request
			c.pending = append(c.pending, res.src...)
			c.pendingErr = res.err
		}
	case r.resultCh <- res:
		if c.copyOut {
			<-r.ack
			if fromPending {
				c.pending = c.pending[res.n:]
				if len(c.pending) == 0 {
					c.pending, c.pendingErr = nil, nil
				}
			}
		}
	}
	close(r.resultCh)
	return true
}

// Request hands buf to the loop and waits for the result or for ctx. inUse reports that
// the loop may still be using buf (write side, cancelled after hand-over).
func (c *contextRW) Request(ctx context.Context, buf []byte) (n int, inUse bool, err error) {
	ch := make(chan contextRWResult)
	ack := make(chan struct{})
	select {
	case <-ctx.Done():
		return 0, false, ctx.Err()
	case <-c.Ctx().Done():
		return 0, false, c.Ctx().Err()
	case c.request <- contextRWRequest{data: buf, resultCh: ch, ack: ack, ctx: ctx}:
	}
	select {
	case <-ctx.Done():
		return 0, !c.copyOut, ctx.Err()
	case <-c.Ctx().Done():
		return 0, !c.copyOut, c.Ctx().Err()
	case result, ok := <-ch:
		if !ok {
			return 0, false, context.Canceled
		}
		if c.copyOut {
			n = copy(buf, result.src)
			close(ack)
			return n, false, result.err
		}
		return result.n, false, result.err
	}
}

type reader struct {
	c   *contextRW
	ctx context.Context
}

func (r reader) Read(buf []byte) (int, error) {
	n, _, err := r.c.Request(r.ctx, buf)
	return n, err
}

// Stream converts regular ReaderWriter into a Remote
type Stream struct {
	dndm.Base
	localPeer dndm.Peer
	handlers  map[types.Type]network.MessageHandler
	rw        io.ReadWriter
	closed    atomic.Bool
	writeMu   sync.Mutex // one frame per rw.Write call; writers that split a call must not interleave

	mu         sync.Mutex
	remotePeer dndm.Peer
	routes     map[string]dndm.Route
}

type readWriter struct {
	r io.Reader
	w io.Writer
}

func (rw readWriter) Read(buf []byte) (int, error)  { return rw.r.Read(buf) }
func (rw readWriter) Write(buf []byte) (int, error) { return rw.w.Write(buf) }

// NewIO creates a Remote using io.Reader and io.Writer.
func NewIO(ctx context.Context, localPeer, remotePeer dndm.Peer, r io.Reader, w io.Writer, handlers map[types.Type]network.MessageHandler) *Stream {
	rw := readWriter{r: r, w: w}
	return New(ctx, localPeer, remotePeer, rw, handlers)
}

// New creates a Remote using provided ReaderWriter.
func New(ctx context.Context, localPeer, remotePeer dndm.Peer, rw io.ReadWriter, handlers map[types.Type]network.MessageHandler) *Stream {
	return &Stream{
		Base:       dndm.NewBaseWithCtx(ctx),
		localPeer:  localPeer,
		remotePeer: remotePeer,
		rw:         rw,
		handlers:   handlers,
		routes:     make(map[string]dndm.Route),
	}
}

func (w *Stream) Local() dndm.Peer {
	return w.localPeer
}

func (w *Stream) Remote() dndm.Peer {
	w.mu.Lock()
	p := w.remotePeer
	w.mu.Unlock()
	return p
}

func (w *Stream) UpdateRemotePeer(p dndm.Peer) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p.Scheme() != w.remotePeer.Scheme() {
		return errors.ErrBadArgument
	}
	w.remotePeer = p
	return nil
}

// Close closes the underlying reader/writer once, then the stream's context and close callbacks.
// The reader/writer closes first so readers blocked on it return before callbacks wait for them.
func (w *Stream) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	slog.Info("Stream.Close")
	var err error
	if closer, ok := w.rw.(io.Closer); ok {
		err = closer.Close()
	}
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
	w.writeMu.Lock()
	n, err := w.rw.Write(buf)
	w.writeMu.Unlock()
	if err == nil && n != len(buf) {
		err = errors.ErrNotEnoughBytes
	}
	codec.Release(buf)
	return err
}
