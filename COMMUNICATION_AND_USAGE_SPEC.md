# Communication and Usage Specification

## Overview

This document specifies the communication protocol, message formats, and usage patterns for the DNDM library.

## Context: Controlled Environment

DNDM is designed for **controlled environments** where:
- The set of intents and interests is known at design time
- The set of clients/producers is known and controlled
- Primary use case is robotics internal message bus
- Runtime validation is minimal (known routes, trusted environment)
- Checks, limits, and validations can be added later (modular architecture)

### Communication Patterns

- **In-Process**: Go channels via Direct endpoint (zero-copy)
- **LAN**: TCP/UDP via Network endpoint (computer-to-computer)
- **Serial**: Serial ports (computer-to-embedded, e.g., RPI ↔ RP2040/ESP32)
- **User Interface**: Network, NATS, or CLI on device (e.g., RPI with Bluetooth keyboard/joystick)

### Broadcast Support

The system may use **UDP broadcast** to reduce network congestion:
- Messages are sent to all instances on the network
- Instances **without interest** should **reject packets early** (at network layer)
- Only interested instances process the message
- This allows efficient broadcast while minimizing processing overhead

## Communication Protocol

### Frame Format

All messages are framed with the following format:

```
[Magic Number: 4 bytes]
[Total Size: 4 bytes]
[Header Size: 4 bytes]
[Header: variable]
[Message Size: 4 bytes]
[Message: variable]
```

**Magic Numbers**:
- `0xFADABEDA`: Standard frame with header
- `0xCEBAFE4A`: Headerless frame (for future use)

**Size Fields**:
- All size fields are 32-bit big-endian unsigned integers
- Total size includes header size, message size, and size fields
- Header size is the size of the protobuf-encoded header
- Message size is the size of the protobuf-encoded message

### Header Format

The header is a protobuf message with the following fields:

```protobuf
message Header {
  uint64 receive_timestamp = 1;  // Set by receiver, overwritten on reception
  uint64 timestamp = 2;           // Set by sender when message is constructed
  Type type = 3;                  // Message type enum
  bool want_result = 4;           // Request response
  bytes signature = 5;            // For authentication (future)
  string route = 6;                // Route identifier
}
```

**Timestamp Handling**:
- `timestamp`: Set by sender when message is constructed (includes marshaling overhead)
- `receive_timestamp`: Set by receiver when message is received (overwrites any value)

### Message Types

#### Control Messages

1. **INTENT** (`Type_INTENT = 2`):
   - Advertises availability to publish data on a route
   - Contains: route, hops, ttl, register flag
   - Sent when Intent is created
   - Can be unregistered (register=false)

2. **INTENTS** (`Type_INTENTS = 3`):
   - Batch advertisement of multiple intents
   - Contains: list of Intent messages
   - Used for efficient bulk operations

3. **INTEREST** (`Type_INTEREST = 4`):
   - Advertises desire to receive data on a route
   - Contains: route, hops, ttl, register flag
   - Sent when Interest is created
   - Can be unregistered (register=false)

4. **INTERESTS** (`Type_INTERESTS = 5`):
   - Batch advertisement of multiple interests
   - Contains: list of Interest messages
   - Used for efficient bulk operations

5. **NOTIFY_INTENT** (`Type_NOTIFY_INTENT = 6`):
   - Notifies Intent that Interest is available
   - Contains: route
   - Sent when Intent-Interest link is established

6. **RESULT** (`Type_RESULT = 7`):
   - Response to a request message
   - Contains: nonce, error code, description
   - Sent when want_result=true

#### Data Messages

7. **MESSAGE** (`Type_MESSAGE = 1`):
   - Actual data payload
   - Contains: user-defined protobuf message
   - Route identifies the message type and path

#### Protocol Messages

8. **PING** (`Type_PING = 8`):
   - Latency measurement request
   - Contains: payload
   - Responded with PONG

9. **PONG** (`Type_PONG = 9`):
   - Latency measurement response
   - Contains: receive_timestamp, ping_timestamp, payload

10. **HANDSHAKE** (`Type_HANDSHAKE = 50`):
    - Connection establishment
    - Contains: me (local peer), you (remote peer), stage, intents, interests
    - Stages: INITIAL, FINAL

11. **PEERS** (`Type_PEERS = 51`):
    - Peer discovery/announcement
    - Contains: remove flag, list of peer IDs

12. **ADDRBOOK** (`Type_ADDRBOOK = 52`):
    - Address book synchronization
    - Contains: address book entries

## Route Format

### Plain Route

Format: `TypeName@path`

Examples:
- `Foo@example.foobar`
- `Bar@sensors.temperature`
- `Image@cameras.front`

**Rules**:
- Path must not contain `@` or `#` characters
- TypeName is the protobuf message name
- Path is hierarchical (dot-separated)

### Hashed Route

Format: `prefix#Base64Hash`

Examples:
- `example#AbCdEfGhIjKlMnOpQrStUvWxYz1234567890`
- `sensors#XYZ123...`

**Rules**:
- Prefix must not contain `@` or `#` characters
- Hash is Base64-encoded SHA1 of `TypeName@fullpath`
- Prefix is used for routing, hash for security

## Peer Format

Format: `scheme://address/path?param1=value1&param2=value2`

Examples:
- `tcp://192.168.1.100:8080/robot.sensors`
- `udp://example.com:9999/cloud.processors`
- `serial:///dev/ttyUSB0/embedded.actuators?baud=115200`

**Components**:
- `scheme`: Transport protocol (tcp, udp, serial, etc.)
- `address`: Network address or device path
- `path`: Hierarchical peer path for routing
- `params`: Query parameters for configuration

## Usage Patterns

### Basic Publish-Subscribe

```go
// Publisher
intent, err := router.Publish("example.data", &MyMessage{})
if err != nil {
    // handle error
}
defer intent.Close()

// Wait for interest
select {
case route := <-intent.Interest():
    // Send messages
    intent.Send(ctx, &MyMessage{Data: "hello"})
}

// Subscriber
interest, err := router.Subscribe("example.data", &MyMessage{})
if err != nil {
    // handle error
}
defer interest.Close()

// Receive messages
for msg := range interest.C() {
    myMsg := msg.(*MyMessage)
    // Process message
}
```

### Multiple Publishers/Subscribers

```go
// Multiple publishers can publish to same route
go publisher1(ctx, router)
go publisher2(ctx, router)

// Multiple subscribers can subscribe to same route
go subscriber1(ctx, router)
go subscriber2(ctx, router)
```

### Route Matching

Routes are matched exactly:
- `Foo@example` matches only `Foo@example`
- `Foo@example.bar` is different from `Foo@example`

Peer paths use prefix matching:
- Peer path `example.foo` matches routes starting with `example.foo`
- Route `example.foo.bar` matches peer path `example.foo`
- Route `example.bar` does not match peer path `example.foo`

## Protocol Questions

### Message Handling
1. **Message Ordering**: 
   - Should messages maintain ordering across network?
   - Should we support ordered vs unordered delivery?
   - How to handle out-of-order messages?

2. **Message Delivery**:
   - Should we support guaranteed delivery?
   - How to handle message acknowledgments?
   - Should we support message retransmission?

3. **Message Priority**:
   - Should we support message priority levels?
   - How to handle priority in routing?
   - Should control messages have higher priority?

### Intent/Interest Advertisement
4. **Advertisement Timing**:
   - When should intents/interests be advertised?
   - Should we support re-advertisement?
   - How to handle advertisement failures?

5. **Batch Advertisement**:
   - When should INTENTS/INTERESTS be used?
   - Should we support automatic batching?
   - How to handle batch size limits?

6. **Advertisement Scope**:
   - Should advertisements be scoped to peer?
   - How to handle advertisement propagation?
   - Should we support advertisement filtering?

### Route Resolution
7. **Route Discovery**:
   - How should routes be discovered in mesh networks?
   - Should we support route advertisement protocol?
   - How to handle route conflicts?

8. **Route Caching**:
   - Should we cache route mappings?
   - How to handle route updates?
   - Should we support route expiration?

9. **Wildcard Routes**:
   - Should we support wildcard routes?
   - How to match wildcard routes?
   - Should we support regex routes?

### Peer Communication
10. **Connection Establishment**:
    - How should connections be established?
    - Should we support connection retries?
    - How to handle connection failures?

11. **State Synchronization**:
    - How should state be synchronized after connection?
    - Should we exchange intents/interests during handshake?
    - How to handle state conflicts?

12. **Peer Discovery**:
    - How should peers discover each other?
    - Should we support peer announcement?
    - How to handle peer updates?

### Error Handling
13. **Error Propagation**:
    - How should errors be propagated?
    - Should we support error channels?
    - How to distinguish error types?

14. **Error Recovery**:
    - How should errors be recovered?
    - Should we support automatic retry?
    - How to handle permanent errors?

15. **Error Reporting**:
    - What errors should be reported to application?
    - Should we support error callbacks?
    - How to handle protocol errors?

### Performance
16. **Message Batching**:
    - When should messages be batched?
    - How to determine batch size?
    - How to handle batch timeouts?

17. **Compression**:
    - Should we compress messages?
    - Which compression algorithms?
    - When should compression be used?

18. **Flow Control**:
    - Should we implement flow control?
    - How to handle backpressure?
    - Should we support rate limiting?

### Security
19. **Authentication**:
    - How should peers authenticate?
    - Should we support TLS? mTLS?
    - How to handle authentication failures?

20. **Authorization**:
    - How to control access to routes?
    - Should we support ACLs?
    - How to handle authorization failures?

21. **Message Signing**:
    - Should messages be signed?
    - Which signing algorithm?
    - How to manage signing keys?

## Usage Questions

### API Design
1. **Type Safety**: 
   - How to improve type safety at API level?
   - Should we support typed wrappers?
   - How to avoid type casting?

2. **Error Handling**:
   - Should we use error channels?
   - How to distinguish error types?
   - Should we support error callbacks?

3. **Configuration**:
   - How should the library be configured?
   - Should we support configuration files?
   - How to handle configuration validation?

### Integration
4. **EasyRobot Integration**:
   - How to migrate from EasyRobot?
   - What compatibility should be maintained?
   - How to handle breaking changes?

5. **Multi-Device Communication**:
   - How to handle multiple sensors/actuators?
   - How to handle multi-board SOCs?
   - How to integrate with online servers?

6. **Deployment**:
   - How to deploy in different environments?
   - How to handle configuration across devices?
   - How to manage updates?

### Testing
7. **Unit Testing**:
   - How to test intent/interest behavior?
   - How to test network code?
   - How to test concurrent behavior?

8. **Integration Testing**:
   - How to test mesh networks?
   - How to test failure scenarios?
   - How to test performance?

9. **End-to-End Testing**:
   - How to test complete systems?
   - How to test multi-device scenarios?
   - How to test distributed behavior?

