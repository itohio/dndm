---
title: "Remote endpoint Close self-deadlock"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-019"
status: "done"
owner: "dndm"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: []
implementation_evidence: ["evidence/dnd-r3/dnd019-before.txt", "evidence/dnd-r3/dnd019-mid-rc5.txt", "evidence/dnd-r3/dnd019-after.txt"]
---

# DND-019 — Remote endpoint Close self-deadlock

## State and permission

Status: **done** (DND-R3; verified, reviewed, committed locally as 214f490 and 7d088f7). Owner: **dndm**. Found by verify in DND-R2 ([verification](../../../docs/verification/DND-R2.md)); created and implemented under DND-R3 with the DND-R2 authorisation ("dndm should be able to support lacking features and existing features should be hardened"). Dependencies: none.

## Defect (pre-existing at the pin)

`remote.Endpoint.messageHandler` runs under `t.wg` (`defer t.wg.Done()`) and calls `t.Close()` on any read error ([messages.go](../../endpoint/remote/messages.go) line 40 at the pin). `Close` ends with `t.wg.Wait()` ([remote.go](../../endpoint/remote/remote.go) line 113), so the handler waits for itself and never exits. Any later `Close` also waits forever.

It is remotely triggerable: a peer sending 8 bytes with a bad magic number makes the read fail. An external `Close` hangs too: it cancels the context, the handler's pending read returns the cancellation error, and the handler calls `Close` and self-waits. DND-001's close-on-`ErrBadFrame` policy goes through this same path.

## Fix

- `Close` = `shutdown()` + `wg.Wait()`. `shutdown()` closes the connection, the endpoint context and the linker, and waits for nothing. Each step was already idempotent after DND-018.
- The message handler calls `shutdown()` on a read error.
- The connection's `OnClose` callback calls `shutdown()`, not `Close`: since DND-018 that callback can run inline on the handler goroutine (handler → shutdown → conn.Close → callback).
- RC5, folded in from [DND-008](DND-008-stream-and-peer-lifecycle-races.md): once `Close` really returns, closing while the handler is reading became the normal path. That path tripped RC5 (`contextRW.Close` closing `request` while `Request` sends) in 1 of 10 `-race` runs (mid-fix log (evidence: `~/src/docs/evidence/dnd-r3/dnd019-mid-rc5.txt`, outside the repo)). `contextRW.Close` now only cancels: the request channel is never closed, `Request` returns on the cancelled context, and the `recover` of "send on closed channel" plus the drain loop and `time.Sleep` are removed.

## Tests (failing first, kept)

`endpoint/remote/remote_test.go` (the package had no tests):
- `TestEndpoint_CloseAfterRemoteBadFrameReturns`: a peer writes 8 non-frame bytes; the endpoint must close and a later `Close` must return within 2 s.
- `TestEndpoint_CloseWhileReadingReturns`: an external `Close` while the handler is blocked reading must return within 2 s.

## Acceptance

- [x] Both tests fail at `c7b5b61` (pinned tree plus the test file only) and in the DND-R2 working tree: "Close hung" after 2 s (before (evidence: `~/src/docs/evidence/dnd-r3/dnd019-before.txt`, outside the repo)).
- [x] Both pass after the fix: ×1 without `-race`, ×10 with `-race`, no race report (after (evidence: `~/src/docs/evidence/dnd-r3/dnd019-after.txt`, outside the repo)).
- [x] `go test -race -count=3 ./...` reports no RC5 and no `endpoint/remote` failure (gate (evidence: `~/src/docs/evidence/dnd-r3/gate-race-unique.txt`, outside the repo)).
- [ ] Owner review. `remote.Endpoint.Close` now returns (it used to hang). Callers that relied on `Close` never returning are not expected.

## Dated log

- 2026-11-08: DND-R3. Created from verify's DND-R2 finding, reproduced at the pin, fixed. RC5 fixed along the way because the fix makes that path common.
- 2026-11-09: status → **done**. Verify accepted it and it is committed locally (DND-commit):
  - 214f490 `fix(remote): Close no longer waits on its own handler`;
  - 7d088f7, the stream part (RC5).

[Ticket index](index.md) · DND-R3 evidence (evidence: `~/src/docs/evidence/dnd-r3/README.md`, outside the repo)
