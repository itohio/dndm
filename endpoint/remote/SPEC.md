# Remote Endpoint Package Specification

## Overview

The `remote` package provides an endpoint implementation for cross-process and cross-system communication. It manages network connections, handles intent/interest propagation, and routes messages between remote peers.

## Design Assumptions (Controlled Environment)

This endpoint is designed for controlled environments where:
- Communication topology is known (LAN between computers, serial to embedded devices)
- Set of routes is known (no dynamic route discovery needed)
- Trusted network environment (no authentication required)
- Broadcast may be used (UDP) - receivers without interest should reject early
- Connection failures handled at application level (known topology)

## Current Implementation

- Manages a network connection via `network.Conn`
- Handles message serialization/deserialization
- Supports intent/interest advertisement to remote peers
- Implements ping/pong for latency measurement
- Handles handshake and protocol messages

## Key Questions

### Connection Management
1. **Connection Lifecycle**:
   - Current: Connection managed via network.Conn interface
   - Assumption: One connection per peer (known topology)
   - Question: ~~Should we support connection pooling?~~ → Not needed (known topology)
   - Question: ~~How should we handle connection failures?~~ → Application handles (known topology, controlled environment)

2. **Connection State**:
   - Current: Basic connection state tracking
   - Question: Should we track connection health? Quality metrics?
   - Question: How to handle connection state changes?

3. **Multiple Connections**:
   - Current: One connection per endpoint
   - Question: Should we support multiple connections to same peer?
   - Question: How to handle connection redundancy/failover?

### Protocol Handling
4. **Message Serialization**:
   - Current: Uses codec package for serialization
   - Question: Should we support compression? Different encoding formats?
   - Question: How to handle message size limits?

5. **Protocol Messages**:
   - Current: Handles INTENT, INTEREST, MESSAGE, PING, PONG, HANDSHAKE, RESULT
   - Question: Should we support message batching? Batch INTENTS/INTERESTS?
   - Question: How to handle protocol versioning?

6. **Result Handling**:
   - Current: Awkward result handling (TODO comment)
   - Question: Should handlers be able to override result sending?
   - Question: How to improve result handling API?

### Intent/Interest Management
7. **Remote Intent/Interest Wrapping**:
   - Current: TODO comments for LocalWrapped intent/interest
   - Question: What should LocalWrapped intent/interest do?
   - Question: How should wrapping work?

8. **Intent/Interest Propagation**:
   - Current: Propagates intents/interests to remote
   - Question: Should we batch intent/interest advertisements?
   - Question: How to handle intent/interest updates?

9. **Route Registration**:
   - Current: Routes registered with connection
   - Question: Should we support route updates? Route removal?
   - Question: How to handle route conflicts?

### Error Handling
10. **Error Propagation**:
    - Current: Errors logged, some propagated
    - Question: How should errors be propagated to application?
    - Question: Should we support error channels?

11. **Network Errors**:
    - Current: Basic error handling
    - Question: How to distinguish transient vs permanent errors?
    - Question: Should we support retry mechanisms?

12. **Protocol Errors**:
    - Current: Returns errors, logs warnings
    - Question: How to handle malformed messages?
    - Question: Should we support error recovery?

### Performance
13. **Latency Measurement**:
    - Current: Basic ping/pong implementation
    - Assumption: Statistics not needed (controlled environment, known latency patterns)
    - Question: ~~Should we track latency statistics?~~ → Can be added if needed

14. **Message Throughput**:
    - Current: No throughput limits
    - Assumption: Rate limiting not needed (controlled environment, known producers)
    - Question: ~~Should we support rate limiting?~~ → Can be added as middleware if needed
    - Note: Rate limiting can be added later via wrapper functions

15. **Buffer Management**:
    - Current: Uses codec buffer pool
    - Question: Should we tune buffer sizes?
    - Question: How to handle large messages?

### Security
16. **Authentication**:
    - Current: No authentication
    - Question: Should we support TLS? mTLS?
    - Question: How to handle peer authentication?

17. **Authorization**:
    - Current: Path-based routing only
    - Question: Should we support access control?
    - Question: How to restrict intent/interest access?

18. **Message Signing**:
    - Current: Signature field in header (not used)
    - Question: Should we sign all messages? Control messages only?
    - Question: How to manage signing keys?

## Potential Issues

1. **TODO: LocalWrapped Intent/Interest**: Implementation incomplete
2. **Result Handling**: Awkward API (TODO comment in messages.go)
3. **Error Handling**: Some errors logged but not properly handled
4. **Connection Management**: No reconnection logic
5. **Message Batching**: INTENTS/INTERESTS types exist but not fully utilized

## Improvements Needed

1. **Complete LocalWrapped Implementation**: Implement TODO items
2. **Improve Result Handling**: Better API for result handling
3. **Add Reconnection Logic**: Automatic reconnection on failures
4. **Implement Message Batching**: Use INTENTS/INTERESTS for efficiency
5. **Add Metrics**: Track connection health, message throughput, latency
6. **Improve Error Handling**: Better error propagation and recovery

## Protocol Questions

1. **Message Ordering**: Should messages maintain ordering across network?
2. **Reliable Delivery**: Should we support guaranteed delivery?
3. **Flow Control**: Should we implement flow control mechanisms?
4. **Compression**: Should we support message compression?
5. **Encryption**: Should we support message encryption?

