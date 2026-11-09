package bus

import (
	"errors"
	"fmt"

	dndmerrors "github.com/itohio/dndm/errors"
)

var (
	// ErrNotLinked is returned when Send() is called before interest is received
	ErrNotLinked = errors.New("bus: producer not linked to any consumer")
	// ErrClosed is returned when operations are attempted on a closed producer, consumer,
	// caller or service, or on a route the router has closed. It wraps the core
	// dndm errors.ErrClosed, so errors.Is matches either sentinel.
	ErrClosed = fmt.Errorf("bus: producer/consumer/caller/service is closed: %w", dndmerrors.ErrClosed)
	// ErrTimeout is returned when a request times out
	ErrTimeout = errors.New("bus: request timeout")
	// ErrNonceSource is returned by Call and HandleReplies when the Caller's NonceSource
	// keeps returning 0 or nonces that are still in use.
	ErrNonceSource = errors.New("bus: nonce source returned 0 or a nonce in use")
)

// busErr reports the core ErrClosed as bus.ErrClosed; other errors pass unchanged.
func busErr(err error) error {
	if err != nil && errors.Is(err, dndmerrors.ErrClosed) && !errors.Is(err, ErrClosed) {
		return ErrClosed
	}
	return err
}
