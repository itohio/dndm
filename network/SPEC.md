# Network Package Specification

## Overview

The `network` package provides abstractions for network communication: Conn (connection), Dialer (outbound connections), Server (inbound connections), and Node (both dialer and server).

## Design Assumptions (Controlled Environment)

This package is designed for controlled environments where:
- **LAN Communication**: TCP/UDP between computers (known topology)
- **Serial Communication**: Serial ports to embedded devices (RP2040/ESP32)
- **Broadcast Support**: UDP broadcast may be used - receivers without interest should reject packets early
- **Trusted Environment**: No authentication required, known peers
- **Known Routes**: Route set is known, no dynamic discovery needed

## Current Implementation

- `Conn` interface for bidirectional communication
- `Dialer` interface for outbound connections
- `Server` interface for inbound connections
- `Node` interface combining both
- Implementations: `net` (TCP/UDP), `serial` (serial ports), `stream` (stream abstraction)

## Key Questions

### Connection Interface
1. **Connection Lifecycle**:
   - Current: Basic connection management
   - Question: Should we support connection pooling?
   - Question: How to handle connection health checks?

2. **Peer Information**:
   - Current: Local/Remote peer methods
   - Question: Should we support peer metadata?
   - Question: How to handle peer updates?

3. **Route Management**:
   - Current: AddRoute/DelRoute methods
   - Question: Should we support route queries?
   - Question: How to handle route conflicts?

4. **Message Handling**:
   - Current: Read/Write methods with context
   - Question: Should we support async message handling?
   - Question: How to handle message priority?

### Dialer Interface
5. **Dialing Strategies**:
   - Current: Basic dial implementation
   - Question: Should we support dial timeouts? Retries?
   - Question: How to handle dial failures?

6. **Connection Options**:
   - Current: DialOpt for options
   - Question: What options should be supported?
   - Question: Should we support TLS options?

7. **Multiple Connections**:
   - Current: One connection per dial
   - Question: Should we support connection pooling?
   - Question: How to handle multiple connections to same peer?

### Server Interface
8. **Listening**:
   - Current: Serve with onConnect callback
   - Question: Should we support multiple listeners?
   - Question: How to handle listener failures?

9. **Connection Acceptance**:
   - Current: onConnect callback
   - Question: Should we support connection filtering?
   - Question: How to handle connection limits?

10. **Server Options**:
    - Current: SrvOpt for options
    - Question: What options should be supported?
    - Question: Should we support TLS options?

### Stream Implementation
11. **Stream Context**:
    - Current: StreamContext for context-aware operations
    - Question: How to handle context cancellation better?
    - Question: Should we support stream timeouts?

12. **Message Handlers**:
    - Current: Handler map for protocol messages
    - Question: Should we support handler chains?
    - Question: How to handle handler errors?

13. **Buffer Management**:
    - Current: Uses codec buffer pool
    - Question: Should we tune buffer sizes?
    - Question: How to handle large messages?

### Transport Protocols
14. **TCP/UDP**:
    - Current: Basic net implementation
    - Primary Use: TCP for reliable LAN communication, UDP for broadcast
    - **UDP Broadcast**: Messages sent to all instances, receivers without interest should reject early
    - Assumption: TCP options (keep-alive, no-delay) can be added if needed
    - Question: ~~Should we support UDP reliability?~~ → Not needed (controlled environment)

15. **Serial Ports**:
    - Current: Basic serial implementation
    - Primary Use: Computer-to-embedded device communication (RPI ↔ RP2040/ESP32)
    - Assumption: Serial port configuration via peer parameters (sufficient)
    - Question: How to handle serial port errors? → Application handles (known topology)

16. **Other Transports**:
    - Current: Limited transport support
    - Question: Should we support WebSocket? QUIC? Unix sockets?
    - Question: How to add new transport types?

### Error Handling
17. **Network Errors**:
    - Current: Basic error handling
    - Question: How to distinguish transient vs permanent errors?
    - Question: Should we support error recovery?

18. **Protocol Errors**:
    - Current: Errors returned
    - Question: How to handle malformed messages?
    - Question: Should we support error recovery?

19. **Connection Errors**:
    - Current: Connection closed on error
    - Question: Should we support connection retry?
    - Question: How to handle partial failures?

### Performance
20. **Connection Pooling**:
    - Current: No pooling
    - Question: Should we support connection pooling?
    - Question: How to manage pool size?

21. **Message Batching**:
    - Current: No batching
    - Question: Should we support message batching?
    - Question: How to handle batch timeouts?

22. **Throughput**:
    - Current: No throughput limits
    - Question: Should we support rate limiting?
    - Question: How to handle high message rates?

### Security
23. **TLS Support**:
    - Current: No TLS support
    - Question: Should we support TLS? mTLS?
    - Question: How to configure TLS?

24. **Authentication**:
    - Current: No authentication
    - Question: Should we support authentication?
    - Question: How to handle peer authentication?

25. **Encryption**:
    - Current: No encryption
    - Question: Should we support message encryption?
    - Question: How to manage encryption keys?

## Potential Issues

1. **Context Cancellation**: StreamContext has complex context handling that may leak
2. **Buffer Management**: Buffer pool management could be improved
3. **Error Handling**: Some errors may not be properly handled
4. **Connection Lifecycle**: Connection cleanup may not always work correctly

## Improvements Needed

1. **Add TLS Support**: Support for secure connections
2. **Improve Context Handling**: Better context cancellation support
3. **Add Connection Pooling**: For better resource management
4. **Improve Error Handling**: Better error propagation and recovery
5. **Add Metrics**: Track connection health, throughput, errors
6. **Add More Transports**: WebSocket, QUIC, etc.

