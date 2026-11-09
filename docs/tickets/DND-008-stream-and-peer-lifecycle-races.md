---
title: "Serialize stream and peer lifecycle state"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-008"
status: "in progress"
owner: "dndm"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: ["DND-001", "DND-004", "DND-007"]
implementation_evidence: ["evidence/dnd-r5/stream-before.txt", "evidence/dnd-r5/stream-after.txt", "evidence/dnd-r5/addrbook-before.txt", "evidence/dnd-r5/addrbook-after.txt", "evidence/dnd-r5/dial-before.txt", "evidence/dnd-r5/dial-after.txt"]
---

# DND-008 — Serialize stream and peer lifecycle state

## State and permission

Status: **in progress**. Every acceptance item below is met, verified, reviewed and committed locally: fff99f7, c93ba0c, 7d088f7, 1e38ecc, d9a1793. It stays open only for three carried parts: R1 (alias address-book entries are redialled every monitor pass, see the log), F9 (document that cancelling a read mid-frame leaves the stream unusable), and the two v2 close/reconnect tests and session/generation ids, which [DND-022](DND-022-link-context-core.md) carries (until then, delivery across a reconnect is at-least-once). Owner: **dndm**. Kind: **implementation / fixable defect, runtime-confirmed**.
Milestone: **M1 remote correctness**. Priority: **P0** — Known unlocked map/state access and unsynchronised writes can invalidate all remote-path results.

Authority: the Leader's DND-R5 task ("Then the remaining DND-008 items (RC6 interest close vs in-flight Send, etc.), failing test first"), under the DND-R2 authorisation. RC6 itself is recorded under [DND-007](DND-007-bounded-dispatch-and-closure.md).

Dependencies: [DND-001](DND-001-bounded-stream-framing.md), [DND-004](DND-004-handshake-registration-replay.md), [DND-007](DND-007-bounded-dispatch-and-closure.md)

## Outcome

Concurrent writes, address-book mutation and shutdown/dial lifecycle have explicit synchronization and cancellation ownership.

## Scope

Serialize frame writes in the plain stream path; protect AddConn map mutation and dialing reads; honor network dial/context options; coordinate cancel/close/reconnect without deadlocks.

## Non-goals

No mesh consensus, partition-tolerance guarantee or universal race-free claim beyond exercised paths.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md), sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [network/stream/stream.go](../../network/stream/stream.go) lines 361–370 — Plain stream write lacks serialized context path.
- [network/stream/stream.go](../../network/stream/stream.go) lines 110–121,145–165 — Existing serialized context-write pattern.
- [endpoint/mesh/addrbook.go](../../endpoint/mesh/addrbook.go) lines 218–229 — AddConn map insertion occurs outside mutex.
- [endpoint/mesh/addrbook.go](../../endpoint/mesh/addrbook.go) lines 322–326 — IsActive reads dialing outside its lock.
- [network/net/net.go](../../network/net/net.go) lines 31–82 — Network dialing context/options gap recorded by source report.

## Unknowns and decisions

Choose lock ordering and close ownership; current deadlock frequency and effect of ignored options are unmeasured.

## Proposed reproduction or decision fixture — NOT RUN

Barrier two writers mid-frame; concurrently AddConn/traverse/IsActive; cancel a blocked dial and race close against reconnect. Feed resulting bytes to the bounded decoder and inspect connection generations.

## Acceptance

- [x] Concurrent writes remain decodable noninterleaved frames: `TestStream_ConcurrentWritesStayFramed` (4 writers × 25 frames through a writer that accepts 3 bytes at a time).
- [x] Shared address-book/dial state is synchronized (`TestAddrbookEntry_IsActiveWhileDialing`), and cancellation interrupts admitted dial work for TCP (`TestNode_DialHonoursContext`). `DialOpt`/`DialOptStruct` are empty, so there are no options to honour yet. The serial dialer is not changed: it opens a local port, and it needs hardware to test.
- [x] Close/reconnect tests terminate within configured deadlines and do not resurrect stale connections (DND-R8, design [close-reconnect.md](../design/close-reconnect.md)). Four of the design's six tests apply to today's v1 mesh, in `endpoint/mesh/reconnect_test.go`:
  - `TestAddrbookEntry_LiveConnNotLostBehindDeadOne` (R2) and `TestMesh_SimultaneousDialKeepsOneConnection` (R1/R3, both address orders) failed 3/3 before the fix; at HEAD the message arrived twice and each side kept two connections.
  - `TestMesh_ReconnectQueuesRedial` and `TestMesh_CloseDuringHandshakeReturns` pin today's behaviour and passed before and after.
  - The two v2 tests (late Close and late frame from an old session) need the link codec and moved to [DND-022](DND-022-link-context-core.md).
  - Fix: `AddrbookEntry` keeps a connection generation (compare-and-clear on close); a dead current connection is always replaced; simultaneous connections keep the one dialled by the lower address (Leader decision 4); the Handshaker closes itself when its connection loses. All ×30 with `-race` ([before](../evidence/dnd-r8/reconnect-before.txt), [after](../evidence/dnd-r8/reconnect-after.txt)).
- [x] `contextRW` never touches a caller's buffer after `Request` returned (`TestStreamContext_CancelledReadKeepsBytes`, `TestStreamContext_CancelledWriteKeepsItsBuffer`). This item was added from the DND-R3 static note.

## Verification procedure — future, not executed

After permission, execute deterministic fixtures plus the Go race detector on the changed paths; record exact toolchain, fixture counts, deadlines and failures. Passing finite runs is not proof of all schedules.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium: stream/network/address-book lifecycle; assumes bounded scope rather than a full mesh rewrite.

## Implementation evidence

All tests were written first. Each failed at the pin and in the working tree before its fix, then passed ([stream before](../evidence/dnd-r5/stream-before.txt) / [after](../evidence/dnd-r5/stream-after.txt), [addrbook before](../evidence/dnd-r5/addrbook-before.txt) / [after](../evidence/dnd-r5/addrbook-after.txt), [dial before](../evidence/dnd-r5/dial-before.txt) / [after](../evidence/dnd-r5/dial-after.txt)).

| Defect | Test (file) | Before (pin and tree) | Fix |
|---|---|---|---|
| Plain `Stream.Write` interleaves frames when the writer splits a call | `TestStream_ConcurrentWritesStayFramed` (`network/stream/ownership_test.go`) | 3/3 with and without `-race`: "frames interleaved after 0–4 intact frames" | `Stream.writeMu` around `rw.Write` |
| `contextRW` read: a Read that gave up on its context still had its buffer filled later, and those bytes were lost to the next Read | `TestStreamContext_CancelledReadKeepsBytes` | 3/3: "bytes read for an abandoned Read were lost" | The read loop reads into its own buffer; the caller copies under a hand-over; bytes nobody took are kept and served to the next Read |
| `contextRW` write: a cancelled `Write` released its frame buffer to the pool while the loop was still writing it; the frame went out with another message's bytes. Its error was also masked as `ErrNotEnoughBytes` | `TestStreamContext_CancelledWriteKeepsItsBuffer` | 3/3 without `-race`: frame read back as `"REUSED!!"`, error not `context.Canceled`. With `-race`, the error check fails 3/3; the reuse shows 1/3 (the race build's `sync.Pool` drops items) | `Request` reports whether the loop may still use the buffer; `Write` releases it only if not. `ErrNotEnoughBytes` only when there is no error |
| `AddrbookEntry.IsActive` read `dialing` outside the lock, and `Dial` cleared it after unlocking (deferred in the wrong order); the retry goroutine read `Failed`/`Backoff` unlocked | `TestAddrbookEntry_IsActiveWhileDialing` (`endpoint/mesh/addrbook_test.go`) | data race 3/3 with `-race` (passes without) | `dialing` set and cleared under the lock; retry log values captured under the lock |
| `net.Node.Dial` ignored its context | `TestNode_DialHonoursContext` (`network/net/net_test.go`, loopback listener) | 3/3: "Dial with a cancelled context connected anyway" | `net.Dialer.DialContext` |

Behaviour changes:

- A cancelled `StreamContext.Write` returns the context error (it used to return `ErrNotEnoughBytes`). Plain `Stream.Write` likewise returns the writer's error.
- Bytes read for a cancelled `StreamContext` read are delivered to the next read instead of being dropped.
- TCP dials stop when their context ends.

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R2. RC5 (`contextRW.Close` closes `request` while `Request` sends; `stream.go:145` vs `:198` at the pin) is still reported after DND-018 (working tree `:146`/`:199`; [evidence](../evidence/dnd-r2/README.md)). DND-018 made `Stream.Close` idempotent and close the reader/writer before callbacks; the `contextRW` request channel is untouched. Dependency DND-001 is in review. Status unchanged.
- 2026-11-08: DND-R3. Two DND-008 items fixed as side effects, each forced by a new failing test: RC5 (`contextRW.Close` no longer closes `request`; see [DND-019](DND-019-remote-endpoint-close-self-deadlock.md)) and `Addrbook.AddConn` map insertion now under `b.mu` (see [DND-004](DND-004-handshake-registration-replay.md)). Still open: plain `Stream.Write` serialisation (`stream.go` 361–370 at the pin), `IsActive` reading `dialing` outside its lock, ignored dial context/options, reconnect coordination, and (static reading, not run) `contextRW.Run` may still be filling a caller's buffer after `Request` returned on cancellation. Status unchanged.
- 2026-11-08: DND-R5. Owner → dndm, status → **in review** with one item open (close/reconnect). Fixed failing-test-first: `Stream.Write` serialisation, `contextRW` buffer ownership (read and write, plus write error identity), `AddrbookEntry` dialing flag, `net.Node.Dial` context.
- 2026-11-09: DND-R7-design. Close/reconnect design written ([design](../design/close-reconnect.md)): defects R1–R5 read from the code, not yet reproduced. No code.
- 2026-11-09: DND-R5-fix (review F1). The `StreamContext` read loop now uses a fixed 1 KiB scratch buffer (`readScratchSize`). Before, it kept a buffer as large as the largest frame body, up to 4 MiB per stream, because `codec.ReadMessage` asks for whole bodies. `TestStreamContext_ReadScratchStaysSmall` (256 KiB frame): before, cap 262 183 B, 3/3 fail; after, passes ×30 with `-race` ([evidence](../evidence/DND-R5-fix.md)). Review F4 (two channel allocations per `Request`) and F9 (document that cancelling mid-frame leaves the stream unusable) remain follow-ups.
- 2026-11-09: DND-R8. Review F4: large reads borrow a pooled buffer for one request instead of splitting into 1 KiB requests; the write side allocates no ack channel. A 64 KiB frame now costs 10 allocations like a 64 B one (was 138; the pin had 8). `TestStreamContext_ReadAllocsDoNotGrowWithFrameSize` and `TestStreamContext_CancelledLargeReadKeepsBytes` ([before](../evidence/dnd-r8/f4-before.txt), [after](../evidence/dnd-r8/f4-after.txt)). Close/reconnect implemented (acceptance item 3). API: `AddrbookEntry.SetConn(c, outbound, self)` (signature change); `Addrbook.DelConn` now only forgets the given connection. F9 (mid-frame cancel doc) still open.
- 2026-11-09: DND-R8-fix (review R-DND-R8 N2). The tie-break compared address strings, so nodes that know each other by other names (`localhost` vs `127.0.0.1`) kept two connections each. It now compares the declared identities (`Peer.String()` from the handshake's `Me`) and is applied when the handshake completes: the acceptor on INITIAL, the dialer on FINAL. A dial is only noted before then (`Addrbook.NoteDial`). `SetConn` is now `(c, outbound, self, remote)`. The new subtest "dial targets differ from declared identities" failed 3/3 before (2 connections per side) and passes after. Deviation from the design recorded in [close-reconnect.md §3.1](../design/close-reconnect.md#31-tie-break-for-simultaneous-connections-decided): for a moment both connections can be ready.
- 2026-11-09: open item from review R-DND-R8 (recheck), **R1: alias entries keep redialling.**
  - The case: a node is dialled under a name that differs from the identity it declares, e.g. `localhost:<p>` vs `127.0.0.1:<p>`. The address book then keeps a separate outbound entry per dial target.
  - The winning connection lives under the declared entry, so the target entry goes inactive. Every monitor pass queues it again; the tie-break refuses the dial; a successful TCP dial resets `Failed`. So it repeats every monitor period.
  - An entry for a node's own alias can even dial itself.
  - Fix to make, failing test first (review's probe: the alias subtest, then dump the entries and call `monitor()`):
    - when the handshake reveals a declared identity that differs from the dial target, bind or merge the target entry with the declared entry, so it counts as active while that entry is;
    - refuse to dial an entry whose declared identity is this node itself.
  - Not fixed in DND-R8.
- 2026-11-09: status → **in progress**. Verify accepted the DND-R5 and DND-R8 work and it is committed locally:
  - fff99f7 `fix(net): Dial honours its context`;
  - c93ba0c `fix(mesh): addrbook AddConn under lock; dialing flag and retry log without races`;
  - 7d088f7 `fix(stream): contextRW owns buffers and never closes its request channel; serialised frame writes; idempotent Close`;
  - 1e38ecc `fix(stream): large reads borrow a pooled buffer for one request; no ack channel on writes` (F4);
  - d9a1793 `fix(mesh): connection generations, refuse closed conns, identity tie-break for simultaneous dials`.
- 2026-11-09: still open on DND-008, and the only reason it is not done:
  - R1 (above);
  - F9 (mid-frame cancel doc);
  - the v2 close/reconnect tests and session ids, carried by DND-022.
- 2026-11-09: two more open items, found by the DND-022-tie gate under host load (load average ~55; [evidence](../evidence/dnd-r10/mesh-gate-finding.txt)). Both are in committed code that this round did not touch. Not fixed yet; each needs a test that fails first.
  - **G1, flaky alias subtest.** `TestMesh_SimultaneousDialKeepsOneConnection/dial_targets_differ_from_declared_identities` failed 18/120 in 12 parallel processes: `intent.Send` returned `no interest`.
    - [INFERENCE] The test waits for the first interest notification and then sends. During the documented window both connections are ready, and the notification can come from the connection that loses the tie-break. When that connection closes, its remote interest goes, and the survivor's interest may not be linked yet.
    - Fix: the test waits until the surviving connection's interest is linked, or retries `Send` until it delivers once. Product behaviour stays the documented residual (at-least-once until DND-022).
  - **G2, data race.** `Handshaker.Name()` (`handshake.go:176`) reads `h.remotePeer` in the Container's close callback, while `handshakeMsg` writes it (`:296`). Seen 1/120.
    - Both lines date from the pin; the alias case (close during the handshake) makes them meet.
    - Fix: guard `remotePeer` (a mutex or `atomic.Pointer`).
- 2026-11-09: DND-008-G1G2, committed as fe26a5f (G2) and 62f9b14 (G1), docs b485ece; awaiting verify and review rechecks ([report](../evidence/DND-008-G1G2.md)).
  - **G2 fixed.** `Handshaker.remotePeer` is guarded by `peerMu`, and every read goes through `h.peer()`.
    - `TestHandshaker_NameDuringHandshake` reads `Name()` on both handshakers during the handshake. Before: a race was detected in 8 of 10 fresh processes. After: 0 of 100 runs.
  - **G1 fixed in the test.** The alias subtest now retries `Send` while it returns `ErrNoInterest` (nothing was sent, so no duplicate), then checks the message arrives once.
    - Before: 16/120 (and 11/120 in a second sample). After: 0/120.
    - A mutant whose container never registers routes still fails it, with "B's interest never reached A over the surviving connection".
    - The product behaviour behind it is the documented residual: during the simultaneous-dial window, `Send` can briefly see no interest. DND-022's session ids remove that window.
  - The whole `endpoint/mesh` package, 12 processes × 10 runs: 0 failures, 0 races.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
