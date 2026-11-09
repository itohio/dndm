//go:build (tinygo || dndm_nonce_counter) && !dndm_nonce_random

package bus

// defaultNonceSource is the build's default on small devices: a counter from a random
// start. Its nonces are predictable once one reply is observed (see NonceSource).
func defaultNonceSource() NonceSource { return NewCounterNonce() }
