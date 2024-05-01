package dndm

import (
	"context"
	"log/slog"
	"slices"

	"github.com/itohio/dndm/errors"
)

type Option func(*Options) error

type Options struct {
	ctx       context.Context
	logger    *slog.Logger
	endpoints []Endpoint
	size      int
}

func defaultOptions() Options {
	return Options{
		ctx:       context.Background(),
		logger:    slog.Default(),
		endpoints: []Endpoint{},
		size:      1,
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

func (o *Options) addEndpoint(t Endpoint) error {
	if t == nil {
		return errors.ErrInvalidEndpoint
	}

	if slices.Contains(o.endpoints, t) {
		return errors.ErrDuplicate
	}

	o.endpoints = append(o.endpoints, t)
	return nil
}

// WithContext option configures context.
func WithContext(ctx context.Context) Option {
	return func(o *Options) error {
		o.ctx = ctx
		return nil
	}
}

// WithLogger option configures Logger.
func WithLogger(l *slog.Logger) Option {
	return func(o *Options) error {
		if l == nil {
			return errors.ErrBadArgument
		}
		o.logger = l
		return nil
	}
}

// WithQueueSize configures the size of send/receive/notification chan sizes.
func WithQueueSize(size int) Option {
	return func(o *Options) error {
		if size < 0 {
			return errors.ErrBadArgument
		}
		o.size = size
		return nil
	}
}

// WithEndpoint adds an endpoint to the collection.
func WithEndpoint(t Endpoint) Option {
	return func(o *Options) error {
		return o.addEndpoint(t)
	}
}

// WithEndpoints sets the collection to specified endpoints.
func WithEndpoints(t ...Endpoint) Option {
	return func(o *Options) error {
		o.endpoints = t
		return nil
	}
}

// Subscription options
type SubOptStruct struct {
}
type SubOpt func(*SubOptStruct) error

// Publishing options
type PubOptStruct struct {
}
type PubOpt func(*PubOptStruct) error
