package bus

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An attacker on the response path sees one reply, predicts the Caller's next nonce and
// injects a forged reply while that call is pending. With the counter source the forgery
// is accepted (residual risk, DND-005); with per-call random sources it is dropped.
func TestCallerPredictedNonceForgery(t *testing.T) {
	cases := []struct {
		name   string
		source func() NonceSource
		forged bool
	}{
		{"counter", func() NonceSource { return NewCounterNonce() }, true},
		{"random", func() NonceSource { return RandomNonce{} }, false},
		{"crypto", func() NonceSource { return &CryptoNonce{} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			router := newTestRouter(t, ctx)

			service, err := NewService[*types.Foo, *types.Foo](ctx, router, "forge.req", "forge.resp")
			require.NoError(t, err)
			defer service.Close()
			secondArrived := make(chan struct{})
			release := make(chan struct{})
			var calls atomic.Int32
			go service.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Foo) error) error { //nolint:errcheck
				if calls.Add(1) == 2 {
					close(secondArrived)
					<-release
				}
				return reply(&types.Foo{Text: req.Text})
			})

			opts := append(nonceOptions(), WithCallerNonceSource[*types.Foo, *types.Foo](tc.source()))
			caller, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "forge.req", "forge.resp", opts...)
			require.NoError(t, err)
			defer caller.Close()

			// The attacker watches and writes the response path.
			spy, err := NewConsumer[*types.Foo](ctx, router, "forge.resp")
			require.NoError(t, err)
			defer spy.Close()
			forger, err := NewProducer[*types.Foo](ctx, router, "forge.resp")
			require.NoError(t, err)
			defer forger.Close()

			_, err = caller.CallWithTimeout(ctx, msg("", "one"), time.Second)
			require.NoError(t, err)
			observed, err := spy.Receive(ctx)
			require.NoError(t, err)
			nonce, _ := splitMsg(observed)
			n, err := strconv.ParseUint(nonce, 10, 64)
			require.NoError(t, err)

			type result struct {
				resp *types.Foo
				err  error
			}
			second := make(chan result, 1)
			go func() {
				resp, err := caller.CallWithTimeout(ctx, msg("", "two"), 2*time.Second)
				second <- result{resp, err}
			}()
			<-secondArrived // the second call's waiter is registered

			require.NoError(t, forger.Send(ctx, msg(strconv.FormatUint(n+1, 10), "forged")))
			for { // once the spy has the forged reply, the Caller's queue has it too
				m, err := spy.Receive(ctx)
				require.NoError(t, err)
				if strings.HasSuffix(m.Text, ":forged") {
					break
				}
			}
			close(release)

			got := <-second
			require.NoError(t, got.err)
			_, payload := splitMsg(got.resp)
			if tc.forged {
				assert.Equal(t, "forged", payload, "counter nonces are predictable; the forgery should land")
			} else {
				assert.Equal(t, "two", payload, "a predicted-nonce forgery was accepted")
				assert.GreaterOrEqual(t, caller.Dropped(), uint64(1))
			}
		})
	}
}

// DND-005 acceptance 3: streaming replies keep their call identity, and a call that has
// ended cannot receive (or pass on) later replies addressed to it.
func TestCallerStreamingRepliesKeepCallIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	service, err := NewService[*types.Foo, *types.Foo](ctx, router, "stream2.req", "stream2.resp")
	require.NoError(t, err)
	defer service.Close()

	// The service hands each request's reply function to the test, which sends chunks in
	// a fixed interleaving.
	type pending struct {
		req   *types.Foo
		reply func(*types.Foo) error
	}
	arrived := make(chan pending, 2)
	done := make(chan struct{})
	go service.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Foo) error) error { //nolint:errcheck
		arrived <- pending{req, reply}
		<-done
		return nil
	})
	defer close(done)

	caller, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "stream2.req", "stream2.resp", nonceOptions()...)
	require.NoError(t, err)
	defer caller.Close()

	var mu sync.Mutex
	got := map[string][]string{}
	stop := errors.New("stop after the first chunk")
	for _, name := range []string{"a", "b"} {
		err := caller.HandleReplies(ctx, msg("", name), func(ctx context.Context, req *types.Foo, resp *types.Foo) error {
			_, payload := splitMsg(resp)
			mu.Lock()
			got[name] = append(got[name], payload)
			mu.Unlock()
			if name == "a" {
				return stop // call a ends after its first chunk
			}
			return nil
		})
		require.NoError(t, err)
	}

	byName := map[string]pending{}
	for range 2 {
		p := <-arrived
		_, name := splitMsg(p.req)
		byName[name] = p
	}
	chunk := func(name, payload string) {
		nonce, _ := splitMsg(byName[name].req)
		require.NoError(t, byName[name].reply(msg(nonce, payload)))
	}

	chunk("a", "a1")
	chunk("b", "b1")
	require.Eventually(t, func() bool { // call a has ended and unregistered
		caller.mu.Lock()
		defer caller.mu.Unlock()
		return len(caller.waiters) == 1
	}, time.Second, time.Millisecond)
	chunk("a", "a2")
	chunk("b", "b2")
	chunk("a", "a3")
	chunk("b", "b3")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got["b"]) >= 3
	}, time.Second, time.Millisecond)
	time.Sleep(50 * time.Millisecond) // let any misrouted chunk arrive

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"a1"}, got["a"])
	assert.Equal(t, []string{"b1", "b2", "b3"}, got["b"], "call b received another call's chunk")
	assert.EqualValues(t, 2, caller.Dropped(), "a2 and a3 belong to an ended call")
}

// Per-call cost of each nonce source (DND-005 nonce generator; embedded overhead).
func BenchmarkNonceSource(b *testing.B) {
	sources := []struct {
		name string
		src  NonceSource
	}{
		{"counter", NewCounterNonce()},
		{"random", RandomNonce{}},
		{"crypto", &CryptoNonce{}},
	}
	for _, s := range sources {
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			var sink uint64
			for b.Loop() {
				sink += s.src.Next()
			}
			_ = sink
		})
	}
}
