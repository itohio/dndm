---
title: "Build a narrow profile-scoped NATS bridge"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-014"
status: "blocked"
priority: "P2"
milestone: "M3 bridge implementation"
kind: "implementation / missing feature"
dependencies: ["DND-003", "DND-004", "DND-005", "DND-007", "DND-008", "DND-012", "DND-013", "DND-017"]
implementation_evidence: []
---

# DND-014 — Build a narrow profile-scoped NATS bridge

## State and permission

Status: **blocked**. Kind: **implementation / missing feature**.
Milestone: **M3 bridge implementation**. Priority: **P2** — No connector exists in the source report; only a scoped bridge with proven remote correctness and isolation is eligible.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Requires explicit approval before the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-003](DND-003-single-owner-remote-forwarding.md), [DND-004](DND-004-handshake-registration-replay.md), [DND-005](DND-005-caller-scoped-correlation.md), [DND-007](DND-007-bounded-dispatch-and-closure.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-012](DND-012-durability-and-command-boundary.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-017](DND-017-enforce-profile-gateway-boundary.md)

## Outcome

An optional typed dndm↔NATS adapter enforcing the selected capability, identity and delivery boundaries.

## Scope

One allowlisted typed ephemeral data path plus explicit refusal/routing of durable commands to their authoritative application lane. Bind profile/zone/call identities; validate bounded schemas/references, prevent echo loops, propagate cancel/close and reject unsupported guarantees. Use synthetic personal data and least-privilege test credentials only after permission.

## Non-goals

No raw dndm exposure to hostile tenants, wildcard cross-profile forwarding, new command authority, full worker scheduler or production deployment.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [route.go](../../route.go) lines 47–68,100–158 — Typed route descriptors are available to adapt.
- [peer.go](../../peer.go) lines 76–77 — Prefix selection cannot enforce bridge isolation.
- [endpoint/mesh/handshake.go](../../endpoint/mesh/handshake.go) lines 203–217 — Advertised identity requires trusted binding.

## Unknowns and decisions

Canonical report comparison records connector as planned, not implemented. Blocked on approved DND-012/013 contracts and verified DND-017 boundary; NATS library/version and test topology remain to select.

## Proposed reproduction or decision fixture — NOT RUN

After authorization, route one synthetic typed payload between two permitted peers. Attempt another profile, forged source/reply, oversized payload/reference, unknown capability, bridge echo, disconnect and replay; request durable delivery through the ephemeral lane.

## Acceptance — unmet until evidence exists

- [ ] Allowed messages preserve type and scoped identity; forbidden messages never cross the gateway.
- [ ] No echo loops or silent capability upgrades; cancellation/closure/overload match selected class.
- [ ] Durable requests are explicitly refused or delegated with correct persistence acknowledgement semantics; transport receipt is never reported as execution.

## Verification procedure — future, not executed

Future integration fixtures require an authorized isolated local broker and recorded ACL/negative-test evidence. No broker launch, connection or credential action is authorized now.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Unknown until security/contract decisions; bound initial implementation to one typed route and one ephemeral class.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and its 2026-11-08 amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
