# NATS Network Transport Implementation Plan

## Overview

This document outlines the implementation plan for adding NATS (NATS.io) support as a network transport for the DNDM library. NATS provides a pub/sub messaging system that can act as a message broker for distributed DNDM systems.

## Architecture Analysis

### NATS Characteristics

Unlike TCP/UDP which are peer-to-peer, NATS is:
- **Broker-based**: Messages go through a NATS server
- **Pub/Sub**: Uses subjects (topics) for routing messages
- **Connection-oriented**: Clients connect to a NATS server, not directly to each other
- **Multiplexed**: Single NATS connection can handle multiple subjects

### Design Considerations

1. **Connection Model**: 
   - Single NATS connection per Node instance
   - Virtual "connections" represented by subject subscriptions
   - Need to map DNDM routes to NATS subjects

2. **Peer Identity**:
   - NATS doesn't have direct peer-to-peer concept
   - Need to embed peer information in messages or subjects
   - Could use subject prefixes for peer paths

3. **Dial vs Server**:
   - NATS doesn't fit traditional Dial/Server pattern
   - "Dial" = publish/subscribe to subjects
   - "Server" = subscribe to subjects (less relevant in broker model)

## Implementation Plan

### Phase 1: Core NATS Connection

#### 1.1 NATS Node Structure

**File**: `network/nats/node.go`

```go
package nats

import (
    "context"
    "io"
    "log/slog"
    
    "github.com/nats-io/nats.go"
    
    "github.com/itohio/dndm"
    "github.com/itohio/dndm/network"
)

type Node struct {
    conn   *nats.Conn
    logger *slog.Logger
    peer   dndm.Peer
}

func New(logger *slog.Logger, peer dndm.Peer) (*Node, error) {
    // Parse NATS URL from peer address
    // Connect to NATS server
    // Return Node instance
}
```

**Tasks**:
- [ ] Parse NATS connection URL from peer address
- [ ] Connect to NATS server with options (TLS, auth, etc.)
- [ ] Handle connection lifecycle
- [ ] Implement connection health monitoring

**Questions to Answer**:
1. What should the peer address format be? (`nats://nats-server:4222/path`?)
2. Should we support NATS connection options (TLS, auth, credentials)?
3. How to handle connection failures and reconnection?

#### 1.2 NATS Connection Management

**File**: `network/nats/conn.go`

```go
type Conn struct {
    // Wraps NATS connection for a specific peer
    // Manages subscriptions for routes
    // Handles message encoding/decoding
}
```

**Tasks**:
- [ ] Implement network.Conn interface
- [ ] Manage NATS subscriptions for routes
- [ ] Handle message routing to/from NATS subjects
- [ ] Implement route registration/unregistration

**Questions to Answer**:
1. Should each "virtual connection" use a separate NATS connection or share one?
2. How to map DNDM routes to NATS subjects?
3. How to handle peer identity in broker model?

### Phase 2: Route to Subject Mapping

#### 2.1 Subject Mapping Strategy

**File**: `network/nats/subject.go`

**Options**:
1. **Direct mapping**: `route.ID()` → NATS subject
2. **Hierarchical**: `path.type` → NATS subject with wildcards
3. **Peer-prefixed**: `peer.path.type` → NATS subject

**Recommended**: Hierarchical mapping with peer path support

```
Route: "Foo@example.sensors"
Subject: "dndm.example.sensors.Foo"  or  "dndm.example.sensors"
```

**Tasks**:
- [ ] Implement route to subject mapping
- [ ] Support wildcard subscriptions for path matching
- [ ] Handle route encoding (special characters)
- [ ] Support both plain and hashed routes

**Questions to Answer**:
1. What subject prefix should we use? (`dndm.`?)
2. How to handle route special characters in subjects?
3. Should we support subject wildcards for route matching?
4. How to handle hashed routes in subjects?

#### 2.2 Peer Path to Subject Mapping

**File**: `network/nats/peer.go`

**Tasks**:
- [ ] Map peer paths to subject prefixes
- [ ] Support subject hierarchy matching
- [ ] Handle peer path updates

**Questions to Answer**:
1. Should peer paths be part of the subject hierarchy?
2. How to match routes with peer path prefixes?
3. Should we support subject namespaces per peer?

### Phase 3: Message Handling

#### 3.1 Message Publishing

**File**: `network/nats/writer.go`

**Tasks**:
- [ ] Implement Write() method
- [ ] Encode DNDM messages to NATS format
- [ ] Publish to appropriate subject
- [ ] Handle message headers/metadata
- [ ] Support message size limits

**Questions to Answer**:
1. Should we embed DNDM header in NATS message?
2. How to handle message metadata (timestamp, route)?
3. Should we use NATS headers feature?
4. What's the maximum message size?

#### 3.2 Message Subscription

**File**: `network/nats/reader.go`

**Tasks**:
- [ ] Implement Read() method
- [ ] Subscribe to NATS subjects
- [ ] Decode NATS messages to DNDM format
- [ ] Handle message filtering
- [ ] Support context cancellation

**Questions to Answer**:
1. How to handle multiple subscriptions per connection?
2. Should we use queue groups for load balancing?
3. How to handle subscription failures?
4. Should we support subscription filters?

### Phase 4: Dial and Server Implementation

#### 4.1 Dial Implementation

**File**: `network/nats/dial.go`

Since NATS is broker-based, "dialing" is more about:
- Creating a virtual connection representation
- Setting up subscriptions for the peer
- Returning a Conn interface

**Tasks**:
- [ ] Implement Dial() method
- [ ] Create virtual connection for peer
- [ ] Set up route subscriptions
- [ ] Return Conn implementation

**Questions to Answer**:
1. Should Dial create a new NATS connection or reuse existing?
2. How to represent "connected peer" in broker model?
3. Should we support peer discovery via NATS?

#### 4.2 Server Implementation

**File**: `network/nats/server.go`

For NATS, "server" mode is less relevant, but we can:
- Subscribe to subjects representing this peer
- Accept incoming messages for this peer
- Handle peer announcements

**Tasks**:
- [ ] Implement Serve() method
- [ ] Subscribe to peer-specific subjects
- [ ] Handle incoming connections (virtual)
- [ ] Support peer announcements

**Questions to Answer**:
1. How should Serve() work in broker model?
2. Should we subscribe to a peer announcement subject?
3. How to handle multiple peers on same NATS connection?

### Phase 5: Integration with DNDM

#### 5.1 Endpoint Integration

**File**: `network/nats/endpoint.go`

**Tasks**:
- [ ] Create NATS endpoint wrapper
- [ ] Integrate with remote endpoint
- [ ] Handle intent/interest propagation
- [ ] Support mesh network patterns

**Questions to Answer**:
1. Should NATS endpoint be used with remote or mesh endpoint?
2. How to handle intent/interest advertisement in NATS?
3. Should we use NATS for peer discovery?

#### 5.2 Factory Integration

**Update**: `network/factory.go`

**Tasks**:
- [ ] Register NATS node in factory
- [ ] Support NATS scheme (`nats://`)
- [ ] Handle NATS-specific options

**Questions to Answer**:
1. What scheme should we use? (`nats://`?)
2. Should we support NATS cluster URLs?
3. How to handle NATS connection options?

### Phase 6: Advanced Features

#### 6.1 NATS JetStream Support (Optional)

**File**: `network/nats/jetstream.go`

JetStream provides:
- Message persistence
- At-least-once delivery
- Message replay

**Tasks**:
- [ ] Add JetStream support
- [ ] Implement persistent message delivery
- [ ] Support message replay

**Questions to Answer**:
1. Should JetStream be optional or required?
2. Which routes should use JetStream?
3. How to configure JetStream streams?

#### 6.2 NATS Request-Reply (Optional)

**File**: `network/nats/request.go`

NATS supports request-reply pattern which could be useful for:
- Handshake messages
- Result messages
- Synchronous operations

**Tasks**:
- [ ] Implement NATS request-reply
- [ ] Use for RESULT messages
- [ ] Support timeout handling

**Questions to Answer**:
1. Should we use request-reply for all RESULT messages?
2. How to map DNDM request-reply to NATS?
3. What timeout should we use?

## File Structure

```
network/nats/
├── node.go              # NATS Node implementation
├── conn.go              # NATS Conn implementation
├── subject.go           # Route to subject mapping
├── peer.go              # Peer path handling
├── writer.go            # Message publishing
├── reader.go            # Message subscription
├── dial.go              # Dial implementation
├── server.go            # Server implementation
├── endpoint.go          # Endpoint integration
├── jetstream.go         # JetStream support (optional)
├── request.go           # Request-reply support (optional)
├── options.go           # NATS-specific options
├── errors.go            # NATS-specific errors
└── IMPLEMENTATION_PLAN.md # This file
```

## Dependencies

### Required

- `github.com/nats-io/nats.go` - NATS Go client library

### Optional

- `github.com/nats-io/jetstream` - JetStream support (if implementing Phase 6.1)

## Configuration Options

### Peer Address Format

Proposed format: `nats://[user:pass@]host:port[/path]?options`

Examples:
- `nats://localhost:4222/example.robot`
- `nats://user:pass@nats.example.com:4222/cloud.processors?tls=true`
- `nats://nats-server:4222?cluster=nats-cluster`

### Peer Parameters

- `tls`: Enable TLS (`true`/`false`)
- `tls_cert`: TLS certificate file
- `tls_key`: TLS key file
- `tls_ca`: TLS CA certificate file
- `credentials`: NATS credentials file
- `user`: NATS username
- `pass`: NATS password
- `token`: NATS token
- `cluster`: NATS cluster name
- `js`: Enable JetStream (`true`/`false`)

### Node Options

```go
type Options struct {
    URL           string
    Name          string
    AllowReconnect bool
    MaxReconnects  int
    ReconnectWait  time.Duration
    Timeout        time.Duration
    TLSConfig      *tls.Config
    Credentials    string
    Token          string
    User           string
    Password       string
}
```

## Implementation Steps

### Step 1: Basic NATS Connection (Phase 1)
1. Create `network/nats/` directory
2. Add NATS dependency to `go.mod`
3. Implement `Node` struct and `New()` function
4. Add connection management
5. Test basic connection to NATS server

### Step 2: Route Mapping (Phase 2)
1. Implement subject mapping functions
2. Add route encoding/decoding
3. Test subject generation from routes
4. Support peer path mapping

### Step 3: Message Handling (Phase 3)
1. Implement message publishing
2. Implement message subscription
3. Test message round-trip
4. Handle message headers

### Step 4: Dial/Server (Phase 4)
1. Implement Dial() method
2. Implement Serve() method
3. Test virtual connections
4. Handle peer announcements

### Step 5: Integration (Phase 5)
1. Create endpoint wrapper
2. Integrate with factory
3. Test with remote endpoint
4. Test with mesh endpoint (if applicable)

### Step 6: Advanced Features (Phase 6) - Optional
1. Add JetStream support
2. Add request-reply support
3. Add additional NATS features

## Testing Strategy

### Unit Tests
- [ ] Test NATS connection establishment
- [ ] Test route to subject mapping
- [ ] Test message encoding/decoding
- [ ] Test subscription management
- [ ] Test error handling

### Integration Tests
- [ ] Test with NATS server
- [ ] Test multiple peers on same connection
- [ ] Test message routing
- [ ] Test peer discovery (if implemented)
- [ ] Test connection failures and reconnection

### Performance Tests
- [ ] Benchmark message throughput
- [ ] Benchmark connection establishment
- [ ] Test under load
- [ ] Compare with TCP implementation

## Design Decisions Needed

### 1. Connection Model

**Option A**: Single NATS connection per Node, multiple virtual connections
- Pros: Efficient, single connection to manage
- Cons: Shared connection state, complex routing

**Option B**: Separate NATS connection per virtual connection
- Pros: Clean separation, simpler routing
- Cons: More connections, higher overhead

**Recommendation**: Option A - more efficient and aligns with NATS design

### 2. Subject Mapping

**Option A**: `dndm.<path>.<type>`
- Example: `dndm.example.sensors.Foo`

**Option B**: `<peer-path>.<path>.<type>`
- Example: `robot.example.sensors.Foo`

**Option C**: Just use route ID directly
- Example: `Foo@example.sensors`

**Recommendation**: Option B - includes peer path for better routing

### 3. Peer Discovery

**Option A**: Use NATS subjects for peer discovery
- Subscribe to peer announcement subject
- Publish peer information

**Option B**: Use external discovery mechanism
- DNS, configuration, etc.

**Recommendation**: Option A - leverage NATS for discovery

### 4. JetStream Usage

**Option A**: Always use JetStream
- Pros: Reliable delivery
- Cons: More overhead, requires JetStream enabled

**Option B**: Optional JetStream
- Use for certain routes
- Configuration-based

**Recommendation**: Option B - flexible, allows optimization

## Questions to Answer Before Implementation

1. **Connection Model**: Single connection or per-peer connections?
2. **Subject Mapping**: What format for subjects?
3. **Peer Identity**: How to identify peers in broker model?
4. **Peer Discovery**: Use NATS for discovery or external?
5. **JetStream**: Required or optional?
6. **Request-Reply**: Use NATS request-reply or custom?
7. **Error Handling**: How to handle NATS-specific errors?
8. **Configuration**: What options should be exposed?
9. **Testing**: How to test without external NATS server?
10. **Performance**: What are the performance targets?

## Estimated Effort

- **Phase 1** (Core Connection): 2-3 days
- **Phase 2** (Route Mapping): 1-2 days
- **Phase 3** (Message Handling): 2-3 days
- **Phase 4** (Dial/Server): 2-3 days
- **Phase 5** (Integration): 1-2 days
- **Phase 6** (Advanced Features): 3-5 days (optional)

**Total**: ~10-15 days for basic implementation, +5 days for advanced features

## Risks and Mitigation

### Risk 1: NATS Doesn't Fit Dial/Server Pattern
**Mitigation**: Use virtual connections and adapt the pattern

### Risk 2: Subject Mapping Complexity
**Mitigation**: Start simple, iterate based on feedback

### Risk 3: Performance Concerns
**Mitigation**: Benchmark early, optimize hot paths

### Risk 4: Peer Identity in Broker Model
**Mitigation**: Embed peer info in messages/subjects

## Next Steps

1. **Answer Design Questions**: Resolve all questions in this document
2. **Create Design Document**: Document final design decisions
3. **Set Up Development Environment**: NATS server, dependencies
4. **Implement Phase 1**: Basic NATS connection
5. **Iterate**: Implement and test each phase
6. **Document**: Update documentation with NATS usage
7. **Integrate**: Add to factory and examples

## References

- [NATS Go Client Documentation](https://docs.nats.io/nats-client/nats-client)
- [NATS Protocol](https://docs.nats.io/nats-protocol/nats-protocol)
- [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
- [NATS Subject-based Messaging](https://docs.nats.io/nats-concepts/subjects)

