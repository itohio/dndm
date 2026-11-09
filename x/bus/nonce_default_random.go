//go:build dndm_nonce_random

package bus

// defaultNonceSource is the build's default with -tags dndm_nonce_random: math/rand/v2 per call.
func defaultNonceSource() NonceSource { return RandomNonce{} }
