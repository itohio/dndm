---
title: "Define explicit capability negotiation"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-013"
status: "backlog"
priority: "P1"
milestone: "M3 bridge contract"
kind: "research / missing feature"
dependencies: ["DND-002", "DND-006", "DND-009", "DND-010", "DND-012"]
implementation_evidence: []
---

# DND-013 — Define explicit capability negotiation

## State and permission

Status: **backlog**. Kind: **research / missing feature**.
Milestone: **M3 bridge contract**. Priority: **P1** — Adapters must refuse unavailable guarantees instead of flattening typed data and calls into one misleading interface.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Requires explicit approval before the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-006](DND-006-delivery-and-overload-classes.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md), [DND-010](DND-010-packet-broadcast-decision.md), [DND-012](DND-012-durability-and-command-boundary.md)

## Outcome

A versioned capability/negotiation contract with supported, unsupported and unverified states and a conservative mismatch policy.

## Scope

Describe schema/media type, inline/reference size and access policy; direct/fan-out/broadcast/worker/RPC/duplex semantics; delivery class, ordering, cancellation, identity, ownership and durability. Include profile/zone/activation and stream/frame/clock provenance without requiring an LLM.

## Non-goals

No claim that advertised capabilities are trusted proof, no mandatory model/media backend or full generic transport framework.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [route.go](../../route.go) lines 47–68,100–158 — Typed descriptors and route identity exist.
- [x/bus/caller.go](../../x/bus/caller.go) lines 173–224 — Reply streaming alone does not satisfy duplex capabilities.
- [peer.go](../../peer.go) lines 76–77 — Raw prefix matching is not profile permission enforcement.

## Unknowns and decisions

Choose version compatibility, maximum descriptor size, registration authority, expiry and reconnect downgrade behavior. Packet capabilities depend on actual implementation evidence, not DND-010 approval alone.

## Proposed reproduction or decision fixture — NOT RUN

Create descriptors for pinned dndm TCP/local paths, a future packet candidate and NATS core/durable lanes. Request unsupported queue work, authenticated broadcast, resume or durable media; downgrade a peer during reconnect.

## Acceptance — unmet until evidence exists

- [ ] Negotiation rejects unsupported required guarantees and never silently upgrades unverified capabilities.
- [ ] Descriptors distinguish payload data from control, ephemeral from durable, and advertisement from authorization.
- [ ] Version mismatch, stale identity and reconnect are explicit; non-LLM peers can participate.

## Verification procedure — future, not executed

Static descriptor/transition examples close research after review; DND-014 must enforce selected negotiation and include negative fixtures. Future packet support is advertised only with implementation evidence.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Small–medium research: a minimal adapter contract, not a second orchestration system.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and its 2026-11-08 amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
