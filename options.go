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
	ctx       context.Context
	logger    *slog.Logger
	endpoints []router.Endpoint
	size      int
}

func defaultOptions() Options {
	return Options{
		ctx:    context.Background(),
		logger: slog.Default(),
		endpoints: []router.Endpoint{
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

func (o *Options) addEndpoint(t router.Endpoint) error {
	if t == nil {
		return errors.ErrInvalidEndpoint
	}

	if slices.Contains(o.endpoints, t) {
		return errors.ErrDuplicate
	}

	o.endpoints = append(o.endpoints, t)
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
			return errors.ErrBadArgument
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

func WithEndpoint(t router.Endpoint) Option {
	return func(o *Options) error {
		return o.addEndpoint(t)
	}
}

func WithEndpoints(t ...router.Endpoint) Option {
	return func(o *Options) error {
		o.endpoints = t
		return nil
	}
}
