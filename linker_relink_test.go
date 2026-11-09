package dndm

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review R-DND-R8 N1: an old match's close must forget only itself. Sequence: the old
// match is dropped from the map (RemoveIntent does this, then releases the lock), a new
// intent for the same route is matched, and only then does the old match's close
// callback run. If that callback deletes whatever is stored for the route, the new match
// becomes untracked, and closing the interest closes its channel under the new intent's
// sends (send on closed channel).
func TestLinker_OldMatchCloseKeepsNewMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	route, err := NewRoute("rematch", &testtypes.Foo{})
	require.NoError(t, err)
	l := NewLinker(ctx, slog.Default(), 1, func(Intent) error { return nil }, func(Interest) error { return nil }, nil)
	defer l.Close()
	_, err = l.AddIntent(route)
	require.NoError(t, err)
	interest, err := l.AddInterest(route)
	require.NoError(t, err)

	l.mu.Lock()
	old := l.links[route.ID()]
	require.NotNil(t, old)
	oldClosed := make(chan struct{})
	go func() { // the old match closes; its linker callback waits for l.mu
		old.Close()
		close(oldClosed)
	}()
	// Meanwhile, as RemoveIntent does: forget the old match and intent, then add a new
	// intent for the same route, which is matched with the interest.
	delete(l.links, route.ID())
	delete(l.intents, route.ID())
	newIntent, err := l.addIntentLocked(route, func(ii IntentInternal) (IntentInternal, error) { return ii, nil })
	require.NoError(t, err)
	require.NotNil(t, l.links[route.ID()])
	l.mu.Unlock()
	<-oldClosed

	l.mu.Lock()
	_, tracked := l.links[route.ID()]
	l.mu.Unlock()
	assert.True(t, tracked, "the old match's close forgot the new match")

	// Closing the interest must unlink the new intent before its channel closes.
	done := make(chan error, 1)
	go func() {
		for {
			if err := sendRecover(ctx, newIntent, &testtypes.Foo{Text: "x"}); err != nil {
				done <- err
				return
			}
		}
	}()
	go func() {
		for range interest.C() {
		}
	}()
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, interest.Close())
	select {
	case err := <-done:
		assert.False(t, strings.HasPrefix(err.Error(), "panic:"), "send raced the interest close: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("the new intent kept sending after the interest closed")
	}
}
