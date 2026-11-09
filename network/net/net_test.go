package net

import (
	"context"
	"log/slog"
	"net"
	"testing"

	"github.com/itohio/dndm"
	"github.com/stretchr/testify/require"
)

// DND-008: Dial honours its context. Loopback only: the test listens on 127.0.0.1.
func TestNode_DialHonoursContext(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	self, err := dndm.NewPeer("tcp", "127.0.0.1:0", "", nil)
	require.NoError(t, err)
	node, err := New(slog.Default(), self)
	require.NoError(t, err)
	target, err := dndm.NewPeer("tcp", l.Addr().String(), "", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := node.Dial(ctx, target)
	if conn != nil {
		conn.Close()
	}
	require.ErrorIs(t, err, context.Canceled, "Dial with a cancelled context connected anyway")
}
