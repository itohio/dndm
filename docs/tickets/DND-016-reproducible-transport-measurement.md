---
title: "Measure only implemented comparable transports"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-016"
status: "backlog"
owner: "unassigned"
priority: "P2"
milestone: "M4 measurement"
kind: "research / empirical measurement"
dependencies: ["DND-001", "DND-003", "DND-004", "DND-005", "DND-007", "DND-008", "DND-013", "DND-015"]
implementation_evidence: []
---

# DND-016 — Measure only implemented comparable transports

## State and permission

Status: **backlog**. Owner: **unassigned**. Kind: **research / empirical measurement**.
Milestone: **M4 measurement**. Priority: **P2** — No throughput, latency, loss, allocation or footprint ranking exists; decisions need reproducible matched workloads.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Obtain explicit owner permission for the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-001](DND-001-bounded-stream-framing.md), [DND-003](DND-003-single-owner-remote-forwarding.md), [DND-004](DND-004-handshake-registration-replay.md), [DND-005](DND-005-caller-scoped-correlation.md), [DND-007](DND-007-bounded-dispatch-and-closure.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-015](DND-015-embedded-target-feasibility.md)

## Outcome

A reproducible measurement protocol and, only in a later authorized run, raw results plus a bounded comparison of eligible transports.

## Scope

Name hardware, toolchain, revisions, transport versions, topology, payload schema/size, fan-out, rates, queue caps, delivery semantics and security mode. Measure latency distributions, throughput/goodput, drops/duplicates/reordering, CPU, allocations/retained memory, bytes and energy where measurable. Include saturation, slow consumers, disconnect and recovery.

## Non-goals

No NATS disadvantage assumed, no comparing pointer handoff to authenticated network transfer as equivalent, no benchmark of nonexistent UDP, no unapproved hardware/network run.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md), sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [endpoint/remote/wrappers.go](../../endpoint/remote/wrappers.go) lines 60–92,168–223 — Remote serialization differs from local pointer delivery.
- [intent.go](../../intent.go) lines 99–117,192–210 — Local pointer path has different costs and semantics.
- [network/net/net.go](../../network/net/net.go) lines 31–82 — Pinned revision has no usable packet receive path.

## Unknowns and decisions

Exact target and workloads depend on DND-015; measurement tool overhead and run-to-run noise need bounds. Packet arm additionally requires DND-011 completion and permitted network access; without that it is excluded, not simulated as working.

## Proposed reproduction or decision fixture — NOT RUN

Prepare a deterministic synthetic payload corpus and fixed offered-load schedule. Interleave comparable candidate runs, warm up consistently, include unloaded and saturated phases, retain raw samples and classify failures. Re-run only under the later authorized measurement assignment.

## Acceptance — unmet until evidence exists

- [ ] Protocol identifies comparable guarantees and separates local, stream, packet and broker arms.
- [ ] Results, when run, include sample counts, repetitions, raw data, uncertainty and failures; unrun arms remain unrun.
- [ ] Report makes only measured claims and records hard resource/deadline gates; packet and target runs have separate eligibility evidence.

## Verification procedure — future, not executed

Now: protocol planning only. Later: authorized isolated benchmark with pinned binaries/configuration and captured raw outputs; do not close this empirical ticket with a written protocol alone.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium study after prerequisites; one target, representative payload classes and bounded load sweep. Additional boards/topologies need separate scope.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
