# Direct Endpoint Package Specification

## Overview

The `direct` package provides an endpoint implementation for in-process communication. It uses channels for zero-copy message passing between intents and interests within the same process.

## Design Assumptions (Controlled Environment)

This endpoint is designed for controlled environments where:
- Multiple modules within the same process communicate
- Routes are known at compile time (no dynamic discovery)
- Type safety is enforced by protobuf (compile-time checking)
- Resource limits are not enforced (controlled environment)
- Simple channel blocking is sufficient for backpressure

## Current Implementation

- Uses a Linker to match intents with interests
- Zero-copy message passing via channels
- Simple initialization and lifecycle management

## Key Questions

### Functionality
1. **Multiple Publishers/Subscribers**: 
   - Current: Supports multiple publishers and subscribers
   - Assumption: Limits not needed (controlled environment, known topology)
   - Question: ~~Should we support fan-out/fan-in limits?~~ → Not needed (known topology)
   - Question: ~~How to handle excessive publishers/subscribers?~~ → Not an issue (controlled environment)

2. **Message Ordering**:
   - Current: Channel-based FIFO ordering
   - Question: Should we support ordered vs unordered delivery?
   - Question: Should we support priority-based ordering?

3. **Backpressure**:
   - Current: Channels block when full
   - Assumption: Channel blocking is sufficient (known topology, controlled environment)
   - Question: ~~Should we support dropping old messages?~~ → Not needed (controlled environment)
   - Question: How to handle slow consumers? → Channel blocking (sufficient for known topology)

4. **Type Safety**:
   - Current: Runtime type checking
   - Question: Should we add compile-time type checking?
   - Question: How to handle type mismatches?

### Performance
5. **Channel Sizing**:
   - Current: Fixed size from Router configuration
   - Question: Should we support dynamic sizing?
   - Question: How to determine optimal channel size?

6. **Zero-Copy Optimization**:
   - Current: Pointer passing via channels
   - Question: Can we optimize further? Use unsafe pointers?
   - Question: How to handle message lifecycle across goroutines?

7. **Goroutine Management**:
   - Current: Uses goroutines for notification handling
   - Question: Should we use worker pools? Limit goroutines?
   - Question: How to prevent goroutine leaks?

### Integration
8. **Router Integration**:
   - Current: Simple integration with Router
   - Question: Should we support direct endpoint priority?
   - Question: How to handle direct endpoint failures?

9. **Observability**:
   - Current: Basic logging
   - Question: Should we expose metrics (message counts, latencies)?
   - Question: Should we support tracing?

### Testing
10. **Test Strategy**:
    - Current: Basic unit tests
    - Question: How to test concurrent behavior?
    - Question: How to test channel blocking/unblocking?

## Potential Issues

1. **Goroutine Leaks**: Goroutines in Router.addInterest might leak if context isn't properly managed
2. **Race Conditions**: Potential races in Linker when adding/removing intents/interests
3. **Blocking Behavior**: Channels can block indefinitely if consumers don't read

## Improvements Needed

1. **Add Metrics**: Track message throughput, channel utilization
2. **Improve Error Handling**: Better error propagation and handling
3. **Add Resource Limits**: Prevent excessive goroutine creation
4. **Improve Testing**: Add concurrent tests, race condition tests

