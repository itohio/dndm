package bus

import (
	"context"
	"testing"
	"time"

	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// constNonce is a broken NonceSource: it always returns the same value.
type constNonce uint64

func (c constNonce) Next() uint64 { return uint64(c) }

// Verify V-DND-R5 F1: a NonceSource that returns 0 or a nonce still in use must make the
// call fail with ErrNonceSource, not spin under the Caller's lock and freeze every call.
func TestCallerRejectsBrokenNonceSource(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		caller, err := NewCaller[*types.Foo, *types.Foo](ctx, newTestRouter(t, ctx), "nz.req", "nz.resp",
			WithCallerNonceSource[*types.Foo, *types.Foo](constNonce(0)))
		require.NoError(t, err)
		defer caller.Close()

		done := make(chan error, 1)
		go func() {
			_, err := caller.CallWithTimeout(ctx, &types.Foo{}, 200*time.Millisecond)
			done <- err
		}()
		select {
		case err := <-done:
			assert.ErrorIs(t, err, ErrNonceSource)
		case <-time.After(2 * time.Second):
			t.Fatal("Caller froze on a nonce source that returns 0")
		}
	})

	t.Run("repeated", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		caller, err := NewCaller[*types.Foo, *types.Foo](ctx, newTestRouter(t, ctx), "nr.req", "nr.resp",
			WithCallerNonceSource[*types.Foo, *types.Foo](constNonce(7)))
		require.NoError(t, err)
		defer caller.Close()

		first := make(chan error, 1)
		go func() { // holds nonce 7 until its timeout: nobody serves the request path
			_, err := caller.CallWithTimeout(ctx, &types.Foo{}, 300*time.Millisecond)
			first <- err
		}()
		require.Eventually(t, func() bool {
			caller.mu.Lock()
			defer caller.mu.Unlock()
			return len(caller.waiters) == 1
		}, time.Second, time.Millisecond)

		second := make(chan error, 1)
		go func() {
			_, err := caller.CallWithTimeout(ctx, &types.Foo{}, 300*time.Millisecond)
			second <- err
		}()
		for name, ch := range map[string]chan error{"second": second, "first": first} {
			select {
			case err := <-ch:
				if name == "second" {
					assert.ErrorIs(t, err, ErrNonceSource, "the second call needs a fresh nonce")
				} else {
					assert.Error(t, err, "the first call times out normally")
					assert.NotErrorIs(t, err, ErrNonceSource)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("the %s call froze: the Caller spins on a nonce in use", name)
			}
		}
	})
}
