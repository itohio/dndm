---
title: "Implement the link-context wire core (LinkFrame, bind, per-session route ids)"
date: "2026-11-09"
scope: "local-only planning; implementation not started"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-022"
status: "backlog"
owner: "dndm"
priority: "P1"
milestone: "M1 remote correctness"
kind: "implementation / missing feature"
dependencies: ["DND-006", "DND-008"]
implementation_evidence: []
---

# DND-022 — Implement the link-context wire core

## State and permission

Status: **backlog**. Owner: **dndm**. Filed in DND-R8 because ideation's research-2 §8.1 names a "link-context core ticket" as the owner of A1, A3 and A8 and of the bytes-per-message measurement. The design is accepted: [link-context-wire.md](../design/link-context-wire.md), verified in DND-R7-design and its fix round. The owner adopted the model, protobuf stays the wire contract, and Leader recorded four decisions on it.

Dependencies:
- [DND-006](DND-006-delivery-and-overload-classes.md) defines the `stages`/`policy` contents. The core can start with `latest` only.
- [DND-008](DND-008-stream-and-peer-lifecycle-races.md) owns sessions and connection generations ([close-reconnect.md](../design/close-reconnect.md)).
- Tooling: generating `link.proto` needs buf and protoc-gen-go. Pinning them as Go tools is **pending the owner** (a network download).

## Scope

- `proto/types/link/link.proto` plus a nanopb `.options` file, as in design §2. Generated Go.
- A codec: encode/decode `LinkFrame`, with the DATA hot path on `protowire` (no copy of the payload).
- Link state machine: HELLO → handshake per profile → BIND/BIND-ACK → DATA → CLOSE. Every frame except hello/handshake is dropped until the handshake completes (A1).
- Route ids per session and per sending side: lowest free id; reuse after `UnbindAck` plus a quarantine on reordering faces (A3); `Hello.max_routes` cap, a BIND rate limit and `BIND_STATUS_TABLE_FULL` (A8).
- Counter low bits (7 or 14) with receiver-side rebuild.
- v1 negotiation inside today's handshake (`link_version`). A link configured as secure refuses to stay v1 unless `Insecure("legacy v1 peer")` (A5, decision 3).
- Per-route "trusted origin" flag, so command routes can refuse frames that entered via ONBOARD/INSECURE links (A5 mitigation a).

## Non-goals

Crypto primitives and key management (DND-009), broadcast group mode (DND-010/011), the CAN face ([DND-023](DND-023-can-face.md)), DND-006 delivery stages beyond `latest`.

## Acceptance (tests from design §8, written first)

- [ ] `TestLinkFrameOverhead`, `TestLinkFrameRouteIdBoundary` (127 → +4 B, 128 → +5 B), `TestLinkFrameRoundTrip`, `TestLinkFrameNoCopy`.
- [ ] `TestRouteIdsStaySmallAcrossReconnects`, `TestRouteIdReuseQuarantine`, `TestCounterLowBits`.
- [ ] A1: DATA/BIND before the handshake completes → dropped and counted.
- [ ] A3: rebind of a live id, and a late frame after unbind and rebind → no retargeting.
- [ ] A5: a frame from an ONBOARD link to a trusted-origin route → refused; a v1 peer on a secure-configured link without `Insecure` → refused.
- [ ] A8: BIND flood → `BIND_STATUS_TABLE_FULL` at the cap, rate-limited.
- [ ] Bytes per message measured on EasyRobot's routes (design §8 scratch cross-check).
- [ ] Wire probe for v1 stays byte-identical; the full `-race` suite passes.

## Dated log

- 2026-11-09: DND-R8. Filed from research-2 §8.1 (A1, A3, A5, A8) and the knowledge map ("link-context core ticket"). No code.

[Ticket index](index.md) · [Design](../design/link-context-wire.md) · [Research-2](../research/ideation/2026-11-08-ndn-delivery-auth-2.md)
