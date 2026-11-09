package bus

import (
	"context"
	"testing"
	"time"

	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DND-002 contract basis: local delivery does not copy. Every local subscriber receives
// the sender's own object, which is why a message is immutable once sent and received
// messages are read-only (see dndm.Intent.Send and dndm.Interest.C).
func TestLocalSubscribersShareTheSentMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)

	c1, err := NewConsumer[*types.Foo](ctx, router, "own.path")
	require.NoError(t, err)
	defer c1.Close()
	c2, err := NewConsumer[*types.Foo](ctx, router, "own.path")
	require.NoError(t, err)
	defer c2.Close()
	producer, err := NewProducer[*types.Foo](ctx, router, "own.path")
	require.NoError(t, err)
	defer producer.Close()

	sent := &types.Foo{Text: "shared"}
	require.NoError(t, producer.Send(ctx, sent))

	got1, err := c1.Receive(ctx)
	require.NoError(t, err)
	got2, err := c2.Receive(ctx)
	require.NoError(t, err)
	assert.Same(t, sent, got1, "first subscriber got a copy")
	assert.Same(t, sent, got2, "second subscriber got a copy")
}
