# DND-R8 report (dndm, 2026-11-09)

**Result:** all five items done, each code item with a test that failed first.

- Two tickets filed: DND-022 (link-context core) and DND-023 (CAN face).
- Review F4 fixed: a 64 KiB frame costs 10 allocations, not 138.
- DND-008 close/reconnect: 4 tests written, the fix made (connection generations and the lower-address tie-break).
- The review nits are applied, and DND-021 is fixed (19 failures in 32,000 parallel runs before, 0 after).
- Gates green. **Not committed.**

The design commit (684c179) came first in this turn; the full commit list went to Leader as a mesh message.

Brief: `docs/tasks/DND-R8.md`.

## 1. Tickets

- [DND-022](../tickets/DND-022-link-context-core.md), link-context wire core: backlog, owner dndm, P1. It owns A1, A3, A5 and A8 from research-2 §8.1 and the design §8 tests, including the two v2 reconnect tests. Depends on DND-006 and DND-008, and on the pending tooling decision.
- [DND-023](../tickets/DND-023-can-face.md), CAN face: blocked until DND-015 names a target. Owns A2 (failed-MAC lockout, mandatory) and the classic-CAN budget (1 B of authenticated command data).

## 2. Review F4: large-frame reads (DND-008)

Fix: requests above 1 KiB borrow a buffer from the shared pool (`go-buffer-pool`) for one request and return it once the caller has copied the bytes. A stream keeps no big buffer (F1 stays fixed), and a body is one request again. The write side no longer allocates an ack channel.

| Test (`network/stream/alloc_test.go`) | Pin | HEAD (before) | After |
|---|---|---|---|
| `TestStreamContext_ReadAllocsDoNotGrowWithFrameSize`: allocations per frame, 64 B vs 64 KiB | 8 / 8 | 10 / **138**, FAIL 3/3 | 10 / 10 |
| `TestStreamContext_CancelledLargeReadKeepsBytes` (pooled path keeps bytes for the next read and never writes the abandoned buffer) | — | (new path) | ok ×20 `-race` |

Logs: [f4-before](dnd-r8/f4-before.txt), [f4-after](dnd-r8/f4-after.txt). `network/stream` passes ×30 with `-race`.

## 3. DND-008 close/reconnect

Tests in `endpoint/mesh/reconnect_test.go`, from design §6:

| Test | HEAD (before) | After |
|---|---|---|
| `TestAddrbookEntry_LiveConnNotLostBehindDeadOne` (R2) | FAIL 3/3: live conn refused (`ErrDuplicate`), then wiped by the dead conn's late callback | ok |
| `TestMesh_SimultaneousDialKeepsOneConnection`, both address orders (R1/R3) | FAIL 3/3: "A has 2, B has 2" connections; "the message arrived 2 times" | ok |
| `TestMesh_ReconnectQueuesRedial` (pins the redial) | ok | ok |
| `TestMesh_CloseDuringHandshakeReturns` (no hang, no goroutines left) | ok | ok |
| `TestMesh_LateCloseDoesNotEndNewSession`, `TestLink_LateFrameFromOldSessionDropped` | need the v2 link codec | moved to DND-022 |

All four ×30 with `-race`; `endpoint/...` and `x/bus` ×5 with `-race`. Logs: [reconnect-before](dnd-r8/reconnect-before.txt), [reconnect-after](dnd-r8/reconnect-after.txt).

Fix:

- `AddrbookEntry`:
  - **generation** counter; the close callback does compare-and-clear, so a stale callback never wipes a newer connection;
  - a current connection that is already closed is replaced;
  - a new connection that is already closed is refused.
- **Tie-break** (Leader decision 4): keep the connection dialled by the lower address. If both were dialled by the same side, the newer one wins.
- `SetConn(c, outbound, self)`: when the new connection wins, the old one is closed outside the lock.
- `Handshaker`: if `AddConn` refuses its connection, the dialer drops it before sending INITIAL (`go h.Close()`, after Init), and the acceptor closes before FINAL, so the loser never becomes ready.
- `Addrbook.DelConn(p, c)` now forgets only `c`, not whatever is current.

Not done here: the design's address-book "connecting" state (§3 rule 3) and identity from verified keys (rule 4) are not needed by these tests. Rule 4 belongs to DND-009/022.

## 4. Review nits (one change)

| Nit | Change | Test | Before → after |
|---|---|---|---|
| F6 | `lifecycle.claim` stops the context's `AfterFunc` when an explicit Close wins | `TestBase_ExplicitCloseStopsAfterFunc` (runtime metric `/sched/goroutines-created`) | 200 goroutines per 200 Close → 0 |
| F8 | `LocalIntent.Link(c)` with the current channel returns under the read lock | `TestLocalIntent_RelinkSameChannelDoesNotBlock` | blocked behind a full-channel Send → returns |
| F11 | `CryptoNonce` is a zero-size value with no lock (`CryptoNonce{}`) | existing nonce tests | — |
| F12 | bounded nonce retries | done in DND-R5-fix | — |
| F13 | the counter case of the forgery test logs its outcome instead of asserting it lands | — | logs "forged" ×10 |
| F14 | `ReadMessageLimit(r, maxBody)`; a cap below 8 returns `ErrBadArgument` | `TestReadMessageLimitRejectsBadCap` | −1 read the frame; 0 and 7 gave ErrBadFrame → ErrBadArgument |
| F15 | `Linker.RemoveIntent` closes its match outside the lock | `TestLinker_RemoveIntentUnlinks` | the removed intent kept delivering → ErrNoInterest |
| F17 | closure tests wait for traffic (or a full fan-in channel) instead of 1 ms sleeps | the tests themselves | — |

Logs: [nits-before](dnd-r8/nits-before.txt), [nits-after](dnd-r8/nits-after.txt). The first F6 test measured allocations (11 → 10) and failed to separate the cases, so it was rewritten to count goroutines. Both versions are in the log.

## 5. DND-021 flake

Two causes:

- The mock expectation (`Interest()`/`C()`) is met only by a background runner, which a 1 ms sleep did not always let run.
- `testutil.CtxRecv` picked at random between an expired context and an already-closed channel.

Fix:

- The tests wait on channels closed by the mocks' `.Run`, with a 5 s deadline.
- `CtxRecv` prefers a ready channel. `TestCtxRecvPrefersAReadyChannel` failed 3/3 before.

| Run (race binaries, `xargs -P 16`, 2,000 runs each) | Runs per test | Failures |
|---|---|---|
| HEAD 684c179 | 32,000 | **19** (15 Publish, 4 Subscribe) |
| after | 32,000 | 0 |

Mutations in a scratch copy (fan-out stops reading `Interest()`; `publish` stops calling the endpoint): the fixed test fails both. Logs: [dnd021-parallel](dnd-r8/dnd021-parallel.txt), [dnd021-mutation](dnd-r8/dnd021-mutation.txt), [before](dnd-r8/dnd021-before.txt), [after](dnd-r8/dnd021-after.txt).

### Correction to earlier evidence

In this tool's shell, a `for …; do (…) & done; wait` loop runs only the last background job (checked: 3 jobs → 1 file; reported as a tool issue). Earlier "parallel" stress claims were therefore over-counted:

- DND-R5 README: "8 processes × 300" was really 300 runs, and "3 × 80" was 80.
- This round's first DND-021 runs ("4 × 2,500", "16 × 2,000") were single processes.

Corrected in `docs/evidence/dnd-r5/README.md` and in the DND-021 logs. The real parallel runs use `xargs -P`. Commit gating was not affected: it ran sequentially.

## Gates

Offline: `GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-mod=readonly`.

| Gate | Result | Log |
|---|---|---|
| gofmt; build; vet (default, counter, random tags) | empty; ok; ok ×3 | [gate-build-vet](dnd-r8/gate-build-vet.txt) |
| `go test -race -count=3 -v ./...` | 9 packages ok (`testutil` now has a test); 0 race reports | [gate-race-count3](dnd-r8/gate-race-count3.txt), [unique](dnd-r8/gate-race-unique.txt) |
| x/bus `-race -count=3` per nonce tag; `go test -count=10 ./...` | ok | [gate-more](dnd-r8/gate-more.txt) |
| API vs HEAD (`go doc -all`) | `ReadMessageLimit` parameter renamed; `AddrbookEntry.SetConn` signature changed; `CryptoNonce` is now a zero-size value type with a value receiver (`&CryptoNonce{}` still works) | [api-diff](dnd-r8/api-diff.txt) |
| Wire probe vs pin | byte-identical | [base](dnd-r8/wire-probe-base.txt), [current](dnd-r8/wire-probe-current.txt) |
| EasyRobot read-only build | unchanged: `cmd/manipulator` 0; the other three 1 | [easyrobot-build](dnd-r8/easyrobot-build.txt) |

EasyRobot's tree shows other peers' edits (`.gitignore`, `Makefile`, `README.md`, `go.mod`, `docs/`, `x/knowledge/`). This round did not touch EasyRobot; every build was `-mod=readonly`. dndm `go.mod`/`go.sum` are unchanged.

## Files

- Code: `endpoint.go`, `intent.go`, `linker.go`, `codec/codec.go`, `network/stream/stream.go`, `endpoint/mesh/addrbook.go`, `endpoint/mesh/handshake.go`, `x/bus/nonce.go`, `x/bus/nonce_default.go`, `testutil/testutil.go`.
- Tests:
  - new: `nits_test.go`, `network/stream/alloc_test.go`, `endpoint/mesh/reconnect_test.go`, `testutil/testutil_test.go`;
  - changed: `closure_test.go`, `container_test.go`, `codec/codec_frame_test.go`, `x/bus/nonce_test.go`.
- Docs:
  - tickets DND-022, DND-023 (new), DND-008, DND-021, index;
  - `docs/evidence/dnd-r5/README.md` (correction);
  - this report and `docs/evidence/dnd-r8/`.
- Still uncommitted from before: `docs/research/ideation/*` (held) and `docs/evidence/DND-commit.md`.

## Network contacts

None.

## Not verified

- TinyGo builds.
- The design's v2 reconnect tests (need DND-022).
- A real redial through the mesh `monitor` loop with the 10 s period: the test calls `monitor()` directly.

## Decisions for the owner

- **API changes:** `AddrbookEntry.SetConn` gains `outbound, self` parameters. Recommend accepting; nothing outside `endpoint/mesh` calls it (EasyRobot checked).
- **Still pending:** buf/protoc-gen-go as Go tools (blocks DND-022 code).

## Proposed mb learnings

- 2026-11-09 [tooling] [high] In the agent's bash tool, `for …; do (cmd) & done; wait` runs only the last job. Use `xargs -P N` (or a Go/Python runner) for parallel stress runs, and count the output files before trusting a "N processes" claim (dndm DND-R8).
- 2026-11-09 [code] [medium] A `select` over an expired context and a ready result picks at random. Wait helpers must check the result first, or tests flake exactly when the host is slow (dndm DND-R8, testutil.CtxRecv).
- 2026-11-09 [code] [medium] Count goroutines spawned with the runtime metric `/sched/goroutines-created` to test "no goroutine per call" claims; allocation counts miss it (dndm DND-R8, F6).
- 2026-11-09 [code] [medium] A copy-out reader: small requests use a fixed scratch buffer, large ones borrow a pooled buffer for one request. Memory per stream stays bounded and allocations do not grow with frame size (dndm DND-R8, F4).

## Next action

Verify and review DND-R8. Then commit per ticket when Leader says: stream F4; mesh reconnect; nits; DND-021 test fix; docs.

## DND-R8-fix (review R-DND-R8), 2026-11-09

**Result:**
- N1 and N2 are fixed; each had a test that failed first.
- N3 is applied, but I could not reproduce its failure first: 0 failures in 16,000 parallel runs before and after.
- N5 is applied. N6 needs no separate fix: the N2 change removed the `go h.Close()` path.
- All gates are green. Not committed.

Brief: Leader message m-95520be10f06; review `docs/review/R-DND-R8.md`.

| Finding | Change | Test | Before | After |
|---|---|---|---|---|
| **N1** (major) | `linker.go`: the match's close callback deletes the route entry only if it is still this match (compare-and-clear) | `linker_relink_test.go` `TestLinker_OldMatchCloseKeepsNewMatch`: an old match closes after a new intent was matched for the same route, then the interest closes during sends | FAIL 5/5 with and without `-race`: "the old match's close forgot the new match", "panic: send on closed channel" (plus a race report) | ok ×30 `-race`, ×100 without |
| **N2** | The tie-break compares declared identities (`Peer.String()` from the handshake's `Me`), not addresses, and is applied when the handshake completes: the acceptor on INITIAL, the dialer on FINAL. Before that a dial is only noted (`Addrbook.NoteDial`) and never displaces or refuses. `SetConn(c, outbound, self, remote)`; the dialer's Init no longer closes asynchronously (N6) | `TestMesh_SimultaneousDialKeepsOneConnection` subtest "dial targets differ from declared identities": each node dials the other as `localhost:<port>` while both declare `127.0.0.1:<port>` | FAIL 3/3: "A has 2, B has 2" connections | ok; whole `TestMesh_`/`TestAddrbookEntry_` set ×30 `-race` |
| **N3** | `Test_NewContainer`, `TestContainer_AddEndpoint`, `TestContainer_SubscribePublish`: no sleeps. Close is synchronous; SubscribePublish waits for the runners' `C()`/`Interest()` reads; deadlines are 5 s instead of 100 ms | the tests themselves | 0 failures in 16,000 runs (16 processes × 1,000, `xargs -P`), so I could not show the failure first. Review saw one in a full-suite run ("Publish returned closed"); [INFERENCE] the 100 ms container context expired on a slow host before Publish | 0 / 16,000; ×30 `-race` |
| **N5** (nit) | `TestBase_ExplicitCloseStopsAfterFunc` threshold `< n/4` (the bug gives n) | — | — | ok |

Not done: N4 (shrinking `pending` after an abandoned large read). It is bounded and rare; I left it, as review allowed.

**Design deviation recorded:** [close-reconnect.md §3.1](../design/close-reconnect.md#31-tie-break-for-simultaneous-connections-decided) and DND-008:
- v1 identity is the declared `Me` string;
- the decision happens at handshake completion;
- residual: both connections can be ready for a moment before the dialers' FINALs settle it.

**Ideation's CAN FD row fix** (`ctr` 0–3, 10–13 B) is in `docs/research/ideation/2026-11-08-ndn-delivery-auth-2.md`. It is ideation's edit, ready to commit with this round.

Logs: [r8fix-before](dnd-r8/r8fix-before.txt), [r8fix-after](dnd-r8/r8fix-after.txt), [n3-parallel](dnd-r8/n3-parallel.txt), [gate build/vet](dnd-r8/r8fix-gate-build-vet.txt), [gate race](dnd-r8/r8fix-gate-race-count3.txt).

**Gates** (offline, `-mod=readonly`):
- gofmt empty; build ok; vet ok with the default, counter and random tags.
- `go test -race -count=3 ./...`: 9/9 ok, 0 race reports.
- x/bus with each nonce tag: ok. `-count=10 ./...`: ok.
- Wire probe byte-identical.
- EasyRobot: same as before (`cmd/manipulator` 0, the other three 1).
- dndm `go.mod`/`go.sum` unchanged.

**API vs HEAD:** `AddrbookEntry.SetConn(c, outbound, self, remote)` (was `(c)` at HEAD); new `Addrbook.NoteDial(target, c)`.

Files: `linker.go`, `endpoint/mesh/addrbook.go`, `endpoint/mesh/handshake.go`, `container_test.go`, `nits_test.go`, `endpoint/mesh/reconnect_test.go`, `linker_relink_test.go` (new), `docs/design/close-reconnect.md`, `docs/tickets/DND-008-*.md`, this section.

Commit split: as review proposed. N1 and `linker_relink_test.go` go in commit 3 (core); N2 in commit 2 (mesh), with `docs/design/close-reconnect.md` in the docs commit; N3 in commit 4 (testutil/DND-021); ideation's row fix as its own docs commit.

Network: none.
