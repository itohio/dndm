package dndm

import (
	"net/url"
	"testing"

	"github.com/itohio/dndm/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeer_String(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer, err := NewPeer(
		"tcp",
		"example.com",
		"path",
		args,
	)
	require.NoError(t, err)
	expected := "tcp://example.com/path?key=value"
	assert.Equal(t, expected, peer.String())
}

func TestNewPeer(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer, err := NewPeer("tcp", "example.com", "path", args)
	assert.NoError(t, err)
	assert.Equal(t, "tcp", peer.Scheme())
	assert.Equal(t, "example.com", peer.Address())
	assert.Equal(t, "path", peer.Path())
	assert.Equal(t, "value", peer.Values().Get("key"))
}

func TestPeerFromString(t *testing.T) {
	input := "tcp://example.com:123/path?key=value"
	peer, err := PeerFromString(input)
	assert.NoError(t, err)
	assert.Equal(t, "tcp", peer.Scheme())
	assert.Equal(t, "example.com:123", peer.Address())
	assert.Equal(t, "path", peer.Path())
	assert.Equal(t, "value", peer.Values().Get("key"))
}

func TestPeer_Methods(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer := errors.Must(NewPeer("tcp", "example.com:123", "path", args))
	assert.Equal(t, "value", peer.Values().Get("key"))
	assert.Equal(t, "path", peer.Path())
	assert.Equal(t, "tcp", peer.Scheme())
	assert.Equal(t, "example.com:123", peer.Address())
}

func TestPeer_Equal(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer1 := errors.Must(NewPeer(
		"tcp",
		"example.com",
		"path",
		args,
	))
	peer2 := errors.Must(NewPeer(
		"tcp",
		"example.com",
		"different",
		args,
	))
	peer3 := errors.Must(NewPeer(
		"udp",
		"example.com",
		"path",
		args,
	))
	assert.True(t, peer1.Equal(peer2))
	assert.False(t, peer1.Equal(peer3))
}
