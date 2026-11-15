---
title: "Choose local shared-message ownership"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-002"
status: "done"
priority: "P0"
milestone: "M0 decisions"
kind: "research / fundamental tradeoff"
dependencies: []
implementation_evidence: ["x/bus/ownership_test.go", "commit 3907a77"]
---

# DND-002 — Choose local shared-message ownership

## State and permission

Status: **done** (DND-R4: decision taken, contract written into godoc, aliasing test added; verified, reviewed, committed locally as 3907a77). Kind: **research / fundamental tradeoff**, now decided.
Milestone: **M0 decisions**. Priority: **P0**.

**Decision (2026-11-08):** accept Option A — messages are immutable after Send, with a written contract.

Dependencies: none.

## The contract (DND-R4)

1. **Sender.** A message is immutable from the moment `Send` is called, not just after it returns. Neither the sender nor anyone else may modify it, or anything it points to (bytes fields, nested messages, slices, maps). To change data, send a new message (or `proto.Clone`, then modify the clone).
2. **Receivers.** Messages from `Interest.C()` or `Consumer.Receive` are read-only. A local message is the sender's own object, shared with every other local subscriber. Receivers may keep it as long as they like; they must `proto.Clone` it before modifying it.
3. **Why.** Local fan-out passes the same pointer to every subscriber and, at the same time, to remote endpoints that marshal it ([intent.go](../../intent.go) `FanOutIntent.Send`, [interest.go](../../interest.go) `routeMsg`). Remote receivers get a fresh decoded object, but senders cannot tell which path a message takes.
4. **Callers.** `bus.Caller.Call` needs a fresh request per call: the request-nonce hook writes into it before it is sent.
5. **Not covered.** Pooled or zero-copy buffers (a future packet path, DND-011) need their own rule; this contract covers protobuf messages.

Where it is written (godoc): `dndm.Intent.Send`, `dndm.Interest.C`, `bus.Producer.Send`/`SendDirect`, `bus.Consumer.Receive`, `bus.Service.Reply`, `bus.Caller.Call`.

## Outcome

A decision-ready ownership contract choosing immutable publication, cloning, or explicit leases for each local payload class.

## Scope

Compare publisher mutation, concurrent subscribers, retained messages, pooled data and cancellation. Define the ownership transfer point, release responsibility and compatibility cost; choose the simplest safe policy.

## Non-goals

No implementation, claimed zero-copy network path, new shared-memory allocator or language/runtime port.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [intent.go](../../intent.go) lines 99–117,192–210 — Local fan-out forwards the same protobuf object.
- [interest.go](../../interest.go) lines 280–289 — Subscribers receive shared objects.
- [endpoint/remote/wrappers.go](../../endpoint/remote/wrappers.go) lines 60–92,168–223 — Remote wrappers serialize per connection.

## Unknowns and decisions

Can existing callers promise immutability, and do any payloads reference recycled buffers? Source audit did not establish the consumer lifetime contract.

## Proposed reproduction or decision fixture — NOT RUN

Draw the timeline for publish → two subscribers → one retained message → publisher mutation/reuse. Repeat with one cancelled subscriber and remote serialization. For each candidate policy assign every read, mutation and release to one owner.

## Decision memo (2026-11-08)

**Question.** Once `Send(ctx, msg)` returns, who may read, change, keep or release `msg` and any buffer it points to?

**What the code does at the pin** (read from source with `git show c7b5b61:<file>`, nothing executed):

- `LocalIntent.Send` puts the caller's pointer on the linked channel and makes no copy (`intent.go:112–118`).
- `FanOutIntent.Send` passes the same pointer to every endpoint intent from concurrent goroutines (`intent.go:203–211`). `InterestRouter.routeMsg` puts the same pointer in every subscriber wrapper (`interest.go:283–289`).
- A remote subscriber's goroutine marshals that shared object for its connection (`endpoint/remote/wrappers.go:220–221` → `network/stream/stream.go:361–362`), possibly while local subscribers are reading it.
- The receiver decodes into a new object (`codec/codec.go:136,164,172`) and returns the pooled frame buffer right after decoding (`network/stream/stream.go:83–90,321–328`). [INFERENCE] By default protobuf-go `Unmarshal` copies bytes fields, so decoded messages do not alias pooled memory.
- No doc comment states any contract (`intent.go:36–37`, `interest.go:33–35`). The docs claim "zero-copy" (`README.md:12`, `DESIGN.md:73`). `codec.Release` is exported (`codec/codec.go:21–23`). `SubOptStruct`/`PubOptStruct` exist but are empty (`options.go:96–104`).
- Two publishers sampled in a robotics library that uses dndm (not a full audit) build a new message for each `Send`.

**Option A: immutable after Send (a documented contract).** The publisher does not change or reuse `msg` after `Send`. Subscribers treat received messages as read-only and may keep them. `codec.Release` takes only buffers that came from `codec`.
- Code: doc comments in `intent.go` and `interest.go`, corrected README/DESIGN wording, and one `-race` fixture where a publisher mutates after `Send`.
- Behaviour: unchanged. Cost: small.
- Risk: nothing enforces it. A caller who breaks it causes a data race that shows up only under `-race` or as corrupted data.
- Compatibility: no API or wire change, so downstream users are unaffected.

**Option B: clone at fan-out.** Call `proto.Clone` for each subscriber in `routeMsg` (`interest.go`) and for each endpoint in `FanOutIntent.Send` (`intent.go`).
- Behaviour: subscribers may mutate, and the publisher may reuse the message once `Send` returns.
- Cost: one allocation and deep copy per subscriber per message (not measured). The "zero-copy" claim must be dropped.
- Risk: large payloads such as lidar scans and frames pay the copy on every route.
- Compatibility: no API or wire change. Behaviour changes only for code that relies on shared mutation; the sample found none.

**Option C: explicit leases.** Messages travel in an envelope with `Release()`, reference counts and pooled payload buffers.
- Code: `intent.go`, `interest.go`, `codec/`, `x/bus/` and every endpoint. Cost: large.
- Risk: leaks and double releases across the concurrent fan-out goroutines.
- Compatibility: breaks `Interest.C() <-chan proto.Message` (`interest.go:35`) and `Intent.Send` (`intent.go:37`). Downstream users and `x/bus` callers must change.

| | A immutable | B clone | C leases |
|---|---|---|---|
| Code change | docs + one fixture | 2 call sites | API + every endpoint |
| Public API | unchanged | unchanged | breaking |
| Wire format | unchanged | unchanged | unchanged |
| Extra cost per message | none | copy per subscriber | refcount + pool |
| Enforcement | convention + `-race` | structural | structural, error-prone |
| DND-003/007 can build on it | now | now | after redesign |

**Recommendation: A.** Add a per-subscription copy option later only if a real consumer needs to mutate; it fits in the empty `SubOptStruct` (`options.go:97–99`) without breaking callers. A writes down what the code already does. It costs nothing at runtime, changes neither API nor wire format, and the sampled robotics-library callers already follow it. Because the remote marshaller and local subscribers would all be readers, A also makes the current marshal-during-local-read pattern correct by contract. B charges every subscriber for a need nobody has shown. C is a new allocator, which this ticket's non-goals exclude, and it breaks the public API.

**What it unblocks.** These tickets list DND-002 in their frontmatter `dependencies:`: [DND-003](DND-003-single-owner-remote-forwarding.md), [DND-007](DND-007-bounded-dispatch-and-closure.md), [DND-011](DND-011-bounded-packet-transport.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-015](DND-015-embedded-target-feasibility.md). Transitively: DND-008 (via DND-007) and DND-014.

**Residual unknowns.**

- Whether any caller mutates or reuses a message after `Send`. Only two call sites in the robotics library were sampled; other downstream users and the `x/bus` helpers were not audited.
- Whether `x/bus` keeps messages after delivery. `x/bus/caller.go` was not read for retention.
- The protobuf no-aliasing assumption above is unverified in this repo.
- A future packet path (DND-011) with zero-copy receive needs its own buffer rule. A covers decoded messages only.

## Acceptance

- [x] Decision records aliasing/lifetime invariants and rejected alternatives (memo above; contract section).
- [x] Every timeline has a valid owner: sender until `Send`, then shared read-only; a modifying receiver must clone first.
- [x] Compatibility: no API, wire or runtime change (godoc only). Verification: `x/bus/ownership_test.go` `TestLocalSubscribersShareTheSentMessage` shows two local subscribers receive the sender's object (`assert.Same`), ×20 with `-race`. A throwaway probe shows that modifying after Send is a data race the receiver can observe: `-race` reports it and the receiver saw the new value "v2". No performance claim.

## Verification procedure — future, not executed

Static contract walkthrough first; later authorized mutation/lifetime fixtures must prove the chosen policy, not merely observe no crash.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Small research decision: one local ownership model; implementation size remains unknown until policy selected.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and the 2026-11-08 scope amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R2. Decision memo added (options, recommendation). Decision pending; status unchanged.
- 2026-11-08: DND-R4. Option A (immutable after Send) accepted. Contract written into godoc and this ticket; aliasing test added; mutate-after-send race probe recorded. Status → **in review**.
- 2026-11-09: status → **done**. Accepted after independent verification and committed locally: 3907a77 `docs(api): message ownership contract for Send and C` (DND-commit).

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
