---
title: "Decide embedded feasibility on a named target"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-015"
status: "backlog"
priority: "P1"
milestone: "M4 feasibility"
kind: "research / feasibility uncertainty"
dependencies: ["DND-002", "DND-006", "DND-010", "DND-013"]
implementation_evidence: []
---

# DND-015 — Decide embedded feasibility on a named target

## State and permission

Status: **backlog**. Kind: **research / feasibility uncertainty**.
Milestone: **M4 feasibility**. Priority: **P1** — Ordinary Go example builds and UDP labels do not prove TinyGo, RP2040 or another device works.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Requires explicit approval before the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-006](DND-006-delivery-and-overload-classes.md), [DND-010](DND-010-packet-broadcast-decision.md), [DND-013](DND-013-capability-negotiation-contract.md)

## Outcome

A decision-ready target/toolchain/workload feasibility record with explicit go/no-go gates for later device testing.

## Scope

Select an actual, explicitly approved target, OS/runtime/toolchain, dependencies and interface. Budget RAM/flash/stack/CPU, schema/reflection/protobuf costs, buffering/reassembly, bandwidth, power and deadlines. Compare local execution versus a gateway; preserve offline and safety boundaries.

## Non-goals

No board assumption, TinyGo compatibility claim, installation, flash, model execution or actuator operation.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [examples/embedded/embedded/main.go](../../examples/embedded/embedded/main.go) lines 207–245 — UDP setup name constructs TCP peers.
- [examples/embedded/host/main.go](../../examples/embedded/host/main.go) lines 278–281 — Host path also uses TCP.
- [go.mod](../../go.mod) (file-level report reference) — Report pins Go 1.24.1; not proof of device runtime compatibility.
- [examples/embedded/Makefile](../../examples/embedded/Makefile) (file-level report reference) — Report records ordinary Go builds, not TinyGo proof.

## Unknowns and decisions

Exact board/ISA/RAM/flash/toolchain and real payload rates are unselected. Source-only compatibility estimates are hypotheses, not target measurements.

## Proposed reproduction or decision fixture — NOT RUN

For each candidate, inventory runtime/OS APIs and dependency compatibility from pinned sources. Build a worksheet for fixed buffers, worst-case concurrent messages, schema/runtime state and transport overhead; mark unknown coefficients rather than zeroing them.

## Acceptance — unmet until evidence exists

- [ ] One named target/workload is selected or a documented no-go is returned.
- [ ] Resource ceilings and margin are explicit; unknowns have a measurement gate.
- [ ] Gateway fallback is visible; native robot safety loops remain outside orchestration.

## Verification procedure — future, not executed

Research closes on cited compatibility analysis and a target measurement plan, not a build success claim. Any later build/board/power test needs explicit permission and DND-016 evidence.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium research for one target and transport; target implementation remains unknown and is not authorized by a positive paper estimate.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and its 2026-11-08 amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-09: R-DND-R5 review follow-ups filed here:
  - F5: the frame-size cap is fixed at 4 MiB for every stream. Add a per-stream option passed to `codec.ReadMessageLimit`, and make the encoder refuse oversize bodies.
  - F10: the TinyGo counter nonce source panics if `crypto/rand` fails.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
