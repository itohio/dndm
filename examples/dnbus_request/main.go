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

// Request message type (using Foo as request)
// In real applications, you would define proper request/response types in protobuf
type Request = types.Foo
type Response = types.Bar

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

	// Start service handler
	go runService(ctx, node)
	time.Sleep(time.Millisecond * 100)

	// Start caller
	go runCaller(ctx, node)
	time.Sleep(time.Millisecond * 100)

	slog.Info("Request/Reply example running...")
	<-ctx.Done()
	slog.Info("Stopped", "reason", ctx.Err())
}

// runService demonstrates using Service to handle requests
func runService(ctx context.Context, router *dndm.Router) {
	service, err := dnbus.NewService[*Request, *Response](
		ctx, router,
		"example.requests",
		"example.responses",
	)
	if err != nil {
		slog.Error("Failed to create service", "err", err)
		return
	}
	defer service.Close()

	slog.Info("Service created", "request_path", "example.requests", "response_path", "example.responses")

	// Handle requests with callback (each request handled in goroutine)
	err = service.Handle(ctx, func(ctx context.Context, req *Request, reply func(resp *Response) error) error {
		slog.Info("Service received request", "text", req.Text)

		// Process request and send reply
		// In a real application, you would do actual processing here
		resp := &Response{
			A: uint32(1),
			B: uint32(2),
		}

		// Reply uses producer.Send internally which waits for interest
		// For request/reply, this should be fast since interest exists
		return reply(resp)
	})

	if err != nil {
		slog.Error("Service error", "err", err)
	}
}

// runServiceMultipleReplies demonstrates sending multiple replies per request
func runServiceMultipleReplies(ctx context.Context, router *dndm.Router) {
	service, err := dnbus.NewService[*Request, *Response](
		ctx, router,
		"example.requests.stream",
		"example.responses.stream",
	)
	if err != nil {
		slog.Error("Failed to create service", "err", err)
		return
	}
	defer service.Close()

	// Handle requests - can send multiple replies
	err = service.Handle(ctx, func(ctx context.Context, req *Request, reply func(resp *Response) error) error {
		// Send initial acknowledgment
		if err := reply(&Response{A: 0, B: 0}); err != nil {
			return err
		}

		// Stream multiple updates
		for i := 1; i <= 5; i++ {
			if err := reply(&Response{
				A: uint32(i),
				B: uint32(i * 2),
			}); err != nil {
				return err
			}
			time.Sleep(time.Millisecond * 200)
		}

		// Send final response
		return reply(&Response{A: 999, B: 999})
	})

	if err != nil {
		slog.Error("Service error", "err", err)
	}
}

// runCaller demonstrates using Caller for single request/reply
func runCaller(ctx context.Context, router *dndm.Router) {
	caller, err := dnbus.NewCaller[*Request, *Response](
		ctx, router,
		"example.requests",
		"example.responses",
	)
	if err != nil {
		slog.Error("Failed to create caller", "err", err)
		return
	}
	defer caller.Close()

	slog.Info("Caller created")

	// Send multiple requests
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req := &Request{
			Text: "Request from dnbus Caller",
		}

		// Call with timeout
		resp, err := caller.CallWithTimeout(ctx, req, time.Second*5)
		if err != nil {
			slog.Error("Call failed", "err", err)
			continue
		}

		buf, _ := json.Marshal(resp)
		slog.Info("Caller received response", "response", string(buf))

		time.Sleep(time.Second)
	}
}

// runCallerHandleReplies demonstrates using Caller with HandleReplies for multiple responses
func runCallerHandleReplies(ctx context.Context, router *dndm.Router) {
	caller, err := dnbus.NewCaller[*Request, *Response](
		ctx, router,
		"example.requests.stream",
		"example.responses.stream",
	)
	if err != nil {
		slog.Error("Failed to create caller", "err", err)
		return
	}
	defer caller.Close()

	req := &Request{
		Text: "Stream request",
	}

	// Handle multiple replies via callback
	err = caller.HandleReplies(ctx, req, func(ctx context.Context, req *Request, resp *Response) error {
		buf, _ := json.Marshal(resp)
		slog.Info("Caller received stream response", "response", string(buf))

		// Can choose to stop receiving or continue
		if resp.A == 999 {
			slog.Info("Received final response, stopping")
			return nil // Stop handling
		}
		return nil // Continue handling more replies
	})

	if err != nil {
		slog.Error("HandleReplies error", "err", err)
	}
}
