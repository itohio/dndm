package network

import (
	"net/url"
	"strings"
)

type Peer struct {
	scheme string
	addr   string
	id     string
	args   url.Values
}

func (r Peer) String() string {
	u := url.URL{
		Scheme:   r.scheme,
		Host:     r.addr,
		Path:     r.id,
		RawQuery: r.args.Encode(),
	}
	return u.String()
}

func PeerFromString(peer string) (Peer, error) {
	u, err := url.Parse(peer)
	if err != nil {
		return Peer{}, err
	}
	return Peer{
		scheme: u.Scheme,
		addr:   u.Host,
		id:     strings.TrimPrefix(u.Path, "/"),
		args:   u.Query(),
	}, nil
}

func (p Peer) Values() url.Values { return p.args }
func (p Peer) Address() string    { return p.addr }
func (p Peer) ID() string         { return p.id }
func (p Peer) Scheme() string     { return p.scheme }
