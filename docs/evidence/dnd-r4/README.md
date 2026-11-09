---
title: "DND-R4 evidence: DND-005, DND-002, DND-003 and follow-ons"
date: "2026-11-08"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec + uncommitted working tree"
toolchain: "go1.27.2 darwin/arm64, GOTOOLCHAIN=local GOPROXY=off"
---

# DND-R4 evidence

Roster task DND-R4, with the same authorisation as DND-R2/R3. Owner decisions recorded verbatim in the roster: DND-005 Accept, DND-002 Accept, DND-006/009 reshaped, DND-010 Build it, behaviour changes from DND-R2/R3 Accept. Working tree on top of `c7b5b61`; nothing committed. No network contact.

"Pinned tree + step 0" means `git archive c7b5b61` in `~/src/tmp/dndm/pin-r4` with only `Caller.Dropped()` declared (counter never incremented) and the named test files copied in.

## Failing first, then passing

| Change | Before | After |
|---|---|---|
| [DND-005](../../tickets/DND-005-caller-scoped-correlation.md) caller-scoped nonces | [dnd005-before.txt](dnd005-before.txt), pinned tree + step 0, 3 runs: shared-path, foreign-nonce, missing-nonce and nonce-uniqueness tests FAIL 3/3; late-reply test FAIL 3/3; FIFO-without-hooks passes (guard). Last sections: with only the readiness fix, the shared-path test misdelivers 10/10 (deterministic) | [dnd005-after.txt](dnd005-after.txt): all Caller tests PASS ×30 with `-race` (late-reply ×20); shared-path ×200 without `-race` |
| Readiness loss (DND-007 item) | `TestIntentRouter_WrapAfterNotifyGetsInterest` FAIL 3/3 at the pin (same log) | PASS ×30 with `-race` |
| Wrapper use after close (DND-007 item) | [wrapper-before.txt](wrapper-before.txt): FAIL at the pin; panics in the DND-R3 tree | [wrapper-after.txt](wrapper-after.txt): PASS ×10 with `-race` |
| [DND-002](../../tickets/DND-002-local-buffer-ownership-contract.md) contract | Contract only (godoc), not a fix. [dnd002-probe.txt](dnd002-probe.txt): modifying after Send is a race (`-race` report; receiver saw "v2"); aliasing test passes at the pin 2/3 (the third timed out on the readiness loss) | `TestLocalSubscribersShareTheSentMessage` PASS ×20 with `-race` ([dnd003-after.txt](dnd003-after.txt)) |
| [DND-003](../../tickets/DND-003-single-owner-remote-forwarding.md) single reader | [dnd003-before.txt](dnd003-before.txt): burst version FAIL 5/5 at the pin and 5/5 in the pre-fix tree; final paced version FAIL 3/3 at the pin ("missed tag-1") | PASS ×20 with `-race`; ×60 without |
| `Link.Link` synchronous (DND-018 follow-on) | `TestLink_Link` failed once in a gate run (the link goroutine had not run yet); the old goroutine could re-link after Close | `TestLink`/`TestLinker` ×30 with `-race` |

## Gates

| Gate | Command | Result | Log |
|---|---|---|---|
| Build / vet / fmt | `go build ./...`, `go vet ./...`, `gofmt -l .` | exit 0, exit 0, empty | [gate-build-vet.txt](gate-build-vet.txt) |
| Race | `go test -race -count=3 -timeout 900s -v ./...` | **all 7 packages ok, x/bus included** | [gate-race-count3.txt](gate-race-count3.txt) |
| Race locations | `race-dedupe.py` | **0 reports** | [gate-race-unique.txt](gate-race-unique.txt) |
| No race ×10 | `go test -count=10 -timeout 900s ./...` | all 7 packages ok | [gate-norace-count10.txt](gate-norace-count10.txt) |
| x/bus flake rate | `go test -count=20 ./x/bus`, without and with `-race` | 0 failures in both (pin: 9/20 without `-race`); 0 race reports | [xbus-flake-count20.txt](xbus-flake-count20.txt) |
| Wire format | fixed-header frame probe, pinned vs working tree | byte-identical | [wire-probe-base.txt](wire-probe-base.txt), [wire-probe-current.txt](wire-probe-current.txt) |
| Exported API | `go doc -all`, 10 packages, pin vs working tree | additive only. New in DND-R4: `bus.(*Caller).Dropped`, `remote.(*RemoteIntent).Peer`, `remote.(*RemoteInterest).Peer`. Earlier: `dndm.ReadyEndpoint`, `Handshaker.Ready/OnReady`, `codec.DefaultMaxFrameSize`, `codec.ReadMessageLimit`, `errors.ErrBadFrame` | [api-diff.txt](api-diff.txt) |
| EasyRobot | 4-target `go build -o /dev/null` (EasyRobot `abb5c9c`) | exit 1, same as at the pin: `cmd/manipulator` builds; opencv4 missing for two; TinyGo-only for one. EasyRobot uses no `Caller` | [easyrobot-build.txt](easyrobot-build.txt) |

The final gate logs include `TestCallerLateReplyDoesNotReachNextCall`. They predate one test-only edit: a comment moved in `caller_test.go` for gofmt. After that edit, `gofmt`, `go vet` and `go test -race -count=3 ./x/bus` were rerun and passed. The x/bus ×20 flake runs predate the late-reply test, which ran alone ×20 with `-race`.

## Behaviour changes vs the pin (new in DND-R4)

The DND-R2/R3 changes are listed in [DND-R3 evidence](../dnd-r3/README.md#behaviour-changes-vs-the-pin) (the owner accepted them, 2026-11-08).

| Area | Pin | Working tree | Ticket |
|---|---|---|---|
| `bus.Caller` with a response-nonce hook | foreign/missing-nonce replies fell back to FIFO (could complete an unrelated call); all Callers started at nonce 1 | delivered only on an exact nonce match, otherwise dropped and counted (`Dropped()`); random 64-bit start per Caller | DND-005 |
| `dndm.Router` and remote interests | built a fan-in around each remote interest (second reader, stole remote-bound messages) | ignores `dndm.RemoteInterest`; the endpoint's forwarder is the only reader | DND-003 |
| `remote.RemoteIntent/RemoteInterest` | did not implement `dndm.RemoteIntent/RemoteInterest` | implement them (`Peer()`), so `Container`'s remote checks now match | DND-003 |
| `IntentRouter.Wrap` after interest was seen | new wrapper never notified | new wrapper starts with one interest notification | DND-007 item |
| Closed router wrapper | `Send` dereferenced a cleared router (panic after sync close); `Route` raced | `Send` returns `ErrClosed`; `Route` keeps working | DND-007 item |
| `Link.Link` | linked from a goroutine that could run after `Close` | links in the caller; no-op once closed | DND-018 follow-on |
| `Router.Publish/Subscribe/addInterest` | used the new router before checking the error (nil deref when construction failed) | check the error first; `addInterest` drops the interest if the router is closing | DND-004 follow-on |

## Remaining

- **RC6** (DND-007, interest close vs in-flight Send) is not fixed. It did not appear in any DND-R4 race run, so its absence is not proof.
- **F21 (new, not fixed):** a remote link cannot carry interest in both directions for the same route; the second `Subscribe` fails with `ErrRemoteInterest`/`ErrLocalInterest`. See [DND-003](../../tickets/DND-003-single-owner-remote-forwarding.md#found-while-testing-not-fixed-f21).
- **Not designed yet:** DND-006, 009, 010 and 011 have the owner's words and design questions, and wait for ideation's research.
- Streaming correlation in `HandleReplies` has no dedicated test (DND-005 acceptance item 3).

[DND-005](../../tickets/DND-005-caller-scoped-correlation.md) · [DND-002](../../tickets/DND-002-local-buffer-ownership-contract.md) · [DND-003](../../tickets/DND-003-single-owner-remote-forwarding.md) · [DND-R3 evidence](../dnd-r3/README.md)
