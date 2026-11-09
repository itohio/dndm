package remote

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/network/stream"
	"github.com/stretchr/testify/require"
)

type pipeRW struct {
	io.Reader
	io.Writer
}

// newPipedEndpoint returns an initialised endpoint and the writer that feeds its reads.
func newPipedEndpoint(t *testing.T, ctx context.Context) (*Endpoint, *io.PipeWriter) {
	t.Helper()
	local, err := dndm.NewPeer("stream", "local", "", nil)
	require.NoError(t, err)
	peer, err := dndm.NewPeer("stream", "peer", "", nil)
	require.NoError(t, err)

	in, feed := io.Pipe()
	out, sink := io.Pipe()
	go io.Copy(io.Discard, out) //nolint:errcheck // drains whatever the endpoint writes
	t.Cleanup(func() {
		feed.Close()
		sink.Close()
	})

	conn := stream.NewWithContext(ctx, local, peer, pipeRW{Reader: in, Writer: sink}, nil)
	ep := New(local, conn, 1, time.Second, 0)
	require.NoError(t, ep.Init(ctx, slog.Default(),
		func(dndm.Intent, dndm.Endpoint) error { return nil },
		func(dndm.Interest, dndm.Endpoint) error { return nil },
	))
	return ep, feed
}

// DND-019: a read error makes the message handler close the endpoint. Close waits for
// the handler, so the handler must not wait for itself, and a later Close must return.
func TestEndpoint_CloseAfterRemoteBadFrameReturns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ep, feed := newPipedEndpoint(t, ctx)

	// A peer sends 8 bytes that are not a frame (bad magic).
	go feed.Write([]byte{1, 2, 3, 4, 0, 0, 0, 8}) //nolint:errcheck

	select {
	case <-ep.Ctx().Done():
	case <-time.After(2 * time.Second):
		t.Fatal("endpoint did not close after a bad frame")
	}

	closed := make(chan struct{})
	go func() {
		ep.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close hung: the message handler is waiting for itself")
	}
}

// Closing the endpoint from outside while the handler is blocked in a read must also return.
func TestEndpoint_CloseWhileReadingReturns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ep, _ := newPipedEndpoint(t, ctx)

	closed := make(chan struct{})
	go func() {
		ep.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close hung while the message handler was reading")
	}
}

// DND-022 follow-up 2 (verify's DND-008 recheck): a connection that is already closed
// when Init registers its close callback runs that callback at once on another
// goroutine. shutdown then read t.linker while Init was still assigning it: a data race,
// or a nil-linker panic. Needs -race to fail reliably.
func TestEndpoint_InitOnClosedConn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	local, err := dndm.NewPeer("stream", "local", "", nil)
	require.NoError(t, err)
	peer, err := dndm.NewPeer("stream", "peer", "", nil)
	require.NoError(t, err)
	in, feed := io.Pipe()
	out, sink := io.Pipe()
	go io.Copy(io.Discard, out) //nolint:errcheck
	defer feed.Close()
	defer sink.Close()

	conn := stream.NewWithContext(ctx, local, peer, pipeRW{Reader: in, Writer: sink}, nil)
	require.NoError(t, conn.Close()) // the peer went away before Init

	ep := New(local, conn, 1, time.Second, 0)
	require.NoError(t, ep.Init(ctx, slog.Default(),
		func(dndm.Intent, dndm.Endpoint) error { return nil },
		func(dndm.Interest, dndm.Endpoint) error { return nil },
	))
	select {
	case <-ep.Ctx().Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the endpoint did not shut down after its connection closed")
	}
	require.NoError(t, ep.Close())
}
