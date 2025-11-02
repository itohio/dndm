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

	// Start producer that feeds the module
	go feedModule(ctx, node)

	// Start processing module
	go runProcessingModule(ctx, node)

	slog.Info("Module example running...")
	<-ctx.Done()
	slog.Info("Stopped", "reason", ctx.Err())
}

// feedModule produces messages that will be consumed by the processing module
func feedModule(ctx context.Context, router *dndm.Router) {
	producer, err := dnbus.NewProducer[*types.Foo](ctx, router, "module.input")
	if err != nil {
		slog.Error("Failed to create producer", "err", err)
		return
	}

	slog.Info("Feeding module with messages")

	err = producer.Send(ctx, func(ctx context.Context, send func(msg *types.Foo) error) error {
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err := send(&types.Foo{
				Text: "Input message for processing",
			}); err != nil {
				slog.Error("Failed to send message", "err", err)
				return err
			}

			time.Sleep(time.Millisecond * 500)
		}
		return nil
	})

	if err != nil {
		slog.Error("Producer.Send error", "err", err)
	}
}

// runProcessingModule demonstrates using Module for multi-input/output processing
func runProcessingModule(ctx context.Context, router *dndm.Router) {
	module := dnbus.NewModule(ctx, router)
	defer module.Close()

	// Define inputs
	input, err := module.AddInput[*types.Foo]("module.input")
	if err != nil {
		slog.Error("Failed to add input", "err", err)
		return
	}

	// Define outputs
	output, err := module.AddOutput[*types.Bar]("module.output")
	if err != nil {
		slog.Error("Failed to add output", "err", err)
		return
	}

	slog.Info("Processing module created with inputs and outputs")

	// Run processing
	err = module.Run(ctx, func(ctx context.Context) error {
		// Receive from input using handler
		return input.Receive(ctx, func(ctx context.Context, msg *types.Foo) error {
			slog.Info("Processing module received message", "text", msg.Text)

			// Process (convert Foo to Bar)
			result := &types.Bar{
				A: uint32(1),
				B: uint32(2),
			}

			// Send to output using handler
			return output.Send(ctx, func(ctx context.Context, send func(msg *types.Bar) error) error {
				if err := send(result); err != nil {
					slog.Error("Failed to send to output", "err", err)
					return err
				}
				slog.Info("Processing module sent result")
				return nil
			})
		})
	})

	if err != nil {
		slog.Error("Module run error", "err", err)
	}
}

// runMultiInputOutputModule demonstrates a module with multiple inputs and outputs
func runMultiInputOutputModule(ctx context.Context, router *dndm.Router) {
	module := dnbus.NewModule(ctx, router)
	defer module.Close()

	// Define multiple inputs
	input1, err := module.AddInput[*types.Foo]("module.input1")
	if err != nil {
		slog.Error("Failed to add input1", "err", err)
		return
	}

	input2, err := module.AddInput[*types.Bar]("module.input2")
	if err != nil {
		slog.Error("Failed to add input2", "err", err)
		return
	}

	// Define multiple outputs
	output1, err := module.AddOutput[*types.Bar]("module.output1")
	if err != nil {
		slog.Error("Failed to add output1", "err", err)
		return
	}

	output2, err := module.AddOutput[*types.Foo]("module.output2")
	if err != nil {
		slog.Error("Failed to add output2", "err", err)
		return
	}

	slog.Info("Multi-input/output module created")

	// Run processing - this example shows multiple inputs/outputs
	// For simplicity, we process input1 in a goroutine and input2 in another
	// In a real scenario, you might want to synchronize inputs or handle them differently
	go func() {
		_ = input1.Receive(ctx, func(ctx context.Context, msg1 *types.Foo) error {
			// Process and send to output1
			return output1.Send(ctx, func(ctx context.Context, send func(msg *types.Bar) error) error {
				return send(&types.Bar{A: 1, B: 2})
			})
		})
	}()

	go func() {
		_ = input2.Receive(ctx, func(ctx context.Context, msg2 *types.Bar) error {
			msg2JSON, _ := json.Marshal(msg2)
			slog.Info("Received from input2", "input2", string(msg2JSON))
			// Process and send to output2
			return output2.Send(ctx, func(ctx context.Context, send func(msg *types.Foo) error) error {
				return send(&types.Foo{Text: "Result"})
			})
		})
	}()

	// Wait for context cancellation
	<-ctx.Done()
	err = ctx.Err()

	if err != nil {
		slog.Error("Module run error", "err", err)
	}
}
