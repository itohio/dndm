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
	"github.com/itohio/dndm/routers"
	"github.com/itohio/dndm/routers/direct"
	"github.com/itohio/dndm/routers/pipe"
	"google.golang.org/protobuf/proto"
)

//go:generate protoc -I ../../proto --proto_path=. --go_opt=paths=source_relative --go_out=. ./msg.proto

var (
	sent atomic.Uint64
	recv atomic.Uint64
)

func main() {
	d := flag.Duration("duration", time.Minute, "Duration for the benchmark test")
	n := flag.Int("N", 1, "Number of receivers")
	m := flag.Int("M", 1, "Number of senders")
	size := flag.Int("size", 3, "size of the buffers")
	t := flag.String("what", "channel", "One of channel, dndm, intent, direct, wire")
	flag.Parse()

	switch *t {
	case "dndm":
		testDNDM(*size, *n, *m, *d)
	case "channel":
		testChannel(*size, *d)
	case "intent":
		testIntent(*size, *d)
	case "direct":
		testDirect(*size, *d)
	case "wire":
		testWire(*size, *d)
	case "all":
	default:
		panic("Unknown")
	}
}

func testDNDM(size, n, m int, d time.Duration) {
	fmt.Println("----------------[ testDNDM ]-----------")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	node, err := dndm.New(dndm.WithContext(ctx), dndm.WithQueueSize(size))
	if err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		go generate(ctx, node)
	}
	for i := 0; i < m; i++ {
		go consume[*Foo](ctx, node)
	}

	c := sender(ctx, size)
	go consumer(c)

	sent.Store(0)
	recv.Store(0)
	now := time.Now()
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		cancel()
		time.Sleep(time.Second)
		fmt.Printf("Sent: %d, %.2f msgs/s\n", sent.Load(), float64(sent.Load())/time.Since(now).Seconds())
		fmt.Printf("Recv: %d, %.2f msgs/s\n", recv.Load(), float64(recv.Load())/time.Since(now).Seconds())
		fmt.Println()
	case <-ctx.Done():
		slog.Info("Stopped", "reason", ctx.Err())
	}
}

func testChannel(size int, d time.Duration) {
	fmt.Println("----------------[ testChannel ]-----------")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	c := sender(ctx, size)
	go consumer(c)

	sent.Store(0)
	recv.Store(0)
	now := time.Now()
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		cancel()
		time.Sleep(time.Second)
		fmt.Printf("Sent: %d, %.2f msgs/s\n", sent.Load(), float64(sent.Load())/time.Since(now).Seconds())
		fmt.Printf("Recv: %d, %.2f msgs/s\n", recv.Load(), float64(recv.Load())/time.Since(now).Seconds())
		fmt.Println()
	case <-ctx.Done():
		slog.Info("Stopped", "reason", ctx.Err())
	}
}

func testIntent(size int, d time.Duration) {
	fmt.Println("----------------[ testIntent ]-----------")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	c := senderIntent(ctx, size, nil)
	go consumer(c)

	sent.Store(0)
	recv.Store(0)
	now := time.Now()
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		cancel()
		time.Sleep(time.Second)
		fmt.Printf("Sent: %d, %.2f msgs/s\n", sent.Load(), float64(sent.Load())/time.Since(now).Seconds())
		fmt.Printf("Recv: %d, %.2f msgs/s\n", recv.Load(), float64(recv.Load())/time.Since(now).Seconds())
		fmt.Println()
	case <-ctx.Done():
		slog.Info("Stopped", "reason", ctx.Err())
	}
}

func testDirect(size int, d time.Duration) {
	fmt.Println("----------------[ testDirect ]-----------")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var t *Foo
	route, err := routers.NewRoute("path", t)
	if err != nil {
		panic(err)
	}
	rtr := direct.New(size)
	err = rtr.Init(ctx, slog.Default(), func(interest routers.Interest, t routers.Transport) error { return nil }, func(interest routers.Interest, t routers.Transport) error { return nil })
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

	senderIntent(ctx, size, intent.(routers.IntentInternal))
	go consumerInterest(interest)

	sent.Store(0)
	recv.Store(0)
	now := time.Now()
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		cancel()
		time.Sleep(time.Second)
		fmt.Printf("Sent: %d, %.2f msgs/s\n", sent.Load(), float64(sent.Load())/time.Since(now).Seconds())
		fmt.Printf("Recv: %d, %.2f msgs/s\n", recv.Load(), float64(recv.Load())/time.Since(now).Seconds())
		fmt.Println()
	case <-ctx.Done():
		slog.Info("Stopped", "reason", ctx.Err())
	}
}

func testWire(size int, d time.Duration) {
	fmt.Println("----------------[ testWire ]-----------")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var t *Foo
	route, err := routers.NewRoute("path", t)
	if err != nil {
		panic(err)
	}
	bridge := makeBridge(ctx)
	remoteA := pipe.NewWire("remoteA", "pipe-name", size, time.Second, bridge.A(), nil)
	remoteB := pipe.NewWire("remoteB", "pipe-name", size, time.Second, bridge.B(), nil)

	err = remoteA.Init(ctx, slog.Default(), func(interest routers.Interest, t routers.Transport) error { return nil }, func(interest routers.Interest, t routers.Transport) error { return nil })
	if err != nil {
		panic(err)
	}
	err = remoteB.Init(ctx, slog.Default(), func(interest routers.Interest, t routers.Transport) error { return nil }, func(interest routers.Interest, t routers.Transport) error { return nil })
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

	senderIntent(ctx, size, intent.(routers.IntentInternal))
	go consumerInterest(interest)

	sent.Store(0)
	recv.Store(0)
	now := time.Now()
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		cancel()
		time.Sleep(time.Second)
		fmt.Printf("Sent: %d, %.2f msgs/s\n", sent.Load(), float64(sent.Load())/time.Since(now).Seconds())
		fmt.Printf("Recv: %d, %.2f msgs/s\n", recv.Load(), float64(recv.Load())/time.Since(now).Seconds())
		fmt.Println()
	case <-ctx.Done():
		slog.Info("Stopped", "reason", ctx.Err())
	}
}

func senderIntent(ctx context.Context, size int, intent routers.IntentInternal) <-chan proto.Message {
	var t *Foo
	route, err := routers.NewRoute("path", t)
	if err != nil {
		panic(err)
	}
	if intent == nil {
		intent = routers.NewIntent(ctx, route, size, func() error { return nil })
	}
	c := make(chan proto.Message, size)
	intent.Link(c)

	go func() {
		defer intent.Link(nil)
		defer intent.Close()
		defer close(c)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			text := fmt.Sprintf("Message: %d for %v", sent.Add(1), reflect.TypeOf(t))
			err := intent.Send(ctx, &Foo{Payload: text})
			if err != nil {
				slog.Error("send failed", "err", err)
			}
		}
	}()
	return c
}

func sender(ctx context.Context, size int) <-chan proto.Message {
	c := make(chan proto.Message, size)
	var t *Foo

	go func() {
		defer close(c)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			text := fmt.Sprintf("Message: %d for %v", sent.Add(1), reflect.TypeOf(t))
			c <- &Foo{Payload: text}
		}
	}()

	return c
}

func consumer(c <-chan proto.Message) {
	for m := range c {
		msg := m.(*Foo)
		_ = msg
		recv.Add(1)
	}
}

func consumerInterest(interest routers.Interest) {
	for m := range interest.C() {
		msg := m.(*Foo)
		_ = msg
		recv.Add(1)
	}
}

func generate(ctx context.Context, node *dndm.Router) {
	var t *Foo
	intent, err := node.Publish("example.foobar", t)
	if err != nil {
		panic(err)
	}
	defer intent.Close()
	slog.Info("Publishing to example.foobar", "type", reflect.TypeOf(t))
	for {
		select {
		case <-ctx.Done():
			slog.Info("generate foo ctx", "err", ctx.Err())
			return
		case route := <-intent.Interest():
			slog.Info("Received interest", "route", route)

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				text := fmt.Sprintf("Message: %d for %v", sent.Add(1), reflect.TypeOf(t))
				c, cancel := context.WithTimeout(ctx, time.Second*10)
				err := intent.Send(c, &Foo{
					Payload: text,
				})
				cancel()
				if err != nil {
					slog.Error("Failed sending Foo", "err", err)
				}
			}
		}
	}
}

func consume[T proto.Message](ctx context.Context, node *dndm.Router) {
	var t T
	interest, err := node.Subscribe("example.foobar", t)
	if err != nil {
		panic(err)
	}
	defer interest.Close()
	slog.Info("Subscribing to example.foobar", "type", reflect.TypeOf(t))
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
