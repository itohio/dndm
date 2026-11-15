---
title: "dndm personal-profile planning handoff"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
---

# dndm planning handoff

Local planning and working-tree implementation, 2026-11-08. Requirements: R1-04 (2026-11-08): update dndm so it can be improved on. DND-R2 (2026-11-08): dndm supports missing features and hardens existing ones; this authorises source and test changes in the working tree. Commits, pushes, broker/network connectivity, credentials and hardware still need explicit owner approval.

## Read first

1. [Baseline 2026-11-08](baseline-2026-11-08.md): first runtime evidence at the pin. Build and vet clean, non-race tests pass, **`go test -race` fails** in the root and `x/bus` packages (55 race reports, 23 unique). The DND-001 decoder defects are reproduced.
2. DND-R2: DND-018 and DND-001 implemented failing-test-first (uncommitted). `go test -race -count=3` passes everywhere except `x/bus`.
3. [Pinned findings](research/pinned-findings.md): inherited static source facts, with dated runtime addenda.
4. [Knowns and unknowns](research/known-unknowns.md): decisions and evidence gaps.
5. [Roadmap](roadmap.md): ordering, gates, feature gap map and document checks.
6. [Ticket index](tickets/index.md): 18 work units and per-ticket gates.

Source: the 2026-11-08 canonical static report (two independent static reviews) plus the baseline above. Pin: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec` (itohio/dndm, equal to `origin/main` on 2026-11-08). Earlier planning authority: section 5 of the 2026-11-08 owner amendment. No throughput, latency, loss, allocation or hardware footprint has been measured.

## Boundary

NATS stays the first orchestration transport; dndm remains an experimental typed data-plane candidate. Current local protobuf-pointer fan-out is not UDP broadcast. Remote endpoints serialize and decode. A route hash, advertised URI, lexical path prefix or profile name is not authentication or isolation. A future adapter must refuse guarantees that are absent or unverified.

Per-ticket gates still apply. Implementation tickets start only when their dependencies are done. Decision tickets (DND-002, -006, -009, -010, -012, -013, -015) close only on an owner decision; a recommendation is not a decision. Blocked tickets stay blocked until their named decision exists. Never mark a fixture passed without its run output.

## Immediate handoff

**21 tickets: 8 in review, 10 backlog, 3 blocked.** In review (uncommitted): [DND-018](tickets/DND-018-race-clean-core-lifecycle.md), [DND-001](tickets/DND-001-bounded-stream-framing.md), [DND-019](tickets/DND-019-remote-endpoint-close-self-deadlock.md), [DND-004](tickets/DND-004-handshake-registration-replay.md), [DND-005](tickets/DND-005-caller-scoped-correlation.md) (plus pluggable nonce sources), [DND-002](tickets/DND-002-local-buffer-ownership-contract.md) (contract), [DND-003](tickets/DND-003-single-owner-remote-forwarding.md) and [DND-008](tickets/DND-008-stream-and-peer-lifecycle-races.md) (close/reconnect open). Next: [DND-020](tickets/DND-020-remote-linker-route-direction.md) (two-way routes on one link) and [DND-021](tickets/DND-021-container-publish-test-flake.md) (a flaky test). The owner's decisions on [DND-006](tickets/DND-006-delivery-and-overload-classes.md), [DND-009](tickets/DND-009-identity-auth-and-profile-boundary.md) and [DND-010](tickets/DND-010-packet-broadcast-decision.md) are recorded; designs wait for further research. The `-race` suite is green. Latest rounds: DND-R5-fix (findings F1–F3 and the nonce fix from verification, not committed yet), and before it DND-R5; the [feature gap map](roadmap.md#feature-gap-map) orders the missing features.

Diagrams: [diagrams/](diagrams/README.md) has 30 Mermaid diagrams covering the core idea, link flows, message paths and robot scenarios, with a glossary, a legend and a reading order. Each is marked as-is or target (not built yet).

Designs (docs only, no code yet): [link-context wire in protobuf](design/link-context-wire.md) and [DND-008 close/reconnect](design/close-reconnect.md).

These documents link source files relative to the repository. Line ranges belong to the pinned revision; later checkout edits must not silently reinterpret old evidence. The canonical report and amendment are not part of this repository. This directory is not approved for external posting.
