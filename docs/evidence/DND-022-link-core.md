# DND-022 link core report (dndm, 2026-11-09)

**Result:** verified at the unit and race level. The new package `network/linkctx` is DND-022's link state machine, with per-session route ids, the reuse quarantine and counter low bits. Design §8's three remaining named tests pass, plus the A1, A3 and A8-cap tests. Each test first failed as a build failure (the package did not exist). A mutation run then showed that each one fails by assertion against a wrong implementation, which answers verify's DND-R9 point that a build failure alone is weak evidence. Gates pass. Not committed.

- **Task:** Leader message m-1a0382c640d6, "then continue DND-022 (route-id/quarantine/counter tests once the link state machine exists), failing-first, no commits until gates".
- **Base:** HEAD `7c52a2f` (after the DND-R9 commits).

## Files written

- `network/linkctx/link.go`:
  - `Link` and `New(face, Config, DeliverFunc)`;
  - `Start`, `Restart`, `Open`, `Bind`, `Unbind`, `Send`, `Receive`, `Routes`, `Stats`;
  - errors `ErrNotOpen`, `ErrTableFull`, `ErrNotBound`, `ErrUnsupportedProfile`, `ErrClosed`.
- `network/linkctx/ids.go`: per-session lowest-free id table with quarantine.
- `network/linkctx/counter.go`: `LowBits`, `RebuildCounter`, `CounterBitsSmall` = 7, `CounterBitsWide` = 14.
- `network/linkctx/linkctx_test.go`: six tests.
- Docs:
  - `docs/tickets/DND-022-link-context-core.md`: acceptance items 2, A1 and A3 ticked; A8 half; log;
  - `docs/design/link-context-wire.md`: §4 tie rule; §8 test locations;
  - `docs/roadmap.md`;
  - this report and `docs/evidence/dnd-r10/`.

## Behaviour

- **Opening:** a link opens when both Hellos have crossed (ONBOARD: the handshake is the Hello exchange).
  - A Hello with another version or profile → `Close(PROFILE_MISMATCH)`, and the link is closed.
  - Until the link is open, every frame except Hello/Handshake/Close is dropped and counted (`Stats.DroppedBeforeOpen`, A1).
- **Sessions:** `Restart` forgets the session (a reconnect), and `Start` begins a new one with a random session id.
  - When an open link receives a Hello with a new peer session, it resets both id tables and answers with its own new Hello.
  - ~~This cannot ping-pong: a fresh side only records the first Hello it gets.~~ **Wrong** (review R-DND-022 L1). That argument holds only while one side is fresh. Two non-fresh sides each saw a "new" session and answered with another one, forever: 1000 Hellos in 1000 rounds after `Start` on an open link, after a late duplicate Hello on a reordering face, or after one spoofed Hello on ONBOARD. The fix and the new session rules are in [R-DND-022 fixes](#r-dnd-022-fixes-l1l6-2026-11-09).
- **Route ids:** the sender allocates the **lowest free id** from 1, per session, capped at min(own `MaxRoutes`, peer's `max_routes`).
  - A refused Bind frees its id at once, since it never carried DATA.
  - An id freed by `UnbindAck` waits out `Config.Quarantine` before reuse (0 on stream faces).
- **Receiving a Bind:**
  - a live id with the same route is acknowledged again;
  - a live id with another route → `DENIED` (no retargeting, A3);
  - more than `MaxRoutes` routes → `TABLE_FULL` (A8 cap);
  - otherwise `Config.Accept` decides (the zone table hook).
- **Receiving DATA:** for an unbound id it is dropped and counted (`DroppedUnbound`).
- **Counter:** `RebuildCounter` takes the closest value with the same low bits. A gap of exactly half the window is a tie and resolves ahead; I wrote this into design §4.

**Not yet**, with the remaining DND-022 acceptance:
- secured profiles and tags (DND-009), and the counter inside DATA frames;
- Bind/Unbind retries on lossy faces;
- the Bind rate limit (A8, second half);
- enforcing `max_frame`;
- v1 negotiation and trusted-origin routes (A5);
- wiring `Link` into the remote endpoint;
- the bytes-per-message cross-check with real EasyRobot messages.

## Tests

| Test | What it proves |
|---|---|
| `TestRouteIdsStaySmallAcrossReconnects` | 50 sessions of 10 binds plus 3 unbind/rebinds each: every id ≤ 10, and every DATA frame is 4 B + payload |
| `TestRouteIdReuseQuarantine` | On a fake face that delays DATA: after Unbind, a new Bind does not reuse the id in quarantine. The late frame is dropped and counted, not delivered on the new route. After the quarantine the lowest id is reused |
| `TestCounterLowBits` | 7 bits: counters 127/128/255/256 give a classic CAN command frame of ≤ 8 B with 1 B of data. The rebuild handles the wrap, 63 ahead and 63 behind; a tie at 64 resolves ahead; 65 either way is wrong. 14 bits: ±8191 rebuilds |
| `TestLinkDropsBeforeHandshake` (A1) | DATA and Bind before the Hello exchange are dropped, counted, and not acknowledged; `Bind` on a link that is not open returns `ErrNotOpen` |
| `TestBindRetargetRefused` (A3) | A duplicate Bind gets OK again; a retargeting Bind gets `DENIED`; DATA still reaches the original route |
| `TestBindTableFull` (A8 cap) | The sender's own cap returns `ErrTableFull`. The receiver holds at most `max_routes` and answers a flooding Bind with `TABLE_FULL` (counted) |

**Before:** a build failure, since the package did not exist ([log](dnd-r10/before-build.txt)): `undefined: Link`, `Config`, `New`, `LowBits`, `RebuildCounter`, … A build failure is weak evidence that a test discriminates, so a mutation run followed.

**Mutation check** ([log](dnd-r10/mutations.txt)). Each wrong implementation was applied alone to a scratch copy of the repo, and the named test failed by assertion:

| Mutant | Test | Failure |
|---|---|---|
| M1: ids increase across sessions (verify DND-R7 F1's old rule) | RouteIdsStaySmall | `"650" is not less than or equal to "10"`; DATA frame 90 has 5 B of overhead |
| M2: no quarantine | RouteIdReuseQuarantine | the id is reused at once; the late frame is delivered as `{route.y types.Y old}` |
| M3: low bits only, no closest-value rebuild | CounterLowBits | "highest 120, sent 130", "a tie resolves ahead", … |
| M4: DATA accepted before the handshake | LinkDropsBeforeHandshake | `DroppedBeforeOpen` 1 ≠ 2 |
| M5: a live id may be retargeted | BindRetargetRefused | "a live id must not be retargeted" |
| M6: no receiver table cap | BindTableFull | 6 ≠ 4 routes |

**One test bug found while implementing:** my first `TestCounterLowBits` asserted that a gap of 64 rebuilds to neither 264 nor 136. Those two share their low bits, so one of them must come out. I changed the test to the tie rule (ahead) plus gaps of 65, and recorded the rule in design §4.

**After** ([log](dnd-r10/after.txt)): all six tests pass ×20 with `-race`; package coverage is 82.9 %.

## Gates ([build/vet](dnd-r10/gate-build-vet.txt), [race](dnd-r10/gate-race-count3.txt))

- gofmt empty; build ok; vet ok with the default, `dndm_nonce_counter` and `dndm_nonce_random` tags.
- `go test -race -count=3 ./...`: 10/10 packages ok, 0 race reports.
- `go test -count=10 ./...`: ok. x/bus `-race` with each nonce tag: ok.
- Wire probe byte-identical.
- API: the new package `network/linkctx` only; no existing package changed.
- `go.mod` unchanged. Network: none.

## Proposed commits (after review and Leader's go)

Updated after verify DND-022 (accepted) and Leader's DND-022-tie items (message m-7cde18840489).

1. `feat(linkctx): link-context core — Hello-gated link, per-session route ids with quarantine, table cap, counter low bits [DND-022]`
   - Files: `network/linkctx/link.go`, `ids.go`, `counter.go`, `linkctx_test.go`, including the same-window tie case and the Close comment.
   - Gate on the exported staged tree, offline:
     - gofmt; `go build ./...`; `go vet ./...` with the default and both nonce tags;
     - `go test -race -count=1 ./...` (10 packages);
     - `go test -race -count=20 ./network/linkctx`;
     - `make proto-breaking` (no proto change, so it must be clean);
     - the wire probe.
   - The body says that the tests first failed as a build failure and were then checked against mutants, including verify's mutant T.
2. `docs: DND-022 link core — ticket, design §4/§8, DND-009 Close item, DND-008 G1/G2, roadmap, index, report [DND-022]`
   - Files:
     - `docs/design/link-context-wire.md`;
     - `docs/tickets/DND-022-link-context-core.md`, `docs/tickets/DND-009-identity-auth-and-profile-boundary.md`, `docs/tickets/DND-008-stream-and-peer-lifecycle-races.md`;
     - `docs/tickets/index.md` (the link-core mentions: evidence link, DND-022 row, starting order);
     - `docs/roadmap.md`;
     - this report and `docs/evidence/dnd-r10/`.
   - Gate: a doc link check on the staged tree; no Go file changes.

The ticket-status housekeeping is already committed on its own, as Leader asked (the owner said "commit"; message m-b9822380b073):
- **442f088** `docs(tickets): mark DND-001…005, 018, 019, 021 done with commit hashes; DND-008 open items [DND-R8]`.
- Contents: the 8 tickets set to done with their commit hashes; DND-008 in progress for R1, F9 and the session ids that DND-022 carries; the index counts, legend and starting order.
- That index version does not mention the uncommitted link core.
- Staged by name; the doc link check on the staged tree found 0 broken links (824 links).

## DND-022-tie (verify DND-022 optional items), 2026-11-09

- **Tie test, second code path:** `TestCounterLowBits` now also asserts `RebuildCounter(130, LowBits(194, 7), 7) == 194`. Here the ahead candidate lies in the same window; the existing 200 → 264 case covers the next-window path.
  - Against verify's mutant T (`c-highest > half` → `>= half`, which resolves this tie behind) in a scratch copy, it fails: `expected: 0xc2, actual: 0x42`, "a tie resolves ahead (same window)".
  - On the real code it passes ×20 with `-race` ([log](dnd-r10/tie-mutant-T.txt)).
- **DND-009:**
  - New acceptance item: on secured profiles, an unauthenticated `Close` before the handshake is not accepted. It is dropped and counted like every other pre-handshake frame, with a test that fails first.
  - Code comment at `linkctx.Link.Receive`'s `Close` case: accepted in any state only because ONBOARD has nothing to authenticate.
  - No behaviour change.
- **Gates** ([log](dnd-r10/tie-gates.txt)):
  - Pass: gofmt, build, vet with all three tag sets, linkctx ×20 `-race`, and the wire probe byte-identical.
  - `go test -race -count=3 ./...`: 9/10 ok. **`endpoint/mesh` failed once.** The host load average was ~55 from other peers. A plain rerun passed (-count=3 and -count=10).
  - A parallel stress (12 processes × 10 runs of the mesh test binary) named it ([log](dnd-r10/mesh-gate-finding.txt)):
    - **G1:** the alias subtest `dial_targets_differ_from_declared_identities` fails 18/120 with `Send` → `no interest`;
    - **G2:** a data race on `Handshaker.remotePeer` (`Name()` read vs `handshakeMsg` write), 1/120.
  - Both are in committed mesh code that this round did not touch: G1 from d9a1793; G2's lines date from the pin.
  - I recorded them as DND-008 open items G1/G2 with fixes outlined, and did not fix them in this round.
  - **Effect on commit 1's gate:** under this load, its full `-race` run can hit G1. I propose fixing G1/G2 first as a small DND-008 round (tests failing first), or gating commit 1 with linkctx ×20 plus the full suite and recording any G1 hit as known.

## R-DND-022 fixes (L1–L6), 2026-11-09

Review R-DND-022 asked for changes (Leader message m-22cd41224291). All of them are fixed in `network/linkctx`, and each new test failed first by assertion against the reviewed code ([before](dnd-r10/l1-before.txt), [after](dnd-r10/l1-after.txt)).

| # | Fix | Test | Before (reviewed code) | After |
|---|---|---|---|---|
| **L1 (a)** | `Start` on a started link resends the **same** Hello; only `Restart` picks a new session | `TestHelloRetryOnOpenLinkSettles` | 101 Hellos in 100 rounds, never settled; A's binding lost | settles, ≤ 2 Hellos, A's route still delivers |
| **L1 (b)** | A peer's new session resets the id tables but **keeps our session**. The peer's replaced sessions are retired (last 4), and a Hello from one is dropped and counted (`Stats.DroppedStale`) | `TestLateHelloFromRetiredSessionIgnored` (one-sided reconnect, then the old Hello arrives late) | never settled; B reset A's new routes | 0 Hellos in answer, `DroppedStale` 1, the route still delivers |
| **L1 (c)** | Answer a Hello only if we have not sent one since the last we received | `TestSpoofedHelloBounded` | 101 Hellos, never settled | settles with ≤ 2 Hellos (B answers the spoof once, A answers B once, B drops A's answer as retired). Residual: the spoof still resets B's tables once on ONBOARD (no authentication; DND-009) |
| L1 liveness | the answer rule keeps a lossy face live | `TestHelloRetryAfterLostHello` (B's Hello lost; A retries) | — (new path) | A opens on the retry and a route delivers |
| **L2** | A `Close` must name the sender's own session, or the receiver's when the sender has none; others are dropped and counted. New `Link.Close(reason, text)` sends a Close for the current session | `TestStaleCloseIgnored` | `Close{session: 12345}` closed the link | ignored, `DroppedStale` 1; a real `Close` closes |
| **L3** | `max_routes` 0 in a peer's Hello means "not specified" → `DefaultMaxRoutes` (64); proto comment says so | `TestHelloWithoutMaxRoutesUsesDefault` | `Bind` → `ErrTableFull` | binds |
| L4 (nit) | `RebuildCounter` masks `low` to the window | case in `TestCounterLowBits`: `RebuildCounter(10, 328, 7) == 72` | 0xc8 instead of 0x48 ([log](dnd-r10/l4-before.txt)) | ok |
| L5 (nit) | `Face` doc: `Send` must not block (it is called with the link locked); a back-pressure face queues or drops | — | — | doc |
| L6 | Coverage of the paths review named | `TestProfileMismatchCloses`, `TestUnbindBeforeBindAck` | pass (the behaviour was right, it was untested) | pass |

- **Retiring sessions:** `Restart` does not retire the peer's session, because a peer that sees our new session keeps its own. A first version did retire it, and that version's one-sided reconnect never opened. `TestLateHelloFromRetiredSessionIgnored` caught it.
- **Design:** §4 now has the session rules (link-context-wire §4 "Session rules"): our session is kept until we reconnect; a peer reconnect resets the tables; at most one answer per received Hello; retired sessions ignored; which session `Close` names; `max_routes` 0; the ONBOARD spoofing residual.
- **Proto:** `link.proto` comments for `max_routes` and `Close.session`. Comment-only, so `link.pb.go` changes in comments only. `make proto-lint` ok; `make proto-breaking` against HEAD ok.
- **The "cannot ping-pong" claim** in "Behaviour" above is struck through and corrected.
- **Tests:** `network/linkctx` now has 14 tests, all ×20 `-race` ok; coverage is 90.2 % (was 82.9 %).
- **Gates:** [log](dnd-r10/l1-gates.txt), [race](dnd-r10/l1-gate-race-count3.txt).

**Committed** (Leader message m-5f5c8e4db5dd: the owner wants regular commits; review and verify recheck the committed code). Each commit was gated on its exported staged tree, offline: build, vet, gofmt, `go test -race -count=1 ./...` (10/10), linkctx ×20 `-race`, buf lint/breaking, and the wire probe.
1. **9e34601** `feat(linkctx): link-context core — Hello-gated link, per-session route ids with quarantine, table cap, counter low bits [DND-022]`
   - The code as verify accepted it (with the same-window tie case).
   - The tree is reconstructed without the L1–L6 changes; it was checked first in a scratch copy, where the 6 tests pass.
   - The body lists L1–L4 as known open findings.
2. **63df1c4** `fix(linkctx): Hello session rules — no ping-pong, stale Close dropped, max_routes default [DND-022 R-DND-022 L1-L6]`
   - The fix with its tests, and the `link.proto` comment changes.
3. The DND-022 docs commit, which carries this report.

G1/G2 (DND-008) follow as their own commits.

## Decisions for the owner

None new. The Bind rate limit (A8) needs a number, for example N Binds per second per link. I propose 32/s with a burst of 64, matching the MCU `max_routes`, to be set in the DND-022 follow-up unless the owner prefers otherwise.

## Proposed mb learnings

- 2026-11-09 [testing] [medium] When a test can only "fail first" as a build failure (a new API), add a mutation run: apply one wrong implementation at a time in a scratch copy and record the assertion each test fails with. It shows the test discriminates, which a compile error cannot (dndm DND-022, verify DND-R9 note).

## Next action

Verify and review the link core. Then: the Bind rate limit, Bind/Unbind retries with timers, `max_frame`, v1 negotiation (`link_version` in `types.p2p.Handshake`) with the secure-link refusal (A5), and wiring `Link` into the remote endpoint behind the v1/v2 switch.
