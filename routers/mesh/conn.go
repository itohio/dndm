package mesh

import (
	"io"
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
	hs := NewHandshaker(t.Ctx, t.Log, t.container, rw, address.Peer, HS_INIT)

	return nil
}

func (t *Mesh) dial(address *AddrbookEntry) error {
	rw, err := address.Dial(t.Ctx, t.Log, t.dialer, t.peerDialerQueue)
	if err != nil {
		return err
	}

	hs := NewHandshaker(t.Ctx, t.Log, t.container, rw, address.Peer, HS_INIT)

	_ = hs

	return nil
}
