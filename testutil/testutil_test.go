package testutil

import (
	"context"
	"testing"
)

// DND-021: a callback that already ran must count as called even when the test context
// has expired too; select would pick between the two at random.
func TestCtxRecvPrefersAReadyChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := make(chan struct{})
	close(called)
	for range 1000 {
		if !CtxRecv(ctx, called) {
			t.Fatal("CtxRecv reported a closed channel as not received because the context was also done")
		}
	}
}
