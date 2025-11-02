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
	"github.com/itohio/dndm/x/dnbus"
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
	producer, err := dnbus.NewProducer[*types.Foo](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create producer", "err", err)
		return
	}

	slog.Info("Producer created for Foo", "route", producer.Route())

	// Producer.Send waits for interest and then calls the handler
	err = producer.Send(ctx, func(ctx context.Context, send func(msg *types.Foo) error) error {
		for i := 0; i < 20; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			text := "Message from dnbus Producer"
			if i == 19 {
				text = "Last message from dnbus Producer"
			}

			if err := send(&types.Foo{
				Text: text,
			}); err != nil {
				slog.Error("Failed to send message", "err", err)
				return err
			}

			slog.Info("Sent message", "index", i)
			time.Sleep(time.Millisecond * 500)
		}

		slog.Info("Producer finished")
		return nil
	})

	if err != nil {
		slog.Error("Producer.Send error", "err", err)
	}
}

// generateBar demonstrates using Producer to send different message types
func generateBar(ctx context.Context, router *dndm.Router) {
	producer, err := dnbus.NewProducer[*types.Bar](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create producer", "err", err)
		return
	}

	slog.Info("Producer created for Bar", "route", producer.Route())

	// Producer.Send waits for interest and then calls the handler
	err = producer.Send(ctx, func(ctx context.Context, send func(msg *types.Bar) error) error {
		i := uint32(0)
		j := uint32(1)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err := send(&types.Bar{
				A: i,
				B: j,
			}); err != nil {
				slog.Error("Failed to send message", "err", err)
				return err
			}

			i++
			if j == 0 {
				j = i
			}
			j *= 2
			time.Sleep(time.Millisecond * 100)
		}
	})

	if err != nil {
		slog.Error("Producer.Send error", "err", err)
	}
}

// consumeFoo demonstrates using Consumer to receive messages
func consumeFoo(ctx context.Context, router *dndm.Router) {
	consumer, err := dnbus.NewConsumer[*types.Foo](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create consumer", "err", err)
		return
	}
	defer consumer.Close()

	slog.Info("Consumer created for Foo", "route", consumer.Route())

	// Consumer.Receive calls handler for each message
	err = consumer.Receive(ctx, func(ctx context.Context, msg *types.Foo) error {
		buf, err := json.Marshal(msg)
		if err != nil {
			slog.Error("Failed to marshal message", "err", err)
			return nil // Continue receiving
		}
		slog.Info("Received Foo message", "msg", string(buf))
		return nil // Continue receiving
	})

	if err != nil {
		slog.Info("Consumer finished", "err", err)
	}
}

// consumeBar demonstrates using Consumer to receive different message types
func consumeBar(ctx context.Context, router *dndm.Router) {
	consumer, err := dnbus.NewConsumer[*types.Bar](ctx, router, "example.foobar")
	if err != nil {
		slog.Error("Failed to create consumer", "err", err)
		return
	}
	defer consumer.Close()

	slog.Info("Consumer created for Bar", "route", consumer.Route())

	// Consumer.Receive calls handler for each message
	err = consumer.Receive(ctx, func(ctx context.Context, msg *types.Bar) error {
		buf, err := json.Marshal(msg)
		if err != nil {
			slog.Error("Failed to marshal message", "err", err)
			return nil // Continue receiving
		}
		slog.Info("Received Bar message", "msg", string(buf))
		return nil // Continue receiving
	})

	if err != nil {
		slog.Info("Consumer finished", "err", err)
	}
}
