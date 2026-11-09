---
title: "Make TestContainer_Publish wait for events instead of sleeping"
date: "2026-11-09"
scope: "local-only planning; test-only change"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-021"
status: "backlog"
owner: "dndm"
priority: "P2"
milestone: "M1 remote correctness"
kind: "implementation / flaky test, runtime-confirmed"
dependencies: []
implementation_evidence: []
---

# DND-021 — Make TestContainer_Publish wait for events instead of sleeping

## State and permission

Status: **backlog**. Owner: **dndm**. Opened in DND-R5-fix on verify's optional item 2 ([verify DND-R5](../../../docs/verification/DND-R5.md)). It is a test-only change and touches no product code.

## Defect (pre-existing at the pin)

`container_test.go` `TestContainer_Publish` passes only if a background goroutine runs within two `time.Sleep(time.Millisecond)` calls:

- The fan-out's notify runner calls `MockIntent.Interest()` from its own goroutine (`FanOutIntent.notifyRunner`).
- The test then checks `mockIntent.AssertExpectations` and `addIntent.WaitCalled()`.

On a loaded host the goroutine may not have run yet. Verify measured, 4-way in parallel:

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

- [ ] 0 failures in 9,000 runs, 4-way in parallel with `-race` (verify's method), where the pin fails about 17.
- [ ] No `time.Sleep` left in the two tests.
- [ ] The tests still fail if `Publish` stops calling the endpoint or the fan-out stops reading `Interest()`. Check by removing the call in a scratch copy.

## Dated log

- 2026-11-09: DND-R5-fix. Opened from verify's DND-R5 optional item. Review R-DND-R5 notes the same cause. No code.

[Ticket index](index.md) · [DND-018](DND-018-race-clean-core-lifecycle.md)
