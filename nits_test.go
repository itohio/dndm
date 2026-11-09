package dndm

import (
	"context"
	"log/slog"
	"runtime/metrics"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Review R-DND-R5 F6: an explicit Close must stop the context's AfterFunc. Otherwise
// cancelling the context in Close starts a goroutine whose only job is to lose the close
// race. Counted with the runtime's cumulative goroutines-created metric.
func TestBase_ExplicitCloseStopsAfterFunc(t *testing.T) {
	ctx := context.Background()
	sample := []metrics.Sample{{Name: "/sched/goroutines-created:goroutines"}}
	created := func() uint64 {
		metrics.Read(sample)
		return sample[0].Value.Uint64()
	}
	const n = 200
	before := created()
	for range n {
		b := NewBaseWithCtx(ctx)
		b.AddOnClose(func() {})
		b.Close()
	}
	spawned := created() - before
	t.Logf("goroutines created by %d create+AddOnClose+Close: %d", n, spawned)
	// The metric is process-wide, so parallel tests add noise; the bug adds one per Close
	// (n here), so n/4 separates them with room (review R-DND-R8 N5).
	assert.Less(t, spawned, uint64(n/4), "explicit Close still fires the context's AfterFunc")
}

// Review F8: re-linking the channel that is already linked is a no-op and must not wait
// behind a Send blocked on that full channel.
func TestLocalIntent_RelinkSameChannelDoesNotBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	route, err := NewRoute("relink", &testtypes.Foo{})
	require.NoError(t, err)
	intent := NewIntent(ctx, route, 1)
	defer intent.Close()
	msgC := make(chan proto.Message, 1)
	intent.Link(msgC)
	require.NoError(t, intent.Send(ctx, &testtypes.Foo{})) // fills the channel
	go intent.Send(ctx, &testtypes.Foo{})                  //nolint:errcheck // blocks on the full channel, holding the read lock
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		intent.Link(msgC)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Link with the same channel waited for a blocked Send")
	}
	<-msgC // unblock the waiting Send
}

// Review F15: Linker.RemoveIntent drops the intent's match; the intent must stop
// delivering into the interest, as RemoveInterest already ensures.
func TestLinker_RemoveIntentUnlinks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	route, err := NewRoute("rmintent", &testtypes.Foo{})
	require.NoError(t, err)
	l := NewLinker(ctx, slog.Default(), 4, func(Intent) error { return nil }, func(Interest) error { return nil }, nil)
	defer l.Close()
	intent, err := l.AddIntent(route)
	require.NoError(t, err)
	_, err = l.AddInterest(route)
	require.NoError(t, err)
	require.NoError(t, intent.Send(ctx, &testtypes.Foo{}), "matched intent delivers")

	require.NoError(t, l.RemoveIntent(route))
	assert.Eventually(t, func() bool {
		return errors.Is(intent.Send(ctx, &testtypes.Foo{}), errors.ErrNoInterest)
	}, time.Second, 5*time.Millisecond, "the removed intent still delivers into the interest")
}
