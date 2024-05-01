package dndm

import (
	"net/url"
	"strings"
)

// Peer defines an interface for network peer entities, encapsulating methods
// that provide details about network connection points such as URL components
// and query parameters.
//
// Peer is identified URI such as [schema]://[address]/[path][?params&...].
type Peer interface {
	String() string
	Values() url.Values
	Address() string
	Path() string
	Scheme() string
	// Equal compares this Peer to another Peer interface to determine if they represent
	// the same peer.
	Equal(v Peer) bool
}

type PeerImpl struct {
	scheme string
	addr   string
	path   string
	args   url.Values
}

func (r PeerImpl) String() string {
	u := url.URL{
		Scheme:   r.scheme,
		Host:     r.addr,
		Path:     r.path,
		RawQuery: r.args.Encode(),
	}
	return u.String()
}

// NewPeer constructs a new PeerImpl object given its components: scheme, address, path,
// and arguments (query parameters). It initializes the PeerImpl with these components.
func NewPeer(scheme, address, path string, args url.Values) (PeerImpl, error) {
	return PeerImpl{
		scheme: scheme,
		addr:   address,
		path:   path,
		args:   args,
	}, nil
}

// PeerFromString parses a string containing a URL into a Peer object. It extracts
// the scheme, host, path, and query parameters from the string.
func PeerFromString(peer string) (PeerImpl, error) {
	u, err := url.Parse(peer)
	if err != nil {
		return PeerImpl{}, err
	}
	return PeerImpl{
		scheme: u.Scheme,
		addr:   u.Host,
		path:   strings.TrimPrefix(u.Path, "/"),
		args:   u.Query(),
	}, nil
}

func (p PeerImpl) Values() url.Values { return p.args }
func (p PeerImpl) Address() string    { return p.addr }
func (p PeerImpl) Path() string       { return p.path }
func (p PeerImpl) Scheme() string     { return p.scheme }
func (p PeerImpl) Equal(v Peer) bool {
	return p.scheme == v.Scheme() && p.addr == v.Address()
}
