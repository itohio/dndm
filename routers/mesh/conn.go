package mesh

import (
	"io"

	"github.com/itohio/dndm/dialers"
)

func (t *Mesh) dialerLoop() {
	for {
		select {
		case <-t.Ctx.Done():
			return
		case peer := <-t.peerDialerQueue:
			t.dial(peer)
		}
	}
}

func (t *Mesh) onConnect(rw io.ReadWriteCloser) error {
	hs := NewHandshaker(t.Size, t.timeout, t.pingDuration, t.container, rw, dialers.Peer{}, HS_WAIT)

	_ = hs

	return nil
}

func (t *Mesh) dial(address *AddrbookEntry) error {
	rw, err := address.Dial(t.Ctx, t.Log, t.dialer, t.peerDialerQueue)
	if err != nil {
		return err
	}

	hs := NewHandshaker(t.Size, t.timeout, t.pingDuration, t.container, rw, address.Peer, HS_INIT)

	_ = hs

	return nil
}
