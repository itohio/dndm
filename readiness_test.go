package dndm

import (
	"context"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/require"
)

// A wrapper created after the router already saw interest must still learn about it;
// otherwise a producer created second waits for an interest notification forever.
func TestIntentRouter_WrapAfterNotifyGetsInterest(t *testing.T) {
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	intent := NewIntent(ctx, route, 1)
	intent.Notify() // interest is already known when the router starts
	ir, err := NewIntentRouter(ctx, route, 1, intent)
	require.NoError(t, err)
	defer ir.Close()

	// Let the notify runner consume the notification before any wrapper exists.
	require.Eventually(t, func() bool { return len(intent.Interest()) == 0 }, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	w := ir.Wrap()
	defer w.Close()
	select {
	case got := <-w.Interest():
		require.Equal(t, route, got)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("wrapper created after the interest notification never learned about it")
	}
}

// A closed wrapper refuses Send instead of dereferencing a cleared router, and still knows
// its route. Before the fix removeWrapper set w.router = nil, racing Send and Route.
func TestRouterWrappers_UseAfterClose(t *testing.T) {
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	ctx := context.Background()

	ir, err := NewIntentRouter(ctx, route, 1)
	require.NoError(t, err)
	keep := ir.Wrap() // keeps the router open
	defer keep.Close()
	w := ir.Wrap()
	require.NoError(t, w.Close())
	require.NotPanics(t, func() {
		require.ErrorIs(t, w.Send(ctx, &testtypes.Foo{}), errors.ErrClosed)
		require.Equal(t, route, w.Route())
	})

	nr, err := NewInterestRouter(ctx, route, 1)
	require.NoError(t, err)
	keepI := nr.Wrap()
	defer keepI.Close()
	wi := nr.Wrap()
	require.NoError(t, wi.Close())
	require.NotPanics(t, func() { require.Equal(t, route, wi.Route()) })
}
