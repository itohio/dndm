package dndm

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_NewContainer(t *testing.T) {
	container := NewContainer("test", 10)
	require.NotNil(t, container)
	assert.Equal(t, "test", container.Name())

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	addCalled := make(chan struct{})
	delCalled := make(chan struct{})
	container.Init(ctx, slog.Default(),
		func(interest Interest, t Endpoint) error {
			close(addCalled)
			return nil
		},
		func(interest Interest, t Endpoint) error {
			close(delCalled)
			return nil
		},
	)

	container.OnClose(nil)
	onCloseCalled := make(chan struct{})
	container.OnClose(func() { close(onCloseCalled) })
	container.Close()
}

func TestContainer_AddEndpoint(t *testing.T) {
	container := NewContainer("test", 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	container.Init(ctx, slog.Default(),
		func(interest Interest, t Endpoint) error {
			return nil
		},
		func(interest Interest, t Endpoint) error {
			return nil
		},
	)

	endpoint := &MockEndpoint{}
	endpoint.On("Init", container.Ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := container.Add(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(container.endpoints))
	assert.Contains(t, container.endpoints, endpoint)

	container.Close()
}

// func TestContainer_RemoveEndpoint(t *testing.T) {
// 	container := NewContainer("test", 10)
// 	endpoint := new(MockEndpoint)
// 	endpoint.On("Init", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
// 	endpoint.On("Close").Return(nil)

// 	container.Add(endpoint)
// 	err := container.Remove(endpoint)
// 	assert.NoError(t, err)
// 	assert.Equal(t, 0, len(container.endpoints))
// }

// func TestContainer_Close(t *testing.T) {
// 	container := NewContainer("test", 10)
// 	endpoint1 := new(MockEndpoint)
// 	endpoint2 := new(MockEndpoint)

// 	endpoint1.On("Close").Return(nil)
// 	endpoint2.On("Close").Return(nil)

// 	container.Add(endpoint1)
// 	container.Add(endpoint2)

// 	err := container.Close()
// 	assert.NoError(t, err)
// }

// func TestContainer_Publish(t *testing.T) {
// 	container := NewContainer("test", 10)
// 	endpoint := new(MockEndpoint)
// 	endpoint.On("Publish", mock.Anything).Return(&emptypb.Empty{}, nil)

// 	intent, err := container.Publish(Route{route: "testRoute"}, nil)
// 	assert.NoError(t, err)
// 	assert.NotNil(t, intent)
// }

// func TestContainer_Subscribe(t *testing.T) {
// 	container := NewContainer("test", 10)
// 	endpoint := new(MockEndpoint)
// 	endpoint.On("Subscribe", mock.Anything).Return(&emptypb.Empty{}, nil)

// 	interest, err := container.Subscribe(Route{route: "testRoute"}, nil)
// 	assert.NoError(t, err)
// 	assert.NotNil(t, interest)
// }
