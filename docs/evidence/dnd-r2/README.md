---
title: "DND-R2 evidence: DND-018 and DND-001"
date: "2026-11-08"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec + uncommitted working tree"
toolchain: "go1.27.2 darwin/arm64, GOTOOLCHAIN=local GOPROXY=off"
---

# DND-R2 evidence

Roster task DND-R2 ("dndm should be able to support lacking features and existing features should be hardened"). Working tree on top of `c7b5b61`; nothing committed. Every Go command ran with `GOTOOLCHAIN=local GOPROXY=off`. No network contact. Throwaway probes and the pinned-tree copy (`git archive c7b5b61`) live in `~/src/tmp/dndm/`.

## Failing first, then passing

| Ticket | Before (pin + new tests; both runs also include DND-001 step 0, which only adds declarations: a sentinel in `errors`, a seam and a stub in `codec`) | After |
|---|---|---|
| [DND-001](../../tickets/DND-001-bounded-stream-framing.md) | [dnd001-before.txt](dnd001-before.txt): T1 one-byte/half and 101 of 104 split offsets FAIL; T2 FAIL; T3 all 7 bodies FAIL (nil, 1 byte, 3 bytes and header-only panic; the other 3 return `ErrNotEnoughBytes`, not `ErrBadFrame`); T4 3 of 4 paths leak (`pool gets 1, puts 0`); T5 seed#1 panics `index out of range [3] with length 1` | [dnd001-after.txt](dnd001-after.txt): T1–T5 PASS with `-race`; [dnd001-fuzz.txt](dnd001-fuzz.txt): `-fuzz=FuzzReadDecode -fuzztime=60s` → 3,394,677 execs, 192 corpus entries, PASS, no crasher |
| [DND-018](../../tickets/DND-018-race-clean-core-lifecycle.md) | [dnd018-before.txt](dnd018-before.txt): `TestBase_CloseWaitsForCallbacks`, `…RegistrationOrder`, `…PendingCallbacksStartNoGoroutines`, `TestLinker_CloseIsSynchronous` FAIL; `TestLocalIntent_CloseConcurrentWithLinkAndNotify` **panics `send on closed channel`** at `intent.go:134` even without `-race`; `TestRouters_ConstructWithPendingNotifications` FAIL under `-race` (`intent.go:270` vs `:320`) | [dnd018-after.txt](dnd018-after.txt): all 9 PASS, without `-race` ×1 and with `-race` ×3 |

DND-001 step 0 (behaviour-neutral, applied before the "before" run so the tests compile): pool seam `bufferPool`, `errors.ErrBadFrame`, `codec.DefaultMaxFrameSize`, and `ReadMessageLimit` as a stub that ignored `max`.

## Gates

| Gate | Command | Result | Log |
|---|---|---|---|
| Build | `go build ./...` | exit 0 | [gate-build-vet.txt](gate-build-vet.txt) |
| Vet | `go vet ./...` | exit 0; `gofmt -l .` empty | same |
| Race | `go test -race -count=3 -timeout 900s -v ./...` | root, `codec`, `network`, `network/stream` **ok**; `x/bus` **FAIL**: `TestStreamStarTopologyBidirectional` ×2, `TestStreamSensorHostWorkflow` ×1 | [gate-race-count3.txt](gate-race-count3.txt) |
| Race locations | `race-dedupe.py` on that log | 3 reports, 2 pairs, both in `TestStreamStarTopologyBidirectional`: RC5 `contextRW.Close` `stream.go:146` vs `Request` `:199` (DND-008), and RC6 `LocalIntent.Send` `intent.go:121` vs `NewInterest` close callback `interest.go:68`. **None at an RC1–RC4 location.** | [gate-race-unique.txt](gate-race-unique.txt) |
| No race, ×10 | `go test -count=10 -timeout 900s ./...` | root, `codec`, `network`, `network/stream` ok (baseline: root panicked); `x/bus` 2 × `TestStreamStarTopologyBidirectional` timeout | [gate-norace-count10.txt](gate-norace-count10.txt) |
| Wire format | Scratch probe: `codec.AppendMessageTo` with a fixed header, pinned tree vs working tree, then `DecodeMessage` | byte-identical hex; both decode `types.Foo@wire.probe fixed <nil>` | [wire-probe-base.txt](wire-probe-base.txt), [wire-probe-current.txt](wire-probe-current.txt) |
| Public API | `go doc -all` for 9 packages, pinned vs working tree | exported symbols additive only: `codec.DefaultMaxFrameSize`, `codec.ReadMessageLimit`, `errors.ErrBadFrame`. **Behaviour change:** `ReadMessage`/`DecodeMessage` errors that used to be `ErrNotEnoughBytes`, `ErrBadArgument` or a raw short-read error now wrap `ErrBadFrame` (plus `io.ErrUnexpectedEOF` for a frame cut off mid-way); no sentinel checker exists in dndm or EasyRobot (added in DND-R3 after verification) | [api-diff.txt](api-diff.txt) |
| EasyRobot | `go build -o /dev/null ./cmd/display/destination ./cmd/manipulator ./drivers/lidar ./drivers/manipulator` (EasyRobot `abb5c9c`) | exit 1, **same as before the change**: `cmd/manipulator` builds; `cmd/display/destination` and `drivers/lidar` stop at `gocv` (no `opencv4` pkg-config); `drivers/manipulator` is excluded by `//go:build sam && xiao` (TinyGo only) | [easyrobot-build.txt](easyrobot-build.txt) |

## Remaining failures (honest list)

- **`x/bus` is flaky before and after.** [xbus-flake-count20.txt](xbus-flake-count20.txt), 20 runs without `-race`: pinned tree 9 failures (StarTopology 4, SensorHost 2, ModuleRun 2, CallerServiceRoundTrip 1); working tree 7 failures (StarTopology 5, ProducerConsumer 2). All are deadline timeouts. [INFERENCE] the cause is F3/[DND-003](../../tickets/DND-003-single-owner-remote-forwarding.md): the remote interest's channel is read by both the remote forwarder and the router's `FanInInterest`, so a message can be taken by the wrong reader and dropped. Not isolated by experiment.
- **RC5** stays with [DND-008](../../tickets/DND-008-stream-and-peer-lifecycle-races.md); it did not fall out of DND-018.
- **RC6** (interest close vs in-flight linked `Send`, `interest.go:68` vs `intent.go:117/121`) is pre-existing: it is already in the baseline's `-count=3` log ([test-race-count3.txt](../../baseline-logs/test-race-count3.txt), report at line 4583). It belongs with closure in [DND-007](../../tickets/DND-007-bounded-dispatch-and-closure.md): the closing side of `msgC` is not its sender.

[DND-018](../../tickets/DND-018-race-clean-core-lifecycle.md) · [DND-001](../../tickets/DND-001-bounded-stream-framing.md) · [Baseline](../../baseline-2026-11-08.md)
