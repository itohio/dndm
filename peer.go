package dndm

import (
	"net/url"
	"strings"
)

type Peer struct {
	scheme string
	addr   string
	path   string
	args   url.Values
}

func (r Peer) String() string {
	u := url.URL{
		Scheme:   r.scheme,
		Host:     r.addr,
		Path:     r.path,
		RawQuery: r.args.Encode(),
	}
	return u.String()
}

func NewPeer(scheme, address, path string, args url.Values) (Peer, error) {
	return Peer{
		scheme: scheme,
		addr:   address,
		path:   path,
		args:   args,
	}, nil
}

func PeerFromString(peer string) (Peer, error) {
	u, err := url.Parse(peer)
	if err != nil {
		return Peer{}, err
	}
	return Peer{
		scheme: u.Scheme,
		addr:   u.Host,
		path:   strings.TrimPrefix(u.Path, "/"),
		args:   u.Query(),
	}, nil
}

func (p Peer) Values() url.Values { return p.args }
func (p Peer) Address() string    { return p.addr }
func (p Peer) Path() string       { return p.path }
func (p Peer) Scheme() string     { return p.scheme }
func (p Peer) Equal(v Peer) bool {
	return p.scheme == v.scheme && p.addr == v.addr
}
