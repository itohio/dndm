---
title: "Define delivery, overload and close classes"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-006"
status: "backlog"
priority: "P0"
milestone: "M0 decisions"
kind: "research / fundamental tradeoff"
dependencies: []
implementation_evidence: []
---

# DND-006 — Define delivery, overload and close classes

## State and permission

Status: **backlog**. Kind: **research / fundamental tradeoff**.
Milestone: **M0 decisions**. Priority: **P0** — Perishable media and reliable commands cannot share an implicit Send guarantee.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Requires explicit approval before the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: None (dependency-free; still not authorized to execute).

## Decision (2026-11-08)

Requirement (2026-11-08): `latest` is the default; `reliable` needs more design and is required. Some processes must keep a buffer to survive a dropped link, a missing UDP packet or a network error. Links that only work in bursts cannot send immediately, so they also need a buffer, possibly configurable. Durable delivery is required too: when a consumer asks for data a service has already processed or produced, the service should not have to produce it again. That durable history may be stored anywhere, even near the consumer, possibly in parts rather than whole; recovering parts missing from one peer's local copy may or may not require a DHT.

Status unchanged: the design waits for the research note (`docs/research/ideation/2026-11-08-ndn-delivery-auth.md`). No code until then. Design questions noted by dndm (DND-R4):

- `latest` default: per-subscriber queue of 1 with drop-oldest and a drop counter, replacing today's silent drop-newest in `InterestRouter.routeMsg`?
- `reliable`: sender-side retransmit buffer with sequence numbers and receiver acks, sized per route (messages, bytes or age)? What happens when the buffer is full: block, drop oldest, or fail Send?
- Bursty links: is store-and-forward the same buffer as `reliable`, or a separate outbound queue that holds data while the link is down?
- Durable history: named, sequence-numbered segments (NDN-style "ask by name") let a consumer request data the producer already made. Who stores which segments, what is the retention policy, and how does a consumer find a missing segment (the peer it came from, a neighbour, a DHT)?
- How does each class appear in capability negotiation (DND-013), and what must an embedded build carry (DND-015)?

## Outcome

A decision-ready class table stating admission, ordering, loss, overload, timeout, cancellation and closure semantics.

## Scope

Distinguish ephemeral telemetry, latest-frame media, request/reply, streaming and durable commands. Specify bounded queue units/caps and overflow outcomes; distinguish receipt from execution completion, fan-out from one-worker distribution, and partial fan-out failure.

## Non-goals

No durable store, queue-group implementation or promise of exactly-once external effects.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the canonical 2026-11-08 static report, sections 1–2, 4–5 and Limits; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [interest.go](../../interest.go) lines 169–191,279–292 — Close checks and wrapper drop behavior are inconsistent.
- [intent.go](../../intent.go) lines 186–220 — Fan-out is not one-worker queue distribution.
- [x/bus/caller.go](../../x/bus/caller.go) lines 258–289 — A slow waiter can block common dispatch.

## Unknowns and decisions

Choose queue capacities/deadlines and whether each class blocks, rejects or drops; values require workload evidence, not guessed defaults.

## Proposed reproduction or decision fixture — NOT RUN

Walk a fast and slow consumer through queue full, close, cancellation, disconnect and reconnect for each class. Write expected producer result, subscriber result, ordering scope and retained state for each transition.

## Decision memo (2026-11-08)

**Question.** For each kind of traffic, what does dndm do when a queue is full, a consumer is slow or a side closes, and which guarantees does it refuse outright?

**What the code does at the pin** (read from source, nothing executed). Every route gets the same unstated policy, and its parts do not agree:

| Point | Behaviour | Source |
|---|---|---|
| Publisher, `LocalIntent.Send` | Blocks until the linked channel accepts or a context ends | `intent.go:112–118` |
| `FanOutIntent.Send` | Waits for every endpoint, so the slowest one gates the send; per-endpoint errors are joined | `intent.go:203–223` |
| Router to subscriber wrappers | Non-blocking: when a buffer is full the message is dropped silently, with no counter | `interest.go:283–291` |
| Readiness notify | Dropped when the channel is full | `intent.go:133–140,372–380` |
| Fan-in receive | No `ok` check on a closed channel | `interest.go:177` |
| Remote ingress | One read loop per connection; each message's `Send` (bounded by `t.timeout`) runs before the next read | `endpoint/remote/messages.go:30–52,327–329` |
| Remote egress | A write error is logged and the message dropped | `endpoint/remote/wrappers.go:220–223` |
| Queue size | One global message count, default 1, for every channel | `options.go:25,70–79`; `dndm.go:62–63,119–121` |
| Reply waiters | Channel holds 1; the blocking send stalls the shared dispatcher | `x/bus/caller.go:259–266,282` |
| Per-route policy | None: `PubOptStruct`/`SubOptStruct` are empty | `options.go:96–104` |

[INFERENCE, from tracing the call chain] Locally, the end-to-end result is already roughly "drop the newest at the subscriber". One slow local route on a connection also stalls every other route on it (head-of-line blocking).

**Option A: two in-session classes; durable refused.**
- `latest` (telemetry, sensor frames): a bounded queue per subscriber that drops the oldest message, never blocks the producer, and counts drops.
- `reliable` (live-session commands, request/reply): bounded; blocks up to a deadline, then returns an explicit overload error; returns `ErrClosed` after close.
- Durable commands, worker-queue distribution and exactly-once: refused and left to NATS ([DND-012](DND-012-durability-and-command-boundary.md)).
- Code: class and capacity options in `options.go`; `interest.go` `routeMsg`/`recvRunner`; `intent.go` `FanOutIntent` (`latest` must not wait for a slow endpoint); per-route dispatch in `endpoint/remote/messages.go`; waiter isolation in `x/bus/caller.go`; counters. Cost: medium (lands in DND-007).
- Risk: the choice of default class.
- Compatibility: options are additive and the wire format is unchanged. The class is local to each side; advertising it is DND-013's job.

**Option B: one explicit class everywhere.** Bounded, block with a deadline, counters.
- Code: the same files, less of it. Cost: small to medium.
- Risk: perishable data blocks producers behind slow subscribers. Choosing "drop" for everything instead loses commands.
- Compatibility: every subscriber changes from silent drop to blocking.

**Option C: the full matrix with acks and credits.** Telemetry, latest-frame, request/reply, streaming and durable command, with receiver acks and credit flow control on the wire.
- Code: A, plus new header types/fields (`proto/types/core/header.proto:6–31`), per-peer state in `endpoint/remote/`, and a persistence layer for durable. Cost: large.
- Risk: re-implements broker semantics that NATS JetStream already provides.
- Compatibility: wire change. New proto3 fields are additive, but both peers must implement the same semantics.

| | A two classes | B one class | C full matrix |
|---|---|---|---|
| Fits telemetry and commands | yes | one of them | yes |
| Memory bound | per subscriber | per subscriber | per peer + store |
| Wire change | none | none | yes |
| Public API | additive options | none | additive + new types |
| Overlap with NATS | none (durable refused) | none | high |
| Size | medium | small–medium | large |

**Recommendation: A, with `latest` as the default and `reliable` opt-in per route.** A matches the two kinds of traffic actually seen: periodic sensor/state messages and occasional commands. EasyRobot sends both, e.g. state every 100 ms (`EasyRobot/drivers/manipulator/main.go:173–183`) and targets (`EasyRobot/cmd/manipulator/main.go:375–379`); those command routes would opt in to `reliable`. A `latest` default is closest to today's de facto drop at the subscriber, and it makes the drop visible. Refusing durable and worker semantics keeps dndm from turning into a second broker beside NATS. Queue caps are counted in messages and set per route from workload data, not guessed; byte caps on frames belong to DND-001.

**What it unblocks.** These tickets list DND-006 in their frontmatter `dependencies:`: [DND-007](DND-007-bounded-dispatch-and-closure.md), [DND-010](DND-010-packet-broadcast-decision.md), [DND-012](DND-012-durability-and-command-boundary.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-015](DND-015-embedded-target-feasibility.md). Transitively: DND-008 (via DND-007), DND-011 and DND-014.

**Residual unknowns.**

- Real message rates, sizes and subscriber counts, which set the caps. Nothing has been measured.
- Whether any caller relies on today's producer blocking.
- Drop-oldest versus drop-newest for `latest` when payloads are large.
- How per-route dispatch in the remote read loop bounds goroutines (DND-007 design).
- Streaming and duplex remain unsupported; `x/bus/caller.go:173–224` is not a protocol.

## Acceptance — unmet until evidence exists

- [ ] Every class has bounded memory and an observable overload/close outcome.
- [ ] No class claims execution success from admission alone.
- [ ] Unsupported worker distribution/duplex/durability remains explicit; queue limits are units plus configuration, not unexplained constants.

## Verification procedure — future, not executed

Static transition-table review now only as future procedure; after authorization use synthetic queues to verify the selected semantics in DND-007.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Small–medium research: one class/transition matrix; implementation depends on selected contracts.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and scope amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R2. Decision memo added (options, recommendation). Decision pending; status unchanged.
- 2026-11-09: DND-R8 note for the delivery-class design. **Until DND-022's session ids land, the v1 transport is at-least-once across a reconnect.**
  - When two nodes dial each other at the same moment, both connections can be ready briefly before the tie-break settles it ([close-reconnect.md §3.1](../design/close-reconnect.md#31-tie-break-for-simultaneous-connections-decided), DND-008).
  - A message sent in that window can arrive twice.
  - Any class that promises "at most once" or exactly-once needs DND-022's session/generation ids, or per-route deduplication by `seq`.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
