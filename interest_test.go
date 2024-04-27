package dndm

import (
	"context"
	"testing"
	"time"

	"github.com/itohio/dndm/testutil"
	testtypes "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	interest := NewInterest(ctx, route, 10)
	require.NotNil(t, interest)
	onClose := testutil.NewFunc(ctx)
	interest.OnClose(nil)
	interest.OnClose(onClose.F)
	onClose.NotCalled(t)
	assert.Equal(t, route, interest.Route())
	assert.Equal(t, 10, cap(interest.MsgC()))
	assert.NotNil(t, interest.C())

	onClose.NotCalled(t)
	require.NoError(t, interest.Close())
	onClose.WaitCalled(t)
	assert.True(t, testutil.CtxRecv(ctx, interest.C()))
}

func TestLocalInterest_ConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	interest := NewInterest(ctx, route, 1000)

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
		<-done1
		interest.Close()
		close(done2)
	}()

	assert.True(t, testutil.CtxRecv(ctx, done1))
	assert.True(t, testutil.CtxRecv(ctx, done2))
	require.Equal(t, context.Canceled, interest.Ctx().Err())
}

// ========== Interest router tests ==========

func TestInterestRouter_CreationAndOperation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	// // Mocking Interest for testing
	mockInterest := &MockInterest{}
	mockInterest.On("Route").Return(route)
	ch := make(chan proto.Message, 10)
	mockInterest.On("C").Return((<-chan proto.Message)(ch)) // Assuming a channel is returned here

	router, err := NewInterestRouter(ctx, route, 10, mockInterest)
	require.NoError(t, err)
	require.NotNil(t, router)
	assert.Equal(t, router.Route(), route)

	router.OnClose(nil)
	onClose := testutil.NewFunc(ctx)
	router.OnClose(onClose.F)

	w := router.Wrap()
	assert.Equal(t, router, w.router)
	assert.Equal(t, router.size, cap(w.c))
	assert.Equal(t, w.Route(), route)

	// Testing RemoveInterest
	router.RemoveInterest(mockInterest)
	// Ensure that interest is no longer in the router
	assert.NotContains(t, router.interests, mockInterest)

	assert.NoError(t, router.Close())

	onClose.WaitCalled(t)

	mockInterest.AssertExpectations(t)
}

func TestInterestWrapperOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	router, _ := NewInterestRouter(ctx, route, 10)
	wrapper := router.Wrap()

	// Test route retrieval and close operation
	assert.Equal(t, route, wrapper.Route())
	assert.Contains(t, router.wrappers, wrapper)
	assert.NoError(t, wrapper.Close())

	// Ensure the channel is properly closed after wrapper.Close()
	_, ok := <-wrapper.C()
	assert.False(t, ok)
	assert.NotContains(t, router.wrappers, wrapper)
}

func TestInterestRouter_Close(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)
	onClose := testutil.NewFunc(ctx)
	router, _ := NewInterestRouter(ctx, route, 10)
	router.OnClose(onClose.F)
	assert.NoError(t, router.Close())

	onClose.WaitCalled(t)

	// Ensure the main message channel is closed
	assert.True(t, testutil.CtxRecv(ctx, router.C()))
}

func TestInterestRouter_MessageRouting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	router, _ := NewInterestRouter(ctx, route, 10)

	mockInterest1 := &MockInterest{}
	mockInterest1.On("Route").Return(route)
	ch1 := make(chan proto.Message, 10)
	mockInterest1.On("C").Return((<-chan proto.Message)(ch1)) // Assuming a channel is returned here

	msg1 := &testtypes.Foo{}

	mockInterest2 := &MockInterest{}
	mockInterest2.On("Route").Return(route)
	ch2 := make(chan proto.Message, 10)
	mockInterest2.On("C").Return((<-chan proto.Message)(ch2)) // Assuming a channel is returned here

	msg2 := &testtypes.Foo{Text: "msg2"}

	t.Log("Register 1st wrapper")
	w1 := router.Wrap()
	w1Done := make(chan struct{})
	go func() {
		receivedMsg := []proto.Message{<-w1.C(), <-w1.C()}
		assert.Contains(t, receivedMsg, msg1)
		assert.Contains(t, receivedMsg, msg2)
		close(w1Done)
	}()

	t.Log("register interest")
	router.AddInterest(mockInterest1)
	router.AddInterest(mockInterest2)

	t.Log("register second wrapper")
	w2 := router.Wrap()
	w2Done := make(chan struct{})
	go func() {
		receivedMsg := []proto.Message{<-w2.C(), <-w2.C()}
		assert.Contains(t, receivedMsg, msg1)
		assert.Contains(t, receivedMsg, msg2)
		close(w2Done)
	}()

	t.Log("Simulate message reception and routing")
	go func() {
		ch1 <- msg1
		ch2 <- msg2
	}()

	testutil.CtxRecv(ctx, w1Done)
	testutil.CtxRecv(ctx, w2Done)
	w2.Close()
	assert.False(t, testutil.IsClosed(router.Ctx().Done()))
	w1.Close()
	assert.True(t, testutil.CtxRecv(ctx, router.Ctx().Done()))
	<-router.ctx.Done()
	assert.Equal(t, context.Canceled, router.ctx.Err())
	mockInterest1.AssertExpectations(t)
	mockInterest2.AssertExpectations(t)
}
