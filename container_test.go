package dndm

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/itohio/dndm/testutil"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func Test_NewContainer(t *testing.T) {
	container := NewContainer("test", 10)
	require.NotNil(t, container)
	assert.Equal(t, "test", container.Name())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // a deadline only (R-DND-R8 N3)
	defer cancel()

	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	container.Init(ctx, slog.Default(),
		func(intent Intent, t Endpoint) error {
			return addIntent.FE(nil)()
		},
		func(interest Interest, t Endpoint) error {
			return addInterest.FE(nil)()
		},
	)

	container.OnClose(nil)
	onClose := testutil.NewFunc(ctx, t, "on close")
	container.OnClose(onClose.F)
	container.Close() // runs the close callbacks before it returns
	onClose.WaitCalled()
	addIntent.NotCalled()
	addInterest.NotCalled()
}

func TestContainer_AddEndpoint(t *testing.T) {
	container := NewContainer("test", 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // a deadline only (R-DND-R8 N3)
	defer cancel()

	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	container.Init(ctx, slog.Default(),
		func(intent Intent, t Endpoint) error {
			return addIntent.FE(nil)()
		},
		func(interest Interest, t Endpoint) error {
			return addInterest.FE(nil)()
		},
	)

	endpoint := &MockEndpoint{}
	endpoint.On("Init", container.Ctx(), mock.Anything, mock.Anything, mock.Anything).Return(nil)
	endpoint.On("Close").Return(nil)
	endpoint.On("OnClose", mock.Anything).Run(func(args mock.Arguments) {}).Return(endpoint)
	err := container.Add(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(container.endpoints))
	assert.Contains(t, container.endpoints, endpoint)
	container.Close() // closes its endpoints before it returns
	endpoint.AssertExpectations(t)
}

func TestContainer_RemoveEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	container := NewContainer("test", 10)
	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	container.Init(ctx, slog.Default(),
		func(intent Intent, t Endpoint) error {
			return addIntent.FE(nil)()
		},
		func(interest Interest, t Endpoint) error {
			return addInterest.FE(nil)()
		},
	)

	endpoint := new(MockEndpoint)
	endpoint.On("Init", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	endpoint.On("Close").Return(nil)
	endpoint.On("OnClose", mock.Anything).Run(func(args mock.Arguments) {}).Return(endpoint)

	container.Add(endpoint)
	err := container.Remove(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(container.endpoints))
}

func TestContainer_Publish(t *testing.T) {
	// A deadline, not a timing assumption: the waits below return as soon as events happen.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	container := NewContainer("test", 10)
	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	container.Init(ctx, slog.Default(),
		func(intent Intent, t Endpoint) error {
			return addIntent.FE(nil)()
		},
		func(interest Interest, t Endpoint) error {
			return addInterest.FE(nil)()
		},
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	mockIntent := new(MockIntent)
	mockIntent.On("Route").Return(route)
	mockIntent.On("OnClose", mock.Anything).Return(mockIntent)
	ch := make(chan Route)
	read := make(chan struct{}) // closed once the fan-out's notify runner reads Interest()
	var readOnce sync.Once
	mockIntent.On("Interest").Run(func(mock.Arguments) { readOnce.Do(func() { close(read) }) }).Return((<-chan Route)(ch))
	endpoint := new(MockEndpoint)
	endpoint.On("Init", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	endpoint.On("Close").Return(nil)
	endpoint.On("OnClose", mock.Anything).Run(func(args mock.Arguments) {}).Return(endpoint)
	endpoint.On("Publish", route).Run(func(mock.Arguments) {
		container.OnAddIntent(mockIntent, endpoint)
	}).Return(mockIntent, nil)

	container.Add(endpoint)

	intent, err := container.Publish(route, nil)
	assert.NoError(t, err)
	assert.NotNil(t, intent)
	// Wait for the notify runner instead of sleeping (DND-021); it calls Interest() from
	// its own goroutine, and Close would otherwise beat it on a loaded host.
	require.True(t, testutil.CtxRecv(ctx, read), "the fan-out never read the intent's Interest()")
	container.Close()
	addIntent.WaitCalled()
	addInterest.NotCalled()

	mockIntent.AssertExpectations(t)
	endpoint.AssertExpectations(t)
}

func TestContainer_Subscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // a deadline only (DND-021)
	defer cancel()
	container := NewContainer("test", 10)
	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	container.Init(ctx, slog.Default(),
		func(intent Intent, t Endpoint) error {
			return addIntent.FE(nil)()
		},
		func(interest Interest, t Endpoint) error {
			return addInterest.FE(nil)()
		},
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	mockInterest := new(MockInterest)
	mockInterest.On("Route").Return(route)
	ch := make(chan proto.Message)
	read := make(chan struct{}) // closed once the fan-in's receive runner reads C()
	var readOnce sync.Once
	mockInterest.On("C").Run(func(mock.Arguments) { readOnce.Do(func() { close(read) }) }).Return((<-chan proto.Message)(ch))
	mockInterest.On("OnClose", mock.Anything).Return(mockInterest)
	endpoint := new(MockEndpoint)
	endpoint.On("Init", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	endpoint.On("Close").Return(nil)
	endpoint.On("OnClose", mock.Anything).Run(func(args mock.Arguments) {}).Return(endpoint)
	endpoint.On("Subscribe", route).Run(func(mock.Arguments) {
		container.OnAddInterest(mockInterest, endpoint)
	}).Return(mockInterest, nil)

	container.Add(endpoint)

	interest, err := container.Subscribe(route, nil)
	assert.NoError(t, err)
	assert.NotNil(t, interest)
	require.True(t, testutil.CtxRecv(ctx, read), "the fan-in never read the interest's C()") // DND-021
	container.Close()
	addInterest.WaitCalled()
	addIntent.NotCalled()

	mockInterest.AssertExpectations(t)
	endpoint.AssertExpectations(t)
}

func TestContainer_SubscribePublish(t *testing.T) {
	// A deadline only (R-DND-R8 N3): with 100 ms a loaded host could cancel the
	// container's context before Publish, which then returned "closed".
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	container := NewContainer("test", 10)
	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	container.Init(ctx, slog.Default(),
		func(intent Intent, t Endpoint) error {
			return addIntent.FE(nil)()
		},
		func(interest Interest, t Endpoint) error {
			return addInterest.FE(nil)()
		},
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	mockIntent := new(MockIntentInternal)
	mockIntent.On("Route").Return(route)
	chI := make(chan Route, 1)
	intentRead := make(chan struct{}) // the fan-out's notify runner read Interest()
	var intentOnce sync.Once
	mockIntent.On("Interest").Run(func(mock.Arguments) { intentOnce.Do(func() { close(intentRead) }) }).Return((<-chan Route)(chI))
	mockIntent.On("OnClose", mock.Anything).Return(mockIntent)

	mockInterest := new(MockInterestInternal)
	mockInterest.On("Route").Return(route)
	ch := make(chan proto.Message)
	interestRead := make(chan struct{}) // the fan-in's receive runner read C()
	var interestOnce sync.Once
	mockInterest.On("C").Run(func(mock.Arguments) { interestOnce.Do(func() { close(interestRead) }) }).Return((<-chan proto.Message)(ch))
	mockInterest.On("OnClose", mock.Anything).Return(mockInterest)
	endpoint := new(MockEndpoint)
	endpoint.On("Init", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	endpoint.On("Close").Return(nil)
	endpoint.On("OnClose", mock.Anything).Run(func(args mock.Arguments) {}).Return(endpoint)
	endpoint.On("Subscribe", route).Run(func(mock.Arguments) {
		container.OnAddInterest(mockInterest, endpoint)
	}).Return(mockInterest, nil)
	endpoint.On("Publish", route).Run(func(mock.Arguments) {
		container.OnAddIntent(mockIntent, endpoint)
	}).Return(mockIntent, nil)

	container.Add(endpoint)

	interest, err := container.Subscribe(route, nil)
	assert.NoError(t, err)
	assert.NotNil(t, interest)
	require.True(t, testutil.CtxRecv(ctx, interestRead), "the fan-in never read the interest's C()")

	intent, err := container.Publish(route, nil)
	assert.NoError(t, err)
	assert.NotNil(t, intent)
	require.True(t, testutil.CtxRecv(ctx, intentRead), "the fan-out never read the intent's Interest()")

	container.Close()
	addInterest.WaitCalled()
	addIntent.WaitCalled()

	mockInterest.AssertExpectations(t)
	endpoint.AssertExpectations(t)
}
