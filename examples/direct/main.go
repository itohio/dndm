package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	reflect "reflect"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/direct"
	types "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	node, err := dndm.New(dndm.WithContext(ctx), dndm.WithQueueSize(3), dndm.WithEndpoint(direct.New(3)))
	if err != nil {
		panic(err)
	}

	go generateFoo(ctx, node)
	time.Sleep(time.Millisecond)
	go consume[*types.Foo](ctx, node)
	time.Sleep(time.Millisecond)
	go monitorFoo(ctx, node)
	go consume[*types.Bar](ctx, node)
	slog.Info("waiting for 5s...")
	time.Sleep(time.Second * 5)
	go generateBar(ctx, node)

	<-ctx.Done()
	slog.Info("Stopped", "reason", ctx.Err())
}

func generateFoo(ctx context.Context, node *dndm.Router) {
	var t *types.Foo
	intent, err := node.Publish("example.foobar", t)
	if err != nil {
		panic(err)
	}
	defer intent.Close()
	slog.Info("Publishing to example.foobar", "type", reflect.TypeOf(t))
	i := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("generate foo ctx", "err", ctx.Err())
			return
		case route := <-intent.Interest():
			slog.Info("Received interest", "route", route)

			for i < 20 {
				time.Sleep(time.Millisecond * 500)
				select {
				case <-ctx.Done():
					return
				default:
				}

				text := fmt.Sprintf("Message: %d for %v", i, reflect.TypeOf(t))
				if i == 19 {
					text = "This is the last message"
				}
				c, cancel := context.WithTimeout(ctx, time.Second*10)
				err := intent.Send(c, &types.Foo{
					Text: text,
				})
				i++
				cancel()
				if err != nil {
					slog.Error("Failed sending Foo", "err", err)
				}
			}
		}
	}
}

func generateBar(ctx context.Context, node *dndm.Router) {
	var t *types.Bar
	intent, err := node.Publish("example.foobar", t)
	if err != nil {
		panic(err)
	}
	defer intent.Close()
	slog.Info("Publishing to example.foobar", "type", reflect.TypeOf(t))
	i := uint32(0)
	j := uint32(1)
	for {
		select {
		case <-ctx.Done():
			slog.Info("generate bar ctx", "err", ctx.Err())
			return
		case route := <-intent.Interest():
			slog.Info("Received interest", "route", route)

			for {
				time.Sleep(time.Millisecond * 100)
				select {
				case <-ctx.Done():
					return
				default:
				}
				c, cancel := context.WithTimeout(ctx, time.Millisecond)
				err := intent.Send(c, &types.Bar{
					A: i,
					B: j,
				})
				i++
				if j == 0 {
					j = i
				}
				j *= 2
				cancel()
				if err != nil {
					slog.Error("Failed sending Bar", "err", err)
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
			m := msg.(T)

			buf, err := json.Marshal(m)
			if err != nil {
				panic(err)
			}
			slog.Info("Message", "route", interest.Route(), "msg", string(buf), "type", reflect.TypeOf(msg))
		}
	}
}

// monitorFoo will receive messages on Foo interest and will restart Foo once after 3 seconds of no messages
func monitorFoo(ctx context.Context, node *dndm.Router) {
	var t *types.Foo
	interest, err := node.Subscribe("example.foobar", t)
	if err != nil {
		panic(err)
	}
	slog.Info("Subscribing to example.foobar for monitoring", "type", reflect.TypeOf(t))
	timer := time.NewTimer(time.Second * 3)
	for {
		select {
		case <-ctx.Done():
			slog.Info("consume ctx", "err", ctx.Err(), "type", reflect.TypeOf(t))
			return
		case <-timer.C:
			slog.Info("no more messages for Foo, starting Foo")
			go generateFoo(ctx, node)
			return
		case msg := <-interest.C():
			m := msg.(*types.Foo)
			_ = m
			timer.Reset(time.Second * 10)
			slog.Info("Monitor", "type", reflect.TypeOf(msg))
		}
	}
}
