# High-Level Primitives for Decentralized Named Data Messaging

## Purpose

Deliver an opinionated, still lightweight layer on top of core `dndm` that simplifies building:

- **Sensors / embedded nodes** publishing data over constrained transports (serial, SPI, I²C).  
- **Hubs / gateways** that bridge multiple transports (serial ↔ UDP/TCP, mesh links).  
- **Message processors** running on richer SBCs/servers that consume, transform, and republish data.

The layer must preserve DNDM principles (named intents, decentralized discovery) while minimizing cognitive load and manual wiring.

## Guiding Goals

1. **Bus-centric ergonomics** – offer an optional high-level bus runtime that owns router, endpoints, discovery, and typed messaging while leaving core DNDM topology-agnostic.  
2. **Named data clarity** – make the intent/interest naming scheme visible and enforceable.  
3. **Transport abstraction** – encapsulate serial, UDP, mesh, etc. behind consistent connectors.  
4. **Broadcast-aware routing** – automatically choose optimized fan-out strategies where available.  
5. **Embedded friendliness** – deterministic resource usage, no hidden goroutines/allocations.  
6. **Server scalability** – able to run many nodes in-process for simulation/testing.

## Workstreams

### 1. High-Level Bus Abstraction

- Build a single `x/bus` runtime that wraps router, endpoints, discovery, and typed messaging.  
- Define configuration structs for identity, intents/interests, connector definitions, and discovery participation.  
- Support managed mode (runtime builds router/endpoints) and unmanaged mode (caller supplies router).  
- Keep DNDM core untouched; the bus runtime remains an optional but ergonomic layer.

### 2. Transport Connectors Library

- Implement plug-and-play connectors under `x/transport`:
  - Serial (blocking RW using existing stream codec).  
  - UDP (supporting unicast + broadcast).  
  - TCP (optional, only if needed).  
  - In-memory (for tests/unit simulations).  
  - NATS bridge (maps routes to subjects for hybrid deployments).  
  - Hooks for custom connectors (SPI/I²C) via interface.
- Each connector exposes a `EndpointFactory` returning properly configured `remote.Endpoint` or direct endpoint.
- Provide resource tuning (buffer sizes, retry policies, handshake timeouts).

### 3. Broadcast & Delivery Policies

- Define policy annotations on `IntentSpec` (`FanoutAny`, `Exclusive`, `TTL`, etc.).  
- Extend builder so transports inspect policy and choose multicast/broadcast vs repeated unicast.  
- For transports lacking native broadcast, fall back gracefully to multi-send without extra code from user.

### 4. Discovery & Caching Helpers

- Offer optional discovery module:
  - Interest propagation strategies (e.g., limited BFS, gossip).  
  - Cache of “last known providers” per route for faster lookup.  
  - Diagnostics to trace interest → intent resolution path.
- Keep discovery optional to maintain embedded footprint; allow compile-time toggles.

### 5. Lifecycle & Observability

- Provide runtime lifecycle management:
  - Start/stop hooks for connectors and background discovery.  
  - Health probes suitable for RTOS or Linux systemd.  
  - Lightweight metrics (message counts, dropped interests, discovery latency).
- Support structured logging with consistent fields (`node`, `route`, `transport`).

### 6. Typed Application Helpers

- Provide typed wrappers around producers/consumers/callers/modules via the bus runtime.  
- Optional codegen (future) for static typing of protobuf/flatbuffer routes.  
- Provide synchronous helpers (`Call`, `Reply`) consistent with bus runtime semantics.

### 7. Embedded Profiles

- Document minimal configuration for rp2040/stm32: static buffer sizes, no goroutines except explicit ones, compile-time flag to drop advanced discovery/logging.  
- Provide sample firmware snippet showing sensor side in <200 lines.

### 8. Server & Simulation Profiles

- Allow running multiple nodes in a single process for testing: in-memory connectors + timeline control.  
- Provide integration with Go testing (`Topology.TestHarness`) for scenario replay.

### 9. Configuration & Naming Guidance

- Produce documentation describing recommended naming conventions (`domain.device.metric`).  
- Offer validation for duplicate route definitions or conflicting delivery policies.  
- Provide upgrade path if nodes split/merge responsibilities.

### 10. Example Suites

- Build canonical examples mirroring real deployments:
  - “Embedded sensor → host gateway → analytics server” using serial + UDP.  
  - “Mesh of SBCs broadcasting telemetry.”  
  - “Single-producer multi-consumer broadcast.”  
  - “Interest discovery across multi-hop links.”
- Each example must use the new topology builder so users can copy/adapt.

## Dependencies & Sequencing

1. Draft API for topology specs and connectors.  
2. Build in-memory connector & example to validate ergonomics.  
3. Implement serial/UDP connectors (feature guarded).  
4. Layer broadcast policy handling.  
5. Integrate optional discovery caching.  
6. Harden lifecycle/observability on embedded profile.  
7. Document usage and migrate legacy `dnbus` examples to the new bus primitives.

## Non-Goals

- Changing core `dndm` low-level APIs.  
- Introducing mandatory configuration files (DSL can be Go structs).  
- Strong coupling to specific messaging semantics beyond named intents/interests.

## Success Criteria

- New API allows describing a three-node sensor → host → analytics topology in <50 lines without manual forwarders.  
- Works on rp2040-class target (limited goroutines, static memory) and on server (CI tests).  
- Broadcast vs unicast behavior requires no application code changes—only policy hints.  
- Developers can trace interest resolution over multi-hop via built-in diagnostics.

