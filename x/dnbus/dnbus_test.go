package dnbus

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/direct"
	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/require"
)

func newTestRouter(t *testing.T, ctx context.Context) *dndm.Router {
	t.Helper()

	router, err := dndm.New(
		dndm.WithContext(ctx),
		dndm.WithQueueSize(4),
		dndm.WithEndpoint(direct.New(4)),
	)
	require.NoError(t, err)
	return router
}

func TestProducerConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	router := newTestRouter(t, ctx)

	consumer, err := NewConsumer[*types.Foo](ctx, router, "test.producer")
	require.NoError(t, err)
	defer consumer.Close()

	producer, err := NewProducer[*types.Foo](ctx, router, "test.producer")
	require.NoError(t, err)
	defer producer.Close()

	require.NoError(t, producer.Send(ctx, &types.Foo{Text: "hello"}))

	msg, err := consumer.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, "hello", msg.Text)

	require.NoError(t, producer.SendWithTimeout(ctx, &types.Foo{Text: "timeout"}, 100*time.Millisecond))

	msg, err = consumer.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, "timeout", msg.Text)

	require.NoError(t, consumer.Close())
	_, err = consumer.Receive(ctx)
	require.ErrorIs(t, err, ErrClosed)
}

func TestCallerServiceRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	router := newTestRouter(t, ctx)

	service, err := NewService[*types.Foo, *types.Bar](ctx, router, "call.req", "call.resp")
	require.NoError(t, err)
	defer service.Close()

	// Start service handler
	go func() {
		_ = service.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Bar) error) error {
			return reply(&types.Bar{A: 42, B: 7})
		})
	}()

	caller, err := NewCaller[*types.Foo, *types.Bar](ctx, router, "call.req", "call.resp")
	require.NoError(t, err)
	defer caller.Close()

	resp, err := caller.Call(ctx, &types.Foo{Text: "ping"})
	require.NoError(t, err)
	require.EqualValues(t, 42, resp.A)
	require.EqualValues(t, 7, resp.B)
}

func TestCallerHandleReplies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	router := newTestRouter(t, ctx)

	service, err := NewService[*types.Foo, *types.Bar](ctx, router, "stream.req", "stream.resp")
	require.NoError(t, err)
	defer service.Close()

	go func() {
		_ = service.Handle(ctx, func(ctx context.Context, req *types.Foo, reply func(*types.Bar) error) error {
			for i := uint32(0); i < 3; i++ {
				if err := reply(&types.Bar{A: i, B: i}); err != nil {
					return err
				}
			}
			return nil
		})
	}()

	caller, err := NewCaller[*types.Foo, *types.Bar](ctx, router, "stream.req", "stream.resp")
	require.NoError(t, err)
	defer caller.Close()

	results := make(chan *types.Bar, 3)

	require.NoError(t, caller.HandleReplies(ctx, &types.Foo{Text: "stream"}, func(ctx context.Context, req *types.Foo, resp *types.Bar) error {
		select {
		case results <- resp:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}))

	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for replies")
		case resp := <-results:
			require.EqualValues(t, resp.A, resp.B)
		}
	}
}

func TestModuleRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	router := newTestRouter(t, ctx)

	module := NewModule(ctx, router)
	defer module.Close()

	input, err := AddInput[*types.Foo](module, "module.in")
	require.NoError(t, err)

	output, err := AddOutput[*types.Bar](module, "module.out")
	require.NoError(t, err)

	// Consumer for module output
	outConsumer, err := NewConsumer[*types.Bar](ctx, router, "module.out")
	require.NoError(t, err)
	defer outConsumer.Close()

	runErr := make(chan error, 1)
	go func() {
		runErr <- module.Run(ctx, func(ctx context.Context) error {
			msg, err := input.Receive(ctx)
			if err != nil {
				return err
			}
			return output.Send(ctx, &types.Bar{A: uint32(len(msg.Text))})
		})
	}()

	producer, err := NewProducer[*types.Foo](ctx, router, "module.in")
	require.NoError(t, err)
	defer producer.Close()

	require.NoError(t, producer.Send(ctx, &types.Foo{Text: "abcd"}))

	resp, err := outConsumer.Receive(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 4, resp.A)

	select {
	case err := <-runErr:
		if err != nil && err != io.EOF {
			require.NoError(t, err)
		}
	case <-ctx.Done():
		t.Fatal("module run did not complete")
	}
}
