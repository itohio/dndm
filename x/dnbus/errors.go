package dnbus

import (
	"errors"
)

var (
	// ErrNotLinked is returned when Send() is called before interest is received
	ErrNotLinked = errors.New("dnbus: producer not linked to any consumer")
	// ErrClosed is returned when operations are attempted on a closed producer/consumer
	ErrClosed = errors.New("dnbus: producer/consumer/caller/service is closed")
	// ErrTimeout is returned when a request times out
	ErrTimeout = errors.New("dnbus: request timeout")
)
