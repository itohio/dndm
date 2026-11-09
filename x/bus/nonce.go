package bus

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand/v2"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
)

// NonceSource produces request nonces for a Caller. A nonce correlates a reply with its
// call; it is not authentication.
//
// Contract: Next must be safe for concurrent use, must not block, and must keep
// returning fresh non-zero values. The Caller calls it outside its lock and skips 0 and
// nonces still in use, but only a few times per call (nonceTries). A source that keeps
// returning 0 or the same value makes Call and HandleReplies fail with ErrNonceSource.
//
// How predictable the nonces are decides who can forge a reply that the Caller accepts:
//   - CounterNonce: after seeing one reply, the next nonce of that Caller is known (n+1).
//   - RandomNonce: not predictable from earlier nonces in practice, but math/rand/v2 makes
//     no cryptographic promise.
//   - CryptoNonce: crypto/rand per call.
//
// None of them stops a participant on the request path, who can copy the nonce from the
// request and answer first; that needs source authentication (DND-009).
type NonceSource interface {
	Next() uint64
}

// CounterNonce counts up from a random start. It is the cheapest source and suits small
// devices, but its nonces are predictable once one is observed.
type CounterNonce struct {
	n atomic.Uint64
}

// NewCounterNonce returns a counter that starts at a random 64-bit value.
func NewCounterNonce() *CounterNonce {
	c := &CounterNonce{}
	c.n.Store(cryptoUint64())
	return c
}

// Next returns the next value of the counter.
func (c *CounterNonce) Next() uint64 {
	return c.n.Add(1)
}

// RandomNonce draws every nonce from math/rand/v2 (runtime-seeded, fast, not a
// cryptographic guarantee).
type RandomNonce struct{}

// Next returns a fresh pseudo-random nonce.
func (RandomNonce) Next() uint64 {
	return mrand.Uint64()
}

// CryptoNonce draws every nonce from crypto/rand. It is a zero-size value with no lock,
// like RandomNonce (review R-DND-R5 F11): the read buffer lives on the stack.
type CryptoNonce struct{}

// Next returns a fresh cryptographically random nonce.
func (CryptoNonce) Next() uint64 {
	return cryptoUint64()
}

func cryptoUint64() uint64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(fmt.Errorf("bus: random nonce: %w", err)) // crypto/rand does not fail on supported platforms
	}
	return binary.LittleEndian.Uint64(b[:])
}

// WithCallerNonceSource selects the Caller's nonce source. Without it the Caller uses
// the build's default (see defaultNonceSource).
func WithCallerNonceSource[Req proto.Message, Resp proto.Message](src NonceSource) CallerOption[Req, Resp] {
	return func(opts *callerOptions[Req, Resp]) {
		opts.nonceSource = src
	}
}
