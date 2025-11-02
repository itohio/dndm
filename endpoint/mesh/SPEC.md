# Mesh Endpoint Package Specification

## Overview

The `mesh` package provides a full-mesh network endpoint implementation. It manages multiple remote endpoints, implements peer discovery, and handles automatic connection establishment.

## Design Assumptions (Controlled Environment)

This endpoint is designed for controlled environments where:
- Network topology is known (LAN environment)
- Set of peers is known (no open discovery needed)
- Trusted network (no authentication required)
- Primary use: Automatic peer discovery within known network
- UDP broadcast may be used - receivers should reject packets early if not interested

## Current Implementation

- Aggregates multiple remote endpoints via Container
- Manages address book for peer discovery
- Implements handshake protocol for connection establishment
- Supports automatic peer dialing
- Routes intents/interests based on peer path prefixes

## Key Questions

### Peer Discovery
1. **Discovery Mechanisms**:
   - Current: Address book with manual/peer-provided entries
   - Question: Should we support mDNS/Bonjour? DHT? Centralized registry?
   - Question: How to handle NAT traversal?

2. **Address Book Management**:
   - Current: Basic address book implementation
   - Question: How should address book be synchronized?
   - Question: Should we support address book persistence?

3. **Peer Announcement**:
   - Current: Peers announced during handshake
   - Question: Should we support periodic peer announcements?
   - Question: How to handle peer updates?

### Connection Management
4. **Connection Establishment**:
   - Current: Automatic dialing with multiple dialers
   - Question: Should we support connection strategies? (aggressive, conservative)
   - Question: How to handle dial failures? Retry? Backoff?

5. **Full-Mesh Requirements**:
   - Current: Attempts full-mesh topology
   - Question: Should we support partial mesh? Star topology?
   - Question: How to handle mesh size limits?

6. **Connection Lifecycle**:
   - Current: Basic connection management
   - Question: How to handle connection failures? Automatic reconnection?
   - Question: Should we support connection health monitoring?

### Handshake Protocol
7. **Handshake Implementation**:
   - Current: Very rudimentary (FIXME comment)
   - Question: What should proper handshake include?
   - Question: Should handshake include intent/interest exchange?

8. **Handshake Stages**:
   - Current: INITIAL, FINAL stages
   - Question: Should we support more stages? Capability negotiation?
   - Question: How to handle handshake failures?

9. **Intent/Interest Synchronization**:
   - Current: FIXME - Intents nil in handshake
   - Question: Should handshake exchange intents/interests?
   - Question: How to synchronize state after connection?

### Routing
10. **Path-Based Routing**:
    - Current: Prefix matching based on peer paths
    - Question: How to handle route conflicts? Multiple peers with same path?
    - Question: Should we support route priorities?

11. **Routing Decisions**:
    - Current: Route based on peer path prefix
    - Question: Should we support load balancing? Path cost?
    - Question: How to handle routing failures?

12. **Route Propagation**:
    - Current: Routes propagated via intent/interest
    - Question: Should we support route advertisement? Route discovery?
    - Question: How to handle route updates?

### Peer Management
13. **Peer Paths**:
    - Current: Each peer has unique path
    - Question: How to handle peer path collisions?
    - Question: Should we support peer path negotiation?

14. **Peer Capabilities**:
    - Current: No capability negotiation
    - Question: Should we support peer capabilities? Feature negotiation?
    - Question: How to handle incompatible peers?

15. **Peer Lifecycle**:
    - Current: Basic peer management
    - Question: How to handle peer disconnection?
    - Question: Should we support peer cleanup?

### Aggregation
16. **Aggregate Peer**:
    - Current: AggregatePeer for remote peer representation
    - Question: How should aggregate peer work?
    - Question: How to handle multiple peers with same prefix?

17. **Endpoint Aggregation**:
    - Current: Container aggregates endpoints
    - Question: Should we support endpoint priority?
    - Question: How to handle endpoint failures in aggregation?

### Scalability
18. **Mesh Size**:
    - Current: No size limits
    - Question: Should we support mesh size limits?
    - Question: How to handle large mesh networks?

19. **Network Partitions**:
    - Current: No partition handling
    - Question: How to detect network partitions?
    - Question: How to handle partition recovery?

20. **Performance**:
    - Current: Full-mesh can be expensive
    - Question: Should we support mesh optimization? Clustering?
    - Question: How to reduce connection overhead?

## Potential Issues

1. **Handshake Implementation**: Very rudimentary (FIXME comment)
2. **Intent/Interest Exchange**: Not included in handshake (FIXME)
3. **Peer Collision**: TBD in README - collision resolution not implemented
4. **Connection Management**: No reconnection logic
5. **Address Book**: Basic implementation, may need improvements

## Improvements Needed

1. **Complete Handshake**: Implement proper handshake protocol
2. **Intent/Interest Exchange**: Include in handshake for state synchronization
3. **Collision Resolution**: Implement peer path collision resolution
4. **Connection Resilience**: Add automatic reconnection
5. **Metrics**: Track mesh health, connection quality, routing efficiency
6. **Testing**: Add integration tests for mesh behavior

## Protocol Questions

1. **Handshake Messages**: What messages should handshake include?
2. **State Synchronization**: How to synchronize state after connection?
3. **Peer Discovery**: How should peers discover each other?
4. **Route Advertisement**: Should we support route advertisement protocol?
5. **Network Topology**: Should we support different topologies?

