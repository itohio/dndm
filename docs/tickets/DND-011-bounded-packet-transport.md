---
title: "Implement the selected bounded packet path"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-011"
status: "blocked"
owner: "unassigned"
priority: "P2"
milestone: "M2 packet implementation"
kind: "implementation / missing feature"
dependencies: ["DND-001", "DND-002", "DND-008", "DND-010"]
implementation_evidence: []
---

# DND-011 — Implement the selected bounded packet path

## State and permission

Status: **blocked**. Owner: **unassigned**. Kind: **implementation / missing feature**.
Milestone: **M2 packet implementation**. Priority: **P2** — Optional engineering follows a positive packet decision; not prerequisite to a safe TCP-only handoff.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Obtain explicit owner permission for the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-001](DND-001-bounded-stream-framing.md), [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-010](DND-010-packet-broadcast-decision.md)

## Owner decision (2026-11-08, verbatim from the roster)

"DND-010: **Build it** (packet broadcast)."

This removes the go/no-go block. DND-011 stays **blocked** on its other dependencies and on the design: DND-001 (in review), DND-002 (decided), DND-008 (open), DND-010's design (waits for ideation's research), plus the delivery (DND-006) and auth (DND-009) designs. No code until then.

## Outcome

If DND-010 approves it, a real packet endpoint honoring that decision rather than reusing the stream reader.

## Scope

Implement selected packet receive/send APIs, demultiplexing, bounded packet codec and only the chosen broadcast/multicast mode. Enforce identity, limits, expiry, duplicate/loss policy and teardown. Align example labels/configuration with actual support.

## Non-goals

No general reliable multicast, all-platform compatibility, hidden TCP fallback or hardware performance promise.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md), sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [network/net/net.go](../../network/net/net.go) lines 31–82 — No usable UDP listener/datagram path found.
- [codec/codec.go](../../codec/codec.go) lines 85–132 — Stream framing cannot substitute for packet framing.
- [examples/embedded/embedded/main.go](../../examples/embedded/embedded/main.go) lines 207–245 — Example currently constructs TCP peers under UDP naming.

## Unknowns and decisions

Blocked on a positive DND-010 decision and exact packet/security contract; implementation size unknown until selected. A no-go cancels this optional ticket explicitly, never marks it implemented.

## Proposed reproduction or decision fixture — NOT RUN

After authorization, begin with an in-memory packet interface delivering whole/truncated/reordered/duplicate/lost datagrams. Exercise fragment caps, assembly expiration and fake source identities; only later use an explicitly permitted isolated network.

## Acceptance — unmet until evidence exists

- [ ] Each receive preserves datagram boundaries and applies DND-010 policy without stream-style assumptions.
- [ ] Resource use stays within chosen sender/assembly/fragment caps; malformed/flooded packets cannot cause unbounded retained state.
- [ ] Only actually supported modes are advertised; broadcast network behavior requires separate target evidence before being labelled verified.

## Verification procedure — future, not executed

Run deterministic packet fixtures first after permission; separate simulation evidence from an authorized real-interface capture. No packet test or radio activity occurs now.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Unknown implementation size; estimate only after DND-010 selects fragmentation, security and topology. Keep first slice to one mode and class.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
