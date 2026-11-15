---
title: "Race-clean core lifecycle"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-018"
status: "done"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: []
---

# DND-018 — Race-clean core lifecycle

## State and permission

Status: **done** (DND-R2 and DND-R4; verified, reviewed, committed locally as 1f85174 and 7d088f7). Authority: the 2026-11-08 authorisation (task DND-R2) to add missing features and harden existing ones, which covers source and test changes in the working tree; commits on approval.

Dependencies: none. Opened from the [baseline](../baseline-2026-11-08.md#race-reports) races RC1–RC4 and the decision recorded in the [ticket index](index.md#baseline-gate).

## Outcome

Close callbacks have an owner and are waited for; the core intent/interest/linker lifecycle produces no race-detector reports.

## Scope

| Race | Defect at the pin | Fix |
|---|---|---|
| RC4 | `Base.AddOnClose` started one goroutine per callback, never waited (`endpoint.go:78–86`) | `Base` owns a callback list (`lifecycle`, shared by copies of the value). `Close`/`CloseCause` claims it, cancels, then runs callbacks in registration order in the caller's goroutine and returns after them. Parent-context cancellation runs them once via `context.AfterFunc`. A `Close` that finds the Base already closing returns without waiting, so a callback may close its own Base. A callback added after the Base has fully closed runs in its own goroutine (inline could deadlock a caller holding a lock it needs). |
| RC1 | `LocalIntent` close callback wrote `linkedC` and closed `notifyC` without the lock; `Notify` could send on the closed channel (`intent.go:71–74,123–140`) | A `closed` flag under `mu`; the callback sets it, clears `linkedC` and closes `notifyC` under the lock; `Link` and `Notify` are no-ops once closed. |
| RC2 | `NewIntentRouter`/`NewInterestRouter` assigned `onNotify`/`onRecv` after `NewFanOut*`/`NewFanIn*` had started the runners that read them (`intent.go:315–326`, `interest.go:225–237`) | Internal `newFanOutIntent`/`newFanInInterest` build without children; routers set the callback, then add children. |
| RC3 | `Linker.Close` mutated `intents`/`interests`/`errarr` in an async callback without `t.mu`, and returned `errors.Join(errarr)` before it ran (`linker.go:84–106`) | Swap the maps under `t.mu`, then close links, intents and interests outside the lock (their callbacks call `RemoveIntent`/`RemoveInterest`), then the Base; return the joined errors. |

Follow-on changes the synchronous callbacks required (each would deadlock or panic otherwise):

- `IntentRouter.removeWrapper`, `InterestRouter.removeWrapper` and `FanOutIntent.RemoveIntent` close outside their mutex: closing waits for runners that take that mutex.
- `Container.Close` snapshots and clears `endpoints` under `t.mu`, then closes them: endpoint close callbacks call `Container.Remove`, which used `slices.Delete` on the slice being ranged.
- `stream.Stream.Close` is idempotent (atomic flag) and closes the reader/writer before the Base, so a reader blocked on it returns before callbacks wait for it; it no longer registers a closer callback on every call.
- `NewFanOutIntent`/`NewFanInInterest` close the half-built fan-out if adding a child fails.

## Non-goals

RC5 (`contextRW.Close` closing `request` while `Request` sends, `network/stream/stream.go:145` vs `:198`) stays with [DND-008](DND-008-stream-and-peer-lifecycle-races.md); it did not fall out of this change. Competing readers on remote interests (F3, [DND-003](DND-003-single-owner-remote-forwarding.md)), the missing receive-ok check (DND-007) and interest-close vs intent-send (`close(msgC)` while a linked `Send` is in flight) are untouched.

## Public API

Additive only. `Base` gains an unexported field; exported signatures are unchanged. Observable behaviour changes: `Close` now returns after callbacks finish (it used to return immediately); callbacks run in registration order instead of concurrently. The downstream robotics library calls none of `Base`, `Linker` or `codec` directly (checked during DND-R2).

## Tests (failing first, kept)

`lifecycle_test.go`: `TestBase_CloseWaitsForCallbacks`, `TestBase_CallbacksRunInRegistrationOrder`, `TestBase_PendingCallbacksStartNoGoroutines`, `TestBase_ParentCancelRunsCallbacks`, `TestBase_CloseInsideCallbackDoesNotDeadlock`, `TestBase_AddOnCloseAfterCloseStillRuns`, `TestLinker_CloseIsSynchronous`, `TestLocalIntent_CloseConcurrentWithLinkAndNotify`, `TestRouters_ConstructWithPendingNotifications`. The three `ParentCancel`/`CloseInside`/`AddOnCloseAfterClose` tests passed at the pin and guard against regressions of the new design.

Changed existing test: `linker_test.go` `TestLinker_AddInterestIntentSend` now waits for the removed interest's close callback after `linker.Close()`. That interest is no longer owned by the linker and closes through its parent context; the test used to return first, which caused the baseline F15 "Log in goroutine after test has completed" panic.

## Acceptance

- [x] Each new test fails at `c7b5b61` (or panics/reports a race) and passes after the fix.
- [x] `go test -race -count=3 ./...`: no race report in the root package or `x/bus` at an RC1–RC4 location.
- [x] Close callbacks have an owner (`Base.lc`) and `Close` waits for them; a pending callback holds no goroutine.
- [ ] Review and approval of the behaviour change (synchronous, ordered callbacks).

## Dated log

- 2026-11-08: created and implemented under DND-R2. Failing tests recorded at `c7b5b61` before the fix.
- 2026-11-08: DND-R3. RC5, listed as a non-goal here, was fixed under [DND-019](DND-019-remote-endpoint-close-self-deadlock.md). Adding to a closed fan-out now returns `ErrClosed` ([DND-004](DND-004-handshake-registration-replay.md)), which closes the WaitGroup Add/Wait race left in the fan-out close path.
- 2026-11-08: DND-R4. One more unowned goroutine removed: `Link.Link()` linked and notified from a goroutine that could run after `Close` and re-link a closed link's intent. It now runs in the caller and does nothing once the link is closed. `link_test.go` lost two mock `Ctx()` expectations that only described the old goroutine. `TestLinker_AddRemoveInterest`'s 10 ms deadline (which cancelled the linker on a loaded machine) is now 1 s.
- 2026-11-09: status → **done**. Accepted after independent verification and committed locally (DND-commit):
  - 1f85174 `fix(core): Base owns close callbacks; synchronous ordered Close; close-vs-send and router readiness`;
  - 7d088f7 `fix(stream): contextRW owns buffers and never closes its request channel; serialised frame writes; idempotent Close`.
  - 7e27991, the core follow-ups: F6 (`Close` stops the AfterFunc) and N1 (a link's close forgets only its own match).

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Baseline](../baseline-2026-11-08.md)
