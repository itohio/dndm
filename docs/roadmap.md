---
title: "Prioritized dndm roadmap"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
---

# Prioritized roadmap

Planning authority: [owner amendment §5](../../docs/2026-11-08-budget-runner-sandbox-storage-amendment.md). Evidence: [canonical static report](../../docs/notes/2026-11-08-dndm-babyai-reuse.md). Pin: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`.

Current authority: DND-R2, the owner's instruction "dndm should be able to support lacking features and existing features should be hardened" (2026-11-08), which authorises source and test changes in the working tree; it follows R1-04's "properly update dndm, easyrobot projects so i can start improving on them". Work starts per ticket once its gates and dependencies are met. Commits, pushes, broker startup, dependency installation, credentials, board tests and public posting still need explicit owner approval. Approving a design does not approve executing it. Runtime evidence: [baseline 2026-11-08](baseline-2026-11-08.md), [DND-R2 evidence](evidence/dnd-r2/README.md).

## Milestones and exit gates

| Milestone | Tickets | Exit gate, not current achievement |
|---|---|---|
| M0 decisions | DND-002, DND-006, DND-009, DND-012 | Ownership, message classes, trust model and durability authority are explicit; no implicit secure/reliable claims |
| M1 remote correctness | DND-018, DND-001, DND-019, DND-003, DND-004, DND-005, DND-007, DND-008 | Race-clean core lifecycle, bounded decoder, endpoint Close that returns, plus deterministic registration/correlation/dispatch/lifecycle evidence on implemented paths |
| M1 isolation enforcement | DND-017 | Actual selected boundary passes allow/deny/revocation fixtures; design alone is insufficient |
| M2 packet decision | DND-010 | Go/no-go plus bounded network/resource/security design |
| M2 packet implementation | DND-011 | Only if go: actual packet path and fixtures; real broadcast remains unverified until an authorized network test |
| M3 bridge contract | DND-013 | Minimal versioned capability negotiation with unsupported/unverified guarantees refused |
| M3 bridge implementation | DND-014 | Narrow scoped adapter passes identity/capability/overload/loop tests using permitted isolated broker setup |
| M4 feasibility | DND-015 | Named target/workload, explicit unknowns, resource ceilings and target-test gate |
| M4 measurement | DND-016 | Authorized raw measurements and comparable claims, not just a proposed benchmark protocol |

Milestones are gates, not calendar promises. M2 packet work is optional and is not a prerequisite to a safe TCP-only bridge. No worker scheduler, full duplex runtime or durable dndm store is silently added: contracts must either refuse those guarantees or obtain separately scoped later implementation work.

## Dependency DAG and first runnable work

Hard dependencies are authoritative in ticket frontmatter and mirrored in the [ticket index](tickets/index.md). Edges mean “must complete before this ticket starts”; permission is an independent gate.

1. Dependency-free: **DND-018, DND-001, DND-019, DND-002, DND-004, DND-005, DND-006, DND-009**.
2. Once those finish: **DND-003, DND-007, DND-010, DND-012** (each still needs only its declared parents).
3. Then **DND-008, DND-013**.
4. Then **DND-011, DND-015, DND-017**; DND-011 remains conditional on a packet go decision.
5. Then **DND-014, DND-016**.

A valid topological order is DND-018, DND-001, DND-019, DND-002, DND-004, DND-005, DND-006, DND-009, DND-003, DND-007, DND-010, DND-012, DND-008, DND-013, DND-011, DND-015, DND-017, DND-014, DND-016. Numeric ID order is not execution order.

**Implemented, in review:** DND-018 and DND-001 (DND-R2); DND-019 and DND-004 (DND-R3); DND-005, DND-002 (contract) and DND-003 (DND-R4); DND-005 nonce sources, DND-008 (close/reconnect still open) and RC6 from DND-007 (DND-R5). **Next:** DND-020 (F21, after the DND-006 delivery design) and the DND-008 close/reconnect item. **Designs pending:** DND-006, DND-009 and DND-010 have the owner's decisions (packet broadcast: build it) and wait for ideation's research before any code. The whole `go test -race` suite passes; see the [baseline gate](tickets/index.md#baseline-gate).

DND-016's packet comparison has an additional conditional gate: DND-011 completion plus explicit network/hardware permission. Exclude it when absent/no-go; do not benchmark a nonexistent packet path or block a separately useful TCP/local/broker study. Target measurements depend on the DND-015 named-target decision and a separate permission boundary.

## Feature gap map

Added 2026-11-08 (DND-R2). Source lines refer to the pin and were read with `git show c7b5b61:<file>`; nothing was executed. Recommendations are in each ticket's decision memo. None is a decision.

| Feature | What exists at the pin (file:line) | Ticket(s) | Decision needed (owner) | Order | Recommended path |
|---|---|---|---|---|---|
| Local buffer ownership (prerequisite) | Every subscriber gets the same pointer (`intent.go:112–118`, `interest.go:283–289`); no contract in the API comments (`intent.go:36–37`) | [DND-002](tickets/DND-002-local-buffer-ownership-contract.md) → [DND-003](tickets/DND-003-single-owner-remote-forwarding.md), [DND-007](tickets/DND-007-bounded-dispatch-and-closure.md) | Immutable, clone or leases | 1 | Immutable after `Send`, documented; per-subscription copy as an opt-in |
| Delivery/overload classes | Producer blocks (`intent.go:112–118`); subscriber drops silently (`interest.go:283–291`); one global queue size, default 1 (`options.go:25`); per-route options empty (`options.go:96–104`) | [DND-006](tickets/DND-006-delivery-and-overload-classes.md) → [DND-007](tickets/DND-007-bounded-dispatch-and-closure.md) | Class set and default class | 1 | Two in-session classes: `latest` (default, counted drops) and `reliable` (opt-in, deadline); durable and worker semantics refused |
| Identity/auth | Self-declared handshake identity (`endpoint/mesh/handshake.go:203–217`); lexical prefix (`peer.go:76–77`); unkeyed route hash (`route.go:113–116`); unused `public_key`/`signature` fields (`proto/types/p2p/peering.proto:22`, `proto/types/core/header.proto:29`) | [DND-009](tickets/DND-009-identity-auth-and-profile-boundary.md) → [DND-017](tickets/DND-017-enforce-profile-gateway-boundary.md) | Trust boundary and key authority | 1 | Trusted-local inside a zone, NATS-terminating gateway across zones; exact-segment match and default-deny allowlist |
| Packet broadcast | No UDP listener (`network/net/net.go:48`); stream-only framing (`codec/codec.go:85–110`); the `udp` example builds TCP (`examples/embedded/embedded/main.go:207–215`) | [DND-010](tickets/DND-010-packet-broadcast-decision.md) → [DND-011](tickets/DND-011-bounded-packet-transport.md) | Go/no-go | 2 | No-go now, with a measured-need revisit trigger; fix the `udp` labels and docs |
| Durability | Only in-memory channels (`intent.go:99–121`) and reply waiters (`x/bus/caller.go:269–285`); no store or NATS module (`go.mod:5–11`) | [DND-012](tickets/DND-012-durability-and-command-boundary.md) | Where commands become durable, and who owns idempotency | 2 | Keep durable commands on NATS/application storage; dndm refuses a durable class |
| Capability negotiation | Typed routes (`route.go:46–53`); registration carries only route/hops/ttl/register (`proto/types/core/intents.proto:11–16`); handshake has no version or capability field (`proto/types/p2p/peering.proto:16–23`) | [DND-013](tickets/DND-013-capability-negotiation-contract.md) | Descriptor version, mismatch and downgrade policy | 3 | Minimal versioned descriptor with supported/unsupported/unverified states; refuse on mismatch |
| Embedded | The `udp` example path builds TCP (`examples/embedded/embedded/main.go:35–43,207–245`); serial transport exists (`network/serial/serial.go:24–81`) | [DND-015](tickets/DND-015-embedded-target-feasibility.md) | Named target and workload | 4 | Name the target first; serial/TCP only while packets are a no-go; no port before a go |
| NATS bridge | Absent: no NATS module (`go.mod:5–11`) | [DND-014](tickets/DND-014-scoped-nats-bridge.md) (needs [DND-012](tickets/DND-012-durability-and-command-boundary.md), [DND-013](tickets/DND-013-capability-negotiation-contract.md), [DND-017](tickets/DND-017-enforce-profile-gateway-boundary.md) and the M1 fixes) | NATS client library/version and test topology; approves the contracts above | 5 | One allowlisted ephemeral route through the DND-009 gateway; commands forwarded to the application lane |

The order column is the depth, in the frontmatter DAG above, of the ticket that gates each feature. Rows with the same number can run in parallel.

- Order 1: DND-002, DND-006 and DND-009 have no dependencies, and every other row needs at least one of them.
- Order 2: DND-010 and DND-012 each need DND-006 and DND-009.
- Order 3: DND-013 needs DND-010 and DND-012.
- Order 4: DND-015 needs DND-013. The DND-011 and DND-017 implementations are also at depth 4 because they need DND-008, which needs DND-007, which needs DND-002/005/006.
- Order 5: DND-014 needs DND-013 and DND-017, so the bridge comes last.

A packet no-go cancels DND-011 but delays no other row; DND-013 and DND-015 need only the DND-010 decision.

## Unresolved choices and sizing

- Frame/queue caps and malformed/overload policy.
- Immutable publication versus copies/leases; single-owner dispatch.
- Caller-scoped identity versus private reply paths and wire compatibility.
- Trusted-local restrictions versus authenticated remote gateway; key/revocation authority.
- Packet go/no-go, network topology, MTU, fragmentation and loss/security policy.
- Durable command authority, execution idempotency and capability refusal boundaries.
- Descriptor versions, minimal bridge routes and broker/runtime versions.
- Named embedded target, actual workload/resource budgets and comparable measurement design.

DND-011/014/017 implementation size is deliberately unknown until their decisions settle. No optimistic estimates substitute for evidence. Research decisions have bounded artifacts; empirical DND-016 requires actual later measurements to close. All owners remain unassigned.

## Static document check record

2026-11-08 — **STATIC document checks executed**, using an in-memory Python validator over these Markdown files; no repository code was imported or executed.

- 22 Markdown files present: five overview/research/roadmap/index files and 17 ticket files.
- 311 relative-link occurrences resolved to existing files; the linked Markdown heading resolved.
- Ticket/index parity: 17/17; IDs, statuses, priorities, milestones and dependency rows agree.
- 42 hard dependency edges reference existing tickets; depth-first traversal found no cycle.
- 14 backlog, 3 blocked; all owners unassigned; all implementation-evidence arrays empty; no acceptance item checked.
- Required ticket sections present; expected file set matched; **zero document-check failures**.

These checks establish document consistency only. They do not validate source behavior, security, compatibility, packet support, performance or hardware readiness. No builds, tests, formatters, dependency installs, runtime experiments or source re-audit were performed.

2026-11-08 (R1-04, rerun after the verification fixes): **static document checks** with an in-memory Python validator (no repo code imported). 23 Markdown files (22 + the baseline); 369 relative links, all targets and Markdown anchors resolve; ticket frontmatter/index parity 17/17; status counts 1 ready, 13 backlog, 3 blocked; one owner assigned (DND-001 → dndm). The first R1-04 check recorded 366 links because it ran before the last roadmap edit. Build/test runs are recorded in [baseline-2026-11-08.md](baseline-2026-11-08.md), not here.

2026-11-08 (DND-R2): **static document checks** with the same in-memory validator. 25 Markdown files (+ DND-018 ticket, + DND-R2 evidence README); 467 relative links, all targets and anchors resolve; ticket frontmatter/index parity 18/18; 2 in review, 13 backlog, 3 blocked; owners dndm 2, unassigned 16.

2026-11-08 (DND-R3): **static document checks**, same validator. 27 Markdown files (+ DND-019 ticket, + DND-R3 evidence README); 534 relative links, all targets and anchors resolve; ticket frontmatter/index parity 19/19; 4 in review, 11 backlog, 4 blocked; owners dndm 5, unassigned 14.

2026-11-08 (DND-R4): **static document checks**, same validator, excluding `research/ideation/` (ideation's file). 28 Markdown files (+ DND-R4 evidence README); 575 relative links, all targets and anchors resolve; ticket frontmatter/index parity 19/19; 7 in review, 9 backlog, 3 blocked; owners dndm 7, unassigned 12.

2026-11-08 (DND-R5): **static document checks**, same validator, excluding `research/ideation/`. 30 Markdown files (+ DND-020 ticket, + DND-R5 evidence README); 646 relative links, all targets and anchors resolve (3 links to the moved roster files repointed to `docs/`); ticket frontmatter/index parity 20/20; 8 in review, 9 backlog, 3 blocked; owners dndm 9, unassigned 11.

## Dated planning log

- 2026-11-08 — Read the authoritative amendment and canonical report in full. Created 22 local Markdown files: five entry/research/roadmap/index documents and 17 separate tickets. 14 backlog, 3 blocked, no implementation evidence. Source report and code untouched; no source re-audit, builds, tests, installs, models, credentials, network services or hardware runs.
- 2026-11-08: R1-04. Baseline build/vet/test/race run recorded in [baseline-2026-11-08.md](baseline-2026-11-08.md); DND-001 moved to ready; authority framing updated to the owner's start-improving instruction; runtime addendum appended to pinned findings. No source edits.
- 2026-11-08: DND-R2. New DND-018 implemented (RC1–RC4) and DND-001 implemented, both failing-test-first, both in review and uncommitted ([evidence](evidence/dnd-r2/README.md)). Decision memos added to DND-002/006/009/010 and a feature gap map to this file. Remaining `-race` failures are in `x/bus` only: RC5 (DND-008), RC6 (DND-007), and flaky remote-stream timeouts (F3, DND-003).
- 2026-11-08: DND-R3. DND-R2 verification fixes applied (error-identity note, ProducerConsumer flake under DND-003). New DND-019 (remote Close self-deadlock) and DND-004 implemented failing-test-first; RC5 and `Addrbook.AddConn` fixed along the way (DND-008 items). DND-005 reproduced, decision memo added, blocked on the owner. Remaining `-race` failures are in `x/bus` only: RC6 (DND-007) and flaky remote-stream timeouts ([evidence](evidence/dnd-r3/README.md)).
- 2026-11-08: DND-R4. DND-R3 verification fixes applied (bimodal cross-caller correction, four behaviour rows). Owner decisions copied verbatim into DND-002/005/006/009/010/011. DND-005 implemented failing-test-first; DND-002 contract written (godoc and ticket) with an aliasing test; DND-003 implemented failing-test-first. Follow-ons: readiness notification for late wrappers, wrapper use-after-close, synchronous `Link.Link`, Router error checks. Full `-race` suite green, x/bus 0/20 failures ([evidence](evidence/dnd-r4/README.md)). New finding F21 (two-way same-route interest on one link) proposed as a ticket.
- 2026-11-08: DND-R5. Verify's DND-R4 required fix applied (residual risk A3/A4 in DND-005 and DND-009); later owner auth/zone statements copied into DND-009. Owner's nonce decision implemented: `bus.NonceSource` with counter, math/rand and crypto/rand sources, default chosen by build tag, per-Caller option, with a forgery test that failed first. Streaming correlation test added (fails at the pin). DND-020 opened for F21 (no code). RC6 and five DND-008 defects fixed failing-test-first. Full `-race` suite green ([evidence](evidence/dnd-r5/README.md)).
- 2026-11-08: DND-R6-diagrams. New [diagrams/](diagrams/README.md), docs only:
  - README with legend, plus core, link flows, message paths and robotics (S1–S9 from research-2 §9).
  - 29 Mermaid blocks; each says as-is or target, and has a one-line caption.
  - All rendered offline with mermaid-cli 12.2.0 / Mermaid 12.3.0, 0 errors ([render check](diagrams/render-check.txt)).
  - No code changed.
- 2026-11-09: DND-R7-design, docs only. [Link-context wire](design/link-context-wire.md): draft `link.proto`, which builds and passes buf STANDARD lint offline; a DATA frame is 4 B + payload (measured with generated code), against 86–94 B today. [Close/reconnect](design/close-reconnect.md): connection generations and a tie-break for simultaneous connections. [Report](evidence/DND-R7-design.md).
- 2026-11-09: DND-R6-fix, docs only. ux's UX-3 fixes applied to the diagrams: the legend is now applied everywhere, there is a glossary (link, bridge, gateway, stage, session, match), refusals are drawn at bridges, LF4 is split into 4a/4b, target frames show 4 B, and the set is now 30 blocks. Leader's decisions on the link-context design are recorded in both design docs: protobuf frames and route ids in the CAN identifier yes; tooling download not approved; legacy fallback only via Insecure yes; lower-identity tie-break yes. [Report](evidence/DND-R6-fix.md).
- 2026-11-09: DND-commit and DND-commit-2. DND-R2…R5, R5-fix, R6 and R7 are committed locally as 14 commits (c7b5b61..684c179), each gated ([report](evidence/DND-commit.md)). Ideation notes held.
- 2026-11-09: DND-R8, not committed. Filed DND-022 (link-context core) and DND-023 (CAN face). Review F4 fixed (large-frame reads no longer allocate per KiB). DND-008 close/reconnect fixed with connection generations and the lower-address tie-break; 4 tests, 2 v2 tests moved to DND-022. Review nits F6/F8/F11/F13–F15/F17 done. DND-021 flake fixed (19 → 0 failures in 32,000 parallel runs). Earlier over-counted "parallel" stress claims corrected. [Report](evidence/DND-R8.md).
- 2026-11-09: DND-R8 and its fix round committed as 9 commits (1e38ecc..0062746). DND-R9-tools and its fix round committed: 0bb4604 (tools), e160c3c (protobuf bump), 954bc81 (codec), then the docs commit.
  - buf v1.75.0 and protoc-gen-go are pinned Go tools in `tools/` (owner-approved download);
  - buf config moved to v2 at the repo root, with the unused WKT dep dropped;
  - protobuf bumped to v1.36.12. EasyRobot makes the same one-line bump in parallel, coordinated by Leader; it was not yet committed when dndm's commit landed;
  - DND-022's `link.proto` (control `oneof`) and DATA codec added, with 4 byte-count tests (they failed first as a build failure: no API yet).
  ([report](evidence/DND-R9-tools.md))

[Overview](index.md) · [Pinned findings](research/pinned-findings.md) · [Unknown map](research/known-unknowns.md) · [Ticket index](tickets/index.md)
