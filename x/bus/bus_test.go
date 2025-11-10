package bus

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/direct"
	"github.com/itohio/dndm/endpoint/remote"
	"github.com/itohio/dndm/network/stream"
	types "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newRouterWithEndpoints(t *testing.T, ctx context.Context, endpoints ...dndm.Endpoint) *dndm.Router {
	t.Helper()

	router, err := dndm.New(
		dndm.WithContext(ctx),
		dndm.WithQueueSize(4),
		dndm.WithEndpoints(endpoints...),
	)
	require.NoError(t, err)
	return router
}

func newTestRouter(t *testing.T, ctx context.Context) *dndm.Router {
	return newRouterWithEndpoints(t, ctx, direct.New(4))
}

func newStreamPair(t *testing.T, ctx context.Context, leftID, rightID string, size int) (*remote.Endpoint, *remote.Endpoint) {
	t.Helper()

	peerLeft, err := dndm.NewPeer("stream", leftID, "", nil)
	require.NoError(t, err)
	peerRight, err := dndm.NewPeer("stream", rightID, "", nil)
	require.NoError(t, err)

	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	wireLeft := stream.NewWithContext(ctx, peerLeft, peerRight, pipeReadWriter{r: r1, w: w2}, nil)
	wireRight := stream.NewWithContext(ctx, peerRight, peerLeft, pipeReadWriter{r: r2, w: w1}, nil)

	left := remote.New(peerLeft, wireLeft, size, time.Second, 0)
	right := remote.New(peerRight, wireRight, size, time.Second, 0)

	return left, right
}

type pipeReadWriter struct {
	r io.Reader
	w io.Writer
}

func (rw pipeReadWriter) Read(p []byte) (int, error)  { return rw.r.Read(p) }
func (rw pipeReadWriter) Write(p []byte) (int, error) { return rw.w.Write(p) }

func startForwarder[T proto.Message](t *testing.T, ctx context.Context, consumer *Consumer[T], producer *Producer[T]) func() {
	t.Helper()

	errCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			msg, err := consumer.Receive(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
					return
				}
				select {
				case errCh <- err:
				default:
				}
				return
			}

			sendCtx, cancel := context.WithTimeout(ctx, time.Second)
			err = producer.Send(sendCtx, msg)
			cancel()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}()

	return func() {
		require.NoError(t, consumer.Close())
		require.NoError(t, producer.Close())
		<-done
		select {
		case err := <-errCh:
			require.NoError(t, err)
		default:
		}
	}
}

func waitForProducerInterest[T proto.Message](t *testing.T, ctx context.Context, producer *Producer[T], timeout time.Duration) {
	t.Helper()

	require.Eventually(t, func() bool {
		waitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		err := producer.WaitForInterest(waitCtx)
		if err == nil {
			return true
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		require.NoError(t, err)
		return true
	}, timeout, 50*time.Millisecond)
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

func TestStreamProducerConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const queueSize = 4

	remoteA, remoteB := newStreamPair(t, ctx, "router-a", "router-b", queueSize)

	routerA := newRouterWithEndpoints(t, ctx, direct.New(queueSize), remoteA)
	routerB := newRouterWithEndpoints(t, ctx, direct.New(queueSize), remoteB)

	producer, err := NewProducer[*types.Foo](ctx, routerA, "streams.simple")
	require.NoError(t, err)
	defer producer.Close()

	consumer, err := NewConsumer[*types.Foo](ctx, routerB, "streams.simple")
	require.NoError(t, err)
	defer consumer.Close()

	waitForProducerInterest(t, ctx, producer, 3*time.Second)

	payload := &types.Foo{Text: "stream-hop"}
	require.NoError(t, producer.Send(ctx, payload))

	received, err := consumer.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, payload.Text, received.Text)
}

func TestStreamStarTopologyBidirectional(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const queueSize = 4

	remoteAToStar, remoteStarFromA := newStreamPair(t, ctx, "router-a", "star-a", queueSize)
	remoteBToStar, remoteStarFromB := newStreamPair(t, ctx, "router-b", "star-b", queueSize)

	routerA := newRouterWithEndpoints(t, ctx, direct.New(queueSize), remoteAToStar)
	routerB := newRouterWithEndpoints(t, ctx, direct.New(queueSize), remoteBToStar)
	routerStar := newRouterWithEndpoints(t, ctx, direct.New(queueSize), remoteStarFromA, remoteStarFromB)
	require.NotNil(t, routerStar)

	routeABSrc := "streams.star.ab.src"
	routeABDst := "streams.star.ab.dst"

	starConsumerAB, err := NewConsumer[*types.Foo](ctx, routerStar, routeABSrc)
	require.NoError(t, err)
	starProducerAB, err := NewProducer[*types.Foo](ctx, routerStar, routeABDst)
	require.NoError(t, err)
	cleanupAB := startForwarder(t, ctx, starConsumerAB, starProducerAB)
	defer cleanupAB()

	consumerB, err := NewConsumer[*types.Foo](ctx, routerB, routeABDst)
	require.NoError(t, err)
	defer consumerB.Close()

	producerA, err := NewProducer[*types.Foo](ctx, routerA, routeABSrc)
	require.NoError(t, err)
	defer producerA.Close()

	waitForProducerInterest(t, ctx, starProducerAB, 5*time.Second)
	waitForProducerInterest(t, ctx, producerA, 5*time.Second)

	payloadAB := &types.Foo{Text: "ab"}
	require.NoError(t, producerA.Send(ctx, payloadAB))

	receivedAB, err := consumerB.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, payloadAB.Text, receivedAB.Text)

	routeBASrc := "streams.star.ba.src"
	routeBADst := "streams.star.ba.dst"

	starConsumerBA, err := NewConsumer[*types.Foo](ctx, routerStar, routeBASrc)
	require.NoError(t, err)
	starProducerBA, err := NewProducer[*types.Foo](ctx, routerStar, routeBADst)
	require.NoError(t, err)
	cleanupBA := startForwarder(t, ctx, starConsumerBA, starProducerBA)
	defer cleanupBA()

	consumerA, err := NewConsumer[*types.Foo](ctx, routerA, routeBADst)
	require.NoError(t, err)
	defer consumerA.Close()

	producerB, err := NewProducer[*types.Foo](ctx, routerB, routeBASrc)
	require.NoError(t, err)
	defer producerB.Close()

	waitForProducerInterest(t, ctx, starProducerBA, 5*time.Second)
	waitForProducerInterest(t, ctx, producerB, 5*time.Second)

	payloadBA := &types.Foo{Text: "ba"}
	require.NoError(t, producerB.Send(ctx, payloadBA))

	receivedBA, err := consumerA.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, payloadBA.Text, receivedBA.Text)
}

func TestStreamSensorHostWorkflow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const queueSize = 4

	// Build routers for embedded sensor and hosts.
	sensorRouter := newRouterWithEndpoints(t, ctx, direct.New(queueSize))
	hostARouter := newRouterWithEndpoints(t, ctx, direct.New(queueSize))
	hostBRouter := newRouterWithEndpoints(t, ctx, direct.New(queueSize))

	const (
		intentAPath = "topology.intentA"
		intentBPath = "topology.intentB"
		intentCPath = "topology.intentC"
		intentDPath = "topology.intentD"
	)

	// Forwarders between routers (simulate network links).
	sensorToHostAIntentAConsumer, err := NewConsumer[*types.Foo](ctx, sensorRouter, intentAPath)
	require.NoError(t, err)
	sensorToHostAIntentAProducer, err := NewProducer[*types.Foo](ctx, hostARouter, intentAPath)
	require.NoError(t, err)
	cleanupSensorA := startForwarder(t, ctx, sensorToHostAIntentAConsumer, sensorToHostAIntentAProducer)
	defer cleanupSensorA()

	sensorToHostAIntentBConsumer, err := NewConsumer[*types.Bar](ctx, sensorRouter, intentBPath)
	require.NoError(t, err)
	sensorToHostAIntentBProducer, err := NewProducer[*types.Bar](ctx, hostARouter, intentBPath)
	require.NoError(t, err)
	cleanupSensorBToHostA := startForwarder(t, ctx, sensorToHostAIntentBConsumer, sensorToHostAIntentBProducer)
	defer cleanupSensorBToHostA()

	sensorToHostBIntentBConsumer, err := NewConsumer[*types.Bar](ctx, sensorRouter, intentBPath)
	require.NoError(t, err)
	sensorToHostBIntentBProducer, err := NewProducer[*types.Bar](ctx, hostBRouter, intentBPath)
	require.NoError(t, err)
	cleanupSensorBToHostB := startForwarder(t, ctx, sensorToHostBIntentBConsumer, sensorToHostBIntentBProducer)
	defer cleanupSensorBToHostB()

	hostAIntentDConsumer, err := NewConsumer[*types.Foo](ctx, hostARouter, intentDPath)
	require.NoError(t, err)
	hostBIntentDProducer, err := NewProducer[*types.Foo](ctx, hostBRouter, intentDPath)
	require.NoError(t, err)
	cleanupHostAToHostB := startForwarder(t, ctx, hostAIntentDConsumer, hostBIntentDProducer)
	defer cleanupHostAToHostB()

	hostBIntentCConsumer, err := NewConsumer[*types.Foo](ctx, hostBRouter, intentCPath)
	require.NoError(t, err)
	sensorIntentCProducer, err := NewProducer[*types.Foo](ctx, sensorRouter, intentCPath)
	require.NoError(t, err)
	cleanupHostBToSensor := startForwarder(t, ctx, hostBIntentCConsumer, sensorIntentCProducer)
	defer cleanupHostBToSensor()

	// Embedded sensor endpoints.
	sensorProducerA, err := NewProducer[*types.Foo](ctx, sensorRouter, intentAPath)
	require.NoError(t, err)
	defer sensorProducerA.Close()

	sensorProducerB, err := NewProducer[*types.Bar](ctx, sensorRouter, intentBPath)
	require.NoError(t, err)
	defer sensorProducerB.Close()

	sensorConsumerC, err := NewConsumer[*types.Foo](ctx, sensorRouter, intentCPath)
	require.NoError(t, err)
	defer sensorConsumerC.Close()

	// Host A (processor) endpoints.
	hostAConsumerA, err := NewConsumer[*types.Foo](ctx, hostARouter, intentAPath)
	require.NoError(t, err)
	defer hostAConsumerA.Close()

	hostAConsumerB, err := NewConsumer[*types.Bar](ctx, hostARouter, intentBPath)
	require.NoError(t, err)
	defer hostAConsumerB.Close()

	hostAProducerD, err := NewProducer[*types.Foo](ctx, hostARouter, intentDPath)
	require.NoError(t, err)
	defer hostAProducerD.Close()

	// Host B endpoints.
	hostBConsumerD, err := NewConsumer[*types.Foo](ctx, hostBRouter, intentDPath)
	require.NoError(t, err)
	defer hostBConsumerD.Close()

	hostBConsumerB, err := NewConsumer[*types.Bar](ctx, hostBRouter, intentBPath)
	require.NoError(t, err)
	defer hostBConsumerB.Close()

	hostBProducerC, err := NewProducer[*types.Foo](ctx, hostBRouter, intentCPath)
	require.NoError(t, err)
	defer hostBProducerC.Close()

	// Ensure producers see downstream interests before we start sending data.
	hostAErr := make(chan error, 1)
	hostBErr := make(chan error, 1)

	// Host A: waits for 2x A and 2x B, then emits D once.
	go func() {
		aCh := hostAConsumerA.C()
		bCh := hostAConsumerB.C()
		countA, countB := 0, 0
		for {
			if countA >= 2 && countB >= 2 {
				sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := hostAProducerD.Send(sendCtx, &types.Foo{Text: "derived-D"})
				cancel()
				if err != nil {
					hostAErr <- err
				} else {
					hostAErr <- nil
				}
				return
			}

			select {
			case <-ctx.Done():
				hostAErr <- ctx.Err()
				return
			case _, ok := <-aCh:
				if !ok {
					hostAErr <- io.EOF
					return
				}
				countA++
			case _, ok := <-bCh:
				if !ok {
					hostAErr <- io.EOF
					return
				}
				countB++
			}
		}
	}()

	// Host B: waits for D and three B messages, then emits C once.
	go func() {
		dCh := hostBConsumerD.C()
		bCh := hostBConsumerB.C()
		gotD := false
		countB := 0
		for {
			if gotD && countB >= 3 {
				sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := hostBProducerC.Send(sendCtx, &types.Foo{Text: "closing-C"})
				cancel()
				if err != nil {
					hostBErr <- err
				} else {
					hostBErr <- nil
				}
				return
			}

			select {
			case <-ctx.Done():
				hostBErr <- ctx.Err()
				return
			case _, ok := <-dCh:
				if !ok {
					hostBErr <- io.EOF
					return
				}
				gotD = true
			case _, ok := <-bCh:
				if !ok {
					hostBErr <- io.EOF
					return
				}
				countB++
			}
		}
	}()

	// Embedded sensor sends readings.
	for i := 0; i < 2; i++ {
		require.NoError(t, sensorProducerA.Send(ctx, &types.Foo{Text: "sensor-A"}))
	}
	for i := 0; i < 3; i++ {
		require.NoError(t, sensorProducerB.Send(ctx, &types.Bar{A: uint32(i), B: uint32(i + 100)}))
	}

	// Sensor waits for response on IntentC.
	msgC, err := sensorConsumerC.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, "closing-C", msgC.Text)

	// Verify both hosts completed without error.
	require.NoError(t, <-hostAErr)
	require.NoError(t, <-hostBErr)

	// Cancel context to ensure graceful shutdown.
	cancel()
}
