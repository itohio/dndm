---
title: "Make TestContainer_Publish wait for events instead of sleeping"
date: "2026-11-09"
scope: "local-only planning; test-only change"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-021"
status: "done"
priority: "P2"
milestone: "M1 remote correctness"
kind: "implementation / flaky test, runtime-confirmed"
dependencies: []
---

# DND-021 — Make TestContainer_Publish wait for events instead of sleeping

## State and permission

Status: **done** (DND-R8 and its fix round; verified, reviewed, committed locally as 67e47bd). Opened in DND-R5-fix from optional item 2 of the DND-R5 verification. Test-only change: `container_test.go` and the `testutil.CtxRecv` helper; no product code.

## Defect (pre-existing at the pin)

`container_test.go` `TestContainer_Publish` passes only if a background goroutine runs within two `time.Sleep(time.Millisecond)` calls:

- The fan-out's notify runner calls `MockIntent.Interest()` from its own goroutine (`FanOutIntent.notifyRunner`).
- The test then checks `mockIntent.AssertExpectations` and `addIntent.WaitCalled()`.

On a loaded host the goroutine may not have run yet. Measured 4-way in parallel:

| Tree | Failures |
|---|---|
| pin | 17 / 9,000 |
| DND-R5 tree | 13 / 9,000 |

Both trees fail the same two ways: `Interest()` never called, or `addIntent` not called. In DND-R5's gate it failed once.

## Fix

- Replace the sleeps with event waits. The `Interest` mock's `.Run` closes a channel, and the test waits on that channel with a deadline.
- Wait for `addIntent` with its existing `WaitCalled`, under a deadline longer than 100 ms. The test context is 100 ms today and is shared by everything.
- Check `TestContainer_Subscribe` for the same pattern.

## Acceptance

- [x] 0 failures in 32,000 runs, 16-way in parallel with `-race` (`xargs -P 16`, more load than the 4-way measurement). The same run before the fix failed 19 times: 15 `TestContainer_Publish`, 4 `TestContainer_Subscribe`.
- [x] No `time.Sleep` left in the two tests. They wait on channels closed by the mocks' `.Run` (Interest()/C() read by the runner), under a 5 s deadline instead of 100 ms.
- [x] The tests still fail if the fan-out stops reading `Interest()` or if `Publish` stops calling the endpoint: two mutations in a scratch copy, both fail.
- [x] Second cause fixed: `testutil.CtxRecv` picked at random between an expired context and an already-closed channel, so `WaitCalled` could fail even after the callback ran. It now prefers a ready channel (`TestCtxRecvPrefersAReadyChannel`, failed 3/3 before).

## Dated log

- 2026-11-09: DND-R5-fix. Opened from the DND-R5 verification's optional item. The R-DND-R5 review notes the same cause. No code.
- 2026-11-09: DND-R8. Fixed as above; status → **in review**. The other container tests (`Test_NewContainer`, `TestContainer_RemoveEndpoint`, `TestContainer_SubscribePublish`) still use 1 ms sleeps and 100 ms contexts. They have not been seen failing; left as they are.
- 2026-11-09: status → **done**. Accepted after independent verification and committed locally: 67e47bd `test(core): container tests wait on events, not sleeps; CtxRecv prefers a ready value`. That commit also covers finding R-DND-R8 N3, the remaining three container tests, so the note above is resolved.

[Ticket index](index.md) · [DND-018](DND-018-race-clean-core-lifecycle.md)
