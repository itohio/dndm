# DNDM Core Package Specification

## Overview

The core package (`dndm`) provides the fundamental abstractions for the DNDM library: Router, Intent, Interest, Link, Linker, Route, Peer, and Endpoint interfaces.

## Design Assumptions

This package is designed for **controlled environments** where:
- The set of intents and interests is known at design time
- The set of clients/producers is known and controlled
- Primary use case is robotics internal message bus
- Runtime validation can be minimal (known routes, trusted environment)
- Security checks can be added later if needed (modular architecture)

These assumptions allow for:
- Simplified validation logic
- Performance optimizations (no excessive runtime checks)
- Easier addition of validation/security later (middleware pattern)

## Components

### Router

The Router is the top-level component that manages endpoints and routes intents/interests across them.

**Current Implementation**:
- Manages a collection of endpoints
- Routes intents/interests to appropriate routers
- Handles intent/interest aggregation across endpoints

**Current Assumptions** (Controlled Environment):
- Single router per process (typical use case)
- Endpoint failures are handled by application (known topology)
- Dynamic endpoint addition supported, but topology typically known
- Multiple publishers on same route are allowed (known, intentional setup)
- Routing policies can be added via middleware if needed

**Key Questions**:
1. ~~Should Router support multiple routers in the same process?~~ → Not needed for controlled environment
2. ~~How should router handle endpoint failures?~~ → Application handles (known topology)
3. Should Router support dynamic endpoint addition/removal after initialization? → Supported but not primary use case
4. How should Router handle conflicts when multiple endpoints publish to the same route? → Multiple publishers allowed (broadcast pattern)
5. Should Router support routing policies? → Can be added via middleware if needed

### Intent and IntentRouter

Intent represents a source of data on a specific route. IntentRouter manages multiple intents and routes messages.

**Components**:
- `LocalIntent`: Basic intent for local communication
- `FanOutIntent`: Routes messages to multiple destinations
- `IntentRouter`: Manages fan-out and wrapping

**Current Implementation**:
- LocalIntent uses channels for zero-copy message passing
- FanOutIntent distributes to multiple intents
- IntentRouter manages wrappers for multiple subscribers

**Current Assumptions** (Controlled Environment):
- Message priority/QoS not needed (known traffic patterns)
- Slow consumers handled by buffer size configuration (known topology)
- Message expiration/TTL not implemented (controlled environment, known producers)
- Partial failures handled at application level (known producers)
- Backpressure via channel blocking (simple, sufficient for known topology)
- Type mismatches are programming errors (caught at compile-time with protobuf)

**Key Questions**:
1. ~~Should Intent support message priority or QoS levels?~~ → Not needed for robotics use case
2. How should Intent handle slow consumers? → Blocking channels (sufficient for known topology)
3. ~~Should Intent support message expiration/TTL?~~ → Not needed (controlled environment)
4. How should IntentRouter handle partial failures? → Log and continue (application handles failures)
5. Should Intent support backpressure mechanisms? → Channel blocking is sufficient
6. How to handle type mismatches? → Programming error (compile-time with protobuf types)

### Interest and InterestRouter

Interest represents a consumer of data on a specific route. InterestRouter manages multiple interests and routes messages.

**Components**:
- `LocalInterest`: Basic interest for local communication
- `FanInInterest`: Routes messages from multiple sources
- `InterestRouter`: Manages fan-in and wrapping

**Current Implementation**:
- LocalInterest uses channels for message delivery
- FanInInterest aggregates from multiple interests
- InterestRouter manages wrappers for multiple publishers

**Key Questions**:
1. Should Interest support message filtering/subscription filters?
2. How should Interest handle message ordering? FIFO? Timestamp-based?
3. Should Interest support message deduplication?
4. How should InterestRouter handle duplicate messages from multiple sources?
5. Should Interest support message batching/aggregation?
6. How to handle type casting errors at runtime?

### Link

Link represents a connection between an Intent and an Interest.

**Current Implementation**:
- Creates channel connection between intent and interest
- Sends notifications when link is established

**Key Questions**:
1. Should Link support multiple links for same intent/interest pair?
2. How should Link handle link failures? Automatic retry?
3. Should Link support link metrics (message count, latency)?
4. How should Link handle link lifecycle events (callbacks)?

### Linker

Linker manages Intent-Interest matching and link creation.

**Current Implementation**:
- Maintains maps of intents and interests
- Automatically links matching routes
- Supports wrapper functions for intent/interest customization

**Key Questions**:
1. Should Linker support multiple linkers? Hierarchical linking?
2. How should Linker handle link conflicts? Multiple intents for same route?
3. Should Linker support link policies (e.g., only link if certain conditions met)?
4. How should Linker handle link failures? Automatic cleanup?
5. Should Linker support link notifications/events?
6. How to handle race conditions between intent/interest registration?

### Route

Route represents a typed, named path for data streams.

**Components**:
- `PlainRoute`: Human-readable route with type and path
- `HashedRoute`: Opaque route for security

**Current Implementation**:
- Format: `Type@path` for plain routes
- Format: `prefix#hash` for hashed routes
- Route validation and matching

**Key Questions**:
1. Should Route support wildcards or pattern matching?
2. How should Route handle versioning? Should we support route versioning?
3. Should Route support metadata (e.g., QoS, priority)?
4. How should hashed routes work in distributed systems? How to distribute keys?
5. Should Route support route aliases or redirects?
6. How to handle route collisions in mesh networks?

### Peer

Peer represents a network peer with scheme, address, and path.

**Current Implementation**:
- URI-based peer identification: `scheme://address/path?params`
- Path-based prefix matching for routing

**Key Questions**:
1. Should Peer support peer discovery mechanisms?
2. How should Peer handle peer authentication/identity?
3. Should Peer support peer capabilities/features negotiation?
4. How should Peer handle peer lifecycle events (connect, disconnect)?
5. Should Peer support peer metadata (e.g., location, capabilities)?
6. How to handle peer path collisions?

### Endpoint and Container

Endpoint is an abstraction for communication mechanisms. Container aggregates multiple endpoints.

**Current Implementation**:
- BaseEndpoint provides common functionality
- Container manages multiple endpoints
- Supports endpoint lifecycle management

**Key Questions**:
1. Should Container support endpoint priorities or ordering?
2. How should Container handle endpoint failures? Remove? Retry?
3. Should Container support endpoint health checks?
4. How should Container handle endpoint configuration?
5. Should Container support endpoint metrics/observability?
6. How to handle routing conflicts between endpoints?

### Base

Base provides context management and lifecycle hooks.

**Current Implementation**:
- Context management with cancellation
- OnClose hooks for cleanup

**Key Questions**:
1. Should Base support multiple context hierarchies?
2. How should Base handle cleanup order? Should there be cleanup phases?
3. Should Base support timeout configuration?
4. How to handle cleanup failures?

## API Design Questions

### General API Design

1. **Type Safety**: 
   - How to improve type safety at API level?
   - Should we support generics for type-safe wrappers?
   - How to avoid runtime type assertions?

2. **Error Handling**:
   - Should we use error channels instead of error returns?
   - How to distinguish transient vs permanent errors?
   - Should we support error callbacks?

3. **Configuration**:
   - Should we support configuration builders?
   - How to handle configuration validation?
   - Should we support configuration inheritance?

4. **Observability**:
   - Should we expose metrics?
   - How to handle distributed tracing?
   - Should we support event callbacks?

### Performance Questions

1. **Zero-Copy**:
   - Can we use unsafe pointers for zero-copy?
   - How to handle message serialization with minimal copying?
   - Should we support memory pools?

2. **Concurrency**:
   - How to minimize lock contention?
   - Should we use lock-free data structures?
   - How to handle goroutine leaks?

3. **Buffering**:
   - How to size channels/buffers?
   - Should we support dynamic buffering?
   - How to handle backpressure?

## Implementation Notes

### Current Limitations

1. **Race Conditions**: Multiple goroutines accessing shared state without proper synchronization in some cases
2. **Resource Management**: Some resources may not be properly cleaned up
3. **Error Propagation**: Some errors are logged but not properly propagated
4. **Type Safety**: Runtime type checking could be improved

### Potential Improvements

1. **Add Metrics**: Track message counts, latencies, error rates
2. **Improve Type Safety**: Use generics where possible, add compile-time checks
3. **Better Error Handling**: Distinguish error types, add error channels
4. **Performance Optimization**: Reduce allocations, minimize copying
5. **Testing**: Add more comprehensive tests, especially for concurrent scenarios

## Testing Questions

1. **Unit Tests**:
   - Should we use table-driven tests?
   - How to test concurrent behavior?
   - How to test context cancellation?

2. **Integration Tests**:
   - How to test Router with multiple endpoints?
   - How to test mesh network behavior?
   - How to test failure scenarios?

3. **Performance Tests**:
   - What metrics should we benchmark?
   - How to test under load?
   - How to test memory usage?

