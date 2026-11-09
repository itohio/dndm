package bus

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DND-005 tests. Messages are Foo{Text: "<nonce>:<payload>"}; an empty nonce means "no nonce".
// The nonce needs all 64 bits, so it travels as decimal text.

func msg(nonce, payload string) *types.Foo { return &types.Foo{Text: nonce + ":" + payload} }

func splitMsg(m *types.Foo) (nonce, payload string) {
	nonce, payload, _ = strings.Cut(m.Text, ":")
	return nonce, payload
}

func nonceOptions() []CallerOption[*types.Foo, *types.Foo] {
	return []CallerOption[*types.Foo, *types.Foo]{
		WithCallerRequestNonce[*types.Foo, *types.Foo](func(r *types.Foo, n uint64) {
			_, payload := splitMsg(r)
			r.Text = strconv.FormatUint(n, 10) + ":" + payload
		}),
		WithCallerResponseNonce[*types.Foo, *types.Foo](func(r *types.Foo) (uint64, bool) {
			nonce, _ := splitMsg(r)
			n, err := strconv.ParseUint(nonce, 10, 64)
			return n, err == nil
		}),
	}
}

// Two Callers on the same request/response paths each get their own reply. The service
// waits until both requests arrived and replies to the first caller first, so the
// second caller's waiter is already registered when the first caller's reply reaches it.
func TestCallersSharingResponsePathGetOwnReplies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)

	service, err := NewService[*types.Foo, *types.Foo](ctx, router, "shared.req", "shared.resp")
	require.NoError(t, err)
	defer service.Close()

	var mu sync.Mutex
	arrived := 0
	both := make(chan struct{})
	firstSent := make(chan struct{})
	go service.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Foo) error) error { //nolint:errcheck
		mu.Lock()
		arrived++
		if arrived == 2 {
			close(both)
		}
		mu.Unlock()
		<-both
		_, payload := splitMsg(req)
		if payload == "two" {
			<-firstSent
		}
		err := reply(&types.Foo{Text: req.Text})
		if payload == "one" {
			close(firstSent)
		}
		return err
	})

	c1, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "shared.req", "shared.resp", nonceOptions()...)
	require.NoError(t, err)
	defer c1.Close()
	c2, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "shared.req", "shared.resp", nonceOptions()...)
	require.NoError(t, err)
	defer c2.Close()

	payloads := []string{"one", "two"}
	results := make([]*types.Foo, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, c := range []*Caller[*types.Foo, *types.Foo]{c1, c2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = c.CallWithTimeout(ctx, msg("", payloads[i]), 2*time.Second)
		}()
	}
	wg.Wait()

	for i := range results {
		require.NoError(t, errs[i], "caller %d", i+1)
		_, got := splitMsg(results[i])
		assert.Equal(t, payloads[i], got, "caller %d received another caller's reply", i+1)
	}
}

type fooService = Service[*types.Foo, *types.Foo]

// echoService answers every request with transform(request).
func echoService(ctx context.Context, svc *fooService, transform func(*types.Foo) *types.Foo) {
	go svc.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Foo) error) error { //nolint:errcheck
		return reply(transform(req))
	})
}

// A reply whose nonce belongs to no call of this Caller is dropped and counted, never
// handed to an unrelated waiter.
func TestCallerDropsReplyWithForeignNonce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	service, err := NewService[*types.Foo, *types.Foo](ctx, router, "foreign.req", "foreign.resp")
	require.NoError(t, err)
	defer service.Close()
	echoService(ctx, service, func(req *types.Foo) *types.Foo {
		nonce, payload := splitMsg(req)
		n, _ := strconv.ParseUint(nonce, 10, 64)
		return msg(strconv.FormatUint(n+1000, 10), payload)
	})

	caller, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "foreign.req", "foreign.resp", nonceOptions()...)
	require.NoError(t, err)
	defer caller.Close()

	_, err = caller.CallWithTimeout(ctx, msg("", "x"), 300*time.Millisecond)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "a reply with a foreign nonce was delivered")
	assert.EqualValues(t, 1, caller.Dropped())
}

// With a response-nonce hook configured, a reply without a nonce is dropped and counted.
func TestCallerMissingNoncePolicy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	service, err := NewService[*types.Foo, *types.Foo](ctx, router, "missing.req", "missing.resp")
	require.NoError(t, err)
	defer service.Close()
	echoService(ctx, service, func(req *types.Foo) *types.Foo {
		_, payload := splitMsg(req)
		return msg("", payload)
	})

	caller, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "missing.req", "missing.resp", nonceOptions()...)
	require.NoError(t, err)
	defer caller.Close()

	_, err = caller.CallWithTimeout(ctx, msg("", "x"), 300*time.Millisecond)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "a reply without a nonce was delivered")
	assert.EqualValues(t, 1, caller.Dropped())
}

// A reply that arrives after its call timed out is dropped; it must not complete the next call.
func TestCallerLateReplyDoesNotReachNextCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	service, err := NewService[*types.Foo, *types.Foo](ctx, router, "late.req", "late.resp")
	require.NoError(t, err)
	defer service.Close()

	var mu sync.Mutex
	// held is the first request; it is answered only together with the second.
	var held *types.Foo

	go service.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Foo) error) error { //nolint:errcheck
		mu.Lock()
		defer mu.Unlock()
		if held == nil {
			held = req
			return nil
		}
		if err := reply(&types.Foo{Text: held.Text}); err != nil { // late reply to call 1
			return err
		}
		return reply(&types.Foo{Text: req.Text})
	})

	caller, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "late.req", "late.resp", nonceOptions()...)
	require.NoError(t, err)
	defer caller.Close()

	_, err = caller.CallWithTimeout(ctx, msg("", "first"), 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	resp, err := caller.CallWithTimeout(ctx, msg("", "second"), time.Second)
	require.NoError(t, err)
	_, got := splitMsg(resp)
	assert.Equal(t, "second", got, "the timed-out call's late reply completed the next call")
	assert.EqualValues(t, 1, caller.Dropped())
}

// Without nonce hooks a Caller keeps FIFO delivery (single-caller use).
func TestCallerWithoutHooksUsesFIFO(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	service, err := NewService[*types.Foo, *types.Foo](ctx, router, "fifo.req", "fifo.resp")
	require.NoError(t, err)
	defer service.Close()
	echoService(ctx, service, func(req *types.Foo) *types.Foo { return &types.Foo{Text: req.Text} })

	caller, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "fifo.req", "fifo.resp")
	require.NoError(t, err)
	defer caller.Close()

	for _, payload := range []string{"a", "b", "c"} {
		resp, err := caller.CallWithTimeout(ctx, msg("", payload), time.Second)
		require.NoError(t, err)
		_, got := splitMsg(resp)
		assert.Equal(t, payload, got)
	}
	assert.Zero(t, caller.Dropped())
}

// Nonces are caller-scoped: two Callers do not start from the same value.
func TestCallerNoncesDifferAcrossCallers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	c1, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "n.req", "n.resp", nonceOptions()...)
	require.NoError(t, err)
	defer c1.Close()
	c2, err := NewCaller[*types.Foo, *types.Foo](ctx, router, "n.req", "n.resp", nonceOptions()...)
	require.NoError(t, err)
	defer c2.Close()

	_, n1, err := c1.registerWaiter(false)
	require.NoError(t, err)
	_, n2, err := c2.registerWaiter(false)
	require.NoError(t, err)
	assert.NotEqual(t, n1, n2)
	assert.NotZero(t, n1)
	assert.NotZero(t, n2)
}
