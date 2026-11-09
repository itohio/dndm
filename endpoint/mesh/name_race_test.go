package mesh

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// DND-008 G2: Handshaker.Name() is read by other goroutines (the Container's close
// callback logs it) while the handshake replaces the remote peer with the identity the
// peer declared. Under -race this reported a data race (1/120 in the alias stress). The
// test reads Name() on both handshakers for the whole handshake; it needs -race to fail.
func TestHandshaker_NameDuringHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a := newNode(t, ctx, "127.0.0.1:1", "a")
	b := newNode(t, ctx, "127.0.0.1:2", "b")

	open := make(chan struct{})
	hsD, hsA := dialPair(t, a, b, open)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for _, h := range []*Handshaker{hsD, hsA} {
		wg.Add(1)
		go func(h *Handshaker) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = h.Name()
				}
			}
		}(h)
	}
	close(open)
	assert.Eventually(t, func() bool { return hsD.Ready() && hsA.Ready() }, 3*time.Second, time.Millisecond)
	close(stop)
	wg.Wait()
	assert.Equal(t, b.book.Self().String(), hsD.Name(), "the dialer names the peer by its declared identity")
	assert.Equal(t, a.book.Self().String(), hsA.Name())
}
