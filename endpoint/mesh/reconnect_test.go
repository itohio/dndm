package mesh

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	types "github.com/itohio/dndm/types/core"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// DND-008 close/reconnect (docs/design/close-reconnect.md §6).

// fakeConn is a network.Conn whose close callbacks run only when the test says so, the
// way a callback added to an already-closed stream runs later on its own goroutine.
type fakeConn struct {
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	callbacks []func()
}

func newFakeConn() *fakeConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeConn{ctx: ctx, cancel: cancel}
}

func (c *fakeConn) Ctx() context.Context { return c.ctx }
func (c *fakeConn) Close() error         { c.cancel(); return nil } // callbacks run later
func (c *fakeConn) OnClose(f func()) network.Conn {
	c.mu.Lock()
	c.callbacks = append(c.callbacks, f)
	c.mu.Unlock()
	return c
}
func (c *fakeConn) runCallbacks() {
	c.mu.Lock()
	cbs := c.callbacks
	c.callbacks = nil
	c.mu.Unlock()
	for _, f := range cbs {
		f()
	}
}
func (c *fakeConn) Local() dndm.Peer                                       { return nil }
func (c *fakeConn) Remote() dndm.Peer                                      { return nil }
func (c *fakeConn) UpdateRemotePeer(dndm.Peer) error                       { return nil }
func (c *fakeConn) AddRoute(...dndm.Route)                                 {}
func (c *fakeConn) DelRoute(...dndm.Route)                                 {}
func (c *fakeConn) Write(context.Context, dndm.Route, proto.Message) error { return nil }
func (c *fakeConn) Read(ctx context.Context) (*types.Header, proto.Message, error) {
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

func testBook(t *testing.T, ctx context.Context, addr, path string) *Addrbook {
	t.Helper()
	self, err := dndm.NewPeer("tcp", addr, path, nil)
	require.NoError(t, err)
	book := NewAddrbook(self, nil, 0)
	require.NoError(t, book.Init(ctx, slog.Default()))
	return book
}

// R2: a live connection must not be lost behind a dead one whose close callback runs late.
func TestAddrbookEntry_LiveConnNotLostBehindDeadOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	book := testBook(t, ctx, "127.0.0.1:1", "a")
	peer, err := dndm.NewPeer("tcp", "127.0.0.1:2", "b", nil)
	require.NoError(t, err)

	dead := newFakeConn()
	require.NoError(t, book.AddConn(peer, true, dead))
	require.NoError(t, dead.Close()) // closed, but its callback has not run yet

	live := newFakeConn()
	assert.NoError(t, book.AddConn(peer, true, live), "a dead connection must not block a live one")

	dead.runCallbacks() // the late callback of the dead connection
	entry, ok := book.Entry(peer)
	require.True(t, ok)
	assert.True(t, entry.IsActive(), "the late callback of a dead connection wiped the live one")
	entry.Lock()
	current := entry.conn
	entry.Unlock()
	assert.Same(t, live, current)
}

// closingPipeRW is a pipe end that closes both directions on Close, so the peer reads EOF.
type closingPipeRW struct {
	r, w *bufPipe
}

func (p closingPipeRW) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p closingPipeRW) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p closingPipeRW) Close() error {
	p.r.Close()
	p.w.Close()
	return nil
}

// gatedRW holds reads until open is closed.
type gatedRW struct {
	closingPipeRW
	open chan struct{}
}

func (g gatedRW) Read(b []byte) (int, error) {
	<-g.open
	return g.closingPipeRW.Read(b)
}

type node struct {
	book      *Addrbook
	container *dndm.Container
}

func newNode(t *testing.T, ctx context.Context, addr, path string) node {
	t.Helper()
	book := testBook(t, ctx, addr, path)
	c := dndm.NewContainer(path, 4)
	require.NoError(t, c.Init(ctx, slog.Default(), noIntent, noInterest))
	t.Cleanup(func() { c.Close() })
	return node{book: book, container: c}
}

// dialPair builds one connection from dialer to acceptor, both reads gated by open.
func dialPair(t *testing.T, dialer, acceptor node, open chan struct{}) (*Handshaker, *Handshaker) {
	t.Helper()
	return dialPairAs(t, dialer, acceptor, acceptor.book.Self(), open)
}

// dialPairAs dials the acceptor under target, which may name it differently from how it
// declares itself (a seed configured as "localhost:2" for a node that says "127.0.0.1:2").
func dialPairAs(t *testing.T, dialer, acceptor node, target dndm.Peer, open chan struct{}) (*Handshaker, *Handshaker) {
	t.Helper()
	d2a, a2d := newBufPipe(), newBufPipe()
	t.Cleanup(func() {
		d2a.Close()
		a2d.Close()
	})
	hsD := NewHandshaker(dialer.book, target, 4, time.Second, 0,
		gatedRW{closingPipeRW{r: a2d, w: d2a}, open}, HS_INIT)
	hsA := NewHandshaker(acceptor.book, dialer.book.Self(), 4, time.Second, 0,
		gatedRW{closingPipeRW{r: d2a, w: a2d}, open}, HS_WAIT)
	require.NoError(t, acceptor.container.Add(hsA))
	require.NoError(t, dialer.container.Add(hsD))
	return hsD, hsA
}

func endpoints(n node) int {
	return len(n.container.Endpoint(func(dndm.Endpoint) bool { return true }))
}

// R1/R3: when A and B dial each other at the same moment, both must keep the same single
// connection (the one dialled by the lower identity), and a message must arrive once.
// Both orders matter: the publisher is the lower identity in one, the higher in the other.
func TestMesh_SimultaneousDialKeepsOneConnection(t *testing.T) {
	t.Run("publisher lower", func(t *testing.T) { simultaneousDial(t, "127.0.0.1:1", "127.0.0.1:2", false) })
	t.Run("publisher higher", func(t *testing.T) { simultaneousDial(t, "127.0.0.1:2", "127.0.0.1:1", false) })
	// Review R-DND-R8 N2: the tie-break must use the identities both sides declare, not
	// the address each dialer happened to be configured with.
	t.Run("dial targets differ from declared identities", func(t *testing.T) { simultaneousDial(t, "127.0.0.1:1", "127.0.0.1:2", true) })
}

func simultaneousDial(t *testing.T, addrA, addrB string, alias bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a := newNode(t, ctx, addrA, "a")
	b := newNode(t, ctx, addrB, "b")

	route := testRoute(t, "sim")
	intent, err := a.container.Publish(route)
	require.NoError(t, err)
	interest, err := b.container.Subscribe(route)
	require.NoError(t, err)

	open := make(chan struct{})
	targetA, targetB := a.book.Self(), b.book.Self()
	if alias { // each knows the other as localhost:<port>; both declare 127.0.0.1:<port>
		targetA, err = dndm.NewPeer("tcp", "localhost:"+strings.Split(addrA, ":")[1], "a", nil)
		require.NoError(t, err)
		targetB, err = dndm.NewPeer("tcp", "localhost:"+strings.Split(addrB, ":")[1], "b", nil)
		require.NoError(t, err)
	}
	dialPairAs(t, a, b, targetB, open) // A dials B
	dialPairAs(t, b, a, targetA, open) // B dials A
	close(open)

	assert.Eventually(t, func() bool { return endpoints(a) == 1 && endpoints(b) == 1 }, 3*time.Second, 10*time.Millisecond,
		"each side must end with one connection (A has %d, B has %d)", endpoints(a), endpoints(b))

	// The interest must reach A over the connection that survives. A notification alone is
	// not enough: while both connections are briefly ready (close-reconnect.md §3.1
	// residual), it can come from the one that loses the tie-break and closes, taking its
	// remote interest with it (DND-008 G1). ErrNoInterest means nothing was sent anywhere,
	// so retrying cannot duplicate the message.
	var sendErr error
	require.Eventually(t, func() bool {
		sendErr = intent.Send(ctx, &testtypes.Foo{Text: "once"})
		return !errors.Is(sendErr, errors.ErrNoInterest)
	}, 5*time.Second, 5*time.Millisecond, "B's interest never reached A over the surviving connection")
	require.NoError(t, sendErr)

	var got atomic.Int32
	deadline := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case msg := <-interest.C():
			if msg != nil {
				got.Add(1)
			}
		case <-deadline:
			break loop
		}
	}
	assert.EqualValues(t, 1, got.Load(), "the message arrived %d times", got.Load())
}

// Redial pins today's behaviour: once a persistent outbound peer's connection closes,
// the next monitor pass queues it for dialing.
func TestMesh_ReconnectQueuesRedial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	book := testBook(t, ctx, "127.0.0.1:1", "a")
	peer, err := dndm.NewPeer("tcp", "127.0.0.1:2", "b", nil)
	require.NoError(t, err)

	conn := newFakeConn()
	require.NoError(t, book.AddConn(peer, true, conn))
	entry, _ := book.Entry(peer)
	entry.Lock()
	entry.Persistent = true
	entry.Unlock()
	for len(book.Dials()) > 0 { // drop anything Init's first monitor pass queued
		<-book.Dials()
	}

	book.monitor()
	assert.Empty(t, book.Dials(), "a connected peer was queued for dialing")

	require.NoError(t, conn.Close())
	conn.runCallbacks()
	book.monitor()
	select {
	case e := <-book.Dials():
		assert.Same(t, entry, e)
	default:
		t.Fatal("a closed persistent peer was not queued for redial")
	}
}

// Closing a node while a dial's handshake is still in flight returns promptly and leaves
// no goroutines behind.
func TestMesh_CloseDuringHandshakeReturns(t *testing.T) {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nodeCtx, nodeCancel := context.WithCancel(ctx)
	a := newNode(t, nodeCtx, "127.0.0.1:1", "a")
	b := newNode(t, nodeCtx, "127.0.0.1:2", "b")
	never := make(chan struct{}) // the handshake never gets its reply
	dialPair(t, a, b, never)

	done := make(chan struct{})
	go func() {
		a.container.Close()
		b.container.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close hung while a handshake was in flight")
	}
	nodeCancel()
	close(never) // release reads parked on the gate
	assert.Eventually(t, func() bool { return runtime.NumGoroutine() <= before+2 }, 3*time.Second, 20*time.Millisecond,
		"goroutines left after close: %d (before %d)", runtime.NumGoroutine(), before)
}
