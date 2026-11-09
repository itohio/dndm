---
title: "Implement the link-context wire core (LinkFrame, bind, per-session route ids)"
date: "2026-11-09"
scope: "local-only planning; implementation not started"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-022"
status: "in progress"
owner: "dndm"
priority: "P1"
milestone: "M1 remote correctness"
kind: "implementation / missing feature"
dependencies: ["DND-006", "DND-008"]
implementation_evidence: []
---

# DND-022 — Implement the link-context wire core

## State and permission

Status: **in progress** (schema and DATA codec in DND-R9-tools, in review; the link state machine not started). Owner: **dndm**. Filed in DND-R8 because ideation's research-2 §8.1 names a "link-context core ticket" as the owner of A1, A3 and A8 and of the bytes-per-message measurement. The design is accepted: [link-context-wire.md](../design/link-context-wire.md), verified in DND-R7-design and its fix round. The owner adopted the model, protobuf stays the wire contract, and Leader recorded four decisions on it.

Dependencies:
- [DND-006](DND-006-delivery-and-overload-classes.md) defines the `stages`/`policy` contents. The core can start with `latest` only.
- [DND-008](DND-008-stream-and-peer-lifecycle-races.md) owns sessions and connection generations ([close-reconnect.md](../design/close-reconnect.md)).
- Tooling: buf v1.75.0 and protoc-gen-go v1.36.12 are pinned Go tools (`tools/`, owner approved the download on 2026-11-09; DND-R9-tools).

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

- [x] `TestLinkFrameOverhead`, `TestLinkFrameRouteIdBoundary` (127 → +4 B, 128 → +5 B), `TestLinkFrameRoundTrip`, `TestLinkFrameNoCopy` (DND-R9-tools, in review).
- [ ] `TestRouteIdsStaySmallAcrossReconnects`, `TestRouteIdReuseQuarantine`, `TestCounterLowBits`.
- [ ] A1: DATA/BIND before the handshake completes → dropped and counted.
- [ ] A3: rebind of a live id, and a late frame after unbind and rebind → no retargeting.
- [ ] A5: a frame from an ONBOARD link to a trusted-origin route → refused; a v1 peer on a secure-configured link without `Insecure` → refused.
- [ ] A8: BIND flood → `BIND_STATUS_TABLE_FULL` at the cap, rate-limited.
- [ ] Bytes per message measured on EasyRobot's routes (design §8 scratch cross-check).
- [ ] Wire probe for v1 stays byte-identical; the full `-race` suite passes.

## Dated log

- 2026-11-09: DND-R8. Filed from research-2 §8.1 (A1, A3, A5, A8) and the knowledge map ("link-context core ticket"). No code.
- 2026-11-09: DND-R9-tools.
  - `proto/types/link/link.proto` follows design §2, with comments added for buf's COMMENTS lint. `link.options` holds the nanopb sizes; it has not been run through nanopb yet. Generated Go is in `types/link`.
  - `buf lint` is clean; `buf breaking` against the step-3 image is clean.
  - DATA codec in `codec/link_frame.go`, on protowire: `AppendLinkData`, `AppendLinkMessage`, `DecodeLinkData`, `AppendLinkLength`, `LinkDataSize`. 0 allocations per frame with a preallocated buffer; the decoded payload aliases the frame.
  - The four byte-count tests failed first (encoder undefined) and pass ×10 with `-race`.
  - Measured in the test: EasyRobot's state route 148 → 66 B, target route 120 → 30 B.
  - Design §8 corrected: the TCP prefix is 3 B for frames of 16 384 B or more.
  - [Evidence](../evidence/DND-R9-tools.md).
- 2026-11-09: DND-R9-fix (review R-DND-R9).
  - L1: `DecodeLinkData` rejects payload together with sealed.
  - L2: the control messages are now `oneof control`, with identical wire bytes. The decoder also rejects two control fields in one frame.
  - L3: `make proto-breaking`.
  - Both new reject cases failed first.

[Ticket index](index.md) · [Design](../design/link-context-wire.md) · [Research-2](../research/ideation/2026-11-08-ndn-delivery-auth-2.md)
