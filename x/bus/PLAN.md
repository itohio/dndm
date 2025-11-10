# Bus Runtime Evolution Plan (formerly `dnbus`)

## Role

The bus runtime is the lightweight, typed façade over core `dndm` primitives. It must scale from microcontrollers to multi-core servers while hiding linker/endpoint management behind approachable APIs. The runtime itself acts as the high-level orchestrator (superseding the idea of a separate `x/node` package) while keeping the core topology-agnostic. During the transition the code lives in `x/bus`; it previously existed under `x/dnbus`.

## Objectives

1. **Bus-friendly API surface** – provide a single runtime abstraction while still allowing manual router usage.  
2. **Transport-agnostic simplicity** – sensors, hubs, and processors can wire producers/consumers without managing endpoints manually.  
3. **Broadcast-aware semantics** – automatically handle fan-out while keeping the API identical on embedded and server targets.  
4. **Deterministic resource usage** – predictable goroutine/memory footprint with knobs for constrained nodes.  
5. **Strong typing & ergonomics** – generics-based producers/consumers/callers that remain testable and mockable.  
6. **Comprehensive documentation & examples** – demonstrate real-world sensor ↔ hub ↔ processor workflows.

## Work Items

### 1. Runtime API Surface

- Maintain existing core types (`Producer`, `Consumer`, `Caller`, `Service`, `Module`) and offer a unified `Runtime` that can either build its own router or wrap an existing one.  
- Provide configuration-driven factories (e.g., `New(ctx, Config)`) as well as helper constructors for unmanaged routers.  
- Validate configuration at startup (duplicate routes, missing connectors, conflicting delivery policies).

### 2. Producer/Consumer Enhancements

- Support optional delivery hints (`DeliveryPolicy`) alongside constructor options (e.g., `WithFanout()`, `WithExclusive()`).  
- Offer zero-allocation send/receive helpers for embedded builds (pre-allocated message reuse patterns).  
- Expose instrumentation hooks (callbacks or counters) without mandating their use.

### 3. Caller/Service Workflow

- Document and harden correlation strategy with option hooks for nonce setters/extractors.  
- Provide explicit shutdown semantics so streaming reply handlers exit cleanly when context canceled.  
- Add high-level convenience functions for single-request/single-response flows used by embedded RPCs.

### 4. Module Abstraction

- Redesign `Module` as a declarative pipeline:  
  - Continue using package-level `AddInput`/`AddOutput`, extend with policy hints (expected throughput, delivery policy).  
  - Introduce `RunPipeline(ctx, func(ctx context.Context, inputs Inputs, outputs Outputs) error)` to encapsulate lifecycle.  
- Provide built-in fan-out/fan-in utilities to simplify multi-sensor processing.

### 5. Connector Cooperation

- Define thin adapter interfaces so the runtime can request connectors by name (`Connector("serial")`).  
- Ensure default connectors (in-memory, serial, UDP, NATS) have ergonomic constructors for both managed and unmanaged modes.  
- Document how subject naming maps to DNDM routes for the NATS bridge and how optional features (queues, JetStream) interact with delivery policies.

### 6. Discovery & Diagnostics

- Add opt-in tracing utilities (e.g., `TraceRoute(routeID)` returning current intent/interest graph).  
- Surface discovery events (interest registered, intent located) via callbacks or structured logs for debugging multi-hop networks.

### 7. Embedded Profiles

- Introduce build tags or options for `tiny` profile:  
  - No background goroutines unless explicitly started.  
  - Minimal logging, compile-time removal of diagnostics.  
  - Default buffer sizes tuned for ≤64 KB RAM.  
- Provide reference firmware example (rp2040) using only the tiny profile.

### 8. Server/Testing Utilities

- Add in-memory harness (`SimulatedRuntime`, `SimulatedLink`) for integration tests.  
- Supply test helpers to spin up multiple runtimes within a Go test for scenario replay.  
- Ensure CI covers mixed transports via mocks to validate fan-out/broadcast behavior.

### 9. Documentation & Examples

- Rewrite `SPEC.md` to match the new unified runtime once stabilization occurs.  
- Expand examples:  
  - Embedded sensor → host gateway → analytics pipeline.  
  - Multi-host broadcast + targeted command flows.  
  - Request/response over mixed transports.  
- Provide migration notes for existing users of low-level DNBus APIs.

### 10. Quality & Tooling

- Maintain exhaustive unit tests for producers/consumers/callers.  
- Add scenario tests covering broadcast, multi-hop forwarding, and graceful shutdown (using both managed and unmanaged runtime usage).  
- Ensure gofmt/linters pass; integrate with repository CI expectations.

## Sequencing

1. Implement managed runtime constructor with connector/config support.  
2. Ensure unmanaged helper wraps existing routers cleanly.  
3. Layer delivery policies and embedded/server profiles.  
4. Expand module abstraction and service/caller ergonomics.  
5. Refresh documentation/examples.  
6. Stabilize diagnostics and test harnesses.  
7. Rename package to `x/bus` once stable and update imports/specs.

## Notes

- Keep the bus runtime free from transport-specific logic; rely on connectors supplied by configuration or caller.  
- Preserve backward-compatible APIs where feasible, but prioritize clarity and reduced boilerplate.  
- Performance and determinism must be measurable on both constrained and server environments.  
- Track renaming tasks (dnbus → bus) in follow-up issues to avoid confusion.
