package stream

import (
	"context"
	"io"
	"testing"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/network"
	types "github.com/itohio/dndm/types/core"
	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockReadWriter struct {
	mock.Mock
}

func (m *mockReadWriter) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *mockReadWriter) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func TestNewWithContext(t *testing.T) {
	ctx := context.Background()
	localPeer, _ := network.NewPeer("scheme", "addr", "path", nil)
	remotePeer, _ := network.NewPeer("scheme1", "addr", "path", nil)
	rw := &mockReadWriter{}
	handlers := make(map[types.Type]network.MessageHandler)

	streamContext := NewWithContext(ctx, localPeer, remotePeer, rw, handlers)
	assert.NotNil(t, streamContext)

	assert.Equal(t, localPeer, streamContext.Local())
	assert.Equal(t, remotePeer, streamContext.Remote())
}

func TestStreamContext_Close(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}
	streamContext := NewWithContext(ctx, network.Peer{}, network.Peer{}, rw, nil)

	err := streamContext.Close()
	assert.NoError(t, err)
}

func TestStreamContext_Read(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}

	rw.On("Read", mock.Anything).Return(10, io.EOF)
	streamContext := NewWithContext(ctx, network.Peer{}, network.Peer{}, rw, nil)

	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	assert.NoError(t, err)
	streamContext.AddRoute(route)

	_, _, err = streamContext.Read(ctx)
	assert.Equal(t, io.EOF, err)
}

func TestStreamContext_Write(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}

	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	b, err := codec.EncodeMessage(&testtypes.Foo{}, route)
	require.NoError(t, err)
	N := len(b)
	codec.Release(b)

	rw.On("Write", mock.Anything).Return(N, nil)
	streamContext := NewWithContext(ctx, network.Peer{}, network.Peer{}, rw, nil)

	err = streamContext.Write(ctx, route, &testtypes.Foo{})
	assert.NoError(t, err)
}

func TestStream_CreateClose(t *testing.T) {
	rw := &mockReadWriter{}
	localPeer, _ := network.NewPeer("scheme", "addr", "path", nil)
	remotePeer, _ := network.NewPeer("scheme", "addr1", "path", nil)
	newRemotePeer, _ := network.NewPeer("scheme", "addr15", "path", nil)
	badRemotePeer, _ := network.NewPeer("scheme1", "addr15", "path", nil)
	stream := New(localPeer, remotePeer, rw, nil)

	assert.Equal(t, localPeer, stream.Local())
	assert.Equal(t, remotePeer, stream.Remote())

	err := stream.UpdateRemotePeer(newRemotePeer)
	assert.NoError(t, err)
	assert.Equal(t, newRemotePeer, stream.Remote())

	err = stream.UpdateRemotePeer(badRemotePeer)
	assert.Error(t, err)
	assert.Equal(t, newRemotePeer, stream.Remote())

	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	assert.NoError(t, err)
	stream.AddRoute(route)
	assert.Contains(t, stream.routes, route.ID())
	stream.DelRoute(route)
	assert.NotContains(t, stream.routes, route.ID())

	onCloseCalled := make(chan struct{})
	stream.OnClose(func() { close(onCloseCalled) })
	err = stream.Close()
	assert.NoError(t, err)
	_, ok := <-stream.done
	assert.False(t, ok)
}

func TestStream_Read(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}
	stream := New(network.Peer{}, network.Peer{}, rw, nil)

	rw.On("Read", mock.Anything).Return(0, io.EOF)

	_, _, err := stream.Read(ctx)
	assert.Equal(t, io.EOF, err)
}

func TestStream_Write(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}

	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	stream := New(network.Peer{}, network.Peer{}, rw, nil)
	b, err := codec.EncodeMessage(&testtypes.Foo{}, route)
	require.NoError(t, err)
	N := len(b)
	codec.Release(b)

	rw.On("Write", mock.Anything).Return(N, nil)

	err = stream.Write(ctx, route, &testtypes.Foo{})
	assert.NoError(t, err)
}
