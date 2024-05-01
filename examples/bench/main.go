package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	reflect "reflect"
	"sync/atomic"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/direct"
	"github.com/itohio/dndm/endpoint/remote"
	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network/stream"
	types "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"
)

var (
	sent atomic.Uint64
	recv atomic.Uint64
)

func main() {
	d := flag.Duration("duration", time.Minute, "Duration for the benchmark test")
	n := flag.Int("N", 1, "Number of receivers")
	m := flag.Int("M", 1, "Number of senders")
	size := flag.Int("size", 3, "size of the buffers")
	t := flag.String("what", "channel", "One of channel, dndm, intent, direct, remote, mesh")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	switch *t {
	case "dndm":
		testDNDM(ctx, *size, *n, *m)
	case "channel":
		testChannel(ctx, *size)
	case "intent":
		testIntent(ctx, *size)
	case "direct":
		testDirect(ctx, *size)
	case "remote":
		testRemote(ctx, *size)
	default:
		panic("Unknown")
	}

	sent.Store(0)
	recv.Store(0)
	now := time.Now()
	timer := time.NewTimer(*d)
	var duration time.Duration
	select {
	case <-timer.C:
		cancel()
		duration = time.Since(now)
		time.Sleep(time.Second)
	case <-ctx.Done():
		duration = time.Since(now)
		slog.Info("Stopped", "reason", ctx.Err())
	}

	fmt.Printf("Sent: %d, %.2f msgs/s\n", sent.Load(), float64(sent.Load())/duration.Seconds())
	fmt.Printf("Recv: %d, %.2f msgs/s\n", recv.Load(), float64(recv.Load())/duration.Seconds())
	fmt.Println("Duration: ", duration)
	fmt.Println()
}

func testDNDM(ctx context.Context, size, n, m int) {
	fmt.Println("----------------[ testDNDM ]-----------")
	node, err := dndm.New(dndm.WithContext(ctx), dndm.WithQueueSize(size))
	if err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		go generate(ctx, node, "example.path")
	}
	for i := 0; i < m; i++ {
		go consume[*types.Foo](ctx, node, "example.path")
	}

	c := sender(ctx, size)
	go consumer(c)
}

func testChannel(ctx context.Context, size int) {
	fmt.Println("----------------[ testChannel ]-----------")
	c := sender(ctx, size)
	go consumer(c)
}

func testIntent(ctx context.Context, size int) {
	fmt.Println("----------------[ testIntent ]-----------")
	c := senderIntent(ctx, size, "path", nil)
	go consumer(c)
}

func testDirect(ctx context.Context, size int) {
	fmt.Println("----------------[ testDirect ]-----------")
	var t *types.Foo
	route, err := dndm.NewRoute("path", t)
	if err != nil {
		panic(err)
	}
	rtr := direct.New(size)
	err = rtr.Init(ctx, slog.Default(), func(intent dndm.Intent, t dndm.Endpoint) error { return nil }, func(interest dndm.Interest, t dndm.Endpoint) error { return nil })
	if err != nil {
		panic(err)
	}

	intent, err := rtr.Publish(route)
	if err != nil {
		panic(err)
	}
	interest, err := rtr.Subscribe(route)
	if err != nil {
		panic(err)
	}

	senderIntent(ctx, size, "path", intent.(dndm.IntentInternal))
	go consumerInterest(interest)
	slog.Info("testDirect", "interest", interest.C())
}

func testRemote(ctx context.Context, size int) {
	fmt.Println("----------------[ testRemote ]-----------")
	var t *types.Foo
	route, err := dndm.NewRoute("path", t)
	if err != nil {
		panic(err)
	}
	bridge := makeBridge(ctx)
	peerA := errors.Must(dndm.PeerFromString("pipe://local/remoteA?some_param=123"))
	peerB := errors.Must(dndm.PeerFromString("pipe://local/remoteB?some_param=123"))
	wireA := stream.NewWithContext(ctx, peerA, peerB, bridge.A(), nil)
	wireB := stream.NewWithContext(ctx, peerB, peerA, bridge.B(), nil)

	remoteA := remote.New(peerA, wireA, size, time.Second, 0)
	remoteB := remote.New(peerB, wireB, size, time.Second, 0)

	err = remoteA.Init(ctx, slog.Default(), func(intent dndm.Intent, t dndm.Endpoint) error { return nil }, func(interest dndm.Interest, t dndm.Endpoint) error { return nil })
	if err != nil {
		panic(err)
	}
	err = remoteB.Init(ctx, slog.Default(), func(intent dndm.Intent, t dndm.Endpoint) error { return nil }, func(interest dndm.Interest, t dndm.Endpoint) error { return nil })
	if err != nil {
		panic(err)
	}

	intent, err := remoteA.Publish(route)
	if err != nil {
		panic(err)
	}
	interest, err := remoteB.Subscribe(route)
	if err != nil {
		panic(err)
	}

	senderIntent(ctx, size, "path", intent.(dndm.IntentInternal))
	go consumerInterest(interest)
}

func senderIntent(ctx context.Context, size int, path string, intent dndm.IntentInternal) <-chan proto.Message {
	var t *types.Foo
	route, err := dndm.NewRoute(path, t)
	if err != nil {
		panic(err)
	}
	localC := false
	var c chan proto.Message
	if intent == nil {
		intent = dndm.NewIntent(ctx, route, size)
		c = make(chan proto.Message, size)
		intent.Link(c)
		localC = true
	}

	go func() {
		defer intent.Link(nil)
		defer intent.Close()
		if localC {
			defer close(c)
		}

		if err := generateFoo(ctx, path, intent.Route(), intent); err != nil {
			return
		}
	}()
	return c
}

func sender(ctx context.Context, size int) <-chan proto.Message {
	c := make(chan proto.Message, size)
	var t *types.Foo

	go func() {
		defer close(c)
		slog.Info("sender", "loop", "start", "C", c)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			text := fmt.Sprintf("Message: %d for %v", sent.Add(1), reflect.TypeOf(t))
			select {
			case <-ctx.Done():
				return
			case c <- &types.Foo{Text: text}:
			}
		}
	}()

	return c
}

func consumer(c <-chan proto.Message) {
	slog.Info("consumer", "loop", "start", "C", c)
	for m := range c {
		msg := m.(*types.Foo)
		_ = msg
		recv.Add(1)
	}
}

func consumerInterest(interest dndm.Interest) {
	slog.Info("consumerInterest", "loop", "start", "C", interest.C())
	for m := range interest.C() {
		msg := m.(*types.Foo)
		_ = msg
		recv.Add(1)
	}
}

func generate(ctx context.Context, node *dndm.Router, path string) {
	var t *types.Foo
	intent, err := node.Publish(path, t)
	if err != nil {
		panic(err)
	}
	defer intent.Close()
	slog.Info("generate", "loop", "start", "path", path, "type", reflect.TypeOf(t))
	for {
		select {
		case <-ctx.Done():
			slog.Info("generate foo ctx", "err", ctx.Err())
			return
		case route := <-intent.Interest():
			if err := generateFoo(ctx, path, route, intent); err != nil {
				return
			}
		}
	}
}

func generateFoo(ctx context.Context, path string, route dndm.Route, intent dndm.Intent) error {
	var t *types.Foo
	slog.Info("generateFoo", "loop", "start", "path", path, "type", reflect.TypeOf(t), "route", route)
	for {
		text := fmt.Sprintf("Message: %d for %v", sent.Add(1), reflect.TypeOf(t))
		// c, cancel := context.WithTimeout(ctx, time.Second*1)
		err := intent.Send(ctx, &types.Foo{
			Text: text,
		})
		// cancel()
		if err != nil {
			slog.Error("Failed sending Foo", "err", err)
			return err
		}
	}
}

func consume[T proto.Message](ctx context.Context, node *dndm.Router, path string) {
	var t T
	interest, err := node.Subscribe(path, t)
	if err != nil {
		panic(err)
	}
	defer interest.Close()
	slog.Info("Subscribing", "loop", "start", "path", path, "type", reflect.TypeOf(t), "C", interest.C())
	for {
		select {
		case <-ctx.Done():
			slog.Info("consume ctx", "err", ctx.Err(), "type", reflect.TypeOf(t))
			return
		case msg := <-interest.C():
			_ = msg.(T)
			recv.Add(1)
		}
	}
}
