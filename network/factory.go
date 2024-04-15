package network

import (
	"context"
	"io"

	"github.com/itohio/dndm/errors"
	"golang.org/x/sync/errgroup"
)

var _ Node = (*Factory)(nil)

type Factory struct {
	dialers map[string]Dialer
	servers map[string]Server
}

func New(dialers ...Dialer) (*Factory, error) {
	dm := make(map[string]Dialer, len(dialers))
	sm := make(map[string]Server, len(dialers))
	for _, d := range dialers {
		_, inDialer := dm[d.Scheme()]
		_, inServer := sm[d.Scheme()]
		s, isServer := d.(Server)
		if inDialer || inServer && isServer {
			return nil, errors.ErrDuplicate
		}
		if d.Scheme() == "" {
			return nil, errors.ErrBadArgument
		}
		dm[d.Scheme()] = d

		if isServer {
			sm[d.Scheme()] = s
		}
	}
	return &Factory{
		dialers: dm,
		servers: sm,
	}, nil
}

func (f *Factory) Scheme() string {
	return ""
}

func (f *Factory) Dial(ctx context.Context, peer Peer, o ...DialOpt) (io.ReadWriteCloser, error) {
	d, ok := f.dialers[peer.Scheme()]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return d.Dial(ctx, peer)
}

func (f *Factory) Serve(ctx context.Context, onConnect func(r io.ReadWriteCloser) error, o ...SrvOpt) error {
	eg, ctx := errgroup.WithContext(ctx)

	for _, s := range f.servers {
		s := s
		eg.Go(func() error { return s.Serve(ctx, onConnect) })
	}

	return eg.Wait()
}
