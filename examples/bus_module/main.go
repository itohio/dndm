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
	producer, err := bus.NewProducer[*types.Foo](ctx, router, "module.input")
	if err != nil {
		slog.Error("Failed to create producer", "err", err)
		return
	}

	slog.Info("Feeding module with messages")

	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := producer.Send(ctx, &types.Foo{
			Text: "Input message for processing",
		}); err != nil {
			slog.Error("Failed to send message", "err", err)
			return
		}

		time.Sleep(time.Millisecond * 500)
	}
}

// runProcessingModule demonstrates using Module for multi-input/output processing
func runProcessingModule(ctx context.Context, router *dndm.Router) {
	module := bus.NewModule(ctx, router)
	defer module.Close()

	// Define inputs
	input, err := bus.AddInput[*types.Foo](module, "module.input")
	if err != nil {
		slog.Error("Failed to add input", "err", err)
		return
	}

	// Define outputs
	output, err := bus.AddOutput[*types.Bar](module, "module.output")
	if err != nil {
		slog.Error("Failed to add output", "err", err)
		return
	}

	slog.Info("Processing module created with inputs and outputs")

	// Run processing
	err = module.Run(ctx, func(ctx context.Context) error {
		for {
			msg, err := input.Receive(ctx)
			if err != nil {
				return err
			}

			slog.Info("Processing module received message", "text", msg.Text)

			// Process (convert Foo to Bar)
			result := &types.Bar{
				A: uint32(1),
				B: uint32(2),
			}

			if err := output.Send(ctx, result); err != nil {
				slog.Error("Failed to send to output", "err", err)
				return err
			}
			slog.Info("Processing module sent result")
		}
	})

	if err != nil {
		slog.Error("Module run error", "err", err)
	}
}

// runMultiInputOutputModule demonstrates a module with multiple inputs and outputs
func runMultiInputOutputModule(ctx context.Context, router *dndm.Router) {
	module := bus.NewModule(ctx, router)
	defer module.Close()

	// Define multiple inputs
	input1, err := bus.AddInput[*types.Foo](module, "module.input1")
	if err != nil {
		slog.Error("Failed to add input1", "err", err)
		return
	}

	input2, err := bus.AddInput[*types.Bar](module, "module.input2")
	if err != nil {
		slog.Error("Failed to add input2", "err", err)
		return
	}

	// Define multiple outputs
	output1, err := bus.AddOutput[*types.Bar](module, "module.output1")
	if err != nil {
		slog.Error("Failed to add output1", "err", err)
		return
	}

	output2, err := bus.AddOutput[*types.Foo](module, "module.output2")
	if err != nil {
		slog.Error("Failed to add output2", "err", err)
		return
	}

	slog.Info("Multi-input/output module created")

	// Run processing - this example shows multiple inputs/outputs
	// For simplicity, we process input1 in a goroutine and input2 in another
	// In a real scenario, you might want to synchronize inputs or handle them differently
	go func() {
		for {
			if _, err := input1.Receive(ctx); err != nil {
				return
			}
			// Process and send to output1
			if err := output1.Send(ctx, &types.Bar{A: 1, B: 2}); err != nil {
				slog.Error("Failed to send output1", "err", err)
				return
			}
		}
	}()

	go func() {
		for {
			msg2, err := input2.Receive(ctx)
			if err != nil {
				return
			}
			msg2JSON, _ := json.Marshal(msg2)
			slog.Info("Received from input2", "input2", string(msg2JSON))
			// Process and send to output2
			if err := output2.Send(ctx, &types.Foo{Text: "Result"}); err != nil {
				slog.Error("Failed to send output2", "err", err)
				return
			}
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	err = ctx.Err()

	if err != nil {
		slog.Error("Module run error", "err", err)
	}
}
