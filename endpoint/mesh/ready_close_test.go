package mesh

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/stretchr/testify/require"
)

// Review R-DND-R5 F2: once Close has begun, Handshaker must not start OnReady callbacks.
// Close waits for running callbacks with a WaitGroup; an Add after that Wait began is a
// misuse, and the started callback runs after Close returned.

func newIdleHandshaker(t *testing.T, ctx context.Context) *Handshaker {
	t.Helper()
	self, err := dndm.NewPeer("tcp", "127.0.0.1:1", "a", nil)
	require.NoError(t, err)
	peer, err := dndm.NewPeer("tcp", "127.0.0.1:2", "b", nil)
	require.NoError(t, err)
	book := NewAddrbook(self, nil, 0)
	require.NoError(t, book.Init(ctx, slog.Default()))
	in, out := newBufPipe(), newBufPipe()
	t.Cleanup(func() {
		in.Close()
		out.Close()
	})
	h := NewHandshaker(book, peer, 4, time.Second, 0, pipeRW{Reader: in, Writer: out}, HS_WAIT)
	require.NoError(t, h.Init(ctx, slog.Default(), noIntent, noInterest))
	return h
}

func requireNotStarted(t *testing.T, ran <-chan struct{}) {
	t.Helper()
	select {
	case <-ran:
		t.Fatal("an OnReady callback started after Close")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandshaker_OnReadyAfterCloseStartsNothing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := newIdleHandshaker(t, ctx)
	h.markReady()
	require.NoError(t, h.Close())

	ran := make(chan struct{}, 1)
	h.OnReady(func() { ran <- struct{}{} })
	requireNotStarted(t, ran)
}

func TestHandshaker_MarkReadyAfterCloseStartsNothing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := newIdleHandshaker(t, ctx)
	ran := make(chan struct{}, 1)
	h.OnReady(func() { ran <- struct{}{} }) // queued: not ready yet
	require.NoError(t, h.Close())

	h.markReady() // a handshake message processed while Close runs
	requireNotStarted(t, ran)
}
