package bus

import (
	"context"
	"testing"
	"time"

	dndmerrors "github.com/itohio/dndm/errors"
	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review R-DND-R5 F3: x/bus reports a closed route with one sentinel. A closed router
// wrapper returns the core ErrClosed; bus callers must still match bus.ErrClosed (and the
// core sentinel, since bus.ErrClosed wraps it).
func TestProducerSendOnClosedRouteIsBusErrClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	router := newTestRouter(t, ctx)
	consumer, err := NewConsumer[*types.Foo](ctx, router, "closed.route")
	require.NoError(t, err)
	defer consumer.Close()
	producer, err := NewProducer[*types.Foo](ctx, router, "closed.route")
	require.NoError(t, err)
	require.NoError(t, producer.WaitForInterest(ctx))

	require.NoError(t, producer.intent.Close()) // the router wrapper closes under the producer

	for name, send := range map[string]func() error{
		"Send":       func() error { return producer.Send(ctx, &types.Foo{Text: "x"}) },
		"SendDirect": func() error { return producer.SendDirect(ctx, &types.Foo{Text: "x"}) },
	} {
		err := send()
		assert.ErrorIs(t, err, ErrClosed, "%s: errors.Is(err, bus.ErrClosed) must match a closed route", name)
		assert.ErrorIs(t, err, dndmerrors.ErrClosed, "%s: the core sentinel must still match", name)
	}
}
