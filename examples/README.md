# DNDM Examples

This directory contains examples demonstrating how to use the DNDM library for different communication scenarios.

## Core Concepts

### Router (Node)

A **Router** (also called **Node** in examples) is the top-level component that manages all endpoints and routes messages between publishers and subscribers.

```go
router, err := dndm.New(
    dndm.WithContext(ctx),
    dndm.WithQueueSize(10),
    dndm.WithEndpoint(direct.New(10)),
)
```

The Router:
- Manages multiple endpoints (Direct, Remote, Mesh)
- Routes intents and interests across endpoints
- Handles message routing between publishers and subscribers
- Provides unified API for publishing and subscribing

### Intent

An **Intent** represents a publisher's desire to send data on a specific route. It's created when you call `Publish()`.

```go
var msg *MyMessage
intent, err := router.Publish("sensors.temperature", msg)
```

**Key Features**:
- Declares availability to publish data
- Receives notifications when subscribers are interested (`intent.Interest()` channel)
- Sends messages via `intent.Send(ctx, message)`
- Automatically links with matching Interests

**Usage Pattern**:
```go
intent, err := router.Publish("example.data", &MyMessage{})
defer intent.Close()

// Wait for subscribers to be interested
select {
case route := <-intent.Interest():
    // Now we can send data
    intent.Send(ctx, &MyMessage{Data: "hello"})
}
```

### Interest

An **Interest** represents a subscriber's desire to receive data on a specific route. It's created when you call `Subscribe()`.

```go
var msg *MyMessage
interest, err := router.Subscribe("sensors.temperature", msg)
```

**Key Features**:
- Declares desire to receive data
- Receives messages via `interest.C()` channel
- Automatically links with matching Intents
- Type-safe message delivery

**Usage Pattern**:
```go
interest, err := router.Subscribe("example.data", &MyMessage{})
defer interest.Close()

// Receive messages
for msg := range interest.C() {
    data := msg.(*MyMessage)
    process(data)
}
```

### Endpoint

An **Endpoint** is an abstraction layer for different communication mechanisms. It handles the transport-specific details while providing a unified interface.

There are three main endpoint types:

1. **Direct Endpoint** - In-process communication
2. **Remote Endpoint** - Cross-process/system communication
3. **Mesh Endpoint** - Distributed full-mesh network

## Endpoint Types Comparison

### Direct Endpoint (`endpoint/direct`)

**Use Case**: Communication within the same process (same application)

**Characteristics**:
- **Zero-copy**: Uses Go channels for message passing
- **Fastest**: No serialization overhead
- **Simple**: Perfect for multi-module applications
- **Transport**: Go channels (in-memory)

**When to Use**:
- Multiple modules in the same Go process
- Internal message bus within an application
- High-performance in-process communication

**Example**:
```go
import "github.com/itohio/dndm/endpoint/direct"

router, err := dndm.New(
    dndm.WithContext(ctx),
    dndm.WithQueueSize(10),
    dndm.WithEndpoint(direct.New(10)),
)
```

### Remote Endpoint (`endpoint/remote`)

**Use Case**: Communication between different processes or systems on the same machine or network

**Characteristics**:
- **Network-based**: Uses TCP/UDP/Serial connections
- **Serialization**: Messages are encoded/decoded automatically
- **Peer-to-Peer**: Direct connection between two endpoints
- **Transport**: TCP/UDP sockets or serial ports

**When to Use**:
- Communication between separate processes
- Client-server architectures
- Point-to-point connections
- Known peer addresses

**Example**:
```go
import (
    "github.com/itohio/dndm/endpoint/remote"
    "github.com/itohio/dndm/network/stream"
)

// Create a network connection (TCP/UDP/Serial)
conn := stream.New(ctx, localPeer, remotePeer, rw, handlers)

// Create remote endpoint
remoteEP := remote.New(localPeer, conn, 10, time.Second*10, time.Second*3)
```

### Mesh Endpoint (`endpoint/mesh`)

**Use Case**: Distributed full-mesh network with automatic peer discovery

**Characteristics**:
- **Automatic Discovery**: Peers discover each other automatically
- **Full-Mesh**: All peers can communicate with each other
- **Path-based Routing**: Routes messages based on peer path prefixes
- **Multiple Transports**: Can use TCP/UDP/Serial simultaneously

**When to Use**:
- Multiple computers/devices on a LAN
- Automatic peer discovery needed
- Distributed systems with known network topology
- Robotics applications with multiple SBCs/devices

**Example**:
```go
import (
    "github.com/itohio/dndm/endpoint/mesh"
    "github.com/itohio/dndm/network"
    "github.com/itohio/dndm/network/net"
)

// Create network factory
factory, _ := network.New(
    net.New(logger, peer),
)

// Create mesh endpoint
meshEP, _ := mesh.New(
    peer,
    10,                    // buffer size
    5,                     // num dialers
    time.Second*10,        // timeout
    time.Second*3,         // ping duration
    factory,
    initialPeers,          // known peers (optional)
)
```

## Example Programs

### `direct/main.go`

Demonstrates in-process communication using the Direct endpoint.

**Key Features**:
- Multiple publishers on the same route
- Multiple subscribers on the same route
- Type-safe message passing (Foo vs Bar)
- Dynamic publisher/subscriber creation

**Run**:
```bash
go run examples/direct/main.go
```

**What it does**:
1. Creates a Router with Direct endpoint
2. Starts multiple publishers (`generateFoo`, `generateBar`)
3. Starts multiple subscribers (`consume`)
4. Shows how Intents wait for Interests before sending
5. Demonstrates type-safe routing (Foo and Bar on same path but different types)

### `node/main.go`

Demonstrates distributed mesh network using the Mesh endpoint.

**Key Features**:
- Command-line configuration of peers
- Automatic peer discovery
- Full-mesh network topology
- TCP-based communication

**Run**:
```bash
# Terminal 1
go run examples/node/main.go -n tcp://localhost:1234/peer1

# Terminal 2
go run examples/node/main.go -n tcp://localhost:1235/peer2 -P tcp://localhost:1234/peer1

# Terminal 3
go run examples/node/main.go -n tcp://localhost:1236/peer3 -P tcp://localhost:1234/peer1 -p "example.foobar" -c "example.foobar"
```

**What it does**:
1. Creates a Mesh endpoint with TCP network
2. Connects to known peers (`-P` flag)
3. Automatically discovers other peers
4. Publishes/subscribes to routes across the mesh

### `bench/main.go`

Benchmarking tool for different communication patterns.

**Run**:
```bash
go run examples/bench/main.go -what direct -duration 10s -N 1 -M 1
```

**Options**:
- `-what`: Type of test (channel, dndm, direct, remote)
- `-duration`: Test duration
- `-N`: Number of receivers
- `-M`: Number of senders

## Serial Communication Example

Here's a complete example showing how to communicate with an embedded device (e.g., RP2040 or ESP32) via serial port:

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "time"

    "github.com/itohio/dndm"
    "github.com/itohio/dndm/endpoint/remote"
    "github.com/itohio/dndm/network"
    "github.com/itohio/dndm/network/serial"
    "github.com/itohio/dndm/network/stream"
    "google.golang.org/protobuf/proto"
)

// SensorData is a protobuf message from the embedded device
type SensorData struct {
    Temperature float32
    Humidity    float32
    Pressure    float32
}

// MotorControl is a protobuf message to the embedded device
type MotorControl struct {
    Speed   int32
    Direction int32
}

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // Create serial node for embedded device
    // Format: serial:///dev/ttyUSB0/path?baud=115200
    embeddedPeer, _ := dndm.PeerFromString("serial:///dev/ttyUSB0/embedded.sensors?baud=115200")
    serialNode, _ := serial.New(embeddedPeer)
    
    factory, _ := network.New(serialNode)

    // Create router with remote endpoint
    router, _ := dndm.New(
        dndm.WithContext(ctx),
        dndm.WithQueueSize(10),
    )

    // Dial the embedded device
    rwc, _ := factory.Dial(ctx, embeddedPeer)
    
    // Create stream connection
    localPeer, _ := dndm.PeerFromString("tcp://localhost:0/rpi.processor")
    conn := stream.NewWithContext(ctx, localPeer, embeddedPeer, rwc, nil)

    // Create remote endpoint
    remoteEP := remote.New(localPeer, conn, 10, time.Second*10, time.Second*3)
    remoteEP.Init(ctx, slog.Default(),
        func(intent dndm.Intent, ep dndm.Endpoint) error { return nil },
        func(interest dndm.Interest, ep dndm.Endpoint) error { return nil },
    )

    // Add remote endpoint to router
    router, _ = dndm.New(
        dndm.WithContext(ctx),
        dndm.WithEndpoint(remoteEP),
    )

    // Subscribe to sensor data from embedded device
    var sensorData *SensorData
    go receiveSensorData(ctx, router, sensorData)

    // Publish motor control commands to embedded device
    var motorControl *MotorControl
    go sendMotorControl(ctx, router, motorControl)

    <-ctx.Done()
}

func receiveSensorData(ctx context.Context, router *dndm.Router, msg *SensorData) {
    // Subscribe to sensor data route
    interest, err := router.Subscribe("sensors.data", msg)
    if err != nil {
        panic(err)
    }
    defer interest.Close()

    slog.Info("Subscribed to sensors.data", "route", interest.Route())

    for {
        select {
        case <-ctx.Done():
            return
        case data := <-interest.C():
            sensor := data.(*SensorData)
            slog.Info("Received sensor data",
                "temp", sensor.Temperature,
                "humidity", sensor.Humidity,
                "pressure", sensor.Pressure,
            )
            // Process sensor data...
        }
    }
}

func sendMotorControl(ctx context.Context, router *dndm.Router, msg *MotorControl) {
    // Publish motor control route
    intent, err := router.Publish("actuators.motors", msg)
    if err != nil {
        panic(err)
    }
    defer intent.Close()

    slog.Info("Publishing motor control", "route", intent.Route())

    // Wait for embedded device to subscribe
    select {
    case <-ctx.Done():
        return
    case <-intent.Interest():
        slog.Info("Embedded device is ready, sending motor commands")
    }

    // Send motor commands
    for {
        select {
        case <-ctx.Done():
            return
        default:
            // Example: Set motor speed and direction
            err := intent.Send(ctx, &MotorControl{
                Speed:     50,
                Direction: 1,
            })
            if err != nil {
                slog.Error("Failed to send motor control", "err", err)
            }
            time.Sleep(time.Second)
        }
    }
}
```

### Serial Configuration Options

The serial endpoint supports configuration via peer URL parameters:

```go
// Basic serial port
peer := "serial:///dev/ttyUSB0/path"

// With baud rate
peer := "serial:///dev/ttyUSB0/path?baud=115200"

// Full configuration
peer := "serial:///dev/ttyUSB0/path?baud=115200&parity=N&stop=1&bits=8"
```

**Parameters**:
- `baud`: Baud rate (default: 115200)
- `parity`: Parity (N, E, O)
- `stop`: Stop bits (1, 1.5, 2)
- `bits`: Data bits (5, 6, 7, 8)

### Complete Serial Example with Mesh

For a more realistic scenario where multiple devices communicate:

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "runtime"
    "time"

    "github.com/itohio/dndm"
    "github.com/itohio/dndm/endpoint/mesh"
    "github.com/itohio/dndm/network"
    "github.com/itohio/dndm/network/net"
    "github.com/itohio/dndm/network/serial"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // Local peer (Raspberry Pi)
    localPeer, _ := dndm.PeerFromString("tcp://192.168.1.100:8080/rpi.controller")
    
    // Embedded device peer (via serial)
    embeddedPeer, _ := dndm.PeerFromString("serial:///dev/ttyUSB0/embedded.sensors?baud=115200")

    // Create network factory with both TCP and Serial
    factory, _ := network.New(
        net.New(slog.Default(), localPeer),      // TCP for LAN
        serial.New(embeddedPeer),                // Serial for embedded
    )

    // Create mesh endpoint
    meshEP, _ := mesh.New(
        localPeer,
        10,                    // buffer size
        runtime.NumCPU()*5,    // num dialers
        time.Second*10,        // timeout
        time.Second*3,         // ping duration
        factory,
        nil,                   // initial peers (optional)
    )

    // Create router
    router, _ := dndm.New(
        dndm.WithContext(ctx),
        dndm.WithEndpoint(meshEP),
    )

    // Now router can communicate with:
    // - Other computers on LAN (via TCP)
    // - Embedded device (via Serial)
    // - Automatic discovery and routing

    <-ctx.Done()
}
```

## Routing Behavior

### Route Format

Routes follow the format: `TypeName@path`

Examples:
- `SensorData@sensors.temperature`
- `CameraImage@cameras.front`
- `MotorControl@actuators.motors`

### Intent-Interest Matching

- **Exact Match**: Intent and Interest must have the same route (TypeName@path)
- **Automatic Linking**: When an Intent and Interest match, they are automatically linked
- **Notification**: Intent receives notification on `Interest()` channel when linked
- **Delivery**: Messages flow from Intent → Interest automatically

### Multiple Publishers/Subscribers

- **Multiple Publishers**: All publishers on the same route send to all subscribers
- **Multiple Subscribers**: Each subscriber receives messages from all publishers
- **Fan-out/Fan-in**: Supported automatically via IntentRouter/InterestRouter

## Best Practices

1. **Always defer Close()**: Clean up Intents and Interests when done
2. **Wait for Interest()**: Publishers should wait for subscribers before sending
3. **Type Safety**: Use protobuf message types for compile-time safety
4. **Context Management**: Use context for cancellation and timeouts
5. **Error Handling**: Check errors from Publish/Subscribe operations
6. **Route Naming**: Use hierarchical paths (e.g., `sensors.temperature`, `cameras.front`)

## Common Patterns

### Publisher Pattern

```go
intent, _ := router.Publish("example.data", &MyMessage{})
defer intent.Close()

// Wait for subscriber
<-intent.Interest()

// Send data
intent.Send(ctx, &MyMessage{Data: "hello"})
```

### Subscriber Pattern

```go
interest, _ := router.Subscribe("example.data", &MyMessage{})
defer interest.Close()

for msg := range interest.C() {
    data := msg.(*MyMessage)
    process(data)
}
```

### Bidirectional Communication

```go
// Device A: Publish and Subscribe
intentA, _ := router.Publish("deviceA.output", &Output{})
interestA, _ := router.Subscribe("deviceB.output", &Output{})

// Device B: Publish and Subscribe
intentB, _ := router.Publish("deviceB.output", &Output{})
interestB, _ := router.Subscribe("deviceA.output", &Output{})
```

## Troubleshooting

### No messages received

1. Check that both Intent and Interest are created
2. Verify routes match exactly (TypeName@path)
3. Wait for Interest() notification before sending
4. Check endpoint is properly initialized

### Type mismatches

1. Ensure protobuf message types match
2. Use same type for Publish and Subscribe
3. Check protobuf message definitions

### Serial connection issues

1. Verify device path (e.g., `/dev/ttyUSB0`)
2. Check baud rate matches device
3. Ensure device has correct permissions
4. Verify serial port is not in use by another process

