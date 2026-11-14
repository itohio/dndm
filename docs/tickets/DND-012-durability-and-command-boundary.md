---
title: "Choose durability and command ownership"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-012"
status: "backlog"
priority: "P1"
milestone: "M0 decisions"
kind: "research / missing feature and tradeoff"
dependencies: ["DND-006", "DND-009"]
implementation_evidence: []
---

# DND-012 — Choose durability and command ownership

## State and permission

Status: **backlog**. Kind: **research / missing feature and tradeoff**.
Milestone: **M0 decisions**. Priority: **P1** — dndm has no durable command/consumer store; transport retries cannot establish exactly-once external effects.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Requires explicit approval before the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-006](DND-006-delivery-and-overload-classes.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md)

## Outcome

A decision-ready boundary keeping durable task/pulse commands on NATS/application storage unless a separately justified dndm persistence design is approved.

## Scope

Define admission versus persistence versus completion; replay, execution idempotency, crash windows, retention and recovery authority. Preserve Markdown domain authority; optional transport journals/indexes are not hidden canonical state. Define unsupported queue groups, affinity and duplex lifecycle explicitly.

## Non-goals

No new durable store, silent JetStream adoption inside dndm, exactly-once execution claim or replay of perishable frames.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [intent.go](../../intent.go) lines 186–220 — Fan-out is not a durable worker queue.
- [interest.go](../../interest.go) lines 279–292 — Subscriber fan-out can drop.
- [x/bus/caller.go](../../x/bus/caller.go) lines 173–224 — Multi-reply helper is not complete duplex state.

## Unknowns and decisions

Canonical report section 2 records absent durable stores; no single source line proves repository-wide absence. Choose authoritative command IDs/checkpoints and intended persistence layer before any future durable implementation.

## Proposed reproduction or decision fixture — NOT RUN

Walk crash points before/after admission, journal persistence, effect execution and acknowledgement. Repeat duplicate replay, expired media, lost acknowledgement, reconnect and worker departure; identify the authority that prevents repeated effects.

## Acceptance — unmet until evidence exists

- [ ] Every class documents supported or refused durability and who owns deduplication.
- [ ] Rebuilding delivery/index state cannot independently repeat a command or external effect.
- [ ] Any proposed durable dndm extension has a separate decision and future ticket; absence is not disguised as reliable Send.

## Verification procedure — future, not executed

Close research only with reviewed crash/replay tables and selected ownership; future persistence acceptance needs restart fixtures and recorded side-effect counts, not a transport ack alone.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Small–medium research: retain existing orchestration boundary by default; persistence implementation intentionally unsized and unapproved.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and its 2026-11-08 amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
