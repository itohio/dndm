package network

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPeer_String(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer := Peer{
		scheme: "tcp",
		addr:   "example.com",
		path:   "path",
		args:   args,
	}
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

func TestPeer_Values(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer := Peer{args: args}
	assert.Equal(t, "value", peer.Values().Get("key"))
}

func TestPeer_Address(t *testing.T) {
	peer := Peer{addr: "example.com:123"}
	assert.Equal(t, "example.com:123", peer.Address())
}

func TestPeer_Path(t *testing.T) {
	peer := Peer{path: "path"}
	assert.Equal(t, "path", peer.Path())
}

func TestPeer_Scheme(t *testing.T) {
	peer := Peer{scheme: "tcp"}
	assert.Equal(t, "tcp", peer.Scheme())
}

func TestPeer_Equal(t *testing.T) {
	args := url.Values{}
	args.Add("key", "value")
	peer1 := Peer{
		scheme: "tcp",
		addr:   "example.com",
		path:   "path",
		args:   args,
	}
	peer2 := Peer{
		scheme: "tcp",
		addr:   "example.com",
		path:   "different",
		args:   args,
	}
	peer3 := Peer{
		scheme: "udp",
		addr:   "example.com",
		path:   "path",
		args:   args,
	}
	assert.True(t, peer1.Equal(peer2))
	assert.False(t, peer1.Equal(peer3))
}
