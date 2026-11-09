package dndm

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// DND-018: close callbacks have an owner (the Base) and Close waits for them.

func TestBase_CloseWaitsForCallbacks(t *testing.T) {
	b := NewBaseWithCtx(context.Background())
	done := false
	b.AddOnClose(func() {
		time.Sleep(10 * time.Millisecond)
		done = true
	})

	require.NoError(t, b.Close())
	assert.True(t, done, "Close returned before its callback finished")
}

func TestBase_CallbacksRunInRegistrationOrder(t *testing.T) {
	b := NewBaseWithCtx(context.Background())
	var order []int
	for i := range 5 {
		b.AddOnClose(func() { order = append(order, i) })
	}

	require.NoError(t, b.Close())
	assert.Equal(t, []int{0, 1, 2, 3, 4}, order)
}

func TestBase_PendingCallbacksStartNoGoroutines(t *testing.T) {
	b := NewBaseWithCtx(context.Background())
	before := runtime.NumGoroutine()
	for range 100 {
		b.AddOnClose(func() {})
	}
	assert.Less(t, runtime.NumGoroutine()-before, 10, "each pending callback holds a goroutine")
	require.NoError(t, b.Close())
}

func TestBase_ParentCancelRunsCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := NewBaseWithCtx(ctx)
	called := make(chan struct{})
	b.AddOnClose(func() { close(called) })

	cancel()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("parent cancellation did not run the close callback")
	}
}

func TestBase_CloseInsideCallbackDoesNotDeadlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := NewBaseWithCtx(ctx)
	called := make(chan struct{})
	b.AddOnClose(func() {
		b.Close()
		close(called)
	})

	go cancel() // parent cancellation path
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("Close called from a close callback deadlocked")
	}

	b2 := NewBaseWithCtx(context.Background())
	b2.AddOnClose(func() { b2.Close() })
	closed := make(chan struct{})
	go func() {
		b2.Close() // Close path
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close called from a close callback deadlocked")
	}
}

func TestBase_AddOnCloseAfterCloseStillRuns(t *testing.T) {
	b := NewBaseWithCtx(context.Background())
	require.NoError(t, b.Close())

	called := make(chan struct{})
	b.AddOnClose(func() { close(called) })
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("callback registered after Close never ran")
	}
}

// RC3: Linker.Close closes every intent and interest and clears its maps before returning.
func TestLinker_CloseIsSynchronous(t *testing.T) {
	ctx := context.Background()
	linker := NewLinker(ctx, slog.Default(), 1,
		func(Intent) error { return nil },
		func(Interest) error { return nil },
		nil,
	)
	var intents []Intent
	var interests []Interest
	for _, path := range []string{"a", "b", "c"} {
		route, err := NewRoute(path, &testtypes.Foo{})
		require.NoError(t, err)
		intent, err := linker.AddIntent(route)
		require.NoError(t, err)
		interest, err := linker.AddInterest(route)
		require.NoError(t, err)
		intents = append(intents, intent)
		interests = append(interests, interest)
	}

	require.NoError(t, linker.Close())

	linker.mu.Lock()
	assert.Empty(t, linker.intents)
	assert.Empty(t, linker.interests)
	assert.Empty(t, linker.links)
	linker.mu.Unlock()
	for _, i := range intents {
		assert.Error(t, i.(*LocalIntent).Ctx().Err(), "intent still open after Linker.Close")
	}
	for _, i := range interests {
		assert.Error(t, i.(*LocalInterest).Ctx().Err(), "interest still open after Linker.Close")
	}
}

// RC1: closing a LocalIntent must not race Link/Notify (send on a closing channel).
func TestLocalIntent_CloseConcurrentWithLinkAndNotify(t *testing.T) {
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	for range 50 {
		intent := NewIntent(context.Background(), route, 1)
		msgC := make(chan proto.Message, 1)
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); intent.Notify() }()
		go func() { defer wg.Done(); intent.Link(msgC) }()
		go func() { defer wg.Done(); intent.Close() }()
		wg.Wait()
		intent.Close()
		assert.Nil(t, intent.LinkedC(), "closed intent keeps a linked channel")
	}
}

// RC2: router callbacks must be in place before the runner goroutines start.
func TestRouters_ConstructWithPendingNotifications(t *testing.T) {
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for range 20 {
		intent := NewIntent(ctx, route, 1)
		intent.Notify() // a notification is already queued when the router starts
		ir, err := NewIntentRouter(ctx, route, 1, intent)
		require.NoError(t, err)

		interest := NewInterest(ctx, route, 1)
		interest.MsgC() <- &testtypes.Foo{} // a message is already queued when the router starts
		nr, err := NewInterestRouter(ctx, route, 1, interest)
		require.NoError(t, err)

		require.NoError(t, ir.Close())
		require.NoError(t, nr.Close())
		require.NoError(t, intent.Close())
		require.NoError(t, interest.Close())
	}
}

// DND-004 follow-on: adding to a closed fan-out must be refused. Otherwise its
// WaitGroup.Add races the close path's Wait and a runner starts on a closed router.
func TestRouters_AddAfterCloseRefused(t *testing.T) {
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	ctx := context.Background()

	ir, err := NewIntentRouter(ctx, route, 1)
	require.NoError(t, err)
	require.NoError(t, ir.Close())
	intent := NewIntent(ctx, route, 1)
	assert.ErrorIs(t, ir.AddIntent(intent), errors.ErrClosed)
	assert.Empty(t, ir.intents)

	nr, err := NewInterestRouter(ctx, route, 1)
	require.NoError(t, err)
	require.NoError(t, nr.Close())
	interest := NewInterest(ctx, route, 1)
	assert.ErrorIs(t, nr.AddInterest(interest), errors.ErrClosed)
	assert.Empty(t, nr.interests)

	require.NoError(t, intent.Close())
	require.NoError(t, interest.Close())
}
