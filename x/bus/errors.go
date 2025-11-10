package bus

import (
	"errors"
)

var (
	// ErrNotLinked is returned when Send() is called before interest is received
	ErrNotLinked = errors.New("bus: producer not linked to any consumer")
	// ErrClosed is returned when operations are attempted on a closed producer/consumer
	ErrClosed = errors.New("bus: producer/consumer/caller/service is closed")
	// ErrTimeout is returned when a request times out
	ErrTimeout = errors.New("bus: request timeout")
)
