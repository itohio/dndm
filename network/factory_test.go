package network

import (
	"context"
	"io"
	"testing"

	"github.com/itohio/dndm/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewFactorySuccess(t *testing.T) {
	dialer1 := &MockDialer{}
	dialer1.On("Scheme").Return("tcp")
	dialer2 := &MockNode{}
	dialer2.On("Scheme").Return("udp")

	factory, err := New(dialer1, dialer2)
	require.NoError(t, err)
	require.NotNil(t, factory)
	assert.Contains(t, factory.dialers, dialer1.Scheme(), dialer2.Scheme())
	assert.Contains(t, factory.servers, dialer2.Scheme())
	assert.Equal(t, "", factory.Scheme())
}

func TestNewFactoryDuplicateError(t *testing.T) {
	dialer1 := &MockDialer{}
	dialer1.On("Scheme").Return("tcp")
	dialer2 := &MockDialer{}
	dialer2.On("Scheme").Return("tcp")

	_, err := New(dialer1, dialer2)
	assert.Error(t, err)
	assert.Equal(t, errors.ErrDuplicate, err)
}

func TestFactoryDialNotFound(t *testing.T) {
	factory, _ := New() // No dialers
	_, err := factory.Dial(context.Background(), Peer{scheme: "tcp"})
	assert.Error(t, err)
	assert.Equal(t, errors.ErrNotFound, err)
}

func TestFactoryDialSuccess(t *testing.T) {
	dialer := &MockDialer{}
	dialer.On("Scheme").Return("tcp")
	dialer.On("Dial", mock.Anything, mock.AnythingOfType("Peer")).Return(nil, nil)

	factory, _ := New(dialer)
	_, err := factory.Dial(context.Background(), Peer{scheme: "tcp"})
	assert.NoError(t, err)
	dialer.AssertExpectations(t)
}

func TestFactoryServe(t *testing.T) {
	server := &MockNode{}
	server.On("Scheme").Return("tcp")
	server.On("Serve", mock.Anything, mock.Anything).Return(nil)

	factory, _ := New(server)
	err := factory.Serve(context.Background(), func(peer Peer, r io.ReadWriteCloser) error {
		return nil
	})
	assert.NoError(t, err)
	server.AssertExpectations(t)
}
