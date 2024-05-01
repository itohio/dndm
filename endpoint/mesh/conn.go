package mesh

import (
	"io"

	"github.com/itohio/dndm"
)

func (t *Endpoint) dialerLoop() {
	for {
		select {
		case <-t.Ctx().Done():
			t.Log.Info("mesh.loop", "err", t.Ctx().Err())
			return
		case peer := <-t.addrbook.Dials():
			t.Log.Info("mesh.dial", "attempts", peer.Attempts, "failed", peer.Failed, "backoff", peer.Backoff, "peer", peer.Peer)
			t.dial(peer)
		}
	}
}

func (t *Endpoint) onConnect(peer dndm.Peer, rw io.ReadWriteCloser) error {
	hs := NewHandshaker(t.addrbook, peer, t.Container.Size, t.timeout, t.pingDuration, rw, HS_WAIT)

	t.Container.Add(hs)

	return nil
}

func (t *Endpoint) dial(address *AddrbookEntry) error {
	rw, err := address.Dial(t.Container.Ctx(), t.Container.Log, t.dialer, t.addrbook.Dials())
	if err != nil {
		return err
	}

	hs := NewHandshaker(t.addrbook, address.Peer, t.Container.Size, t.timeout, t.pingDuration, rw, HS_INIT)

	t.Container.Add(hs)

	return nil
}
