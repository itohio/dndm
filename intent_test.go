package dndm

import (
	"context"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	testtypes "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
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
	assert.Equal(t, intent.Route(), route)
	onCloseCalled := make(chan struct{})
	intent.OnClose(func() {
		close(onCloseCalled)
	})
	intent.OnClose(nil)
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

	ctxRecv(ctx, notifyDone)
	ctxRecv(ctx, recvDone)
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

// ========== Intent router tests ==========

func TestIntentRouter_Creation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	closer := func() error { return nil }

	mockIntent := &MockIntent{}
	mockIntent.On("Route").Return(route)
	ch := make(chan Route, 10)
	mockIntent.On("Interest").Return((<-chan Route)(ch)) // Assuming a channel is returned here

	router, err := NewIntentRouter(ctx, route, closer, 10, mockIntent)
	require.NoError(t, err)
	require.NotNil(t, router)
	assert.Contains(t, router.intents, mockIntent)
	assert.Equal(t, router.Route(), route)

	router.OnClose(nil)
	onCloseCalled := make(chan struct{})
	router.OnClose(func() { close(onCloseCalled) })

	w := router.Wrap()
	assert.Equal(t, router, w.router)
	assert.Equal(t, router.size, cap(w.notifyC))
	assert.Equal(t, w.Route(), route)

	router.RemoveIntent(mockIntent)
	assert.NotContains(t, router.intents, mockIntent)

	assert.NoError(t, router.Close())

	ctxRecv(ctx, onCloseCalled)

	mockIntent.AssertExpectations(t)
}

func TestIntentRouter_WrapAndSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	closer := func() error { return nil }

	router, err := NewIntentRouter(ctx, route, closer, 10)
	require.NoError(t, err)

	w1 := router.Wrap()

	mockIntent1 := &MockIntent{}
	mockIntent1.On("Route").Return(route)
	mockIntent1.On("Send", mock.Anything, mock.Anything).Return(nil).Times(2)
	ch1 := make(chan Route, 10)
	mockIntent1.On("Interest").Return((<-chan Route)(ch1)) // Assuming a channel is returned here
	router.AddIntent(mockIntent1)

	w2 := router.Wrap()

	mockIntent2 := &MockIntent{}
	mockIntent2.On("Route").Return(route)
	mockIntent2.On("Send", mock.Anything, mock.Anything).Return(nil).Times(2)
	ch2 := make(chan Route, 10)
	mockIntent2.On("Interest").Return((<-chan Route)(ch2)) // Assuming a channel is returned here
	router.AddIntent(mockIntent2)

	err = w1.Send(ctx, &testtypes.Foo{Text: "A"})
	assert.NoError(t, err)
	err = w2.Send(ctx, &testtypes.Foo{Text: "B"})
	assert.NoError(t, err)

	mockIntent1.AssertExpectations(t)
	mockIntent2.AssertExpectations(t)
}

func TestIntentRouter_Close(t *testing.T) {
	ctx := context.Background()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	closerCalled := false
	closer := func() error {
		closerCalled = true
		return nil
	}

	router, err := NewIntentRouter(ctx, route, closer, 10)
	require.NoError(t, err)
	err = router.Close()
	assert.NoError(t, err)
	assert.True(t, closerCalled)
}

func TestIntentRouter_NotifyWrappers(t *testing.T) {
	ctx := context.Background()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	closer := func() error { return nil }

	router, err := NewIntentRouter(ctx, route, closer, 10)
	require.NoError(t, err)

	mockIntent1 := &MockIntent{}
	mockIntent1.On("Route").Return(route)
	ch1 := make(chan Route, 10)
	mockIntent1.On("Interest").Return((<-chan Route)(ch1)) // Assuming a channel is returned here
	router.AddIntent(mockIntent1)

	mockIntent2 := &MockIntent{}
	mockIntent2.On("Route").Return(route)
	ch2 := make(chan Route, 10)
	mockIntent2.On("Interest").Return((<-chan Route)(ch2)) // Assuming a channel is returned here

	t.Log("Register 1st wrapper")
	w1 := router.Wrap()
	w1Done := make(chan struct{})
	go func() {
		receivedNotification := []Route{<-w1.Interest(), <-w1.Interest()}
		assert.Contains(t, receivedNotification, route)
		assert.Contains(t, receivedNotification, route)
		close(w1Done)
	}()

	t.Log("register intent")
	router.AddIntent(mockIntent2)

	t.Log("register second wrapper")
	w2 := router.Wrap()
	w2Done := make(chan struct{})
	go func() {
		receivedNotification := []Route{<-w2.Interest(), <-w2.Interest()}
		assert.Contains(t, receivedNotification, route)
		assert.Contains(t, receivedNotification, route)
		close(w2Done)
	}()

	t.Log("Simulate interest notification")
	go func() {
		ch1 <- route
		ch2 <- route
	}()

	ctxRecv(ctx, w1Done)
	ctxRecv(ctx, w2Done)
	w2.Close()
	w1.Close()
	<-router.ctx.Done()
	assert.Equal(t, context.Canceled, router.ctx.Err())
	mockIntent1.AssertExpectations(t)
	mockIntent2.AssertExpectations(t)
}
