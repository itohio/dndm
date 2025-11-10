# High-Level Specification for DNDM Auxiliary Packages (`x/`)

## Purpose

The `x/` workspace houses higher-level utilities that sit on top of the core **Decentralized Named Data Messaging (DNDM)** runtime. Its mission is to reduce the cognitive load for application developers—especially teams building embedded sensors, gateways, and distributed processing nodes—while preserving the flexibility and topology-agnostic nature of the core library.

This document catalogues DNDM principles and philosophy, contrasts the model with conventional pub/sub systems such as NATS, and outlines the primitive building blocks that `x/` packages (notably `x/bus` and the planned `x/node`) should expose. Each section includes illustrative code snippets.

---

## DNDM Principles

1. **Named Data, Not Addressed Peers**
   - Every unit of communication is identified by a *Route* (`TypeName@path`), not by an endpoint address.
   - Producers (“intents”) and consumers (“interests”) meet via the shared name; their physical location is irrelevant.

2. **Intent / Interest Handshake**
   - An **Intent** declares a willingness to provide data for a route.
   - An **Interest** declares a desire to receive data for that route.
   - DNDM’s linker automatically connects matching intents and interests and notifies the intent that someone is listening.

3. **Topology Agnosticism**
   - Routers manage intents and interests without knowing the network topology.
   - **Endpoints** (direct, remote, mesh, etc.) encapsulate transport concerns (serial, UDP, TCP, etc.).
   - Discovery logic is pluggable; the core remains unaware of network shape.

4. **Typed Messaging**
   - Routes couple a protobuf message type with a human-readable path.
   - Delivering a route with a mismatched type is an error, providing stronger guarantees than untyped pub/sub systems.

5. **Zero-Copy Intra-process Delivery**
   - Direct endpoints use Go channels to avoid unnecessary copies.

6. **Decentralized Discovery**
   - Interests can find intents across multi-hop networks through interest advertisements, caching, or gossip (implementation-specific).

7. **Transport Efficiency**
   - The runtime is free to optimize delivery (broadcast, multicast, unicast, batching) depending on endpoint capabilities.
   - Intents/Interests remain ignorant of delivery mechanics; they simply send/receive.

---

## Philosophy & Differentiators vs. Pub/Sub (e.g., NATS)

| Aspect | DNDM | NATS / typical pub-sub |
|--------|------|-------------------------|
| Addressing model | Named data (`Type@path`) with type safety | Topics (strings); type checking is up to the application |
| Roles | Intent (producer) & Interest (consumer) with automatic linking | Publishers & subscribers; applications coordinate |
| Discovery | Decentralized; interests can discover intents across network | Typically centralized broker or cluster |
| Delivery optimization | Runtime chooses broadcast/unicast; zero-copy local delivery | Depends on broker plugin/config |
| Topology awareness | Core router is topology-agnostic; endpoints bridge transports | Brokers manage transport; clients connect to known servers |
| Embedded targets | Designed for microcontrollers + SBCs; no heavy dependencies | Usually requires TCP/IP stack + message broker |
| Extensibility | Build-your-own endpoints/transports; signed routes possible | Limited to provided transports or plugins depending on broker |

In short, DNDM borrows Named Data Networking ideas: applications talk about *what data* they want, not *where* it lives. The `x/` packages exist to make this model approachable while respecting the underlying philosophy.

---

## Core Primitives (Review)

### Routes
```go
route, _ := dndm.NewRoute("sensors.temperature", &pb.SensorData{})
fmt.Println(route.ID()) // SensorData@sensors.temperature
```

### Intents (Producers)
```go
intent, _ := router.Publish("sensors.temperature", &pb.SensorData{})
<-intent.Interest() // wait for first consumer
intent.Send(ctx, &pb.SensorData{Celsius: 25.5})
```

### Interests (Consumers)
```go
interest, _ := router.Subscribe("sensors.temperature", &pb.SensorData{})
defer interest.Close()
for msg := range interest.C() {
    data := msg.(*pb.SensorData)
    handle(data)
}
```

### Caller/Service Pattern (Request/Reply)
```go
caller, _ := bus.NewCaller[*pb.CommandRequest, *pb.CommandReply](ctx, router,
    "commands.x", "replies.x")
reply, _ := caller.Call(ctx, &pb.CommandRequest{...})

service, _ := bus.NewService[*pb.CommandRequest, *pb.CommandReply](ctx, router,
    "commands.x", "replies.x")
service.Handle(ctx, func(ctx context.Context, req *pb.CommandRequest, reply func(*pb.CommandReply) error) error {
    return reply(process(req))
})
```

---

## `x/` Layer: High-Level Building Blocks

### 1. `x/bus` – Unified Node/Bus Orchestrator

Purpose: wrap a router, endpoints, discovery, and typed messaging into a cohesive runtime object while preserving the topology-agnostic core. `x/bus` supersedes the old `x/dnbus` package and serves as the single entry point for application developers.

```go
cfg := bus.Config{
    Name: "sensor-board",
    Intents: []bus.IntentConfig{
        {Route: "SensorData@sensors.raw", Message: (*pb.SensorData)(nil)},
    },
    Interests: []bus.InterestConfig{
        {Route: "ControlCommand@actuators"},
    },
    Connectors: []bus.ConnectorConfig{
        {Type: "serial", URI: "serial:///dev/ttyUSB0?baud=115200"},
        {Type: "udp", URI: "udp://239.1.1.1:9000"},
    },
}

runtime, _ := bus.New(ctx, cfg)
producer := runtime.Producer[*pb.SensorData]("SensorData@sensors.raw")
consumer := runtime.Consumer[*pb.ControlCommand]("ControlCommand@actuators")
```

Responsibilities of `x/bus`:
- Manage router lifecycle and endpoint attachment.
- Register intents/interests from configuration or code.
- Coordinate discovery/broadcast logic (while leaving core router unchanged).
- Expose generics-based producers, consumers, callers, services, and modules with minimal overhead.
- Provide embedded/server profiles, delivery policies, and test harnesses.

The same runtime may also be constructed manually when a caller prefers direct control:
```go
router, _ := dndm.New(dndm.WithEndpoint(direct.New(4)))
rt := bus.Unmanaged(router) // wraps existing router without additional connectors
producer := rt.Producer[*pb.SensorData]("SensorData@sensors.raw")
```

### 2. Typed Messaging APIs

Goals for the bus runtime (see `x/bus/PLAN.md`):
- Provide typed producers/consumers/callers/services with low overhead.  
- Work both in managed mode (via `bus.Config`) and unmanaged mode (direct router usage).  
- Offer embedded/server profiles (no hidden goroutines, optional metrics).  
- Support delivery policies (fan-out vs exclusive).  
- Supply test harnesses and documentation.

### 3. Transport Connectors (`x/transport`)

- Serial connector using existing stream codec (`io.Reader`/`io.Writer`).  
- UDP connector with optional multicast/broadcast support.  
- In-memory connector for simulations/tests.  
- Extension hooks for SPI/I²C or custom transports.

Each connector returns an endpoint compatible with `dndm.WithEndpoint`.

### 4. Discovery Helpers (`x/discovery`)

- Interest advertisement strategies (TTL, hop limits).  
- Cache of recent providers for faster re-linking.  
- Diagnostics to trace interest → intent paths (helpful in mesh networks).  
- Optional: compile-time flag to disable for tiny embedded builds.

### 5. Lifecycle & Observability (`x/runtime`)

- Unified `Start`, `Stop`, `HealthCheck` for nodes.  
- Metrics collection (message counts, link latency).  
- Structured logging tags: node, route, connector.

---

## Example Scenarios with `x/`

### Embedded Sensor → Host Gateway

```go
// Sensor firmware
sensorNode := node.MustNew(ctx, node.Config{
    Name: "sensor-board",
    Intents: []node.IntentConfig{
        {Route: "SensorData@sensors.raw", Message: (*pb.SensorData)(nil)},
    },
    Connectors: []node.ConnectorConfig{
        {Type: "serial", URI: "serial:///dev/ttyUSB0?baud=115200"},
    },
})
out := sensorNode.Producer[*pb.SensorData]("SensorData@sensors.raw")
out.Send(ctx, readSensor())

// Host gateway
hostNode := node.MustNew(ctx, node.Config{
    Name: "gateway",
    Interests: []node.InterestConfig{
        {Route: "SensorData@sensors.raw"},
    },
    Intents: []node.IntentConfig{
        {Route: "SensorFusion@sensors.fused", Message: (*pb.SensorFusion)(nil)},
    },
    Connectors: []node.ConnectorConfig{
        {Type: "serial", URI: "serial:///dev/ttyUSB0?baud=115200"},
        {Type: "udp", URI: "udp://239.10.0.1:9000"},
    },
})
sensorIn := hostNode.Consumer[*pb.SensorData]("SensorData@sensors.raw")
fusionOut := hostNode.Producer[*pb.SensorFusion]("SensorFusion@sensors.fused")
```

### Broadcast Telemetry to Multiple Hosts

No change to producer code: the UDP connector decides to use multicast/broadcast if multiple interests register for the same route. Consumers simply subscribe via their node configs.

### Request/Reply Across Mesh

Use `runtime.Caller` / `runtime.Service` helpers and rely on discovery utilities to locate services even if they move between nodes.

---

## Summary

- **Core DNDM** stays topology-agnostic, focused on named-data intents/interests and transport flexibility.  
- **`x/node`** provides an optional high-level orchestrator that manages routers, endpoints, discovery, and DNBus runtimes for a node.  
- **`x/bus`** exposes strongly typed primitives that remain lightweight and suitable for embedded or server deployments.  
- **Connectors, discovery helpers, and lifecycle utilities** round out the developer experience, letting teams describe *what* a node does while `x/` handles *how* it routes data.

This specification should guide future `x/` packages to keep the DNDM vision intact: decentralized, named data messaging with minimal cognitive overhead for application developers.

