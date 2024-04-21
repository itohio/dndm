package dndm

import (
	"context"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIntent(t *testing.T) {
	ctx := context.Background()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	called := false
	closer := func() error {
		called = true
		return nil
	}
	intent := NewIntent(ctx, route, 10, closer)
	require.NotNil(t, intent)
	onCloseCalled := make(chan struct{})
	intent.OnClose(func() {
		close(onCloseCalled)
	})
	assert.Equal(t, route, intent.Route())
	assert.NotNil(t, intent.Interest())
	assert.Nil(t, intent.LinkedC())

	assert.False(t, called)
	assert.Equal(t, recvChan1(onCloseCalled, time.Millisecond), context.DeadlineExceeded)
	require.NoError(t, intent.Close())
	assert.True(t, called)
	assert.Equal(t, recvChan1(onCloseCalled, time.Millisecond), nil)
}

func TestLocalIntent_Send(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	intent := NewIntent(ctx, route, 10, nil)

	err = intent.Send(ctx, &testtypes.Foo{Text: "Something"})
	require.Equal(t, err, errors.ErrNoInterest)

	msgC := make(chan proto.Message)
	intent.Link(msgC)

	notifyDone := make(chan struct{})
	go func() {
		n := <-intent.Interest()
		assert.Equal(t, n, route)
		err = intent.Send(ctx, &testtypes.Foo{Text: "Something"})
		require.NoError(t, err)
		close(notifyDone)
	}()

	recvDone := make(chan struct{})
	go func() {
		msg := <-msgC
		assert.NotNil(t, msg)
		intent.Close()
		close(recvDone)
	}()

	intent.Notify()

	<-notifyDone
	<-recvDone
	<-intent.Ctx().Done()
	require.Equal(t, context.Canceled, intent.Ctx().Err())
	intent.Close()
}

func TestLocalIntent_Link(t *testing.T) {
	ctx := context.Background()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	intent := NewIntent(ctx, route, 10, nil)
	msgC := make(chan proto.Message)
	msgC1 := make(chan proto.Message)

	intent.Link(msgC)
	assert.Panics(t, func() { intent.Link(msgC1) }) // Linking twice should panic if already linked
}
