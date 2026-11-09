---
title: "Enforce bounded dispatch and explicit closure"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-007"
status: "backlog"
owner: "unassigned"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable static defect"
dependencies: ["DND-002", "DND-005", "DND-006"]
implementation_evidence: []
---

# DND-007 — Enforce bounded dispatch and explicit closure

## State and permission

Status: **backlog**. Owner: **unassigned**. Kind: **implementation / fixable static defect**.
Milestone: **M1 remote correctness**. Priority: **P0** — Silent drops, closed-channel reads and dispatcher head-of-line blocking undermine correctness.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Obtain explicit owner permission for the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-002](DND-002-local-buffer-ownership-contract.md), [DND-005](DND-005-caller-scoped-correlation.md), [DND-006](DND-006-delivery-and-overload-classes.md)

## Outcome

Runtime delivery follows the selected class policies with finite queues and deterministic terminal states.

## Scope

Check receive-ok on closure; prevent nil-message loops; make overload/readiness loss visible; isolate slow streaming waiters; release subscriber buffers and goroutines on cancellation. Implement only DND-006 selected policies.

## Non-goals

No global unbounded goroutine workaround, durable replay or unsolicited telemetry platform.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md), sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [interest.go](../../interest.go) lines 169–191 — Underlying receive lacks ok check.
- [interest.go](../../interest.go) lines 279–292 — Full wrapper buffers drop messages.
- [x/bus/caller.go](../../x/bus/caller.go) lines 258–289 — Waiter delivery blocks shared dispatcher.

## Unknowns and decisions

Which existing readiness consumers require coalesced state versus event delivery? Specify within the selected contract, not a silent drop.

## Proposed reproduction or decision fixture — NOT RUN

Fill one subscriber/waiter queue while another remains fast; close the upstream channel; cancel/remove the slow consumer; repeat readiness transitions at capacity. Count accepted/rejected/dropped messages, live dispatchers and ownership releases.

## Acceptance — unmet until evidence exists

- [ ] Fast consumers progress while a stalled stream follows its chosen bounded policy.
- [ ] Closed upstream channels terminate or transition once without nil-message spin.
- [ ] Each overload/close is observable as specified; queues stay within caps and ownership is released.

## Verification procedure — future, not executed

After permission, deterministic queue-saturation/closure fixtures and leak accounting compare actual transitions to DND-006. Do not claim existing drop counters already exist.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium: interest routers and caller dispatcher; no external queue dependency assumed.

## Implementation evidence

RC6 (DND-R5; the Leader's DND-R5 task listed it with the DND-008 items). Tests are in `closure_test.go`, written first:

| Test | At the pin | Working tree before the fix | After |
|---|---|---|---|
| `TestLinker_InterestCloseUnblocksSend`: a Send blocked on a full interest channel must return when the interest closes | `panic: send on closed channel`, 3/3 with and without `-race` | same, 3/3 | ok ×30 `-race`, ×100 without |
| `TestLinker_InterestCloseDuringSends`: 4 senders race an interest close, 50 rounds | panic 3/3 | panic 3/3 | ok |
| `TestLinker_ContextCancelDuringSends`: the linker context is cancelled, so the link and the interest close from two goroutines | panic + race 3/3 | — | ok ×30 `-race` |
| `TestFanInInterest_CloseWhileForwarding`: fan-in closes while its runner forwards into its own channel | data race 3/3 (`-race` only) | same | ok |

Evidence: [before](../evidence/dnd-r5/rc6-before.txt), [after](../evidence/dnd-r5/rc6-after.txt).

Fix (the closer of a channel now knows its senders have stopped):

- `LocalIntent.Send` holds the intent's read lock for the whole delivery. Each link has a `done` signal; `Link(nil)` closes it first, which releases a Send waiting on a full channel (that Send returns `ErrNoInterest`), then takes the write lock. So once `Link(nil)` returns, no Send uses the old channel.
- `LocalInterest` closes `msgC` last: its close callback queues the `close(msgC)` behind every later callback, including the linker's `RemoveInterest`, which unlinks the intent.
- `Link.Close` returns only after the intent is unlinked, also when another goroutine is already closing the link (context cancellation closes the link and the interest concurrently). An ablation without this wait fails `TestLinker_ContextCancelDuringSends` 9/10.
- `FanInInterest` waits for its receive runners inside the inner interest's close callbacks, before the channel closes.

Behaviour changes:

- A Send waiting on a full channel whose link is removed now returns `ErrNoInterest`. Before, it panicked if the interest closed, or stayed blocked otherwise.
- `Link(nil)` and `Link.Close` now wait for in-flight Sends on that link. Those Sends are released by `done`, so the wait is bounded by one select.

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R2. Runtime finding RC6 belongs here: `LocalInterest`'s close callback closes `msgC` (`interest.go:66–69` at the pin) while a linked `LocalIntent.Send` that read `linkedC` earlier can still send on it (`intent.go:112–118`). Race reports in the baseline `-count=3` log and in the DND-R2 gate log ([evidence](../evidence/dnd-r2/README.md)). The closer of `msgC` is not its sender. Status unchanged.
- 2026-11-08: DND-R4. Two readiness/closure items fixed in the working tree, each with a failing-first test (`readiness_test.go`):
  - readiness loss: an `IntentRouter` notified before its first `Wrap()` lost the notification, so a second producer waited forever. The router now remembers it was notified and seeds new wrappers (`TestIntentRouter_WrapAfterNotifyGetsInterest`; fails at the pin);
  - use after close: `removeWrapper` set `w.router = nil`, which raced `Send`/`Route` and made `Send` on a closed wrapper panic. The router is kept, and `Send` on a closed wrapper returns `ErrClosed` (`TestRouterWrappers_UseAfterClose`; fails at the pin and panics in the DND-R3 tree).
  RC6 (interest close vs in-flight Send) is still open; it did not appear in the DND-R4 gate runs. Bounded dispatch and the other closure items wait for the DND-006 design. Status unchanged.
- 2026-11-08: DND-R5. RC6 fixed failing-test-first (see Implementation evidence), together with the same close-vs-send race in `FanInInterest`. Bounded dispatch, nil-message loops and visible overload still wait for the DND-006 design. Status unchanged (backlog): RC6 was one item, not the ticket's outcome.
- 2026-11-09: DND-R5-fix (review F3). A closed router wrapper returns the core `errors.ErrClosed`; x/bus now reports it as `bus.ErrClosed`, which wraps the core sentinel, so `errors.Is` matches either one. The forwarder test helper again checks one sentinel. `TestProducerSendOnClosedRouteIsBusErrClosed` failed before the fix and passed ×30 with `-race` after. Review follow-ups recorded here: F7 (a Close that always waits for callbacks, a `done` channel in Base's lifecycle) and F16 (`Container.reconcile` holds its lock while writing to a peer).

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
