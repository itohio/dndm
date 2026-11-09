---
title: "DND-R3 evidence: DND-019, DND-004, DND-005 memo"
date: "2026-11-08"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec + uncommitted working tree"
toolchain: "go1.27.2 darwin/arm64, GOTOOLCHAIN=local GOPROXY=off"
---

# DND-R3 evidence

Roster task DND-R3, with the same authorisation as DND-R2. Working tree on top of `c7b5b61`; nothing committed. Every Go command ran with `GOTOOLCHAIN=local GOPROXY=off`. No network contact. "Pinned tree" means `git archive c7b5b61` unpacked into `~/src/tmp/dndm/base`, with only the named test file copied in. Throwaway probes ran in `_scratch` copies and were deleted.

## Verification fixes from DND-R2

- Error-identity change recorded in [DND-001](../../tickets/DND-001-bounded-stream-framing.md#implementation-evidence) and the [DND-R2 API row](../dnd-r2/README.md#gates).
- `TestStreamProducerConsumer` flake noted under [DND-003](../../tickets/DND-003-single-owner-remote-forwarding.md#dated-log).

## Failing first, then passing

| Ticket | Before | After |
|---|---|---|
| [DND-019](../../tickets/DND-019-remote-endpoint-close-self-deadlock.md) | [dnd019-before.txt](dnd019-before.txt): `TestEndpoint_CloseAfterRemoteBadFrameReturns` FAILs "Close hung" after 2 s every time, in the pinned tree and in the DND-R2 working tree; this is the deterministic failing-first test. `TestEndpoint_CloseWhileReadingReturns` failed in this log but fails only intermittently at the pin (verify: 2 of 6 runs) | [dnd019-after.txt](dnd019-after.txt): both PASS ×1, and ×10 with `-race`, no race. [dnd019-mid-rc5.txt](dnd019-mid-rc5.txt): with the DND-019 fix but the pinned `contextRW`, RC5 fails 1 of 10 `-race` runs, so RC5 was fixed too |
| [DND-004](../../tickets/DND-004-handshake-registration-replay.md) | [dnd004-before.txt](dnd004-before.txt), pinned tree: mid-handshake Publish returns `forbidden`; "existing"/"during" never reach the peer; the reconnect test fails; `TestRouters_AddAfterCloseRefused` FAILs. Sections 1–2 used a first io.Pipe harness (the handshake also deadlocked); sections 3–4 use the final harness | [dnd004-after.txt](dnd004-after.txt): both mesh tests and `TestRouters_AddAfterCloseRefused` PASS ×10 with `-race`. [dnd004-race-count60-summary.txt](dnd004-race-count60-summary.txt): 60 `-race` runs, 0 races after the `AddConn` fix (3 pairs before it) |
| [DND-005](../../tickets/DND-005-caller-scoped-correlation.md) | Not implemented in DND-R3: needed an owner decision. [dnd005-probe-pin.txt](dnd005-probe-pin.txt): one probe run per tree; corrected after verification: the outcome is bimodal per run (stale reply or timeout) at **both** trees, so the misdelivery is pre-existing, not a difference between trees. Implemented in DND-R4 | — |

## Gates

| Gate | Command | Result | Log |
|---|---|---|---|
| Build / vet / fmt | `go build ./...`, `go vet ./...`, `gofmt -l .` | exit 0, exit 0, empty | [gate-build-vet.txt](gate-build-vet.txt) |
| Race | `go test -race -count=3 -timeout 900s -v ./...` | **ok**: root, codec, endpoint/mesh, endpoint/remote, network, network/stream. **FAIL**: x/bus (StarTopology ×3, SensorHost ×1) | [gate-race-count3.txt](gate-race-count3.txt) |
| Race locations | `race-dedupe.py` | 1 report, 1 pair: RC6 `intent.go:121` vs `interest.go:68` (DND-007). RC5 gone. No report outside x/bus | [gate-race-unique.txt](gate-race-unique.txt) |
| No race ×10 | `go test -count=10 -timeout 900s ./...` | all packages ok except x/bus (ProducerConsumer ×2, StarTopology ×1) | [gate-norace-count10.txt](gate-norace-count10.txt) |
| x/bus flake rate | see [Remaining failures](#remaining-failures) | | [xbus-flake-count20.txt](xbus-flake-count20.txt) |
| Wire format | fixed-header frame probe, pinned vs working tree | byte-identical | [wire-probe-base.txt](wire-probe-base.txt), [wire-probe-current.txt](wire-probe-current.txt) |
| Exported API | `go doc -all`, 10 packages, pin vs working tree | additive only: `dndm.ReadyEndpoint` (`Ready`, `OnReady`), `mesh.(*Handshaker).Ready/OnReady`, plus DND-R2's `codec.DefaultMaxFrameSize`, `codec.ReadMessageLimit`, `errors.ErrBadFrame`. Nothing removed or changed | [api-diff.txt](api-diff.txt) |
| EasyRobot | `go build -o /dev/null ./cmd/display/destination ./cmd/manipulator ./drivers/lidar ./drivers/manipulator` (EasyRobot `abb5c9c`) | exit 1, same as at the pin: `cmd/manipulator` builds; two packages stop at gocv/opencv4; `drivers/manipulator` is TinyGo-only | [easyrobot-build.txt](easyrobot-build.txt) |

## Behaviour changes vs the pin

Exported signatures are unchanged except the additions above. Changed behaviour:

| Area | Pin | Working tree | Ticket |
|---|---|---|---|
| `codec.ReadMessage`/`DecodeMessage` errors | `ErrNotEnoughBytes`, `ErrBadArgument`, raw short-read errors | wrap `ErrBadFrame` (+ `io.ErrUnexpectedEOF` mid-frame) | DND-001 |
| `Base.Close`, `AddOnClose` | callbacks async, one goroutine each, Close returns at once | callbacks inline, in order, Close returns after them; parent cancel runs them via `context.AfterFunc` | DND-018 |
| `remote.Endpoint.Close` | hung forever (handler self-wait) | returns after handler and pinger exit; a connection close or read error shuts the endpoint down without waiting | DND-019 |
| `stream` request loop close | closed the request channel; `Request` recovered "send on closed channel" | only cancels; pending `Request` returns the context error | DND-019 (RC5) |
| `Container.Publish`/`Subscribe` with a mid-handshake connection | failed with `forbidden` | skip not-ready endpoints; routes registered on readiness | DND-004 |
| `Container.Add` of a `ReadyEndpoint` | synchronous registration (silently skipped before handshake) | registration on `OnReady`, in its own goroutine | DND-004 |
| `FanOutIntent.AddIntent` | duplicate appended (second runner); add after close accepted | duplicate ignored; after close → `ErrClosed` | DND-004 |
| `FanInInterest.AddInterest` | add after close accepted | → `ErrClosed` | DND-004 |
| `mesh.Handshaker` | Publish allowed once `state == HS_DONE` (set before the peer update); Close not idempotent | Publish allowed after `markReady` (after FINAL is written); Close idempotent and waits for OnReady callbacks | DND-004 |
| `mesh.Addrbook.AddConn` | map insert without lock | under `b.mu` | DND-004 (DND-008 item) |
| `Linker.Close` | errors appended in an async callback, so effectively returned nil; links not closed | closes links, intents and interests synchronously and returns their joined errors (rows added in DND-R4 after verification) | DND-018 |
| `stream.Stream.Close` | registered an async closer on every call, returned nil in practice | idempotent; closes the reader/writer once and returns its error | DND-018 |
| `LocalIntent.Link`/`Notify` after close | could re-link or send on the closed notify channel (panic) | no-ops once the intent is closed | DND-018 |
| `Container.Close`, `IntentRouter`/`InterestRouter` `removeWrapper`, `FanOutIntent.RemoveIntent` | closed members while holding their lock (Container: iterated without lock) | snapshot under the lock, close members outside it; callbacks see the lock free | DND-018 |

No caller in dndm or EasyRobot checks the changed error sentinels. EasyRobot uses `remote.New`, `stream.NewWithContext`/`Close`, `dndm.New`/`NewContainer` and `bus.*`; none of their signatures changed.

## Remaining failures

- **RC6** (DND-007): interest close vs an in-flight linked `Send`. Pre-existing.
- **x/bus timeouts.** 20 runs without `-race` ([xbus-flake-count20.txt](xbus-flake-count20.txt)): pin 9 failures, after DND-R2 7, after DND-R3 10 (StarTopology 9, `TestProducerConsumer` 1). In the targeted 40-run comparison ([flake-star40.txt](flake-star40.txt)), StarTopology failed 4/40 at the pin and 9/40 then 4/40 in two runs of the working tree; variants without the DND-004 fan-out changes (5/40) or without DND-019/RC5 (4/40) look like the pin. The spread between two runs of the same tree is as large as any difference, so I attribute no regression to one change. Not proven either way. [INFERENCE] causes:
  - F3 competing readers (DND-003);
  - a lost readiness notification: an `IntentRouter` that is notified before its first `Wrap()` has no wrapper to tell (DND-007; static reading, not isolated).
- **Seen, not fixed:** the remote endpoint calls its `addIntent` callback twice for each new remote intent (`messages.go:231` plus the linker callback).

[DND-019](../../tickets/DND-019-remote-endpoint-close-self-deadlock.md) · [DND-004](../../tickets/DND-004-handshake-registration-replay.md) · [DND-005](../../tickets/DND-005-caller-scoped-correlation.md) · [DND-R2 evidence](../dnd-r2/README.md)
