---
title: "Register routes across handshake completion"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-004"
status: "in review"
owner: "dndm"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: []
implementation_evidence: ["evidence/dnd-r3/dnd004-before.txt", "evidence/dnd-r3/dnd004-after.txt", "evidence/dnd-r3/dnd004-race-count60-summary.txt"]
---

# DND-004 — Register routes across handshake completion

## State and permission

Status: **in review** (working tree, not committed). Owner: **dndm**. Made ready and implemented under DND-R3, with the DND-R2 authorisation ("dndm should be able to support lacking features and existing features should be hardened"). No owner decision was needed: it is a local snapshot-and-reconcile with no wire or handshake-payload change. Dependencies: none.

## Outcome

Route registration has a complete, idempotent handoff across the handshake state transition.

## Defect (runtime-confirmed at the pin)

`mesh.Endpoint` adds each connection's `Handshaker` to its `dndm.Container` before the handshake completes ([conn.go](../../endpoint/mesh/conn.go) lines 22–41). `Container.Add` registers existing routes immediately ([container.go](../../container.go) lines 65–89, 111–139). The `Handshaker` rejects Publish/Subscribe with `ErrForbidden` until HS_DONE ([handshake.go](../../endpoint/mesh/handshake.go) lines 115–131), so:

1. routes that existed when the connection was added were skipped forever (warning logged only);
2. `Container.Publish`/`Subscribe` failed outright while any connection was mid-handshake (`closeAll`, `return nil, err`);
3. at the pin, `Handshaker.state` was written by `Init` after `tr.Init` had started the message handler that reads it, and read by `Publish`/`Subscribe` from other goroutines (data race).

## Fix

- `dndm.ReadyEndpoint` (new, additive): `Ready() bool` and `OnReady(func())`, which runs f once on its own goroutine after readiness.
- `Container`: `Add` of a `ReadyEndpoint` defers registration to `OnReady → reconcile`. `reconcile` takes `t.mu`, skips endpoints no longer in the container (closed or stale generations), and registers every current route. `publish`/`subscribe` skip not-ready endpoints instead of failing. Routes published after readiness go through the normal path.
- `FanOutIntent.AddIntent` ignores an intent it already holds (FanIn already did), so reconcile racing a normal publish creates no duplicate.
- `Handshaker`:
  - implements `ReadyEndpoint`;
  - `markReady` runs after FINAL is written;
  - Publish/Subscribe gate on `Ready()`;
  - the dial-side state is set before `tr.Init` starts the handler;
  - `Close` is idempotent (atomic flag; it no longer nils `conn`/`rw`, which raced the handlers) and waits for running `OnReady` callbacks.
- Found by the new tests under `-race`, fixed here:
  - **WaitGroup misuse.** `AddIntent`/`AddInterest` on a closing fan-out raced its `wg.Wait`. Both now return `ErrClosed` once the context is cancelled, and the close path orders the wait through the mutex.
  - **Map race.** `Addrbook.AddConn` inserted into `b.peers` without `b.mu` while `monitor` iterated it. This is the DND-008 pinned item `addrbook.go` 218–229; only AddConn is fixed, the other DND-008 items remain.

## Non-goals

No new discovery protocol, durable identity or handshake payload change. The handshake still sends empty `Intents`/`Interests` lists (FIXME at the pin); reconcile makes them unnecessary for correctness.

## Tests (failing first, kept)

- `endpoint/mesh/handshake_test.go` (the package had no tests):
  - `TestContainerRegistersRoutesAcrossHandshakeCompletion`: routes existing before Add, published mid-handshake and removed mid-handshake. The handshake is held by a gated reader.
  - `TestContainerRegistersRoutesOnReconnect`: generation 1 completes, is removed, a route is dropped and another added, then generation 2 completes.
  - Both assert distinct remote intents per route on the peer. Pipes are buffered like a socket: io.Pipe deadlocks the handshake, because both sides write from their read loops.
- `lifecycle_test.go` `TestRouters_AddAfterCloseRefused`: adding to a closed IntentRouter/InterestRouter returns `ErrClosed` and starts no runner.

## Acceptance

- [x] Every route active at completion propagates once; removed routes are not resurrected. At the pin + tests: mid-handshake Publish returns `forbidden`, "existing"/"during" never reach the peer, and the reconnect test fails ([before](../evidence/dnd-r3/dnd004-before.txt), last two sections use the final harness). After: both pass ×10 with `-race` ([after](../evidence/dnd-r3/dnd004-after.txt)).
- [x] Registrations after completion keep their behaviour ("after"/"new" routes, one remote intent each).
- [x] Reconnect does not replay stale-generation registrations or duplicate live links (reconnect test; membership check in `reconcile`).
- [x] 60 `-race` runs of both mesh tests: no race report after the AddConn fix; 3 pairs before it ([summary](../evidence/dnd-r3/dnd004-race-count60-summary.txt)).
- [ ] Owner review of the behaviour changes listed in the [DND-R3 evidence](../evidence/dnd-r3/README.md#behaviour-changes-vs-the-pin).

## Pinned evidence

Revision `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`: [container.go](../../container.go) lines 65–89, 111–139; [handshake.go](../../endpoint/mesh/handshake.go) lines 115–131, 166–177, 203–224; [conn.go](../../endpoint/mesh/conn.go) lines 22–41; [addrbook.go](../../endpoint/mesh/addrbook.go) lines 218–237.

## Implementation evidence

See [DND-R3 evidence](../evidence/dnd-r3/README.md). Separate pre-existing defect seen while testing (not fixed): the remote endpoint reports each new remote intent to its `addIntent` callback twice, once from the linker callback and once from `handleRemoteIntent` ([messages.go](../../endpoint/remote/messages.go) line 231).

## Dated log

- 2026-11-08: created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed.
- 2026-11-08: DND-R3. Made ready (no owner decision needed), reproduced at the pin, implemented; status → **in review**. Not committed.
- 2026-11-09: DND-R5-fix (review F2). After `Close` has begun, `Handshaker.OnReady`/`markReady` start no callbacks: a `closing` flag is set under `readyMu`, and the `WaitGroup.Add` happens under the same lock. Before this, a late `OnReady` or a `markReady` racing `Close` started a callback after `Close` began waiting. Tests `TestHandshaker_OnReadyAfterCloseStartsNothing`/`MarkReadyAfterCloseStartsNothing` (`endpoint/mesh/ready_close_test.go`) failed 3/3 before the fix and passed ×30 with `-race` after ([evidence](../evidence/DND-R5-fix.md)).

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
