package mesh

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/network"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DND-008: the dialing flag is shared state; it is read and written under the entry lock.

type gateDialer struct {
	entered chan struct{}
	release chan struct{}
}

func (d *gateDialer) Scheme() string { return "tcp" }

func (d *gateDialer) Dial(ctx context.Context, peer dndm.Peer, o ...network.DialOpt) (io.ReadWriteCloser, error) {
	d.entered <- struct{}{}
	<-d.release
	return nopRWC{}, nil
}

type nopRWC struct{}

func (nopRWC) Read([]byte) (int, error)    { return 0, io.EOF }
func (nopRWC) Write(p []byte) (int, error) { return len(p), nil }
func (nopRWC) Close() error                { return nil }

func TestAddrbookEntry_IsActiveWhileDialing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entry := NewAddrbookEntry(slog.Default(), &p2ptypes.AddrbookEntry{Peer: "tcp://127.0.0.1:1/x", MaxAttempts: 3})

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() { // a monitor polls IsActive while dials start and finish
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				entry.IsActive()
			}
		}
	}()
	defer func() { close(stop); wg.Wait() }()

	for range 20 {
		d := &gateDialer{entered: make(chan struct{}), release: make(chan struct{})}
		done := make(chan error, 1)
		go func() {
			_, err := entry.Dial(ctx, slog.Default(), d, make(chan *AddrbookEntry, 1))
			done <- err
		}()
		<-d.entered
		assert.True(t, entry.IsActive(), "an entry that is dialing counts as active")
		close(d.release)
		require.NoError(t, <-done)
		assert.False(t, entry.IsActive(), "the dial finished and no connection was set")
	}
}
