package mesh

import (
	"github.com/itohio/dndm"
)

var (
	_ dndm.Peer = (*aggregatePeer)(nil)
)

type aggregatePeer struct {
	dndm.PeerImpl
	ep *Endpoint
}

// Equal returns true if the aggregatePeer belongs to the same Mesh Endpoint.
func (p aggregatePeer) Equal(v dndm.Peer) bool {
	if vp, ok := v.(aggregatePeer); ok {
		return p.ep == vp.ep
	}
	return false
}

// HasPrefix checks if any of the peers have a prefix that matches the route.
func (p aggregatePeer) HasPrefix(r dndm.Route) bool {
	return p.ep.hasPrefix(r)
}
