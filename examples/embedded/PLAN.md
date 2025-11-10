# Embedded Example Implementation Plan

## Overview

This example demonstrates a complete embedded system scenario using DNDM x/bus primitives. It consists of two components:

1. **Embedded Component** (RP2040/TinyGo): Sensor node publishing telemetry and providing control services
2. **Host Component**: Gateway consuming telemetry, sending control commands, and calling services

Both components communicate via serial transport, with optional UDP support for local testing.

## Requirements Analysis

### Embedded Component Requirements
- **Path**: `example.rp2040`
- **Counter Intent**: Publish incremental counter values every 100ms
- **Control Interest**: Consume control messages with single `N` field (float32)
- **Service**: Provide request/reply service accepting `int` and `float32`, returning `result` (float32), `counter` (int), and `controlN` (float32)
- **Transport**: Serial endpoint
- **Build Target**: Must build as TinyGo program and regular Go service for testing

### Host Component Requirements
- **Path**: `example.host`
- **Counter Consumption**: Consume embedded Counter, log every 10th value, detect gaps
- **Control Provision**: Send control commands with `N` value
- **Service Calls**: Read keyboard input (int, float32), call embedded service, verify response
- **Transport**: Serial endpoint
- **Testing**: Support UDP for docker-compose local testing

## Current x/bus API Assessment

### Strengths
- **Typed Messaging**: Producer/Consumer/Service/Caller provide type-safe APIs without boilerplate
- **Composition**: Components reuse primitive types effectively
- **Goroutine Management**: Automatic goroutine handling for concurrent operations
- **Interest Waiting**: Automatic waiting for consumers on first send
- **Correlation**: Built-in request/reply correlation with nonce support

### Limitations for This Use Case

1. **No Unified Runtime**: No high-level runtime managing endpoints/connectors automatically
2. **Manual Endpoint Setup**: Serial endpoints require manual router/endpoint construction
3. **No Transport Profiles**: No built-in support for embedded vs server configurations
4. **No Connector Abstraction**: No pluggable connectors for serial/UDP switching
5. **No Build Profiles**: No way to conditionally enable/disable features for TinyGo builds

## Implementation Feasibility

### Current API Suitability: HIGH

The existing x/bus API can implement all requirements:

**Embedded Component**:
```go
// Counter producer
counterProducer, _ := bus.NewProducer[*Counter](ctx, router, "example.rp2040.counter")

// Control consumer
controlConsumer, _ := bus.NewConsumer[*Control](ctx, router, "example.host.control")

// Service handler
service, _ := bus.NewService[*Request, *Response](ctx, router,
    "example.host.requests", "example.rp2040.responses")
```

**Host Component**:
```go
// Counter consumer
counterConsumer, _ := bus.NewConsumer[*Counter](ctx, router, "example.rp2040.counter")

// Control producer
controlProducer, _ := bus.NewProducer[*Control](ctx, router, "example.host.control")

// Service caller
caller, _ := bus.NewCaller[*Request, *Response](ctx, router,
    "example.host.requests", "example.rp2040.responses")
```

### Missing Features

1. **Transport Connectors**: No abstraction for serial/UDP endpoint creation
2. **Runtime Orchestration**: No unified runtime managing router + endpoints
3. **Profile Support**: No embedded vs server configuration profiles
4. **Discovery**: No automatic peer discovery for multi-device scenarios

## Recommended x/bus API Improvements

### 1. Transport Connector Interface

Add `x/bus/connector` package with standardized connector interface:

```go
type Connector interface {
    Scheme() string
    NewEndpoint(ctx context.Context, config map[string]interface{}) (dndm.Endpoint, error)
}

type Config struct {
    Type   string                 // "serial", "udp", "inmemory"
    Config map[string]interface{} // connector-specific config
}
```

### 2. Unified Runtime

Add `x/bus/runtime` package providing managed runtime:

```go
type Runtime struct {
    router     *dndm.Router
    endpoints  []dndm.Endpoint
    connectors []Connector
}

type RuntimeConfig struct {
    Name       string
    Connectors []ConnectorConfig
    Intents    []IntentSpec
    Interests  []InterestSpec
}

func NewRuntime(ctx context.Context, config RuntimeConfig) (*Runtime, error)
func (r *Runtime) Producer[T proto.Message](path string) (*Producer[T], error)
func (r *Runtime) Consumer[T proto.Message](path string) (*Consumer[T], error)
func (r *Runtime) Service[Req, Resp proto.Message](reqPath, respPath string) (*Service[Req, Resp], error)
func (r *Runtime) Caller[Req, Resp proto.Message](reqPath, respPath string) (*Caller[Req, Resp], error)
```

### 3. Embedded Profile Support

Add build tags and configuration for resource-constrained environments:

```go
//go:build tinygo || embedded

func init() {
    // Disable goroutines, use polling instead
    // Reduce buffer sizes
    // Disable logging
}
```

### 4. Path-Based Peer Discovery

Add optional peer discovery for multi-device scenarios:

```go
type DiscoveryConfig struct {
    Path       string        // Path prefix for discovery
    Timeout    time.Duration // Discovery timeout
    Broadcast  bool          // Use broadcast/multicast
}

func (r *Runtime) EnableDiscovery(config DiscoveryConfig) error
```

## Implementation Plan

### Phase 1: Basic Implementation (Current API)

1. **Define Protobuf Messages**:
   - `Counter`: `{value: int32}`
   - `Control`: `{n: float}`
   - `Request`: `{int_val: int32, float_val: float}`
   - `Response`: `{result: float, counter: int32, control_n: float}`

2. **Embedded Component**:
   - Manual router + serial endpoint setup
   - Counter producer with 100ms ticker
   - Control consumer storing N value
   - Service handler computing response

3. **Host Component**:
   - Manual router + serial endpoint setup
   - Counter consumer with gap detection
   - Control producer for commands
   - Service caller with keyboard input

4. **Testing Setup**:
   - Docker Compose with UDP endpoints
   - Shared volume for code
   - Build scripts for both targets

### Phase 2: Enhanced Implementation (Improved API)

1. **Implement Transport Connectors**:
   - Serial connector with TinyGo support
   - UDP connector with multicast
   - In-memory connector for testing

2. **Implement Unified Runtime**:
   - Configuration-driven setup
   - Automatic endpoint management
   - Typed producer/consumer factories

3. **Add Embedded Profiles**:
   - TinyGo build constraints
   - Reduced memory footprint
   - Polling-based alternatives

4. **Add Discovery Support**:
   - Path-based peer discovery
   - Automatic route advertisement

## Success Criteria

1. **Functionality**: Both components communicate correctly via serial
2. **Build Targets**: Embedded builds for RP2040, host builds for Linux
3. **Testing**: Docker Compose setup works for UDP testing
4. **Performance**: Embedded version runs on constrained hardware
5. **Maintainability**: Code is readable and follows DNDM patterns

## API Improvement Priorities

### High Priority
1. **Transport Connectors**: Enable easy serial/UDP switching
2. **Unified Runtime**: Reduce boilerplate for endpoint setup
3. **Embedded Profiles**: Support TinyGo builds

### Medium Priority
1. **Peer Discovery**: Automatic device discovery
2. **Broadcast Policies**: Efficient multi-consumer delivery

### Low Priority
1. **Advanced Monitoring**: Metrics collection
2. **Configuration Files**: External config support

## Conclusion

The current x/bus API is **highly suitable** for implementing this embedded example. The main gaps are in transport abstraction and runtime orchestration, which should be addressed to make DNDM more accessible for embedded development scenarios.

The example will serve as both a demonstration of current capabilities and a driver for API improvements in the transport and runtime layers.
