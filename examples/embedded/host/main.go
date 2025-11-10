package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

	slog.Info("Host starting", "transport", transport, "path", "example.host")

	// Create Counter consumer
	counterConsumer, err := bus.NewConsumer[*types.Counter](ctx, router, "example.rp2040.counter")
	if err != nil {
		slog.Error("Failed to create counter consumer", "err", err)
		os.Exit(1)
	}
	defer counterConsumer.Close()

	// Create Control producer
	controlProducer, err := bus.NewProducer[*types.Control](ctx, router, "example.host.control")
	if err != nil {
		slog.Error("Failed to create control producer", "err", err)
		os.Exit(1)
	}
	defer controlProducer.Close()

	// Create Service caller
	caller, err := bus.NewCaller[*types.Request, *types.Response](
		ctx, router,
		"example.host.requests",
		"example.rp2040.responses",
	)
	if err != nil {
		slog.Error("Failed to create caller", "err", err)
		os.Exit(1)
	}
	defer caller.Close()

	slog.Info("Host components created")

	// Track counter for gap detection
	lastCounter := int32(0)
	counterCount := 0

	// Start counter consumer goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				counter, err := counterConsumer.Receive(ctx)
				if err != nil {
					slog.Error("Failed to receive counter", "err", err)
					return
				}

				counterCount++

				// Check for gaps every 10th counter
				if counterCount%10 == 0 {
					expectedCounter := lastCounter + 10
					if counter.Value != expectedCounter {
						slog.Error("Counter gap detected",
							"expected", expectedCounter,
							"received", counter.Value,
							"gap", counter.Value-expectedCounter)
					} else {
						slog.Info("Counter check passed", "value", counter.Value)
					}
				}

				lastCounter = counter.Value
			}
		}
	}()

	// Start keyboard input goroutine
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Enter commands:")
		fmt.Println("  control <N> - send control value")
		fmt.Println("  request <int> <float> - call service")
		fmt.Println("  quit - exit")

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			parts := strings.Split(line, " ")
			if len(parts) == 0 {
				continue
			}

			switch parts[0] {
			case "control":
				if len(parts) != 2 {
					fmt.Println("Usage: control <N>")
					continue
				}
				n, err := strconv.ParseFloat(parts[1], 32)
				if err != nil {
					fmt.Printf("Invalid number: %v\n", err)
					continue
				}

				err = controlProducer.Send(ctx, &types.Control{N: float32(n)})
				if err != nil {
					slog.Error("Failed to send control", "err", err)
				} else {
					slog.Info("Sent control", "n", n)
				}

			case "request":
				if len(parts) != 3 {
					fmt.Println("Usage: request <int> <float>")
					continue
				}

				intVal, err := strconv.ParseInt(parts[1], 10, 32)
				if err != nil {
					fmt.Printf("Invalid int: %v\n", err)
					continue
				}

				floatVal, err := strconv.ParseFloat(parts[2], 32)
				if err != nil {
					fmt.Printf("Invalid float: %v\n", err)
					continue
				}

				req := &types.Request{
					IntVal:   int32(intVal),
					FloatVal: float32(floatVal),
				}

				// Call service with timeout
				resp, err := caller.CallWithTimeout(ctx, req, time.Second*5)
				if err != nil {
					slog.Error("Service call failed", "err", err)
					continue
				}

				// Verify response
				expectedResult := float32(intVal) + float32(floatVal) + resp.ControlN
				if resp.Result != expectedResult {
					slog.Error("Response verification failed",
						"expected", expectedResult,
						"received", resp.Result)
				} else {
					slog.Info("Service call successful",
						"int_val", intVal,
						"float_val", floatVal,
						"result", resp.Result,
						"counter", resp.Counter,
						"control_n", resp.ControlN)
				}

			case "quit":
				cancel()
				return

			default:
				fmt.Printf("Unknown command: %s\n", parts[0])
			}
		}
	}()

	slog.Info("Host running - waiting for commands...")
	<-ctx.Done()
	slog.Info("Host stopped", "reason", ctx.Err())
}

func setupSerialRouter(ctx context.Context) (*dndm.Router, error) {
	// Host peer (connecting to serial)
	hostPeer, err := dndm.PeerFromString("serial:///dev/ttyACM0/example.host?baud=115200")
	if err != nil {
		return nil, err
	}

	// Embedded device peer
	embeddedPeer, err := dndm.PeerFromString("serial:///dev/ttyACM0/example.rp2040?baud=115200")
	if err != nil {
		return nil, err
	}

	// Create serial network node
	serialNode, err := serial.New(hostPeer)
	if err != nil {
		return nil, err
	}

	// Create network factory
	factory, err := network.New(serialNode)
	if err != nil {
		return nil, err
	}

	// Dial the serial connection
	rwc, err := factory.Dial(ctx, embeddedPeer)
	if err != nil {
		return nil, err
	}

	// Create stream connection
	conn := stream.NewWithContext(ctx, hostPeer, embeddedPeer, rwc, nil)

	// Create remote endpoint
	remoteEP := remote.New(hostPeer, conn, 10, time.Second*10, time.Second*3)
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
	hostPeer, err := dndm.PeerFromString("tcp://0.0.0.0:9002/example.host")
	if err != nil {
		return nil, err
	}

	// Create TCP network node
	tcpNode, err := net.New(slog.Default(), hostPeer)
	if err != nil {
		return nil, err
	}

	// Create mesh endpoint for TCP
	meshEP, err := mesh.New(
		hostPeer,
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
