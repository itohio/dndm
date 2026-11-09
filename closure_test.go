package dndm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// RC6: an interest's channel is closed while a linked intent is sending on it.

func newLinkedPair(t *testing.T, ctx context.Context, size int) (*Linker, Intent, Interest) {
	t.Helper()
	route, err := NewRoute("rc6", &testtypes.Foo{})
	require.NoError(t, err)
	l := NewLinker(ctx, slog.Default(), size,
		func(Intent) error { return nil },
		func(Interest) error { return nil },
		nil,
	)
	intent, err := l.AddIntent(route)
	require.NoError(t, err)
	interest, err := l.AddInterest(route)
	require.NoError(t, err)
	return l, intent, interest
}

// sendRecover runs Send and turns a panic ("send on closed channel") into an error.
func sendRecover(ctx context.Context, intent Intent, msg proto.Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return intent.Send(ctx, msg)
}

// A Send blocked on a full interest channel must return, not panic, when the interest closes.
func TestLinker_InterestCloseUnblocksSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, intent, interest := newLinkedPair(t, ctx, 1)
	defer l.Close()

	require.NoError(t, intent.Send(ctx, &testtypes.Foo{Text: "fills the buffer"}))
	result := make(chan error, 1)
	go func() { result <- sendRecover(ctx, intent, &testtypes.Foo{Text: "blocks"}) }()
	time.Sleep(20 * time.Millisecond) // let the second Send block on the full channel

	require.NoError(t, interest.Close())
	select {
	case err := <-result:
		require.Error(t, err, "Send cannot succeed: nobody reads the closed interest")
		assert.NotContains(t, err.Error(), "panic", "Send panicked on the closed interest channel")
	case <-time.After(2 * time.Second):
		t.Fatal("Send stayed blocked after the interest closed")
	}
}

// Many sends racing an interest close: no panic, and (under -race) no report.
func TestLinker_InterestCloseDuringSends(t *testing.T) {
	for range 50 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		l, intent, interest := newLinkedPair(t, ctx, 1)

		var wg sync.WaitGroup
		panics := make(chan error, 4)
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					err := sendRecover(ctx, intent, &testtypes.Foo{Text: "x"})
					if err != nil {
						if strings.HasPrefix(err.Error(), "panic:") {
							panics <- err
						}
						return
					}
				}
			}()
		}
		go func() { // a slow reader keeps senders moving
			for range interest.C() {
			}
		}()
		time.Sleep(time.Millisecond)
		require.NoError(t, interest.Close())
		wg.Wait()
		close(panics)
		for err := range panics {
			t.Fatalf("send raced the interest close: %v", err)
		}
		l.Close()
		cancel()
	}
}

// A fan-in's receive runners forward into its own channel; closing the fan-in must not
// close that channel under a runner.
func TestFanInInterest_CloseWhileForwarding(t *testing.T) {
	route, err := NewRoute("rc6.fanin", &testtypes.Foo{})
	require.NoError(t, err)
	for range 50 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		inner := NewInterest(ctx, route, 4)
		fanIn, err := NewFanInInterest(ctx, route, 1, inner)
		require.NoError(t, err)
		stop := make(chan struct{})
		fed := make(chan struct{})
		go func() { // keep the inner interest full so the runner is always forwarding
			defer close(fed)
			for {
				select {
				case <-stop:
					return
				case inner.MsgC() <- &testtypes.Foo{Text: "x"}:
				}
			}
		}()
		time.Sleep(time.Millisecond)
		require.NoError(t, fanIn.Close()) // a runner panic here crashes the test binary
		close(stop)
		<-fed
		inner.Close()
		cancel()
	}
}

// Cancelling the linker's context closes the link and the interest from two goroutines at
// once (context.AfterFunc); the interest must still not close its channel under a Send.
func TestLinker_ContextCancelDuringSends(t *testing.T) {
	for range 50 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lctx, lcancel := context.WithCancel(ctx)
		l, intent, interest := newLinkedPair(t, lctx, 1)

		var wg sync.WaitGroup
		panics := make(chan error, 4)
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if err := sendRecover(ctx, intent, &testtypes.Foo{Text: "x"}); err != nil {
						if strings.HasPrefix(err.Error(), "panic:") {
							panics <- err
						}
						return
					}
				}
			}()
		}
		go func() {
			for range interest.C() {
			}
		}()
		time.Sleep(time.Millisecond)
		lcancel()
		wg.Wait()
		close(panics)
		for err := range panics {
			t.Fatalf("send raced the interest close: %v", err)
		}
		l.Close()
		cancel()
	}
}
