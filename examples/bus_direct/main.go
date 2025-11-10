package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/direct"
	types "github.com/itohio/dndm/types/test"
	"github.com/itohio/dndm/x/bus"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	node, err := dndm.New(
		dndm.WithContext(ctx),
		dndm.WithQueueSize(3),
		dndm.WithEndpoint(direct.New(3)),
	)
	if err != nil {
		panic(err)
	}

	// Start producer
	go generateFoo(ctx, node)
	time.Sleep(time.Millisecond * 10)

	// Start consumers
	go consumeFoo(ctx, node)
	time.Sleep(time.Millisecond * 10)
	go consumeBar(ctx, node)
	time.Sleep(time.Millisecond * 10)

	// Start another producer
	go generateBar(ctx, node)

	slog.Info("Waiting for messages...")
	<-ctx.Done()
	slog.Info("Stopped", "reason", ctx.Err())
}

// generateFoo demonstrates using Producer to send messages
func generateFoo(ctx context.Context, router *dndm.Router) {
	producer, err := bus.NewProducer[*types.Foo](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create producer", "err", err)
		return
	}

	slog.Info("Producer created for Foo", "route", producer.Route())

	for i := 0; i < 20; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		text := "Message from bus Producer"
		if i == 19 {
			text = "Last message from bus Producer"
		}

		if err := producer.Send(ctx, &types.Foo{
			Text: text,
		}); err != nil {
			slog.Error("Failed to send message", "err", err)
			return
		}

		slog.Info("Sent message", "index", i)
		time.Sleep(time.Millisecond * 500)
	}

	slog.Info("Producer finished")
}

// generateBar demonstrates using Producer to send different message types
func generateBar(ctx context.Context, router *dndm.Router) {
	producer, err := bus.NewProducer[*types.Bar](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create producer", "err", err)
		return
	}

	slog.Info("Producer created for Bar", "route", producer.Route())

	i := uint32(0)
	j := uint32(1)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := producer.Send(ctx, &types.Bar{
			A: i,
			B: j,
		}); err != nil {
			slog.Error("Failed to send message", "err", err)
			return
		}

		i++
		if j == 0 {
			j = i
		}
		j *= 2
		time.Sleep(time.Millisecond * 100)
	}
}

// consumeFoo demonstrates using Consumer to receive messages
func consumeFoo(ctx context.Context, router *dndm.Router) {
	consumer, err := bus.NewConsumer[*types.Foo](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create consumer", "err", err)
		return
	}
	defer consumer.Close()

	slog.Info("Consumer created for Foo", "route", consumer.Route())

	for {
		msg, err := consumer.Receive(ctx)
		if err != nil {
			slog.Info("Consumer finished", "err", err)
			return
		}

		buf, err := json.Marshal(msg)
		if err != nil {
			slog.Error("Failed to marshal message", "err", err)
			continue
		}
		slog.Info("Received Foo message", "msg", string(buf))
	}
}

// consumeBar demonstrates using Consumer to receive different message types
func consumeBar(ctx context.Context, router *dndm.Router) {
	consumer, err := bus.NewConsumer[*types.Bar](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create consumer", "err", err)
		return
	}
	defer consumer.Close()

	slog.Info("Consumer created for Bar", "route", consumer.Route())

	for {
		msg, err := consumer.Receive(ctx)
		if err != nil {
			slog.Info("Consumer finished", "err", err)
			return
		}

		buf, err := json.Marshal(msg)
		if err != nil {
			slog.Error("Failed to marshal message", "err", err)
			continue
		}
		slog.Info("Received Bar message", "msg", string(buf))
	}
}
