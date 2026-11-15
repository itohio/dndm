---
title: "dndm ticket index"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
---

# dndm ticket index

23 stable tickets: **8 done, 2 in progress, 9 backlog, 4 blocked, 0 in review**. Done means verified, reviewed and committed locally; each done ticket's log names its commits. DND-008 is in progress only for R1, F9 and the session ids that DND-022 carries. Authority: requirement R1-04 (update dndm so further improvement can start) and, for DND-R2–R8, the 2026-11-08 authorisation to add missing features and harden existing ones; together they cover working-tree source and test changes; commits only on approval, never pushes. The 2026-11-08 decisions are recorded in DND-002, 005, 006, 009, 010 and 011; on 2026-11-09 downloading buf and protoc-gen-go was approved (DND-R9-tools). The DND-R2/R3 behaviour changes and the four extra DND-R4 fixes were also accepted, and an F21 ticket (DND-020) was approved. Per-ticket gates below still apply. IDs are never reused. Runtime evidence: [baseline](../baseline-2026-11-08.md); later rounds record their evidence in each ticket's tests and dated log.

- `done`: verified, reviewed and committed locally (commit hashes in the ticket's log).
- `in review`: implemented in the working tree with failing-first evidence; awaiting verification and approval; not committed.
- `in progress`: part of the acceptance is implemented and in review; the rest is not started.
- `ready`: checkable acceptance, reproduction plan and files to touch; no unmet dependency.
- `backlog`: scoped future work; dependencies and the ticket's own gate still apply.
- `blocked`: an explicit security/protocol or design decision prevents reliable implementation sizing/start.
- P0: remote correctness or isolation prerequisite. P1: resolve decision/feasibility. P2: optional integration or measurement after gates.
- Research decision-ready means the question, alternatives and closure artifact are bounded. It does not mean the decision is answered. Decision tickets close only on an explicit decision. DND-002, DND-006, DND-009 and DND-010 carry a decision memo (options, recommendation) since DND-R2.

| Ticket | Kind | Status | Priority | Milestone | Hard dependencies |
|---|---|---|---|---|---|
| [DND-001](DND-001-bounded-stream-framing.md) — Bound stream framing and return pooled buffers | implementation / fixable defect, runtime-confirmed | **done** (5e2532a) | P0 | M1 remote correctness | None |
| [DND-002](DND-002-local-buffer-ownership-contract.md) — Choose local shared-message ownership | research / fundamental tradeoff (decided: immutable after Send) | **done** (3907a77) | P0 | M0 decisions | None |
| [DND-003](DND-003-single-owner-remote-forwarding.md) — Remove competing remote-interest readers | implementation / fixable defect, runtime-confirmed | **done** (464a02d) | P0 | M1 remote correctness | [DND-002](DND-002-local-buffer-ownership-contract.md) |
| [DND-004](DND-004-handshake-registration-replay.md) — Register routes across handshake completion | implementation / fixable defect, runtime-confirmed | **done** (c93ba0c, 9d6515f) | P0 | M1 remote correctness | None |
| [DND-005](DND-005-caller-scoped-correlation.md) — Make reply correlation caller-scoped | implementation / fixable defect, runtime-confirmed | **done** (12b4916) | P0 | M1 remote correctness | None |
| [DND-006](DND-006-delivery-and-overload-classes.md) — Define delivery, overload and close classes | research / fundamental tradeoff | backlog | P0 | M0 decisions | None |
| [DND-007](DND-007-bounded-dispatch-and-closure.md) — Enforce bounded dispatch and explicit closure | implementation / fixable static defect | backlog | P0 | M1 remote correctness | [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-005](DND-005-caller-scoped-correlation.md), [DND-006](DND-006-delivery-and-overload-classes.md) |
| [DND-008](DND-008-stream-and-peer-lifecycle-races.md) — Serialize stream and peer lifecycle state | implementation / fixable defect, runtime-confirmed | **in progress** (acceptance met and committed; open: R1 alias redial, F9 doc, v2 session ids via DND-022) | P0 | M1 remote correctness | [DND-001](DND-001-bounded-stream-framing.md), [DND-004](DND-004-handshake-registration-replay.md), [DND-007](DND-007-bounded-dispatch-and-closure.md) |
| [DND-009](DND-009-identity-auth-and-profile-boundary.md) — Decide authenticated identity and profile isolation | research / missing feature | backlog | P0 | M0 decisions | None |
| [DND-010](DND-010-packet-broadcast-decision.md) — Decide whether to build packet broadcast | research / missing feature and tradeoff | backlog | P1 | M2 packet decision | [DND-006](DND-006-delivery-and-overload-classes.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md) |
| [DND-011](DND-011-bounded-packet-transport.md) — Implement the selected bounded packet path | implementation / missing feature | blocked | P2 | M2 packet implementation | [DND-001](DND-001-bounded-stream-framing.md), [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-010](DND-010-packet-broadcast-decision.md) |
| [DND-012](DND-012-durability-and-command-boundary.md) — Choose durability and command ownership | research / missing feature and tradeoff | backlog | P1 | M0 decisions | [DND-006](DND-006-delivery-and-overload-classes.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md) |
| [DND-013](DND-013-capability-negotiation-contract.md) — Define explicit capability negotiation | research / missing feature | backlog | P1 | M3 bridge contract | [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-006](DND-006-delivery-and-overload-classes.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md), [DND-010](DND-010-packet-broadcast-decision.md), [DND-012](DND-012-durability-and-command-boundary.md) |
| [DND-014](DND-014-scoped-nats-bridge.md) — Build a narrow profile-scoped NATS bridge | implementation / missing feature | blocked | P2 | M3 bridge implementation | [DND-003](DND-003-single-owner-remote-forwarding.md), [DND-004](DND-004-handshake-registration-replay.md), [DND-005](DND-005-caller-scoped-correlation.md), [DND-007](DND-007-bounded-dispatch-and-closure.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-012](DND-012-durability-and-command-boundary.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-017](DND-017-enforce-profile-gateway-boundary.md) |
| [DND-015](DND-015-embedded-target-feasibility.md) — Decide embedded feasibility on a named target | research / feasibility uncertainty | backlog | P1 | M4 feasibility | [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-006](DND-006-delivery-and-overload-classes.md), [DND-010](DND-010-packet-broadcast-decision.md), [DND-013](DND-013-capability-negotiation-contract.md) |
| [DND-016](DND-016-reproducible-transport-measurement.md) — Measure only implemented comparable transports | research / empirical measurement | backlog | P2 | M4 measurement | [DND-001](DND-001-bounded-stream-framing.md), [DND-003](DND-003-single-owner-remote-forwarding.md), [DND-004](DND-004-handshake-registration-replay.md), [DND-005](DND-005-caller-scoped-correlation.md), [DND-007](DND-007-bounded-dispatch-and-closure.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-015](DND-015-embedded-target-feasibility.md) |
| [DND-017](DND-017-enforce-profile-gateway-boundary.md) — Enforce the selected identity/isolation boundary | implementation / missing feature | blocked | P0 | M1 isolation enforcement | [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md) |
| [DND-018](DND-018-race-clean-core-lifecycle.md) — Race-clean core lifecycle | implementation / fixable defect, runtime-confirmed | **done** (1f85174, 7d088f7) | P0 | M1 remote correctness | None |
| [DND-019](DND-019-remote-endpoint-close-self-deadlock.md) — Remote endpoint Close self-deadlock | implementation / fixable defect, runtime-confirmed | **done** (214f490) | P0 | M1 remote correctness | None |
| [DND-020](DND-020-remote-linker-route-direction.md) — Key the remote linker by route and direction | implementation / fixable defect, runtime-observed | backlog | P1 | M1 remote correctness | [DND-003](DND-003-single-owner-remote-forwarding.md), [DND-006](DND-006-delivery-and-overload-classes.md) |
| [DND-021](DND-021-container-publish-test-flake.md) — Make TestContainer_Publish wait for events instead of sleeping | implementation / flaky test, runtime-confirmed | **done** (67e47bd) | P2 | M1 remote correctness | None |
| [DND-022](DND-022-link-context-core.md) — Implement the link-context wire core | implementation / missing feature | **in progress** (schema + DATA codec committed in DND-R9; link core 9e34601, 63df1c4) | P1 | M1 remote correctness | [DND-006](DND-006-delivery-and-overload-classes.md), [DND-008](DND-008-stream-and-peer-lifecycle-races.md) |
| [DND-023](DND-023-can-face.md) — CAN face for the link-context core | implementation / missing feature | blocked | P2 | M4 feasibility | [DND-022](DND-022-link-context-core.md), [DND-015](DND-015-embedded-target-feasibility.md) |

## Starting order

Done (verified, reviewed, committed): **DND-018**, **DND-001** (DND-R2); **DND-019**, **DND-004** (DND-R3); **DND-005**, **DND-002** (contract), **DND-003** (DND-R4, R5); **DND-021** (DND-R8). In progress: **DND-008** (its acceptance is committed; open: R1, F9, and the session ids that DND-022 carries) and **DND-022** (schema and DATA codec committed in DND-R9; the link core committed as 9e34601 and 63df1c4, under review and verification). RC6 (DND-007) was fixed in DND-R5; the rest of DND-007 waits for DND-006. Waiting for designs: **DND-006**, **DND-009**, **DND-010** (decisions recorded), then DND-011/012/013. Next: the rest of **DND-022** (Bind rate limit and retries, `max_frame`, v1 negotiation, wiring into the remote endpoint; stages beyond `latest` after DND-006), DND-008 R1, and **DND-020** (F21) after the DND-006 delivery design. Finish dependencies before integration; backlog status alone does not make a ticket runnable.

DND-011 now has a positive packet decision (build it) but still waits for its dependencies and the design; DND-014 needs approved contracts and verified enforcement; DND-017 needs a selected threat model and enforcement primitive. DND-016 has a conditional packet arm requiring DND-011.

## Baseline gate

At the pin `go test -race ./...` was red ([baseline](../baseline-2026-11-08.md#race-reports)). DND-018 fixed RC1–RC4 (DND-R2), and DND-019 fixed RC5 (DND-R3). DND-R4 made the whole suite pass. DND-R5 fixed RC6 and the DND-008 races. Its gates:

- `go test -race -count=3 ./...`: all 8 packages ok, 0 race reports.
- `go test -count=10 ./...`: ok.
- x/bus ×20 without and with `-race`: ok.
- One later read-only rerun failed `TestContainer_Publish` once (0.03 s). That was the pin-era flake now fixed as DND-021 (DND-R8: 19 failures in 32,000 parallel runs before, 0 after).

Every ticket verifies with the full `-race` suite: no failing package and no race report.

[Overview](../index.md) · [Roadmap](../roadmap.md) · [Pinned findings](../research/pinned-findings.md)
