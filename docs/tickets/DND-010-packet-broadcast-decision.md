---
title: "Decide whether to build packet broadcast"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-010"
status: "backlog"
owner: "unassigned"
priority: "P1"
milestone: "M2 packet decision"
kind: "research / missing feature and tradeoff"
dependencies: ["DND-006", "DND-009"]
implementation_evidence: []
---

# DND-010 — Decide whether to build packet broadcast

## State and permission

Status: **backlog**. Owner: **unassigned**. Kind: **research / missing feature and tradeoff**.
Milestone: **M2 packet decision**. Priority: **P1** — Packet fan-out is absent, so design and need must be established before implementation or performance comparison.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Obtain explicit owner permission for the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-006](DND-006-delivery-and-overload-classes.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md)

## Owner decision (2026-11-08, verbatim from the roster)

"DND-010: **Build it** (packet broadcast)."

The memo's recommendation (no-go for now) is overridden by this decision. Status unchanged until Leader records how decided tickets close; the design waits for ideation's research. Design questions noted by dndm (DND-R4):

- Broadcast vs multicast vs per-peer UDP unicast on the same datagram codec; which networks (wired LAN, Wi-Fi, ESP-NOW-like links)?
- Datagram format: reuse the stream frame (magic, length, header, body) with a hard MTU cap and no fragmentation first, or define fragmentation?
- Loss handling comes from DND-006 (`latest` tolerates loss; `reliable` needs acks and retransmit); datagram authentication comes from DND-009.
- Embedded footprint and a test plan that does not need real network access until the owner approves it.

## Outcome

A go/no-go decision for a bounded packet transport and, if go, a complete datagram contract.

## Scope

Compare existing TCP, packet unicast, LAN broadcast/multicast and broker routing. Specify source/frame identity, MTU, bounded fragmentation/reassembly, sequence/loss/duplicate rules, authentication, congestion/rate control and target network assumptions. Correctly classify the embedded example naming mismatch.

## Non-goals

No claim of current UDP/broadcast, no network setup changes, radio experiments or earlier Python implementation equivalence.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md), sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [network/net/net.go](../../network/net/net.go) lines 31–82 — Stream Listen/Accept does not supply a UDP listener/packet codec.
- [codec/codec.go](../../codec/codec.go) lines 85–110 — Separate prefix/body reads do not preserve datagram boundaries.
- [examples/embedded/embedded/main.go](../../examples/embedded/embedded/main.go) lines 207–245 — setupUDPRouter constructs tcp peers.
- [examples/embedded/host/main.go](../../examples/embedded/host/main.go) lines 278–281 — Host example also constructs tcp peers.

## Unknowns and decisions

Actual transport need, multicast availability, MTU, loss tolerance, receiver count and security overhead are unmeasured. A no-go result is valid.

## Proposed reproduction or decision fixture — NOT RUN

Use a paper packet trace with loss, duplication, reordering, missing fragments, oversized assembly, forged source and receiver departure. For every candidate calculate upper-bound receiver state from configured caps and define expiration.

## Decision memo (2026-11-08, for the owner)

**Question.** Should dndm get a UDP packet transport (unicast, broadcast or multicast) now, later, or never?

This memo assumes the recommendations in [DND-006](DND-006-delivery-and-overload-classes.md) (two in-session classes) and [DND-009](DND-009-identity-auth-and-profile-boundary.md) (trusted-local plus a NATS gateway). If the owner picks differently there, revisit this memo.

**What the code does at the pin** (read from source, nothing executed):

- `Node.Dial` passes the URI scheme to `net.Dial` (`network/net/net.go:38`), so `udp://` would dial a connected UDP socket [INFERENCE, Go stdlib]. `Serve` uses `net.Listen` (`network/net/net.go:48`), which accepts only stream networks, so a `udp://` server fails [INFERENCE, stdlib docs; not run].
- Framing does two separate `Read` calls (`codec/codec.go:87,101`). On a datagram socket each `Read` takes one whole datagram and discards what does not fit [INFERENCE], so the stream codec cannot carry packets. The frame size comes off the wire as an unbounded `uint32` (`codec/codec.go:99–100`).
- Remote fan-out runs one goroutine per remote interest; each encodes and writes its own copy (`endpoint/remote/wrappers.go:211–226`, `network/stream/stream.go:361–366`). [INFERENCE, unmeasured] N remote subscribers cost N encodes and N sends. This is the cost broadcast would attack.
- The route travels inside the protobuf header (`proto/types/core/header.proto:30`). Receivers could reject early only after decoding the header, not "at network layer" as the docs say (`COMMUNICATION_AND_USAGE_SPEC.md:25–29`, `DESIGN.md:416–420`).
- The `udp` setup in the embedded example builds TCP peers (`examples/embedded/embedded/main.go:38–39,207–215`). README lists "TCP/UDP/Serial" (`README.md:14`).
- No sender authentication exists (DND-009). Broadcast would expose every message to every LAN host and accept injection from any of them.

**Option A: no-go now, with a written revisit trigger.** Keep TCP and serial. Cancel [DND-011](DND-011-bounded-packet-transport.md) explicitly. Correct the misleading `udp` example label and the UDP/broadcast claims in README/DESIGN; the owner places that small fix, which is not DND-011 scope.
- Revisit when a DND-016 measurement shows one LAN sender serving enough remote subscribers that per-connection sends are the bottleneck.
- Cost: none. Risk: the docs' UDP broadcast intent stays unrealised. Compatibility: no change.

**Option B: go, UDP unicast datagrams for the `latest` class only.**
- Code: a new `network/udp` (or `network/packet`) package using `net.ListenPacket`; a packet codec with one message per datagram, a size cap below path MTU, no fragmentation, a per-route sequence number for drop/duplicate detection, and a new magic number; an endpoint that skips stream framing. Cost: medium.
- Risk: unauthenticated, so it inherits the trusted-local restriction. It saves no sender-side copies compared with TCP.
- Compatibility: additive package; new datagram format alongside the unchanged stream format.

**Option C: go, LAN broadcast/multicast.** B plus group addressing, bounded fragmentation and reassembly, rate limiting, early reject by route, and per-sender authentication (MAC or signature). That last part needs keys, i.e. DND-009 option B.
- Cost: large.
- Risk: Wi-Fi multicast at low basic rates, loss, injection from any host, reassembly memory exhaustion [INFERENCE].
- Compatibility: new wire format; key configuration in the public API.

| | A no-go | B UDP unicast | C broadcast/multicast |
|---|---|---|---|
| Code | docs/example label | new package + codec | B + groups, reassembly, auth |
| Sender copies vs TCP | same | same | one transmission per group |
| Needs keys (DND-009 B) | no | no | yes |
| Wire format | unchanged | new, additive | new, additive |
| Measured need | none | none | none |
| DND-011 | cancelled | sized (smaller) | sized (larger) |

**Recommendation: A.** No measurement shows that per-connection TCP sends are a bottleneck, and nothing has been measured at all. Safe broadcast needs per-sender authentication, which DND-009's recommendation defers. NATS remains the cross-host path, and the recommended `latest` class already bounds slow-subscriber cost on TCP. A no-go is a valid, reversible outcome. Fixing the `udp` labels turns a misleading claim into an accurate one. If the owner wants packets anyway, choose B before C: it tests datagram framing and loss handling without taking on keys or group membership.

**What it unblocks.** These tickets list DND-010 in their frontmatter `dependencies:`: [DND-011](DND-011-bounded-packet-transport.md) (cancelled on no-go, sizable on go), [DND-013](DND-013-capability-negotiation-contract.md) (packet capability recorded as "unsupported" or "supported"), [DND-015](DND-015-embedded-target-feasibility.md) (embedded transport set). Transitively: DND-014 and DND-016.

**Residual unknowns.**

- Receiver count, payload sizes and rates, and the wired/Wi-Fi mix. None are measured.
- Whether the target network allows multicast.
- Design history of the owner's earlier Python broadcast robot, not located.
- Whether anything dials `udp://` today; `Dial` would succeed while `Serve` fails.

## Acceptance — unmet until evidence exists

- [ ] Decision distinguishes one transmission from all network/receiver copies and from local pointer fan-out.
- [ ] A go result specifies packet boundaries, resource caps, overload/security policy and compatibility; a no-go records rationale and disables DND-011.
- [ ] No TCP path is labelled working UDP; no assumed NATS performance disadvantage.

## Verification procedure — future, not executed

Decision can be closed with a reviewed design and explicit unresolved measurements, not claimed runtime support. Actual packet implementation/measurement are DND-011/DND-016 after permission.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium research: one selected topology and delivery class; multi-network reliable multicast is outside this slice.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R2. Decision memo added (options, recommendation). Decision pending with the owner; status unchanged.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
