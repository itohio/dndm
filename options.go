package dndm

import (
	"context"
	"log/slog"
	"slices"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
	"github.com/itohio/dndm/router/direct"
)

type Option func(*Options) error

type Options struct {
	ctx        context.Context
	logger     *slog.Logger
	transports []router.Transport
	size       int
}

func defaultOptions() Options {
	return Options{
		ctx:    context.Background(),
		logger: slog.Default(),
		transports: []router.Transport{
			direct.New(0),
		},
		size: 1,
	}
}

func (o *Options) Config(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return err
		}
	}
	return nil
}

func (o *Options) addTransport(t router.Transport) error {
	if t == nil {
		return errors.New("invalid transport")
	}

	if slices.Contains(o.transports, t) {
		return errors.New("already registered")
	}

	o.transports = append(o.transports, t)
	return nil
}

func WithContext(ctx context.Context) Option {
	return func(o *Options) error {
		o.ctx = ctx
		return nil
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(o *Options) error {
		if l == nil {
			return errors.New("nil logger")
		}
		o.logger = l
		return nil
	}
}

func WithQueueSize(size int) Option {
	return func(o *Options) error {
		if size < 0 {
			return errors.ErrBadArgument
		}
		o.size = size
		return nil
	}
}

func WithTransport(t router.Transport) Option {
	return func(o *Options) error {
		return o.addTransport(t)
	}
}

func WithTransports(t ...router.Transport) Option {
	return func(o *Options) error {
		o.transports = t
		return nil
	}
}
