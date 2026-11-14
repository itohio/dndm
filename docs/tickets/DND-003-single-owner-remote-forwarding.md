---
title: "Remove competing remote-interest readers"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-003"
status: "done"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: ["DND-002"]
implementation_evidence: ["x/bus/forwarding_test.go", "commit 464a02d"]
---

# DND-003 — Remove competing remote-interest readers

## State and permission

Status: **done** (DND-R4; verified, reviewed, committed locally as 464a02d). Kind: **implementation / fixable defect, runtime-confirmed**.
Milestone: **M1 remote correctness**. Priority: **P0**: two consumers on one channel steal remote-bound messages.

No further decision was needed. The intended semantics follow from the remote design and the DND-002 contract: a remote interest exists to carry data to its remote peer, so its forwarder must be the only reader.

Dependencies: [DND-002](DND-002-local-buffer-ownership-contract.md) (decided 2026-11-08).

## Fix (DND-R4)

- `dndm.Router.addInterest` ignores interests that are `dndm.RemoteInterest`. It used to build an `InterestRouter` (fan-in) around them, a second reader of the channel the forwarder in `endpoint/remote/wrappers.go` (`wrapRemoteInterest`) already reads.
- `remote.RemoteInterest` and `remote.RemoteIntent` gain `Peer()` (the connection's remote peer), so they implement `dndm.RemoteInterest`/`dndm.RemoteIntent`. At the pin they did not, so the existing `RemoteInterest` checks in `Container` never matched.
- Single owner by construction: the only code that reads a remote interest's channel is its forwarder goroutine, which ends with the interest.

## Outcome

One owner reads each remote-bound interest channel and explicitly dispatches to intended destinations.

## Scope

Reconcile remote registration with FanInInterest construction; preserve intended local fan-out and remote forwarding; apply DND-002 ownership policy.

## Non-goals

No queue-worker feature, durable delivery, transport replacement or blanket message duplication.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [endpoint/remote/wrappers.go](../../endpoint/remote/wrappers.go) lines 197–223 — Remote forwarding consumes li.C().
- [endpoint/remote/remote.go](../../endpoint/remote/remote.go) lines 70–74 — Remote interest registration.
- [dndm.go](../../dndm.go) lines 51–65 — Registration constructs router fan-in.
- [interest.go](../../interest.go) lines 138–177 — FanInInterest consumes the same channel.

## Unknowns and decisions

Confirm intended self/remote delivery semantics in a fixture before choosing the sole dispatcher; exact symptom frequency is unmeasured.

## Proposed reproduction or decision fixture — NOT RUN

Create a synthetic local publisher, a remote interest and a separate local subscriber. Gate both current reader paths with barriers, publish tagged messages and record which consumer receives each. Repeat registration/removal while maintaining a defined active interval.

## Acceptance

- [x] One channel reader owns dispatch by construction (router skips remote interests; the forwarder is the only reader).
- [x] Each tagged message reaches the remote subscriber exactly once in the no-fault fixture. `x/bus/forwarding_test.go` `TestRemoteForwardingHasOneReader` sends 20 tagged messages, one at a time. At the pin + test it misses `tag-1` 3/3 (alternating readers); in the working tree before this fix (with DND-005 and the readiness fix) an earlier, burst-sending version missed messages 5/5. After the fix it passes ×20 with `-race` and ×60 without.
- [ ] No unrelated local consumer steals or duplicates a remote-bound message. Covered for the stealing reader; a local subscriber on the same route as a remote interest cannot be set up today because of F21 (below).

## Found while testing (not fixed, F21)

A remote endpoint's linker keeps one interest per route ID, so a link cannot carry interest in both directions for the same route. If A subscribes to route R through the link, B's remote endpoint holds a `RemoteInterest` for R; a later local `Subscribe(R)` on B's router then fails with `ErrRemoteInterest` (and symmetrically `ErrLocalInterest`). Observed in the first version of the forwarding test ("remote interest" error from `NewConsumer`, 3/3). Fixing it means keying the remote linker by route and direction. Proposed as a new ticket; it may touch the delivery design under DND-006.

## Verification procedure — future, not executed

After approval, deterministic barrier tests exercise both reader schedules and cancellation; record the failing old schedule and fixed outcome.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium: remote wrapper/router integration with deterministic fixtures; assumes no new routing API.

## Implementation evidence

`dndm.go` (`addInterest` skips `RemoteInterest`; error checks before use in `addInterest`/`Publish`/`Subscribe`), `endpoint/remote/wrappers.go` (`Peer()` on both remote wrappers plus compile-time interface checks). x/bus without `-race` ×20: 0 failures (pin 9, after DND-R3 10).

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and the 2026-11-08 scope amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R3. x/bus flake data: `TestStreamProducerConsumer` started failing after DND-R2: 2/20 runs without `-race` in the working tree, 0/20 at the pin, and once in the DND-R2 verification `-race` run. Overall x/bus timeouts: 9/20 at the pin, 7/20 after DND-R2. [INFERENCE] same competing-reader cause; DND-018's synchronous close changed timing. Track here until single-owner forwarding lands. Status unchanged.
- 2026-11-08: DND-R4. Failing test first (at the pin and in the DND-R3 tree), then fixed; status → **in review**. `TestCallerHandleReplies` also failed once in the DND-R3 verification race run (x/bus flake list). After DND-R4: x/bus ×20 without and with `-race`, 0 failures.
- 2026-11-09: status → **done**. Accepted after independent verification and committed locally: 464a02d `fix(core,remote): remote interests have one reader; remote wrappers implement RemoteIntent/RemoteInterest` (DND-commit).

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
