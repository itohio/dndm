package dndm

import (
	"context"
	"testing"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/testutil"
	testtypes "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewIntent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	intent := NewIntent(ctx, route, 10)
	require.NotNil(t, intent)
	assert.Equal(t, intent.Route(), route)
	onClose := testutil.NewFunc(ctx, t, "close intent")
	intent.OnClose(onClose.F)
	intent.OnClose(nil)
	assert.Equal(t, route, intent.Route())
	assert.NotNil(t, intent.Interest())
	assert.Nil(t, intent.LinkedC())

	onClose.NotCalled()
	require.NoError(t, intent.Close())
	onClose.WaitCalled()
}

func TestLocalIntent_Send(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	intent := NewIntent(ctx, route, 10)

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

	assert.True(t, testutil.CtxRecv(ctx, notifyDone))
	assert.True(t, testutil.CtxRecv(ctx, recvDone))
	<-intent.Ctx().Done()
	require.Equal(t, context.Canceled, intent.Ctx().Err())
	intent.Close()
}

func TestLocalIntent_Link(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	intent := NewIntent(ctx, route, 10)
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

	mockIntent := &MockIntent{}
	mockIntent.On("Route").Return(route)
	ch := make(chan Route, 10)
	mockIntent.On("Interest").Return((<-chan Route)(ch)) // Assuming a channel is returned here

	router, err := NewIntentRouter(ctx, route, 10, mockIntent)
	require.NoError(t, err)
	require.NotNil(t, router)
	assert.Contains(t, router.intents, mockIntent)
	assert.Equal(t, router.Route(), route)

	router.OnClose(nil)
	onClose := testutil.NewFunc(ctx, t, "close router")
	router.OnClose(onClose.F)

	w := router.Wrap()
	assert.Equal(t, router, w.router)
	assert.Equal(t, router.size, cap(w.notifyC))
	assert.Equal(t, w.Route(), route)
	time.Sleep(time.Millisecond * 10)
	router.RemoveIntent(mockIntent)
	assert.NotContains(t, router.intents, mockIntent)

	assert.NoError(t, router.Close())

	onClose.WaitCalled()

	mockIntent.AssertExpectations(t)
}

func TestIntentRouter_WrapAndSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	router, err := NewIntentRouter(ctx, route, 10)
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

	router.Close()

	mockIntent1.AssertExpectations(t)
	mockIntent2.AssertExpectations(t)
}

func TestIntentRouter_Close(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	router, err := NewIntentRouter(ctx, route, 10)
	require.NoError(t, err)
	onClose := testutil.NewFunc(ctx, t, "close router")
	router.OnClose(onClose.F)
	onClose.NotCalled()
	assert.NoError(t, router.Close())
	onClose.WaitCalled()
}

func TestIntentRouter_NotifyWrappers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	router, err := NewIntentRouter(ctx, route, 10)
	require.NoError(t, err)
	onClose := testutil.NewFunc(ctx, t, "close router")
	router.OnClose(onClose.F)

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
	onW1Close := testutil.NewFunc(ctx, t, "close wrapper 1")
	w1.OnClose(onW1Close.F)
	w1.OnClose(func() { t.Log("w1 OnClose") })

	w1Done := make(chan struct{})
	go func() {
		receivedNotification := []Route{<-w1.Interest(), <-w1.Interest()}
		assert.Contains(t, receivedNotification, route)
		assert.Contains(t, receivedNotification, route)
		close(w1Done)
	}()

	t.Log("register intent")
	router.AddIntent(mockIntent2)

	t.Log("register 2nd wrapper")
	w2 := router.Wrap()
	onW2Close := testutil.NewFunc(ctx, t, "close wrapper 2")
	w2.OnClose(onW2Close.F)
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

	go func() {
		select {
		case <-ctx.Done():
			t.Log("timeout")
		case <-router.Ctx().Done():
			t.Log("router ctx done")
		}
	}()

	assert.True(t, testutil.CtxRecv(ctx, w1Done))
	assert.True(t, testutil.CtxRecv(ctx, w2Done))
	assert.Len(t, router.wrappers, 2)
	w2.Close()
	assert.False(t, testutil.IsClosed(router.Ctx().Done()))
	onClose.NotCalled()
	onW2Close.WaitCalled()
	time.Sleep(time.Millisecond) // NOTE: Allow for removal to complete
	assert.Len(t, router.wrappers, 1)
	w1.Close()
	onW1Close.WaitCalled()
	time.Sleep(time.Millisecond) // NOTE: Allow for removal to complete
	assert.Len(t, router.wrappers, 0)
	t.Log("Before WaitCalled")
	onClose.WaitCalled()
	t.Log("After WaitCalled")
	assert.True(t, testutil.CtxRecv(ctx, router.Ctx().Done()))
	<-router.Ctx().Done()
	assert.Equal(t, context.Canceled, router.ctx.Err())
	mockIntent1.AssertExpectations(t)
	mockIntent2.AssertExpectations(t)
}
