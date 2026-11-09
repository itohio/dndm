//go:build !tinygo && !dndm_nonce_counter && !dndm_nonce_random

package bus

// defaultNonceSource is the build's default: crypto/rand per call. Build with
// -tags dndm_nonce_counter or dndm_nonce_random to change it; TinyGo builds default to
// the counter. WithCallerNonceSource overrides it per Caller.
func defaultNonceSource() NonceSource { return &CryptoNonce{} }
