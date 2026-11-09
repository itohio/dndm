---
title: "Source-pinned dndm findings"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
---

# Source-pinned findings

## Provenance and limits

Pin: **`c7b5b61f79ea07d3359d8963ac81b967e3e544ec`**, source project itohio/dndm, commit dated 2025-11-15 in the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md). Report date 2026-11-08; **two independent static source reviews completed**, corrections incorporated. Go 1.24.1 and MIT are report facts; dependency/target compatibility remains unverified. The canonical report was read in full for this handoff. No source audit was repeated.

This is a compact planning evidence map, not a substitute source audit or a runtime result. Paths below are relative links into the checkout; line ranges are pinned-revision references. Repository-wide absence findings come from the report's inspected scope, not a claim that one cited line proves absence everywhere. Every future behavior/performance assertion remains **#verify** until its ticket's evidence exists.

## Evidence-to-ticket map

| Finding | Classification | Inherited finding | Source anchor at pin | Tickets |
|---|---|---|---|---|
| F1 | Fixable static defect | Partial stream reads, unbounded length parsing and pool returns. | [codec/codec.go](../../codec/codec.go) lines 85–132 | [DND-001](../tickets/DND-001-bounded-stream-framing.md) |
| F2 | Fundamental ownership tradeoff | Same local protobuf object is fanned out; remote serialization creates new objects. | [intent.go](../../intent.go) lines 99–117,192–210 | [DND-002](../tickets/DND-002-local-buffer-ownership-contract.md) |
| F3 | Fixable static defect | Remote forwarding and FanInInterest consume the same channel. | [endpoint/remote/wrappers.go](../../endpoint/remote/wrappers.go) lines 197–223 | [DND-003](../tickets/DND-003-single-owner-remote-forwarding.md) |
| F4 | Fixable static defect | Pre-handshake registration can be rejected without replay. | [container.go](../../container.go) lines 65–89,111–139 | [DND-004](../tickets/DND-004-handshake-registration-replay.md) |
| F5 | Fixable static defect | Caller nonces collide across callers; unknown replies fall back to FIFO. | [x/bus/caller.go](../../x/bus/caller.go) lines 88–96,112–128 | [DND-005](../tickets/DND-005-caller-scoped-correlation.md) |
| F6 | Fixable defect plus policy choice | Closed-channel receive, silent drops and blocked common dispatcher. | [interest.go](../../interest.go) lines 169–191,279–292 | [DND-006](../tickets/DND-006-delivery-and-overload-classes.md), [DND-007](../tickets/DND-007-bounded-dispatch-and-closure.md) |
| F7 | Fixable static defect | Unsynchronised stream writes and address-book/dial state. | [endpoint/mesh/addrbook.go](../../endpoint/mesh/addrbook.go) lines 218–229,322–326 | [DND-008](../tickets/DND-008-stream-and-peer-lifecycle-races.md) |
| F8 | Missing feature | Identity claims and lexical prefixes are not access control. | [peer.go](../../peer.go) lines 76–77 | [DND-009](../tickets/DND-009-identity-auth-and-profile-boundary.md), [DND-017](../tickets/DND-017-enforce-profile-gateway-boundary.md) |
| F9 | Missing feature plus tradeoff | No usable UDP listener/datagram codec or packet fan-out found. | [network/net/net.go](../../network/net/net.go) lines 31–82 | [DND-010](../tickets/DND-010-packet-broadcast-decision.md), [DND-011](../tickets/DND-011-bounded-packet-transport.md) |
| F10 | Missing feature plus tradeoff | Fan-out is not durable commands, one-worker queues or execution deduplication. | [intent.go](../../intent.go) lines 186–220 | [DND-006](../tickets/DND-006-delivery-and-overload-classes.md), [DND-012](../tickets/DND-012-durability-and-command-boundary.md) |
| F11 | Missing feature | Typed routes exist, but negotiated capabilities/NATS connector remain new work. | [route.go](../../route.go) lines 47–68,100–158 | [DND-013](../tickets/DND-013-capability-negotiation-contract.md), [DND-014](../tickets/DND-014-scoped-nats-bridge.md) |
| F12 | Feasibility unknown | UDP-named embedded setup actually constructs TCP peers. | [examples/embedded/embedded/main.go](../../examples/embedded/embedded/main.go) lines 207–245 | [DND-015](../tickets/DND-015-embedded-target-feasibility.md) |
| F13 | Measurement unknown | No established performance ranking; pointer and serialized paths differ. | [endpoint/remote/wrappers.go](../../endpoint/remote/wrappers.go) lines 60–92,168–223 | [DND-016](../tickets/DND-016-reproducible-transport-measurement.md) |

## Cross-file anchors that must not be lost

- **Competing readers:** remote registration at [remote.go](../../endpoint/remote/remote.go) lines 70–74 and [dndm.go](../../dndm.go) lines 51–65 creates FanInInterest; [interest.go](../../interest.go) lines 138–177 consumes the same `li.C()` used by [wrappers.go](../../endpoint/remote/wrappers.go) lines 197–223. This is an unintended competing-reader path, not worker load balancing.
- **Handshake registration:** [handshake.go](../../endpoint/mesh/handshake.go) lines 115–131 rejects before HS_DONE; route lists at 170–175 and 219–224 are nil. Completion racing registration matters; the no-op callback alone does not prove discovery failure.
- **Streaming/closure:** [caller.go](../../x/bus/caller.go) lines 173–224 supports multiple callback replies; lines 258–289 can block the common dispatcher on a one-item waiter channel. This is not a complete duplex accept/data/end/cancel/resume protocol.
- **Lifecycle:** plain writes at [stream.go](../../network/stream/stream.go) lines 361–370 lack the serialized path at 110–121 and 145–165. [addrbook.go](../../endpoint/mesh/addrbook.go) lines 218–229 and 322–326 carry reported synchronization gaps; the report also records ignored network context/dial options. No race test was run.
- **Identity:** [route.go](../../route.go) lines 100–119 hashes known type/path; [handshake.go](../../endpoint/mesh/handshake.go) lines 203–217 accepts advertised identity text. Neither creates a permission boundary. `profile.a` can lexically match `profile.ab` under [peer.go](../../peer.go) lines 76–77.
- **Embedded example:** [host/main.go](../../examples/embedded/host/main.go) lines 278–281 also constructs TCP peers. The report records Compose selecting `TRANSPORT=tcp` while program switches accept serial/udp, and ordinary Go builds in the [embedded Makefile](../../examples/embedded/Makefile). None proves TinyGo/RP2040 compatibility.

## What is implemented versus proposed

The report traces router → intent/interest routers → endpoint/linker → channel delivery locally, or remote wrapper → stream connection → protobuf framing/decoding remotely. Typed type/path routes and local object fan-out exist. TCP/serial stream plumbing exists. Remote message paths marshal per connection and decode to new objects.

A usable packet listener, datagram codec, broadcast/multicast packet dispatch, NATS connector, authenticated profile boundary and durable message/consumer/command store were **not found in the pinned source audit**. They are not merely configuration switches. An earlier owner-reported Python robot implementation is a different implementation and is not evidence that this Go revision broadcasts.

## Fundamental choices, not bug fixes

1. Local pointer sharing reduces a different cost than network fan-out; ownership/lifetime cannot be left implicit.
2. One LAN broadcast transmission does not eliminate receiver/device copies, loss, ordering, slow recipients, Wi-Fi rate constraints or authentication overhead.
3. Perishable frames often need expiry/drop; commands need persistent authority and idempotent effects. One delivery guarantee is inappropriate.
4. Brokerless mesh replaces broker dependence with discovery, connection, authentication and partition complexity. Cost/fault-tolerance superiority is unmeasured.
5. Keep control, binary data and knowledge/storage separate. NATS/application command handling and Markdown domain authority are not replaced by this dndm plan; optional vectors/model/media/robot work are not prerequisites.

## Evidence state

Static source evidence: inherited from the final reviewed report. Runtime evidence: **none**. Implementation evidence: **none**. Performance ranking: **unknown**. Hardware compatibility: **unknown**. Only the [roadmap's document-check record](../roadmap.md#static-document-check-record) may describe checks actually executed in this handoff.

## Addendum 2026-11-08: runtime baseline

Appended by R1-04; sections above are unchanged history. Evidence: [baseline 2026-11-08](../baseline-2026-11-08.md), same pin, go1.27.2 darwin/arm64, `GOTOOLCHAIN=local GOPROXY=off`.

| Earlier statement | Baseline observation | Effect |
|---|---|---|
| Evidence state: "Runtime evidence: none"; lifecycle anchor: "No race test was run" | `go build`, `go vet`, `go test` and `go test -race` run on every package with tests | Superseded for those 5 packages; untested packages still have no runtime evidence |
| Provenance: "Go 1.24.1 … dependency/target compatibility remains unverified" | Build, vet and non-race tests pass with go1.27.2 on darwin/arm64 from the module cache | Host compatibility shown for one toolchain; TinyGo/embedded still unverified |
| F1 "Fixable static defect" | Probe: valid frames rejected under one-byte/half reads; `DecodeMessage` panics on 1–3-byte and header-only bodies; 256 MiB allocated from one preamble | Runtime-confirmed (pool leak on error path still static). Severity up: [INFERENCE] remote-triggerable process crash, since the read loop has no `recover` |
| F7 stream/address-book races | `endpoint/mesh` has no tests; the plain `Stream.Write` path produced no race report | Neither confirmed nor refuted. A different stream race exists (F14, RC5) |

New findings (not in the canonical report):

- **F14 (runtime): core lifecycle data races.** 55 race reports, 23 unique access pairs, 5 classes: RC1 `LocalIntent` close callback vs `Link`/`Notify` ([intent.go](../../intent.go) 72–73, 125–137); RC2 router callbacks assigned after runner goroutines start (intent.go 269–270, 320–324, 370–372; [interest.go](../../interest.go) 179–180, 230–234, 283); RC3 `Linker.Close` callback mutating maps and `errarr` unlocked ([linker.go](../../linker.go) 85–105 vs 164, 227–231, 274); RC4 `Base.AddOnClose` runs each callback in an unowned goroutine ([endpoint.go](../../endpoint.go) 78–86); RC5 `contextRW.Close` vs `Request` ([stream.go](../../network/stream/stream.go) 145 vs 198). RC1–RC4: no ticket (owner decision in the [ticket index](../tickets/index.md#baseline-gate)); RC5 overlaps DND-008.
- **F15 (runtime): the non-race suite is nondeterministic.** In `-count=10`, an `AddOnClose` callback logged after `TestLinker_AddInterestIntentSend` returned (test-framework panic, package aborted), and `TestStreamStarTopologyBidirectional` timed out once. Same root mechanism as RC4.
- **F16 (static): `make build` cannot work.** It builds `./cmd/<example>`, and the repo has no `cmd/` directory. Not run.
- **Test-coverage caveat for F1.** The `x/bus` remote tests run over `io.Pipe`, where each frame is one `Write` and reads drain that write in order. That hides the single-`Read` defect; TCP and serial do not preserve write boundaries.
- **Correction 2026-11-08 (R1-04 verification).** In F14, "RC1–RC4: no ticket" means no ticket targets them *as races*. DND-006 and DND-007 pin `interest.go` 169–191, which contains RC2's 179–180, for the missing receive-ok check. RC5's `stream.go:145` falls inside DND-008's pinned 145–165, which DND-008 cites as the serialized pattern. Race report and pair counts vary per run (rerun: 51 reports, 18 pairs; see [baseline](../baseline-2026-11-08.md#race-reports)).

## Addendum 2026-11-08: DND-R2 working tree

Appended; earlier sections unchanged. Evidence: [DND-R2 evidence](../evidence/dnd-r2/README.md). Line numbers below are at the pin.

- **F14 RC1–RC4: fixed in the working tree** by [DND-018](../tickets/DND-018-race-clean-core-lifecycle.md). `go test -race -count=3 ./...` reports no race at an RC1–RC4 location. RC1 was worse than a race: `LocalIntent.Notify` panicked with `send on closed channel` without `-race` (dnd018-before.txt).
- **F15: root package now deterministic in `-count=10`.** The test that logged after it returned now waits for the parent-cancel callback.
- **F1: fixed in the working tree** by [DND-001](../tickets/DND-001-bounded-stream-framing.md), including the pooled-buffer leak, which a counting pool made observable before the fix (`pool gets 1, puts 0`).
- **New F17 (runtime): RC6, interest close vs an in-flight linked send.** The `LocalInterest` close callback closes `msgC` ([interest.go](../../interest.go) 66–69) while a `LocalIntent.Send` that read `linkedC` before the unlink can still send on it ([intent.go](../../intent.go) 112–118). This is a close/send race and a possible `send on closed channel` panic. It is pre-existing: already in the baseline `-count=3` log (line 4583). It is not one of RC1–RC4. Ticket: [DND-007](../tickets/DND-007-bounded-dispatch-and-closure.md) (closure).
- **x/bus flakiness is unchanged in kind.** In 20 runs without `-race` there were 9 timeouts at the pin and 7 in the working tree, with a different mix of tests. [INFERENCE] the cause is F3 competing readers.

## Addendum 2026-11-08: DND-R3 working tree

Appended; earlier sections unchanged. Evidence: [DND-R3 evidence](../evidence/dnd-r3/README.md).

- **New F18 (runtime): remote endpoint Close self-deadlock.** The message handler calls `Close` on a read error, and `Close` waits for the handler ([messages.go](../../endpoint/remote/messages.go) 40, [remote.go](../../endpoint/remote/remote.go) 113 at the pin). An external Close hangs the same way. Fixed in the working tree ([DND-019](../tickets/DND-019-remote-endpoint-close-self-deadlock.md)).
- **RC5: fixed in the working tree** (DND-019; it became the common path once Close returned).
- **F7 partly fixed:** `Addrbook.AddConn` now inserts under the lock ([DND-004](../tickets/DND-004-handshake-registration-replay.md)); the other DND-008 items stay open.
- **DND-004 static finding runtime-confirmed and fixed:** routes existing when a mesh connection is added were skipped forever, and `Container.Publish` failed while any connection was mid-handshake.
- **New F19 (runtime): WaitGroup Add/Wait race** when adding to a fan-out that is closing ([intent.go](../../intent.go) 226–240 at the pin). Fixed: adds after close return `ErrClosed`.
- **DND-005 runtime-confirmed:** two Callers sharing paths can cross-deliver replies. Corrected in DND-R4 after verification: the outcome is bimodal per run at both trees. In each run the second caller either always gets the first caller's stale reply or always times out, and both modes occur at the pin (verify: 3 of 16 runs misdelivered at each tree). The misdelivery is pre-existing, not caused by DND-R2/R3. Fixed in DND-R4 ([DND-005](../tickets/DND-005-caller-scoped-correlation.md)).
- **Seen, not fixed (F20):** the remote endpoint calls its `addIntent` callback twice per new remote intent (`messages.go` 231 plus the linker callback). [INFERENCE, static] an `IntentRouter` notified before its first `Wrap()` loses that readiness notification (DND-007).

## Addendum 2026-11-08: DND-R4 working tree

Appended; earlier sections unchanged except line 99, corrected at verify's request. Evidence: [DND-R4 evidence](../evidence/dnd-r4/README.md).

- **F3 (competing readers): fixed** ([DND-003](../tickets/DND-003-single-owner-remote-forwarding.md)). `Router.addInterest` no longer builds a fan-in around remote interests. Before the fix, a paced 20-message test missed `tag-1` 3/3 at the pin.
- **F20, second half: fixed.** A router wrapper created after interest was seen now starts with a notification; at the pin this test fails 3/3. This was the timeout mode of the two-Caller probe and the cause of second-producer hangs.
- **New F22 (runtime): use after close in router wrappers.** `removeWrapper` cleared `w.router`, racing `Send`/`Route`; after synchronous close, `Send` panicked. Fixed.
- **New F23: `Link.Link` could re-link after Close** (its goroutine could run after `Close`). Fixed: it now runs in the caller.
- **New F21 (runtime, not fixed): one interest per route per remote link.** A link cannot carry interest in both directions for the same route; the second `Subscribe` fails with `ErrRemoteInterest`/`ErrLocalInterest` ([DND-003](../tickets/DND-003-single-owner-remote-forwarding.md#found-while-testing-not-fixed-f21)).
- **x/bus is no longer flaky in these runs:** 0/20 failures with and without `-race` (pin 9/20). The DND-R2/R3 "unchanged in kind" notes describe the earlier state.
- RC6 is still open, but not observed in DND-R4 runs.

## Addendum 2026-11-08: DND-R5 working tree

Appended; earlier sections unchanged. Evidence: [DND-R5 evidence](../evidence/dnd-r5/README.md).

- **F17 / RC6: fixed** ([DND-007](../tickets/DND-007-bounded-dispatch-and-closure.md#implementation-evidence)). At the pin a Send blocked on a full interest channel panics (`send on closed channel`) when the interest closes, 3/3. The same close-vs-send race exists in `FanInInterest` (race 3/3), and on linker context cancellation, where the link and the interest close concurrently (3/3).
- **New F24 (runtime): DND-005 nonces were predictable** (verify's A3). The counter source is kept for small devices. Plain Go builds now default to crypto/rand per call. A4 (copying the nonce from the request) stays open until DND-009.
- **DND-008 static notes confirmed at runtime and fixed:**
  - plain `Stream.Write` interleaves frames when the writer splits a call;
  - `contextRW` fills an abandoned read buffer and loses those bytes;
  - a cancelled `StreamContext.Write` returns its frame buffer to the pool while the loop is still writing it (the frame was read back with another message's text), and masks the error as `ErrNotEnoughBytes`;
  - `AddrbookEntry.dialing` is raced;
  - `net.Node.Dial` ignores its context.
- **F21** is now [DND-020](../tickets/DND-020-remote-linker-route-direction.md). [INFERENCE, static] In the other setup order (local `Subscribe` first), the remote interest is dropped silently, because `handleRemoteInterest`'s `ErrLocalInterest` is only logged.
- Open: DND-008 close/reconnect coordination; one unexplained `TestContainer_Publish` failure in a late gate rerun (see the evidence README).

[Overview](../index.md) · [Unknown map](known-unknowns.md) · [Tickets](../tickets/index.md)
