# DNBus Package Specification

## Overview

The `dnbus` package provides high-level, type-safe wrappers around the core DNDM API to reduce boilerplate code when implementing processing modules. These wrappers eliminate common patterns like type variable creation, type assertions, interest waiting, and context management.

**Package**: `github.com/itohio/dndm/x/dnbus`  
**Import**: `import "github.com/itohio/dndm/x/dnbus"`

## Design Principles

1. **Type Safety**: Eliminate runtime type assertions using Go generics
2. **Boilerplate Reduction**: Reduce common patterns to single method calls
3. **Ergonomics**: Make processing modules easy to implement
4. **Backward Compatibility**: Wrappers are optional, core API unchanged
5. **Performance**: Minimal overhead, maintain zero-copy where possible
6. **Composition**: Reuse Producer/Consumer for Caller/Service pattern

## Components

### 1. Producer Wrapper

**Purpose**: Type-safe producer (publisher) with automatic interest waiting

**API**:

```go
type Producer[T proto.Message] struct {
    intent Intent
    router *Router
    path   string
}

// NewProducer creates a typed producer
func NewProducer[T proto.Message](
    ctx context.Context,
    router *Router,
    path string,
) (*Producer[T], error)

// Send sends a message (blocks until interest received if needed)
func (p *Producer[T]) Send(ctx context.Context, msg T) error

// SendWithTimeout sends with custom timeout
func (p *Producer[T]) SendWithTimeout(ctx context.Context, msg T, timeout time.Duration) error

// WaitForInterest waits for first consumer (optional)
func (p *Producer[T]) WaitForInterest(ctx context.Context) error

// Close closes the producer
func (p *Producer[T]) Close() error

// Route returns the route
func (p *Producer[T]) Route() Route
```

**Key Features**:
- No need for type variable (`var t *Type`)
- Auto-wait for interest on first Send()
- Optional explicit WaitForInterest()
- Type-safe Send() (no type assertion)

**Usage**:

```go
import "github.com/itohio/dndm/x/dnbus"

// Before: ~15 lines of boilerplate
// After: ~5 lines
producer, err := dnbus.NewProducer[*SensorData](ctx, router, "sensors.temperature")
if err != nil {
    return err
}
defer producer.Close()

// Send messages (auto-waits for interest)
for {
    err := producer.Send(ctx, &SensorData{Value: 25.5})
    if err != nil {
        return err
    }
}
```

### 2. Consumer Wrapper

**Purpose**: Type-safe consumer (subscriber) with no type assertions

**API**:

```go
type Consumer[T proto.Message] struct {
    interest Interest
    router   *Router
    path     string
    typedC   <-chan T  // Type-safe channel
}

// NewConsumer creates a typed consumer
func NewConsumer[T proto.Message](
    ctx context.Context,
    router *Router,
    path string,
) (*Consumer[T], error)

// Receive receives a message (blocks until message or context cancelled)
func (c *Consumer[T]) Receive(ctx context.Context) (T, error)

// C returns the typed message channel (for advanced use cases)
func (c *Consumer[T]) C() <-chan T

// Close closes the consumer
func (c *Consumer[T]) Close() error

// Route returns the route
func (c *Consumer[T]) Route() Route
```

**Key Features**:
- No need for type variable (`var t *Type`)
- No type assertions needed
- Type-safe channel (`<-chan T` instead of `<-chan proto.Message`)
- Clean Receive() API with context

**Usage**:

```go
import "github.com/itohio/dndm/x/dnbus"

// Before: ~15 lines with type assertions
// After: ~5 lines
consumer, err := dnbus.NewConsumer[*SensorData](ctx, router, "sensors.temperature")
if err != nil {
    return err
}
defer consumer.Close()

// Receive messages (no type assertion)
for {
    msg, err := consumer.Receive(ctx)
    if err != nil {
        return err // Context cancelled
    }
    process(msg) // msg is already *SensorData
}
```

**Alternative Channel-based API**:

```go
import "github.com/itohio/dndm/x/dnbus"

consumer, _ := dnbus.NewConsumer[*SensorData](ctx, router, "sensors.temperature")
defer consumer.Close()

for msg := range consumer.C() {
    process(msg) // msg is already *SensorData
}
```

### 3. Caller Wrapper (Request/Reply Client)

**Purpose**: Type-safe request/reply pattern (client side)

**Naming Alternatives**:
- `Caller` - Clear meaning, calls a service (RECOMMENDED)
- `Invoker` - Similar meaning, more formal
- `Client` - Generic, less descriptive

**Design**: Reuses `Producer` for requests and `Consumer` for responses

**API**:

```go
type Caller[Req proto.Message, Resp proto.Message] struct {
    requestProducer  *Producer[Req]
    responseConsumer *Consumer[Resp]
    router           *Router
    requestPath      string
    responsePath     string
    correlation      map[uint64]chan Resp
    mu               sync.Mutex
    nonce            uint64
}

// NewCaller creates a request/reply caller
func NewCaller[Req proto.Message, Resp proto.Message](
    ctx context.Context,
    router *Router,
    requestPath string,
    responsePath string,
) (*Caller[Req, Resp], error)

// CallerOption allows configuring optional request/response correlation behaviour.
type CallerOption[Req proto.Message, Resp proto.Message] func(*callerOptions[Req, Resp])

// WithCallerRequestNonce registers a setter that places the generated nonce on outgoing requests.
func WithCallerRequestNonce[Req proto.Message, Resp proto.Message](fn func(Req, uint64)) CallerOption[Req, Resp]

// WithCallerResponseNonce registers an extractor that reads a nonce from incoming responses.
func WithCallerResponseNonce[Req proto.Message, Resp proto.Message](fn func(Resp) (uint64, bool)) CallerOption[Req, Resp]

// Call sends a request and waits for a single reply
func (c *Caller[Req, Resp]) Call(ctx context.Context, req Req) (Resp, error)

// CallWithTimeout sends a request with timeout and waits for a single reply
func (c *Caller[Req, Resp]) CallWithTimeout(ctx context.Context, req Req, timeout time.Duration) (Resp, error)

// HandleReplies handles multiple replies via callback (goroutine-based)
// The callback is called in a goroutine and can receive one or multiple replies
func (c *Caller[Req, Resp]) HandleReplies(ctx context.Context, req Req, handler func(ctx context.Context, req Req, resp Resp) error) error

// Close closes the caller
func (c *Caller[Req, Resp]) Close() error
```

**Key Features**:
- Type-safe request/reply
- Reuses Producer/Consumer internally
- Automatic correlation management
- Single reply via Call() method
- Multiple replies via HandleReplies() callback
- Handler callback runs in goroutine
- Optional nonce setters/extractors for custom correlation. Without custom hooks,
  replies are dispatched in FIFO order to the oldest pending caller.

**Usage**:

**Single Reply**:

```go
import "github.com/itohio/dndm/x/dnbus"

caller, err := dnbus.NewCaller[*GetStatusRequest, *GetStatusResponse](
    ctx, router,
    "commands.get_status",
    "responses.get_status",
)
if err != nil {
    return err
}
defer caller.Close()

// Send request and receive single reply
resp, err := caller.Call(ctx, &GetStatusRequest{})
if err != nil {
    return err
}
process(resp)
```

**Multiple Replies via Handler**:

```go
import "github.com/itohio/dndm/x/dnbus"

caller, err := dnbus.NewCaller[*SubscribeRequest, *SubscribeResponse](
    ctx, router,
    "commands.subscribe",
    "responses.subscribe",
)
if err != nil {
    return err
}
defer caller.Close()

// Send request once
req := &SubscribeRequest{Topic: "events"}

// Handle multiple replies via callback (runs in goroutine)
err = caller.HandleReplies(ctx, req, func(ctx context.Context, req *SubscribeRequest, resp *SubscribeResponse) error {
    // Can receive one or multiple replies
    process(resp)
    
    // Can choose to stop receiving or continue
    if resp.Done {
        return nil // Stop handling
    }
    return nil // Continue handling more replies
})
```

### 4. Service Wrapper (Request/Reply Handler)

**Purpose**: Type-safe request handler for processing requests and sending replies

**Design**: Reuses `Consumer` for requests and `Producer` for responses

**API**:

```go
type Service[Req proto.Message, Resp proto.Message] struct {
    requestConsumer  *Consumer[Req]
    responseProducer *Producer[Resp]
    router           *Router
    requestPath      string
    responsePath     string
}

// NewService creates a request handler service
func NewService[Req proto.Message, Resp proto.Message](
    ctx context.Context,
    router *Router,
    requestPath string,
    responsePath string,
) (*Service[Req, Resp], error)

// Handle handles requests with a callback (goroutine-based)
// The callback is called in a goroutine for each request and can send one or multiple replies
func (s *Service[Req, Resp]) Handle(ctx context.Context, handler func(ctx context.Context, req Req, reply func(resp Resp) error) error) error

// Receive receives a request (manual handling)
func (s *Service[Req, Resp]) Receive(ctx context.Context) (Req, error)

// Reply sends a reply for a request (manual handling)
func (s *Service[Req, Resp]) Reply(ctx context.Context, resp Resp) error

// Close closes the service
func (s *Service[Req, Resp]) Close() error
```

**Key Features**:
- Type-safe request handling
- Reuses Consumer/Producer internally
- Callback-based or manual handling
- Handler callback runs in goroutine for each request
- Can send one or multiple replies per request
- Automatic correlation (via request ID/nonce)

**Usage**:

**Single Reply per Request**:

```go
import "github.com/itohio/dndm/x/dnbus"

service, err := dnbus.NewService[*GetStatusRequest, *GetStatusResponse](
    ctx, router,
    "commands.get_status",
    "responses.get_status",
)
if err != nil {
    return err
}
defer service.Close()

// Handle requests with callback (each request handled in goroutine)
service.Handle(ctx, func(ctx context.Context, req *GetStatusRequest, reply func(resp *GetStatusResponse) error) error {
    // Process request
    status := processRequest(req)
    
    // Send single reply
    return reply(&GetStatusResponse{Status: status})
})
```

**Multiple Replies per Request**:

```go
import "github.com/itohio/dndm/x/dnbus"

service, err := dnbus.NewService[*SubscribeRequest, *SubscribeResponse](
    ctx, router,
    "commands.subscribe",
    "responses.subscribe",
)
if err != nil {
    return err
}
defer service.Close()

// Handle requests - can send multiple replies
service.Handle(ctx, func(ctx context.Context, req *SubscribeRequest, reply func(resp *SubscribeResponse) error) error {
    // Send initial acknowledgment
    if err := reply(&SubscribeResponse{Type: "ack"}); err != nil {
        return err
    }
    
    // Stream multiple updates
    for i := 0; i < 10; i++ {
        if err := reply(&SubscribeResponse{
            Type:    "update",
            Data:    fmt.Sprintf("Update %d", i),
        }); err != nil {
            return err
        }
        time.Sleep(time.Second)
    }
    
    // Send final message
    return reply(&SubscribeResponse{Type: "done"})
})
```

**Manual Handling**:

```go
import "github.com/itohio/dndm/x/dnbus"

service, err := dnbus.NewService[*GetStatusRequest, *GetStatusResponse](
    ctx, router,
    "commands.get_status",
    "responses.get_status",
)
if err != nil {
    return err
}
defer service.Close()

// Manual handling
for {
    req, err := service.Receive(ctx)
    if err != nil {
        return err
    }
    
    resp := processRequest(req)
    if err := service.Reply(ctx, resp); err != nil {
        return err
    }
}
```

### 5. Processing Module Wrapper

**Purpose**: Simplify multi-input/output processing modules

**API**:

```go
type Module struct {
    ctx     context.Context
    router  *Router
    inputs  []Input
    outputs []Output
}

type Input[T proto.Message] struct {
    consumer *Consumer[T]
    path     string
}

type Output[T proto.Message] struct {
    producer *Producer[T]
    path     string
}

// NewModule creates a new processing module
func NewModule(ctx context.Context, router *Router) *Module

// AddInput adds an input consumer (package-level helper to avoid method type params
// which are not supported before Go 1.22)
func AddInput[T proto.Message](m *Module, path string) (*Input[T], error)

// AddOutput adds an output producer
func AddOutput[T proto.Message](m *Module, path string) (*Output[T], error)

// Close closes all inputs and outputs
func (m *Module) Close() error

// Run runs a processing function
func (m *Module) Run(ctx context.Context, fn func(ctx context.Context) error) error
```

**Key Features**:
- Manage multiple inputs/outputs
- Simplified lifecycle management
- Clean processing function
- Type-safe inputs/outputs

**Usage**:

```go
import "github.com/itohio/dndm/x/dnbus"

module := dnbus.NewModule(ctx, router)
defer module.Close()

// Define inputs
cameraInput, _ := dnbus.AddInput[*CameraImage](module, "cameras.front")
sensorInput, _ := dnbus.AddInput[*SensorData](module, "sensors.imu")

// Define outputs
featuresOutput, _ := dnbus.AddOutput[*ImageFeatures](module, "image.features")
depthOutput, _ := dnbus.AddOutput[*DepthMap](module, "image.depth")

// Run processing
module.Run(ctx, func(ctx context.Context) error {
    for {
        // Receive from inputs
        image, err := cameraInput.Receive(ctx)
        if err != nil {
            return err
        }
        sensor, err := sensorInput.Receive(ctx)
        if err != nil {
            return err
        }
        
        // Process
        features := processImage(image, sensor)
        depth := calculateDepth(image)
        
        // Send to outputs
        featuresOutput.Send(ctx, features)
        depthOutput.Send(ctx, depth)
    }
})
```

## Design Decisions

### Composition over Duplication

**Decision**: Caller and Service reuse Producer and Consumer internally

**Rationale**:
- Reduces code duplication
- Maintains consistent behavior
- Easier to maintain and test
- Single source of truth for producer/consumer logic

**Implementation**:

```go
type Caller[Req, Resp] struct {
    requestProducer  *Producer[Req]
    responseConsumer *Consumer[Resp]
    // ... correlation management
}

type Service[Req, Resp] struct {
    requestConsumer  *Consumer[Req]
    responseProducer *Producer[Resp]
}
```

### Handler Callback Goroutine Execution

**Decision**: Handler callbacks run in separate goroutines

**Rationale**:
- Allows service to process multiple requests concurrently
- Allows client to handle multiple replies concurrently
- Handler can choose whether to send one or multiple replies
- Non-blocking for other requests/replies

**Implementation**:

```go
// Service handler - each request handled in goroutine
func (s *Service[Req, Resp]) Handle(ctx context.Context, handler func(ctx context.Context, req Req, reply func(resp Resp) error) error) error {
    for {
        req, err := s.requestConsumer.Receive(ctx)
        if err != nil {
            return err
        }
        
        // Each request handled in separate goroutine
        go func(request Req) {
            handler(ctx, request, func(resp Resp) error {
                return s.responseProducer.Send(ctx, resp)
            })
        }(req)
    }
}

// Caller handler - receives replies in goroutine
func (c *Caller[Req, Resp]) HandleReplies(ctx context.Context, req Req, handler func(ctx context.Context, req Req, resp Resp) error) error {
    // Send request once, then handle replies in goroutine
    go func() {
        for {
            resp, err := c.responseConsumer.Receive(ctx)
            if err != nil {
                return
            }
            handler(ctx, req, resp)
        }
    }()
    return nil
}
```

### Single Reply Method (Call)

**Decision**: Provide `Call()` method for single request/reply pattern

**Rationale**:
- Common pattern for request/reply
- Simplifies API for single reply case
- Handler pattern available for multiple replies

### Interest Waiting Strategy

**Decision**: Block on first Send() until interest received, with optional WaitForInterest()

**Rationale**:
- Most common pattern is to wait for interest before sending
- Optional explicit waiting for advanced use cases
- Simpler API (one less method call needed)

### Request/Reply Correlation

**Decision**: Use nonce in message correlation map

**Rationale**:
- Keeps routes simple (no nonce in route)
- Single route for all responses
- Efficient lookup (map-based correlation)

**Alternative Considered**: Route-based nonce (`responses.get_status.{nonce}`)
- **Rejected**: Creates many routes, harder to manage

### Type Channel Conversion

**Decision**: Use goroutine to convert `<-chan proto.Message` to `<-chan T`

**Rationale**:
- Provides type-safe channel
- Maintains buffering characteristics
- Acceptable overhead for ergonomics

**Alternative Considered**: Direct type assertion in Receive()
- **Rejected**: Still requires type assertions, less ergonomic

### Module Input Synchronization

**Decision**: Independent goroutines per input

**Rationale**:
- Most common pattern in robotics (independent sensors)
- User can implement synchronization if needed
- Simpler default behavior

**Alternative Considered**: Synchronized receiving
- **Available**: User can implement via sync primitives

## Key Questions

### 1. Caller Naming

**Question**: What should we call the request/reply client?

**Options**:
- `Caller` - Clear meaning, calls a service (RECOMMENDED)
- `Invoker` - Similar meaning, more formal
- `Client` - Generic, less descriptive

**Recommendation**: Use `Caller` as it's clear and descriptive.

### 2. Service Handler Reply Function

**Question**: Should the reply function in Service handler be synchronous or asynchronous?

**Options**:
- Synchronous (blocking) - Simpler, ensures ordering
- Asynchronous (non-blocking) - Better for high throughput

**Recommendation**: Synchronous (blocking) for simplicity, but can be made configurable later.

### 3. Request Correlation Mechanism

**Question**: How to correlate requests with replies?

**Options**:
- Nonce in message header (requires message modification)
- Correlation map with message ID (requires message ID field)
- Route-based nonce (creates many routes)

**Recommendation**: Correlation map with nonce (store in message or use header timestamp).
If messages cannot carry a nonce, fall back to FIFO routing between pending calls.
Expose optional hooks so callers can set/extract nonces without custom wrappers.

### 4. Handler Error Handling

**Question**: How should handler errors be handled?

**Options**:
- Return error from handler (stops processing)
- Log error and continue
- Error callback

**Recommendation**: Return error from handler to stop processing, with optional error callback for logging.

## Implementation Notes

### Generic Constraints

Go generics with protobuf messages:

```go
type MessageType[T any] interface {
    proto.Message
    *T  // Pointer type constraint
}
```

### Type Erasure Pattern

```go
func NewProducer[T proto.Message](ctx context.Context, router *Router, path string) (*Producer[T], error) {
    var zero T
    intent, err := router.Publish(path, zero)
    if err != nil {
        return nil, err
    }
    return &Producer[T]{
        intent: intent,
        router: router,
        path:   path,
    }, nil
}
```

### Interest Waiting Implementation

```go
func (p *Producer[T]) Send(ctx context.Context, msg T) error {
    // Wait for interest if not already linked
    if !p.interestReceived {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-p.intent.Interest():
            p.interestReceived = true
        }
    }
    
    // Send with default timeout
    sendCtx, cancel := context.WithTimeout(ctx, defaultSendTimeout)
    defer cancel()
    return p.intent.Send(sendCtx, msg)
}
```

### Type Channel Conversion

```go
func NewConsumer[T proto.Message](ctx context.Context, router *Router, path string) (*Consumer[T], error) {
    var zero T
    interest, err := router.Subscribe(path, zero)
    if err != nil {
        return nil, err
    }
    
    // Create typed channel
    typedC := make(chan T, cap(interest.C()))
    
    // Start goroutine to convert types
    go func() {
        defer close(typedC)
        for msg := range interest.C() {
            typedMsg := msg.(T)
            select {
            case <-ctx.Done():
                return
            case typedC <- typedMsg:
            }
        }
    }()
    
    return &Consumer[T]{
        interest: interest,
        router:   router,
        path:     path,
        typedC:   typedC,
    }, nil
}
```

### Request Correlation

```go
func (c *Caller[Req, Resp]) Call(ctx context.Context, req Req) (Resp, error) {
    // Generate nonce
    c.mu.Lock()
    nonce := c.nonce
    c.nonce++
    respC := make(chan Resp, 1)
    c.correlation[nonce] = respC
    c.mu.Unlock()
    
    // Set nonce in request (if message supports it)
    // Or use correlation map with message ID
    setNonce(req, nonce)
    
    // Send request
    err := c.requestProducer.Send(ctx, req)
    if err != nil {
        c.mu.Lock()
        delete(c.correlation, nonce)
        c.mu.Unlock()
        return nil, err
    }
    
    // Wait for reply
    select {
    case <-ctx.Done():
        c.mu.Lock()
        delete(c.correlation, nonce)
        c.mu.Unlock()
        return nil, ctx.Err()
    case resp := <-respC:
        return resp, nil
    }
}
```

### Service Handler Implementation

```go
func (s *Service[Req, Resp]) Handle(ctx context.Context, handler func(ctx context.Context, req Req, reply func(resp Resp) error) error) error {
    for {
        req, err := s.requestConsumer.Receive(ctx)
        if err != nil {
            return err
        }
        
        // Each request handled in separate goroutine
        go func(request Req) {
            replyFunc := func(resp Resp) error {
                return s.responseProducer.Send(ctx, resp)
            }
            
            if err := handler(ctx, request, replyFunc); err != nil {
                // Handle error (log or callback)
            }
        }(req)
    }
}
```

### Caller Handler Implementation

```go
func (c *Caller[Req, Resp]) HandleReplies(ctx context.Context, req Req, handler func(ctx context.Context, req Req, resp Resp) error) error {
    // Send request once
    if err := c.requestProducer.Send(ctx, req); err != nil {
        return err
    }
    
    // Handle replies in goroutine
    go func() {
        for {
            resp, err := c.responseConsumer.Receive(ctx)
            if err != nil {
                return
            }
            
            if err := handler(ctx, req, resp); err != nil {
                // Handle error (log or callback)
                return
            }
        }
    }()
    
    return nil
}
```

## Usage Examples

### Simple Producer

```go
import "github.com/itohio/dndm/x/dnbus"

producer, _ := dnbus.NewProducer[*SensorData](ctx, router, "sensors.temperature")
defer producer.Close()

for {
    producer.Send(ctx, &SensorData{Value: 25.5})
    time.Sleep(time.Second)
}
```

### Simple Consumer

```go
import "github.com/itohio/dndm/x/dnbus"

consumer, _ := dnbus.NewConsumer[*SensorData](ctx, router, "sensors.temperature")
defer consumer.Close()

for {
    msg, err := consumer.Receive(ctx)
    if err != nil {
        return err
    }
    process(msg)
}
```

### Request/Reply (Single)

```go
import "github.com/itohio/dndm/x/dnbus"

// Caller
caller, _ := dnbus.NewCaller[*GetStatusRequest, *GetStatusResponse](
    ctx, router,
    "commands.get_status",
    "responses.get_status",
)
defer caller.Close()

resp, err := caller.Call(ctx, &GetStatusRequest{})
if err != nil {
    return err
}

// Service
service, _ := dnbus.NewService[*GetStatusRequest, *GetStatusResponse](
    ctx, router,
    "commands.get_status",
    "responses.get_status",
)
defer service.Close()

service.Handle(ctx, func(ctx context.Context, req *GetStatusRequest, reply func(resp *GetStatusResponse) error) error {
    return reply(&GetStatusResponse{Status: "ok"})
})
```

### Request/Reply (Multiple)

```go
import "github.com/itohio/dndm/x/dnbus"

// Caller with handler for multiple replies
caller, _ := dnbus.NewCaller[*SubscribeRequest, *SubscribeResponse](
    ctx, router,
    "commands.subscribe",
    "responses.subscribe",
)
defer caller.Close()

req := &SubscribeRequest{Topic: "events"}
caller.HandleReplies(ctx, req, func(ctx context.Context, req *SubscribeRequest, resp *SubscribeResponse) error {
    process(resp)
    if resp.Done {
        return nil // Stop handling
    }
    return nil // Continue
})

// Service with multiple replies
service, _ := dnbus.NewService[*SubscribeRequest, *SubscribeResponse](
    ctx, router,
    "commands.subscribe",
    "responses.subscribe",
)
defer service.Close()

service.Handle(ctx, func(ctx context.Context, req *SubscribeRequest, reply func(resp *SubscribeResponse) error) error {
    // Send initial ack
    reply(&SubscribeResponse{Type: "ack"})
    
    // Send multiple updates
    for i := 0; i < 10; i++ {
        reply(&SubscribeResponse{Type: "update", Data: fmt.Sprintf("Update %d", i)})
    }
    
    // Send final
    return reply(&SubscribeResponse{Type: "done"})
})
```

### Processing Module

```go
import "github.com/itohio/dndm/x/dnbus"

module := dnbus.NewModule(ctx, router)
defer module.Close()

input, _ := dnbus.AddInput[*CameraImage](module, "cameras.front")
output, _ := dnbus.AddOutput[*ImageFeatures](module, "image.features")

module.Run(ctx, func(ctx context.Context) error {
    for {
        image, err := input.Receive(ctx)
        if err != nil {
            return err
        }
        features := process(image)
        output.Send(ctx, features)
    }
})
```

## Performance Considerations

### Overhead Analysis

1. **Type Erasure**: Minimal (compile-time)
2. **Type Channel Conversion**: One goroutine per consumer (acceptable for ergonomics)
3. **Interest Waiting**: One-time cost on first Send()
4. **Request Correlation**: Map lookup per request (O(1), minimal overhead)
5. **Handler Goroutines**: One goroutine per request/reply (acceptable for concurrency)

### Optimization Opportunities

1. **Channel Pooling**: If many consumers, pool typed channels
2. **Correlation Map**: Use sync.Map for concurrent access
3. **Interest Waiting**: Cache interest state to avoid repeated waiting
4. **Goroutine Pool**: Pool goroutines for handlers if needed

## Testing Strategy

### Unit Tests
- Test Producer creation and Send()
- Test Consumer creation and Receive()
- Test Caller Call() and HandleReplies()
- Test Service Handle() with single and multiple replies
- Test Module lifecycle
- Test error handling

### Integration Tests
- Test with Direct endpoint
- Test with Remote endpoint
- Test with Mesh endpoint
- Test concurrent operations
- Test request/reply correlation

### Performance Tests
- Compare overhead with raw API
- Test channel conversion overhead
- Test correlation map performance
- Test handler goroutine overhead

## Migration Guide

### Step 1: Replace Intent with Producer

```go
import "github.com/itohio/dndm/x/dnbus"

// Before
var t *SensorData
intent, _ := router.Publish("sensors.temperature", t)
defer intent.Close()

// After
producer, _ := dnbus.NewProducer[*SensorData](ctx, router, "sensors.temperature")
defer producer.Close()
```

### Step 2: Replace Interest with Consumer

```go
import "github.com/itohio/dndm/x/dnbus"

// Before
var t *SensorData
interest, _ := router.Subscribe("sensors.temperature", t)
defer interest.Close()

// After
consumer, _ := dnbus.NewConsumer[*SensorData](ctx, router, "sensors.temperature")
defer consumer.Close()
```

### Step 3: Simplify Send/Receive

```go
// Before
select {
case <-ctx.Done():
    return
case route := <-intent.Interest():
    c, cancel := context.WithTimeout(ctx, time.Second)
    intent.Send(c, msg)
    cancel()
}

// After
producer.Send(ctx, msg)
```

### Step 4: Eliminate Type Assertions

```go
// Before
msg := <-interest.C()
data := msg.(*SensorData)

// After
data, err := consumer.Receive(ctx)
```

## Questions to Answer

1. **Caller Naming**: Should we use Caller, Invoker, or another name? (Recommendation: Caller)
2. **Handler Error Handling**: How should handler errors be handled? (Recommendation: Return error to stop)
3. **Service Reply Function**: Should reply function be synchronous or asynchronous? (Recommendation: Synchronous)
4. **Request Correlation**: Nonce in message or correlation map? (Recommendation: Correlation map with message ID)
5. **Goroutine Pool**: Should we pool goroutines for handlers? (Recommendation: Not initially)

