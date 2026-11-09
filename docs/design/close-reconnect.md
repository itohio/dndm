# DND-008 close and reconnect (design, DND-R7)

Status: **design draft, no code**. DND-008's open acceptance item: "close/reconnect tests terminate within configured deadlines and do not resurrect stale connections". The owner asked for a short design before code ([decisions](../../../docs/decisions.md)). It builds on the [link-context wire design](link-context-wire.md). Diagrams: [link flows 3 and 6](../diagrams/link-flows.md).

**Decision** (Leader, 2026-11-09, under the owner's autonomy instruction; `docs/tasks/DND-R6-fix.md`): simultaneous connections keep the one dialled by the lower identity (§3.1), **yes**. Words follow the [diagram glossary](../diagrams/README.md#glossary): a *link* is a connection, and today's in-process `Link` object is called a *match*.

## 1. How it works today (as is, read from the code)

- **Dialling.** `Addrbook.monitor` runs every 10 s and queues outbound peers that are not active. A dialer loop (`endpoint/mesh/conn.go`) calls `AddrbookEntry.Dial`, which backs off on failure and re-queues; non-persistent peers stop after `MaxAttempts`. On success it builds a `Handshaker` (`HS_INIT`) and calls `Container.Add`. An accepted connection (`onConnect`) also builds a `Handshaker` (`HS_WAIT`) and calls `Container.Add`.
- **Recording the connection.** The dialer side calls `addrbook.AddConn(remotePeer, true, conn)` in `Handshaker.Init`. The acceptor calls it after the INITIAL handshake, keyed by the peer's **self-declared** `Me` text. `AddConn` → `AddrbookEntry.SetConn(c)`:
  - It returns `ErrDuplicate` if the entry already has a connection.
  - Otherwise it records `c` and registers `c.OnClose(func(){ a.SetConn(nil) })`.
- **Ending.** A read error or `Close` → remote endpoint `shutdown` → its Linker closes every intent, interest and match (`Link` object) → `Handshaker.Close` → `Container.Remove`. The stream's close callback clears the entry (`SetConn(nil)`). After the next `monitor` tick, the peer is redialled and the routes are registered again on the new connection (`Container.reconcile` on ready, DND-004).

## 2. Defects in that flow

These are read from the code; none is reproduced yet, and the failing tests in §6 will check each one.

| # | Defect | Where | Effect |
|---|---|---|---|
| R1 | `AddConn`'s error is ignored at both call sites | `handshake.go` `Init` (dialer) and `handshakeMsg` (acceptor) | When A and B dial each other at the same time, the second connection gets `ErrDuplicate` but its Handshaker keeps running and stays in the Container. There are **two live connections** for one peer: both remote endpoints announce and match the same routes, so messages can arrive twice. |
| R2 | The entry refuses a second connection, and its close callback clears the entry unconditionally (`SetConn(nil)`) | `addrbook.go` `SetConn` | A connection that is already dead can be recorded: its close callback was added after it closed, so the callback runs later on its own goroutine (`lifecycle.add`). Until that callback runs, a live connection arriving in the window is refused (`ErrDuplicate`, ignored, R1) and never recorded. When the callback clears the entry, the live connection is untracked, `monitor` dials again, and a duplicate appears. The unconditional clear also rules out "replace the current connection" (§3.1) unless the clear becomes compare-and-clear. |
| R3 | The duplicate connection from R1 is never recorded | same | When the recorded connection closes, the entry is cleared, although the unrecorded one is still alive. Again a redial, and duplicates. |
| R4 | The acceptor keys the entry by the peer's self-declared `Me` | `handshake.go` `handshakeMsg` | Anyone can claim to be B. Today that only produces an `ErrDuplicate` that is ignored (R1). It is an identity question for DND-009, noted here so the generation design does not trust `Me`. |
| R5 | No session identity | wire | Nothing in a frame says which connection it belongs to. On TCP the old socket is closed, so late frames die with it. On future datagram faces, a late frame from an old session could be decoded by a new one (verify A3). |

## 3. Connection generations

Each `AddrbookEntry` gets a **generation**: a counter that only goes up, plus the current connection and its session id.

```text
AddrbookEntry
  gen        uint64       // increments on every accepted connection
  conn       network.Conn // current, or nil
  session    uint64       // link-context Hello.session of conn (v2), random per connection (v1)
```

**Rules.**

1. **Accept returns a token.** `SetConn(c)` either refuses (the current connection is live and wins, §3.1), or replaces the current one. A current connection that is already closed is always replaced. It returns `gen`. The close callback captures that `gen` and does compare-and-clear: `if a.gen == gen { a.conn = nil }`. A stale callback can then never wipe a newer connection. This fixes R2.
2. **One connection per pair, decided the same way on both sides** (§3.1). The loser is closed with `CLOSE_REASON_DUPLICATE` and removed from the Container, so its routes never get linked. This fixes R1 and R3.
3. **`IsActive` means "a recorded, ready connection exists"**, or a dial is in flight. A Handshaker that has not finished its handshake does not count as active, and `monitor` does not redial while one is in progress. Dial and accept both register "connecting" under the entry lock.
4. **The key is the verified peer identity when there is one.** Static-DH links use the static key id; v1 and PSK links use the declared address, a documented residual risk (R4 → DND-009).

### 3.1 Tie-break for simultaneous connections (decided)

Both sides see both connections, and both must keep the **same** one:

- Keep the connection **dialled by the peer whose identity sorts lower**: byte compare of the verified key id, or, on v1/PSK links, of the identity string each peer declares in the handshake (`Peer.String()`, its `Me`). Not the address a dialer was configured with: two nodes can know each other by different names (`localhost:4000` vs `127.0.0.1:4000`), and then they would disagree (review R-DND-R8 N2).
- Each side applies the rule once its handshake completes: the acceptor when INITIAL arrives, the dialer when FINAL arrives. Before that, a dial is only noted (`Addrbook.NoteDial`) and never displaces or refuses a connection. If the new connection loses, it closes itself (with `CLOSE_REASON_DUPLICATE` on v2). If it wins, the old one gets closed: `SetConn` replaces it, bumps `gen`, then closes the old connection.
- Residual (v1, implemented in DND-R8): both connections can be ready for a moment before the dialers' FINALs settle it, so a message sent in that window may arrive twice.

Because the rule is symmetric, both sides end with the same connection and never ping-pong.

## 4. What happens to links and binds

**A reconnect is a new session.** Nothing carries over except the address-book entry.

| State | On close | On reconnect |
|---|---|---|
| Keys, counters, replay windows (v2) | dropped | new handshake → new keys; counters restart safely because the keys are new |
| Route ids (v2) | the table is dropped | new `Bind`s in a fresh, per-session id space: the lowest free id from 1, so ids stay small however often the link reconnects ([wire §4](link-context-wire.md#4-binding-rules-zone-security-ids)). Late frames from the old session do not land on a new id: on secured links they fail the new keys' tag, and on stream faces the old connection is gone. That fixes R5. The residual is only on `INSECURE` datagram links, which accept anyone's frames anyway. *(Revised after verify DND-R7 F1: the earlier rule carried `nextRoute` across sessions, so ids grew past 127 after about 13 reconnects and DATA frames grew to 5 B.)* |
| Announce/Bind of current intents and interests | the remote endpoint and its linker close (as today) | `Container.reconcile` registers every current route again (as today); on v2 that is Announce, then Bind |
| Application handles (`Producer`, `Consumer`, `Caller`) | unchanged, kept by the local Router | `Send` returns `ErrNoInterest` while the link is down, then works again once the remote interest is bound |
| In-flight messages | dropped. Latest routes lose nothing they would keep anyway; buffered routes repair across the gap by `seq`, which is per route, not per session | DND-006 decides buffered/history behaviour |
| Pending `Caller` calls | time out (as today) | not replayed; a call is not retried silently |

**Order of operations on close.** The order matters so that no callback races the next session:

1. Mark the entry "closing" and save `gen`.
2. Close the remote endpoint and its Linker. Every match waits for in-flight Sends (the DND-R5 RC6 fix).
3. Remove the Handshaker from the Container.
4. Compare-and-clear the entry.
5. Only now may `monitor` or an incoming connection start a new session.

## 5. Interaction with the link-context bind

- **Hello and session.** `Hello.session` (random 64-bit, per connection) is stored in the entry and echoed in `Close`. A `Close` whose session is not the current one is ignored, so an old connection's late `Close` cannot end the new one.
- **Binding before the handshake.** Frames other than hello/handshake are dropped until the handshake completes (link-context-wire §4). So a stale or duplicate connection that loses the tie-break never reaches Bind.
- **Legacy v1 connections** get a random `session` locally. They have no wire session field, which is fine because TCP closes the old socket.
- **DND-020.** Ids are per sending side, so reconnect logic does not care which direction a route runs.

## 6. Failing tests to write first (when code is allowed)

All in-process. Pipes or loopback only, nothing beyond 127.0.0.1. Each has a deadline.

| Test | Package | Expected today | Expected after |
|---|---|---|---|
| `TestAddrbookEntry_LiveConnNotLostBehindDeadOne`: record conn 1, already closed so its callback runs late; record live conn 2 straight away; wait for conn 1's callback | `endpoint/mesh` | expected to fail: conn 2 gets `ErrDuplicate`, then the entry is cleared and `IsActive` is false although conn 2 is live (R2) | entry holds conn 2, `gen` 2, `IsActive` true |
| `TestMesh_SimultaneousDialKeepsOneConnection`: A and B dial each other at the same moment through a gate dialer | `endpoint/mesh` | two Handshakers in each Container; a message published on A arrives twice at B, or the entries disagree (R1/R3) | one connection each, the same one on both sides; the message arrives once; the loser was closed with reason DUPLICATE |
| `TestMesh_ReconnectRebindsRoutes`: A↔B up, B subscribed; kill the transport; A redials (persistent peer). The test needs the monitor period, today a fixed 10 s, to be an option | `endpoint/mesh` | expected to pass, slowly; the test pins the behaviour | B receives A's messages again within the deadline; no duplicates; the old Handshaker is gone from both Containers |
| `TestMesh_CloseDuringHandshakeReturns`: close the endpoint while a dial and a handshake are in flight | `endpoint/mesh` | measure (hang or leaked goroutines suspected; goroutine count before and after) | returns within the deadline; goroutine count back to baseline |
| `TestMesh_LateCloseDoesNotEndNewSession` (v2 only, after link-context code) | link codec | n/a | a `Close` carrying the old session id is ignored |
| `TestLink_LateFrameFromOldSessionDropped` (v2, datagram fake face, secured profile) | link codec | n/a | a frame from the old session, carrying an id that the new session has reused, fails the new keys' tag. It is dropped and counted, never delivered on the new route |
| `TestLink_IdsStaySmallAcrossReconnects` (v2) | link codec | n/a | after 50 reconnects with 10 routes, every id is ≤ 10, so DATA frames stay at 4 B + payload (verify DND-R7 F1) |

The first two are expected to fail on today's code and are the core of the DND-008 item. The third pins the redial behaviour.

## 7. Out of scope

- Who may connect (identity, keys): DND-009.
- What buffered or history routes replay after a gap: DND-006.
- Multi-hop or relay reconnect: DND-012.

[Overview](../index.md) · [link-context wire](link-context-wire.md) · [DND-008](../tickets/DND-008-stream-and-peer-lifecycle-races.md)
