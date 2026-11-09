---
title: "Key the remote linker by route and direction"
date: "2026-11-08"
scope: "local-only planning; implementation not started"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-020"
status: "backlog"
owner: "dndm"
priority: "P1"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-observed"
dependencies: ["DND-003", "DND-006"]
implementation_evidence: []
---

# DND-020 — Key the remote linker by route and direction

## State and permission

Status: **backlog** (no code). Owner: **dndm**. Opened in DND-R5 on the owner's decision for F21: "**Yes, open a ticket**" (2026-11-08, [decisions](../../../docs/decisions.md)). The DND-R5 instructions say: open it with acceptance and a failing-test sketch, and do not implement it yet, because it may touch [DND-006](DND-006-delivery-and-overload-classes.md).

Dependencies: [DND-003](DND-003-single-owner-remote-forwarding.md) (single reader per remote interest, in review). Also [DND-006](DND-006-delivery-and-overload-classes.md), for the delivery class a two-way route gets. It is a soft dependency: the linker change can be designed without it, but it should not ship a delivery policy of its own.

## Defect (F21, found in DND-R4)

`remote.Endpoint` keeps one `dndm.Linker` per link. The linker stores intents and interests in maps keyed by route ID only ([linker.go](../../linker.go) `intents`/`interests`, lines 52–53 at the pin). Local and remote objects for the same route share one slot:

| First on B's link for route R | Then | Result |
|---|---|---|
| A subscribes (B holds a `RemoteInterest`) | B subscribes locally (`Endpoint.Subscribe`) | the linker returns the existing `RemoteInterest`; `Subscribe` fails with `ErrRemoteInterest` ([remote.go](../../endpoint/remote/remote.go) `Subscribe`). Observed in DND-R4, 3/3 ("remote interest" from `NewConsumer`) |
| B subscribes locally (`LocalInterest`) | A's interest message arrives | `handleRemoteInterest` gets the `LocalInterest` and returns `ErrLocalInterest`. The message handler only logs it ([messages.go](../../endpoint/remote/messages.go) `messageHandler`), so A's interest is dropped silently and A never gets B's data. [INFERENCE, static] |
| A publishes (`RemoteIntent`) | B publishes locally | `Publish` fails with `ErrRemoteIntent`. Symmetric: `handleRemoteIntent` → `ErrLocalIntent`. [INFERENCE, static] |

So a link cannot carry a route in both directions. Examples: two peers that both publish and subscribe the same telemetry or chat route, or a service and caller pair whose paths are mirrored on both nodes.

## Outcome

One remote link carries any route in both directions. Each direction is linked on its own: B's `LocalIntent` with A's `RemoteInterest`, and A's `RemoteIntent` with B's `LocalInterest`. Messages never loop back to the side that sent them.

## Scope

- Key the remote endpoint's linker by (route, direction), or keep two linkers (local→remote, remote→local). Decide in the design; the second may need no change to `dndm.Linker`.
- Unregister messages (`Register: false`) close only the remote-side object of their direction (`handleUnregisterIntent`/`handleUnregisterInterest` today look up by route only).
- Echo prevention: a message that arrives from A on route R must not be forwarded back to A through B's local intent for R. [INFERENCE] The router's fan-out currently has no source tag, so the design must say how echo is prevented, for example by not linking a remote intent's messages into the same link's local interest.
- Wire format: unchanged if possible (the direction is implied by who sent the intent or interest). Any change needs the wire probe and an owner decision. The v2 link-context schema ([design](../design/link-context-wire.md#2-schema-draft)) already gives each sending side its own route ids, so a route used in both directions gets two ids.

## Non-goals

Delivery classes, buffering or reliability (DND-006); auth or zones (DND-009); mesh multi-hop loops beyond one link (DND-012).

## Failing test sketch (not written yet)

`x/bus/forwarding_test.go` (or `endpoint/remote`), stream pair A↔B as in `TestRemoteForwardingHasOneReader`:

1. A: `NewConsumer(R)` and `NewProducer(R)`. B: `NewConsumer(R)` and `NewProducer(R)`. Expect no error. Today the second `Subscribe` on one side fails with `ErrRemoteInterest`.
2. A sends `a-1…a-10` and B sends `b-1…b-10`, one at a time (paced, as in DND-003, so DND-006 overload drops cannot confound the result).
3. B's consumer receives exactly `a-1…a-10`, and A's consumer exactly `b-1…b-10`. Local echo is the separate question below: no message comes back to its sender over the link.
4. Both orders of setup (A first, B first), because the failure differs by order (error vs silent drop).
5. Unregister: A closes its consumer. B's producer then sees no interest from A, B's consumer still receives A's messages, and A's producer is unaffected.

Expected at the pin and at DND-R5: step 1 fails (`ErrRemoteInterest`) or step 3 times out (silent drop), depending on the order.

Open question for the design: should a local subscriber also receive the messages of a local producer on the same node? That already holds through the router, independent of the link. The test must separate "local delivery" from "came back over the link".

## Acceptance

- [ ] The failing test above, written first and recorded at the pin, fails as predicted for both setup orders.
- [ ] After the fix it passes ×20 with `-race`, and the whole suite passes `go test -race -count=3 ./...`.
- [ ] No echo: messages that arrived over a link are not sent back over the same link (a counter or test assertion).
- [ ] Unregister of one direction leaves the other direction working.
- [ ] Wire probe byte-identical, or the change is documented and approved.
- [ ] `ErrRemoteInterest`/`ErrLocalInterest`/`ErrRemoteIntent`/`ErrLocalIntent` from these paths are either gone or documented as a real conflict. They are exported errors, so keep them.

## Dated log

- 2026-11-08: DND-R5. Opened from F21 (DND-003, DND-R4) on the owner's decision. No code.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [DND-003](DND-003-single-owner-remote-forwarding.md)
