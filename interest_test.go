package dndm

import (
	"context"
	"testing"
	"time"

	testtypes "github.com/itohio/dndm/types/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recvChan[T any](c <-chan T, t time.Duration) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()
	select {
	case <-ctx.Done():
	case v := <-c:
		return v, nil
	}
	var z T
	return z, ctx.Err()
}

func recvChan1[T any](c <-chan T, t time.Duration) error {
	_, err := recvChan(c, t)
	return err
}

func sendChan[T any](c chan<- T, v T, t time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()
	select {
	case <-ctx.Done():
	case c <- v:
		return nil
	}
	return ctx.Err()
}

func TestNewInterest(t *testing.T) {
	ctx := context.Background()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	called := false
	closer := func() error {
		called = true
		return nil
	}
	interest := NewInterest(ctx, route, 10, closer)
	require.NotNil(t, interest)
	onCloseCalled := make(chan struct{})
	interest.OnClose(func() {
		close(onCloseCalled)
	})
	assert.Equal(t, route, interest.Route())
	assert.Equal(t, 10, cap(interest.MsgC()))
	assert.NotNil(t, interest.C())

	assert.False(t, called)
	assert.Equal(t, context.DeadlineExceeded, recvChan1(onCloseCalled, time.Millisecond))
	require.NoError(t, interest.Close())
	assert.True(t, called)
	assert.Equal(t, nil, recvChan1(onCloseCalled, time.Millisecond))
	assert.Nil(t, interest.C())
}

func TestLocalInterest_ConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	interest := NewInterest(ctx, route, 10, nil)

	done1 := make(chan bool)
	done2 := make(chan bool)

	// Concurrent sending to MsgC
	go func() {
		for i := 0; i < 100; i++ {
			select {
			case <-interest.Ctx().Done():
				close(done1)
				return
			case interest.MsgC() <- &testtypes.Foo{Text: "Assume this is correctly constructed"}:
			}
		}
		close(done1)
	}()

	// Concurrent closing
	go func() {
		interest.Close()
		close(done2)
	}()

	<-done1
	<-done2
	require.Equal(t, context.Canceled, interest.Ctx().Err())
}
