package mesh

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/itohio/dndm"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pipeRW struct {
	io.Reader
	io.Writer
}

// bufPipe is a one-way byte pipe whose writes never block, like a socket with send
// buffer space. io.Pipe would make the handshake deadlock in the test: both sides write
// from their read loops (FINAL vs PEERS) and an unbuffered pipe needs a reader for each write.
type bufPipe struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    bytes.Buffer
	closed bool
}

func newBufPipe() *bufPipe {
	p := &bufPipe{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *bufPipe) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for p.buf.Len() == 0 && !p.closed {
		p.cond.Wait()
	}
	if p.buf.Len() == 0 {
		return 0, io.EOF
	}
	return p.buf.Read(b)
}

func (p *bufPipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	p.cond.Broadcast()
	return p.buf.Write(b)
}

func (p *bufPipe) Close() {
	p.mu.Lock()
	p.closed = true
	p.cond.Broadcast()
	p.mu.Unlock()
}

// gatedReader blocks reads until open is closed, holding a handshake before completion.
type gatedReader struct {
	r    io.Reader
	open chan struct{}
}

func (g *gatedReader) Read(p []byte) (int, error) {
	<-g.open
	return g.r.Read(p)
}

// routeCounter records the distinct intents the remote side created for each route.
// It counts intents, not callback calls: the remote endpoint reports each new remote
// intent twice (linker callback and handleRemoteIntent), which is a separate defect.
type routeCounter struct {
	mu      sync.Mutex
	intents map[string]map[dndm.Intent]struct{}
}

func (c *routeCounter) add(intent dndm.Intent, _ dndm.Endpoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	path := intent.Route().Path()
	if c.intents[path] == nil {
		c.intents[path] = map[dndm.Intent]struct{}{}
	}
	c.intents[path][intent] = struct{}{}
	return nil
}

func (c *routeCounter) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.intents[path])
}

var (
	noIntent   = func(dndm.Intent, dndm.Endpoint) error { return nil }
	noInterest = func(dndm.Interest, dndm.Endpoint) error { return nil }
)

func testRoute(t *testing.T, path string) dndm.Route {
	t.Helper()
	r, err := dndm.NewRoute(path, &testtypes.Foo{})
	require.NoError(t, err)
	return r
}

// connection is one A→B connection generation: B answers handshakes and counts the
// intents A registers with it; A's handshake completes only after gate opens.
type connection struct {
	hsA  *Handshaker
	seen *routeCounter
	gate *gatedReader
}

func newConnection(t *testing.T, ctx context.Context, bookA, bookB *Addrbook) connection {
	t.Helper()
	aToB, bToA := newBufPipe(), newBufPipe()
	t.Cleanup(func() {
		aToB.Close()
		bToA.Close()
	})
	seen := &routeCounter{intents: map[string]map[dndm.Intent]struct{}{}}
	hsB := NewHandshaker(bookB, bookA.Self(), 4, time.Second, 0, pipeRW{Reader: aToB, Writer: bToA}, HS_WAIT)
	require.NoError(t, hsB.Init(ctx, slog.Default(), seen.add, noInterest))
	t.Cleanup(func() { hsB.Close() })

	gate := &gatedReader{r: bToA, open: make(chan struct{})}
	hsA := NewHandshaker(bookA, bookB.Self(), 4, time.Second, 0, pipeRW{Reader: gate, Writer: aToB}, HS_INIT)
	return connection{hsA: hsA, seen: seen, gate: gate}
}

func newBooksAndContainer(t *testing.T, ctx context.Context) (*Addrbook, *Addrbook, *dndm.Container) {
	t.Helper()
	peerA, err := dndm.NewPeer("tcp", "127.0.0.1:1", "a", nil)
	require.NoError(t, err)
	peerB, err := dndm.NewPeer("tcp", "127.0.0.1:2", "b", nil)
	require.NoError(t, err)
	bookA := NewAddrbook(peerA, nil, 0)
	require.NoError(t, bookA.Init(ctx, slog.Default()))
	bookB := NewAddrbook(peerB, nil, 0)
	require.NoError(t, bookB.Init(ctx, slog.Default()))

	container := dndm.NewContainer("a", 4)
	require.NoError(t, container.Init(ctx, slog.Default(), noIntent, noInterest))
	t.Cleanup(func() { container.Close() })
	return bookA, bookB, container
}

func waitRegistered(t *testing.T, seen *routeCounter, paths ...string) {
	t.Helper()
	for _, path := range paths {
		assert.Eventually(t, func() bool { return seen.count(path) > 0 }, 3*time.Second, 10*time.Millisecond,
			"route %q never reached the remote after the handshake completed", path)
	}
}

// DND-004: routes that exist, are added or are removed while a handshake is pending reach
// the remote exactly as they stand when the handshake completes; nothing is skipped forever.
func TestContainerRegistersRoutesAcrossHandshakeCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bookA, bookB, container := newBooksAndContainer(t, ctx)
	conn := newConnection(t, ctx, bookA, bookB)

	_, err := container.Publish(testRoute(t, "existing"))
	require.NoError(t, err)
	removed, err := container.Publish(testRoute(t, "removed"))
	require.NoError(t, err)

	require.NoError(t, container.Add(conn.hsA))

	_, err = container.Publish(testRoute(t, "during"))
	assert.NoError(t, err, "publishing while one connection is mid-handshake must not fail")
	require.NoError(t, removed.Close())

	close(conn.gate.open)
	waitRegistered(t, conn.seen, "existing", "during")

	_, err = container.Publish(testRoute(t, "after"))
	require.NoError(t, err)
	waitRegistered(t, conn.seen, "after")

	time.Sleep(200 * time.Millisecond) // let any duplicate registration arrive
	for _, path := range []string{"existing", "during", "after"} {
		assert.Equal(t, 1, conn.seen.count(path), "route %q has more than one remote intent", path)
	}
	assert.Zero(t, conn.seen.count("removed"), "a route removed before completion was resurrected")
}

// DND-004 reconnect: a new connection generation gets the routes that are live when it
// completes, once; the closed generation gets nothing more.
func TestContainerRegistersRoutesOnReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bookA, bookB, container := newBooksAndContainer(t, ctx)

	_, err := container.Publish(testRoute(t, "kept"))
	require.NoError(t, err)
	dropped, err := container.Publish(testRoute(t, "dropped"))
	require.NoError(t, err)

	first := newConnection(t, ctx, bookA, bookB)
	require.NoError(t, container.Add(first.hsA))
	close(first.gate.open)
	waitRegistered(t, first.seen, "kept", "dropped")

	require.NoError(t, container.Remove(first.hsA))
	require.NoError(t, dropped.Close())
	_, err = container.Publish(testRoute(t, "new"))
	require.NoError(t, err)

	second := newConnection(t, ctx, bookA, bookB)
	require.NoError(t, container.Add(second.hsA))
	close(second.gate.open)
	waitRegistered(t, second.seen, "kept", "new")

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 1, second.seen.count("kept"))
	assert.Equal(t, 1, second.seen.count("new"))
	assert.Zero(t, second.seen.count("dropped"), "a route removed before reconnect was replayed")
	assert.Zero(t, first.seen.count("new"), "a closed generation received a new registration")
}
