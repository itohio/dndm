package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/endpoint/mesh"
	"github.com/itohio/dndm/endpoint/remote"
	"github.com/itohio/dndm/network"
	"github.com/itohio/dndm/network/net"
	"github.com/itohio/dndm/network/serial"
	"github.com/itohio/dndm/network/stream"
	"github.com/itohio/dndm/x/bus"

	types "github.com/itohio/dndm/examples/embedded/types"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Determine transport type from environment variable
	transport := os.Getenv("TRANSPORT")
	if transport == "" {
		transport = "serial"
	}

	var router *dndm.Router
	var err error

	switch transport {
	case "serial":
		router, err = setupSerialRouter(ctx)
	case "udp":
		router, err = setupUDPRouter(ctx)
	default:
		slog.Error("Unknown transport", "transport", transport)
		os.Exit(1)
	}

	if err != nil {
		slog.Error("Failed to setup router", "err", err)
		os.Exit(1)
	}

	slog.Info("Embedded device starting", "transport", transport, "path", "example.rp2040")

	// Create Counter producer
	counterProducer, err := bus.NewProducer[*types.Counter](ctx, router, "example.rp2040.counter")
	if err != nil {
		slog.Error("Failed to create counter producer", "err", err)
		os.Exit(1)
	}
	defer counterProducer.Close()

	// Create Control consumer
	controlConsumer, err := bus.NewConsumer[*types.Control](ctx, router, "example.host.control")
	if err != nil {
		slog.Error("Failed to create control consumer", "err", err)
		os.Exit(1)
	}
	defer controlConsumer.Close()

	// Create Service for requests
	service, err := bus.NewService[*types.Request, *types.Response](
		ctx, router,
		"example.host.requests",
		"example.rp2040.responses",
	)
	if err != nil {
		slog.Error("Failed to create service", "err", err)
		os.Exit(1)
	}
	defer service.Close()

	slog.Info("Embedded components created")

	// Start counter ticker
	counter := int32(0)
	controlN := float32(0.0)

	// Start counter goroutine
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counter++
				err := counterProducer.Send(ctx, &types.Counter{Value: counter})
				if err != nil {
					slog.Error("Failed to send counter", "err", err)
					return
				}
			}
		}
	}()

	// Start control consumer goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				control, err := controlConsumer.Receive(ctx)
				if err != nil {
					slog.Error("Failed to receive control", "err", err)
					return
				}
				controlN = control.N
				slog.Info("Received control", "n", controlN)
			}
		}
	}()

	// Start service handler
	go func() {
		err := service.Handle(ctx, func(ctx context.Context, req *types.Request, reply func(resp *types.Response) error) error {
			slog.Info("Service received request", "int_val", req.IntVal, "float_val", req.FloatVal)

			// Compute result: int_val + float_val + controlN
			result := float32(req.IntVal) + req.FloatVal + controlN

			resp := &types.Response{
				Result:   result,
				Counter:  counter,
				ControlN: controlN,
			}

			return reply(resp)
		})

		if err != nil {
			slog.Error("Service handler error", "err", err)
		}
	}()

	slog.Info("Embedded device running")
	<-ctx.Done()
	slog.Info("Embedded device stopped", "reason", ctx.Err())
}

func setupSerialRouter(ctx context.Context) (*dndm.Router, error) {
	// Embedded device peer (listening on serial)
	embeddedPeer, err := dndm.PeerFromString("serial:///dev/ttyACM0/example.rp2040?baud=115200")
	if err != nil {
		return nil, err
	}

	// Create serial network node
	serialNode, err := serial.New(embeddedPeer)
	if err != nil {
		return nil, err
	}

	// Create network factory
	factory, err := network.New(serialNode)
	if err != nil {
		return nil, err
	}

	// Dial the serial connection (embedded device acts as server)
	rwc, err := factory.Dial(ctx, embeddedPeer)
	if err != nil {
		return nil, err
	}

	// Create stream connection
	hostPeer, err := dndm.PeerFromString("serial:///dev/ttyACM0/example.host?baud=115200")
	if err != nil {
		return nil, err
	}

	conn := stream.NewWithContext(ctx, embeddedPeer, hostPeer, rwc, nil)

	// Create remote endpoint
	remoteEP := remote.New(embeddedPeer, conn, 10, time.Second*10, time.Second*3)
	err = remoteEP.Init(ctx, slog.Default(),
		func(intent dndm.Intent, ep dndm.Endpoint) error { return nil },
		func(interest dndm.Interest, ep dndm.Endpoint) error { return nil },
	)
	if err != nil {
		return nil, err
	}

	// Create router with remote endpoint
	router, err := dndm.New(
		dndm.WithContext(ctx),
		dndm.WithQueueSize(10),
		dndm.WithEndpoint(remoteEP),
	)
	if err != nil {
		return nil, err
	}

	return router, nil
}

func setupUDPRouter(ctx context.Context) (*dndm.Router, error) {
	// For TCP testing in docker-compose setup
	embeddedPeer, err := dndm.PeerFromString("tcp://0.0.0.0:9001/example.rp2040")
	if err != nil {
		return nil, err
	}

	// Create TCP network node
	tcpNode, err := net.New(slog.Default(), embeddedPeer)
	if err != nil {
		return nil, err
	}

	// Create mesh endpoint for TCP
	meshEP, err := mesh.New(
		embeddedPeer,
		10,             // buffer size
		5,              // num dialers
		time.Second*10, // timeout
		time.Second*3,  // ping duration
		tcpNode,        // dialer
		nil,            // initial peers
	)
	if err != nil {
		return nil, err
	}

	// Create router
	router, err := dndm.New(
		dndm.WithContext(ctx),
		dndm.WithEndpoint(meshEP),
	)
	if err != nil {
		return nil, err
	}

	return router, nil
}
