package dndm

import (
	"context"
	"log/slog"
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
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
	container.Close()
	time.Sleep(time.Millisecond)
	onClose.WaitCalled()
	addIntent.NotCalled()
	addInterest.NotCalled()

	<-ctx.Done()
}

func TestContainer_AddEndpoint(t *testing.T) {
	container := NewContainer("test", 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
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
	container.Close()
	time.Sleep(time.Millisecond)
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

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	mockIntent := new(MockIntent)
	mockIntent.On("Route").Return(route)
	ch := make(chan Route)
	mockIntent.On("Interest").Return((<-chan Route)(ch))
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
	time.Sleep(time.Millisecond)
	container.Close()
	time.Sleep(time.Millisecond)
	addIntent.WaitCalled()
	addInterest.NotCalled()

	mockIntent.AssertExpectations(t)
	endpoint.AssertExpectations(t)
}

func TestContainer_Subscribe(t *testing.T) {
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

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	mockInterest := new(MockInterest)
	mockInterest.On("Route").Return(route)
	ch := make(chan proto.Message)
	mockInterest.On("C").Return((<-chan proto.Message)(ch))
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
	time.Sleep(time.Millisecond)
	container.Close()
	time.Sleep(time.Millisecond)
	addInterest.WaitCalled()
	addIntent.NotCalled()

	mockInterest.AssertExpectations(t)
	endpoint.AssertExpectations(t)
}

func TestContainer_SubscribePublish(t *testing.T) {
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

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	mockIntent := new(MockIntentInternal)
	mockIntent.On("Route").Return(route)
	chI := make(chan Route, 1)
	mockIntent.On("Interest").Return((<-chan Route)(chI))

	mockInterest := new(MockInterestInternal)
	mockInterest.On("Route").Return(route)
	ch := make(chan proto.Message)
	mockInterest.On("C").Return((<-chan proto.Message)(ch))
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
	time.Sleep(time.Millisecond)

	intent, err := container.Publish(route, nil)
	assert.NoError(t, err)
	assert.NotNil(t, intent)
	time.Sleep(time.Millisecond)

	container.Close()
	time.Sleep(time.Millisecond)
	addInterest.WaitCalled()
	addIntent.WaitCalled()

	mockInterest.AssertExpectations(t)
	endpoint.AssertExpectations(t)
}
