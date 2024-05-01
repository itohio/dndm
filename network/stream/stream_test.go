package stream

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/codec"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	"github.com/itohio/dndm/testutil"
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
	localPeer, _ := dndm.NewPeer("scheme", "addr", "path", nil)
	remotePeer, _ := dndm.NewPeer("scheme1", "addr", "path", nil)
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
	streamContext := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)

	err := streamContext.Close()
	assert.NoError(t, err)
}

func TestStreamContext_Read(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}

	rw.On("Read", mock.Anything).Return(10, io.EOF)
	streamContext := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)

	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	assert.NoError(t, err)
	streamContext.AddRoute(route)

	_, _, err = streamContext.Read(ctx)
	assert.Equal(t, io.EOF, err)

	time.Sleep(time.Millisecond)

	rw.AssertExpectations(t)
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
	streamContext := NewWithContext(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)

	err = streamContext.Write(ctx, route, &testtypes.Foo{})
	assert.NoError(t, err)

	time.Sleep(time.Millisecond)

	rw.AssertExpectations(t)
}

func TestStream_CreateClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	rw := &mockReadWriter{}
	localPeer, _ := dndm.NewPeer("scheme", "addr", "path", nil)
	remotePeer, _ := dndm.NewPeer("scheme", "addr1", "path", nil)
	newRemotePeer, _ := dndm.NewPeer("scheme", "addr15", "path", nil)
	badRemotePeer, _ := dndm.NewPeer("scheme1", "addr15", "path", nil)
	stream := New(ctx, localPeer, remotePeer, rw, nil)

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

	onClosed := testutil.NewFunc(ctx, t, "close stream")
	stream.OnClose(onClosed.F)
	err = stream.Close()
	assert.NoError(t, err)
	onClosed.WaitCalled()
}

func TestStream_Read(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}
	stream := New(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)

	rw.On("Read", mock.Anything).Return(0, io.EOF)

	_, _, err := stream.Read(ctx)
	assert.Equal(t, io.EOF, err)
}

func TestStream_Write(t *testing.T) {
	ctx := context.Background()
	rw := &mockReadWriter{}

	route, err := dndm.NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	stream := New(ctx, errors.Must(dndm.NewPeer("", "", "", nil)), errors.Must(dndm.NewPeer("", "", "", nil)), rw, nil)
	b, err := codec.EncodeMessage(&testtypes.Foo{}, route)
	require.NoError(t, err)
	N := len(b)
	codec.Release(b)

	rw.On("Write", mock.Anything).Return(N, nil)

	err = stream.Write(ctx, route, &testtypes.Foo{})
	assert.NoError(t, err)
}
