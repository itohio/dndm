---
title: "Bound stream framing and return pooled buffers"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-001"
status: "done"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: []
implementation_evidence: ["codec/codec_frame_test.go", "commit 5e2532a", "commit 7e27991"]
---

# DND-001 — Bound stream framing and return pooled buffers

## State and permission

Status: **done** (implemented in DND-R2, verified and reviewed, committed locally as 5e2532a; follow-up nit F14 in 7e27991). Kind: **implementation / fixable defect, runtime-confirmed** ([baseline probe](../baseline-2026-11-08.md#dnd-001-probe)). Milestone: **M1 remote correctness**. Priority: **P0**: a remote peer could make the decoder panic, reject valid fragmented frames, or force a large allocation from one 8-byte preamble. Measured: 256 MiB (probe P6). [INFERENCE] up to ~4 GiB: the size is a wire `uint32`, and `go-buffer-pool` `Get` calls `make` directly above `MaxInt32` (pool.go:63–64). Not run.

Authority: R1-04 made it ready under the 2026-11-08 instruction to update dndm so it can be improved on; DND-R2 implemented it under the 2026-11-08 authorisation to add missing features and harden existing ones, which covers working-tree source and test changes. Commits, pushes and network/hardware use require explicit approval. The documented defaults (4 MiB cap, `ErrBadFrame`, close without resync) were used as written.

Dependencies: none.

## Outcome

`codec.ReadMessage`/`codec.DecodeMessage` read complete frames from any `io.Reader`, reject out-of-range lengths before allocating or slicing, never panic on wire input, and return every pooled buffer on every exit.

## Decisions (proposed defaults; may be overridden before merge; they do not block start)

1. **Frame cap.** `codec.DefaultMaxFrameSize = 4 MiB` (gRPC's default maximum receive size, a familiar ceiling). A lower-level `ReadMessageLimit(r, max)` takes an explicit cap; `ReadMessage(r)` uses the default. The existing 2048-byte header cap stays. Minimum body is 8 bytes (4-byte header length + 4-byte message length). No workload measurement exists; DND-016 revisits the default.
2. **Malformed-frame policy.** Bad magic, length outside `[8, max]`, EOF inside a frame, or a nested length that overruns the body returns an error wrapping a new `errors.ErrBadFrame`. No resync: the format has no delimiter, so the caller closes the connection. This matches today's `endpoint/remote/messages.go:37–41`, which closes on any `Read` error. A clean EOF at a frame boundary stays `io.EOF`; EOF mid-frame becomes `io.ErrUnexpectedEOF` (`io.ReadFull` semantics) wrapped in `ErrBadFrame`.

## Files to touch

| File | Change |
|---|---|
| [codec/codec.go](../../codec/codec.go) | Lines 17–23: pool seam (below). Lines 85–110 `ReadMessage`: `io.ReadFull` for preamble and body; validate size before `buffers.Get`; `Release` on every error after `Get`. Lines 113–132 `DecodeMessage`: check `len(data) ≥ 4` before each length read, `hSize ≤ remaining` and `≤ 2048`, `mSize == remaining`; slice only after the checks. Package comment documents the policy. |
| [errors/errors.go](../../errors/errors.go) | Add `ErrBadFrame` beside `ErrNotEnoughBytes` (line 13). |
| [network/stream/stream.go](../../network/stream/stream.go) | Lines 76–90 and 314–330: no logic change expected (both already `Release` after a successful read); touch only if the cap becomes a per-stream setting. Do not change constructor signatures for this. |
| `codec/codec_frame_test.go` (new) | Tests T1–T5 below. |

Not touched: wire format, `EncodeMessage`/`AppendMessageTo` (an encoder-side cap is a separate change), `endpoint/remote` close policy, UDP/packet paths.

## Failing-test reproduction plan (write first, observe the failures, then fix)

**Step 0, seam (no behaviour change).** In `codec/codec.go` replace `var buffers pool.BufferPool` with `var buffers bufferPool = &pool.BufferPool{}` and `type bufferPool interface{ Get(int) []byte; Put([]byte) }`. The existing `buffers.Put` calls in `codec_test.go` still compile. The test helper `countingPool` wraps the real pool and records `gets`, `puts` and `maxRequested`; tests install it with `t.Cleanup` restore and must not use `t.Parallel`. Count calls rather than measure memory: `runtime.MemStats`/`AllocsPerRun` are process-wide, and `sync.Pool.Put` randomly drops 1 in 4 items under `-race` (`$GOROOT/src/sync/pool.go:104–108`), so reuse-based checks flake.

All in `codec/codec_frame_test.go`, package `codec`:

| Test | Inputs | Expected after fix | Fails today because |
|---|---|---|---|
| T1 `TestReadMessageFragmented` | Two concatenated valid frames via `iotest.OneByteReader`, `iotest.HalfReader`, and a scripted reader splitting at every offset `1..len-1` | Both frames decode once, payloads equal, second frame aligned | Single `r.Read` calls: probe P2/P3 return `not enough bytes` for a valid frame |
| T2 `TestReadMessageLengthBounds` | Declared size ∈ {0, 1, 3, 7, 8, max, max+1, 1<<31, `math.MaxUint32`} | Sizes < 8 and > max → `ErrBadFrame`; `countingPool.maxRequested ≤ max`; 8 and max accepted | Sizes 1–7 accepted and then panic in decode (P4); `buffers.Get(size)` is called uncapped (P6: 256 MiB for 2^28) |
| T3 `TestDecodeMessageTruncated` | Body nil, 1–3 bytes, header length > remaining, header length > 2048, header-only with no message length (P5), message length ≠ remaining | Error wrapping `ErrBadFrame`; no panic | P4/P5: `index out of range [3]` panics |
| T4 `TestReadMessageReleasesOnError` | EOF right after preamble, EOF mid-body, `iotest.ErrReader` mid-body, bad magic | `gets == puts` on every path | `codec.go:100–107` returns without `Put` after `Get` (static; the seam makes it observable) |
| T5 `FuzzReadDecode` | Seeds: valid frame, P4/P5 bytes, max+1 preamble | Never panics; on error `gets == puts`; `maxRequested ≤ max` | Seeds alone reproduce the P4/P5 panics |

Record T1–T4 failure output at `c7b5b61` (after step 0) in the dated log before writing the fix.

## Acceptance

- [x] T1–T5 exist in `codec/codec_frame_test.go`; output at `c7b5b61` + step 0 shows the failures. Step 0 also added `ErrBadFrame`, `DefaultMaxFrameSize` and a `ReadMessageLimit` stub ignoring `max`, so the tests compile.
- [x] After the fix: `go test -race -count=1 ./codec ./network/stream` passes (also `-race -count=3 ./codec ./network/...` ok).
- [x] `go test -run='^$' -fuzz=FuzzReadDecode -fuzztime=60s ./codec`: 3,394,677 execs, no crasher.
- [ ] `go vet ./...` clean ✔; `go test -count=1 ./...` passes for every package except `x/bus`, which stays flaky at about the pinned rate (timeouts in remote-stream tests, measured over `-count=20`). Open until x/bus is deterministic (DND-003/007/008).
- [x] `go test -race -count=3 ./...`: no newly failing package; no race report with a frame in `codec/`, and the only `stream.go` report is RC5 (now lines 146/199 because an import line was added).
- [x] Policy (cap, `ErrBadFrame`, close without resync) documented in the `codec` package comment and on `ReadMessageLimit`.
- [x] Wire format unchanged: a fixed-header frame encodes byte-identically on the pinned and working trees and decodes on both; `TestDecodeMessage` passes.

## Non-goals

No UDP decoder, wire-format redesign, authentication, encoder-side cap or performance claim. No fixing of RC1–RC5 lifecycle races.

## Pinned evidence

Revision `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Static source: the 2026-11-08 canonical static report §§1–2, 4–5. Runtime: [baseline 2026-11-08](../baseline-2026-11-08.md).

- [codec/codec.go](../../codec/codec.go) lines 85–110: single `Read` for the 8-byte preamble and for the body; size taken from the wire with no bound; no `Put` on body error/short read.
- [codec/codec.go](../../codec/codec.go) lines 113–132: `binary.BigEndian.Uint32(data)` and `data[4:]` before any length check.
- [codec/codec.go](../../codec/codec.go) lines 153–172: decoded protobuf objects are distinct remote instances (context for DND-002).
- [endpoint/remote/messages.go](../../endpoint/remote/messages.go) lines 37–41: read loop closes on error and has no `recover` ([INFERENCE] a decode panic crashes the process).

## Size

Small: roughly 40–80 changed source lines and 150–250 test lines [estimate]. Grows only if the cap must become per-stream configuration.

## Implementation evidence

- `codec/codec.go`: package comment; `bufferPool` seam; `ReadMessage` delegates to `ReadMessageLimit(r, DefaultMaxFrameSize)`; `io.ReadFull` for preamble and body; body length checked against `[8, max]` before `buffers.Get`; buffer returned on every body-read error; clean EOF before a frame stays `io.EOF`, everything else wraps `ErrBadFrame`. `DecodeMessage` checks each length field and remaining bytes before reading or slicing.
- `errors/errors.go`: `ErrBadFrame`.
- `network/stream/stream_test.go`: `TestStreamContext_Read`'s mock returned `(10, io.EOF)` for an 8-byte read without filling the buffer, which breaks the `io.Reader` contract; it now returns `(0, io.EOF)`. With `io.ReadFull` the old mock produced a zero preamble (bad magic) instead of EOF.
- Public API: exported symbols are additive only (`DefaultMaxFrameSize`, `ReadMessageLimit`, `ErrBadFrame`).
- **Behaviour change for callers that check sentinels** (DND-R2 verification fix): `ReadMessage`/`DecodeMessage` used to return `ErrNotEnoughBytes` (short read, bad nested lengths), `ErrBadArgument` (bad magic) or the reader's raw error (e.g. `io.EOF` after a partial preamble or body). They now return errors wrapping `ErrBadFrame`; a frame cut off mid-way also wraps `io.ErrUnexpectedEOF`. A clean `io.EOF` before a frame starts is unchanged. No caller in dndm or the sampled downstream robotics library tests these sentinels (grep for `ErrNotEnoughBytes`/`ErrBadArgument` with `errors.Is` finds none); `endpoint/remote/messages.go:37–41` closes on any read error either way.

## Dated log

- 2026-11-08: created local planning ticket from the final canonical static report and the 2026-11-08 scope amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance unchecked; implementation evidence empty.
- 2026-11-08: R1-04. Status backlog → **ready** under the 2026-11-08 start-improving instruction. Throwaway probe (deleted after the run) confirmed partial-read rejection, decode panics on 1–3-byte and header-only bodies, and a 256 MiB allocation from one preamble ([probe log](../baseline-logs/dnd001-probe.txt)). Proposed defaults for cap and policy. No test written, no source changed.
- 2026-11-08: DND-R2. Implemented with the documented defaults; failing tests recorded first; status ready → **in review**. Not committed.
- 2026-11-08: DND-R3. Recorded the error-identity change above (DND-R2 verification fix 1).
- 2026-11-09: status → **done**. Accepted after independent verification and committed locally: 5e2532a `fix(codec): read whole frames with a body cap; malformed input wraps ErrBadFrame` (DND-commit), and 7e27991 (finding F14: `ReadMessageLimit` rejects a cap below 8). Evidence in 31321d3.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Baseline](../baseline-2026-11-08.md)
