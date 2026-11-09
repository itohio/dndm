# DND-R5-fix report (dndm, 2026-11-09)

**Result:** review's F1–F3 and verify's optional F1 are fixed, each with a test that failed first. The `TestContainer_Publish` flake is ticketed as DND-021. All gates are green. **Not committed**, as instructed.

Brief: `docs/tasks/DND-R5-fix.md`. Inputs: `docs/review/R-DND-R5.md`, `docs/verification/DND-R5.md`.

## Fixes

| Item | Change | Test (failing first) | Before | After |
|---|---|---|---|---|
| Review F1: read scratch sized to the largest frame | `network/stream/stream.go`: the read loop uses a fixed `readScratchSize` = 1 KiB buffer, and reads short. `codec.ReadMessage`/`io.ReadFull` ask again for the rest | `TestStreamContext_ReadScratchStaysSmall` (`network/stream/scratch_test.go`): a 256 KiB frame must arrive whole and leave scratch ≤ 4 KiB | 3/3 FAIL: cap 262 183 B | ok ×30 `-race` |
| Review F2: OnReady callbacks after Close | `endpoint/mesh/handshake.go`: `closing` is set under `readyMu` before `Close` waits. `OnReady`/`markReady` start nothing once it is set, and `readyWG.Add` happens under `readyMu`, so it is ordered before `Wait`. Queued callbacks are dropped on Close | `TestHandshaker_OnReadyAfterCloseStartsNothing`, `TestHandshaker_MarkReadyAfterCloseStartsNothing` (`endpoint/mesh/ready_close_test.go`) | 3/3 FAIL each: "an OnReady callback started after Close" | ok ×30 `-race` |
| Review F3: two `ErrClosed` sentinels in x/bus | `x/bus/errors.go`: `bus.ErrClosed` wraps the core `errors.ErrClosed`. `busErr` maps a core ErrClosed from `Producer.Send`/`SendDirect` to `bus.ErrClosed`, which also covers `Service.Reply` and `Caller.Call`. `x/bus/bus_test.go`'s forwarder helper again checks one sentinel (`bus.ErrClosed`); the `dndmerrors` import is gone | `TestProducerSendOnClosedRouteIsBusErrClosed` (`x/bus/errclosed_test.go`): after the router wrapper closes, Send and SendDirect errors match `bus.ErrClosed` **and** the core sentinel | FAIL: "errors.Is(err, bus.ErrClosed) must match a closed route" (Send and SendDirect) | ok ×30 `-race` |
| Verify's optional F1: a broken NonceSource freezes the Caller | `x/bus/caller.go`: `registerWaiter` calls `Next` outside `c.mu`, with at most 8 tries (`nonceTries`), skipping 0 and live nonces. Then `Call`/`HandleReplies` return the new `bus.ErrNonceSource`. The contract is documented on `NonceSource` | `TestCallerRejectsBrokenNonceSource` (`x/bus/nonce_source_test.go`): `zero` (always 0) and `repeated` (always 7, two calls) | froze: "Caller froze on a nonce source that returns 0", then the binary timed out at 2 min (the deferred `Close` blocked on the held lock) | ok ×30 `-race`; the first call in `repeated` times out normally and the second gets `ErrNonceSource` |
| Verify's optional 2: `TestContainer_Publish` flake | ticket [DND-021](../tickets/DND-021-container-publish-test-flake.md): replace the sleeps with event waits; acceptance uses verify's 4-way method | — | — | backlog, no code |

The step-0 declaration of `ErrNonceSource` was made before the failing run. API change: `bus.ErrClosed` now wraps the core sentinel (an addition to what `errors.Is` matches), plus the new `bus.ErrNonceSource`. `registerWaiter` is unexported, and its signature now returns an error; `caller_test.go` was updated.

Logs: [before](dnd-r5-fix/before.txt), [after](dnd-r5-fix/after.txt).

## Gates

Offline: `GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-mod=readonly`.

| Gate | Result | Log |
|---|---|---|
| `gofmt -l .` | empty | [gate-build-vet.txt](dnd-r5-fix/gate-build-vet.txt) |
| `go build ./...`; `go vet ./...` with the default, `dndm_nonce_counter` and `dndm_nonce_random` tags | exit 0 ×4 | same |
| `go test -race -count=3 -timeout 900s -v ./...` | 8 packages ok; race-dedupe "0 reports" | [gate-race-count3.txt](dnd-r5-fix/gate-race-count3.txt), [gate-race-unique.txt](dnd-r5-fix/gate-race-unique.txt) |
| x/bus `-race -count=3` with each non-default nonce tag | ok, ok | [gate-xbus-tags.txt](dnd-r5-fix/gate-xbus-tags.txt) |
| Wire probe, pin vs tree | byte-identical | [base](dnd-r5-fix/wire-probe-base.txt), [current](dnd-r5-fix/wire-probe-current.txt) |
| EasyRobot read-only build (`-mod=readonly`), 4 targets | same as at the pin: `cmd/manipulator` 0; destination, lidar, drivers/manipulator 1 | [easyrobot-build.txt](dnd-r5-fix/easyrobot-build.txt) |

**EasyRobot tree:** `git status` there now shows ` M .gitignore`, ` M go.mod` (new requires: malgo, go-audio, cobra) and `?? docs/`. The mtimes are 23:25Z and 23:33Z, before this task's builds (23:54Z), and every dndm build here was `-mod=readonly`, which cannot write. These are another peer's changes; I did not touch EasyRobot.

dndm `go.mod`/`go.sum` are unchanged.

## Not done (review minors and nits, recorded as follow-ups)

| Finding | Recorded under |
|---|---|
| F5: per-stream frame cap; F10: TinyGo nonce panic | DND-015 |
| F7: a Close that always waits; F16: `reconcile` holds its lock while writing | DND-007 |
| F4: two channel allocations per `Request`; F9: mid-frame cancel doc | DND-008 |

Not ticketed yet: F6 (AfterFunc `stop`), F8 (`Link(c)` re-link fast path), F11–F15, F17. These are small. Recommend doing them as one "review nits" change after the commits.

## Not verified

- A TinyGo build (no toolchain).
- Review's commit-gating plan (that happens at commit time).
- The full 10+5 commit plan was not dry-run. F1–F3 are in the files the plan assigns them to (F1 → commit 6, F2 → 5, F3 → 4). The nonce fix belongs to commit 10 (`x/bus/caller.go`, `nonce.go`, new test file `x/bus/nonce_source_test.go`), and the new F3 test `x/bus/errclosed_test.go` goes with commit 4.

## Network contacts

None.

## Proposed mb learnings

- 2026-11-09 [code] [medium] A plug-in interface called under a lock (here `NonceSource.Next`) turns any misbehaving implementation into a whole-object deadlock: call it outside the lock and bound the retries (dndm DND-R5-fix).
- 2026-11-09 [code] [medium] WaitGroup with late starters: set a "closing" flag and do `Add` under the same mutex, so no `Add` can follow `Wait` (dndm DND-R5-fix).

## Next action

Wait for review's recheck of F1–F3 and verify's spot check. Then commit per review's 10+5 plan when Leader says, gating each commit as it is made.
