# DND-R7-design-fix report (dndm, 2026-11-09)

**Result:** verify's F1 and the optional F2 are fixed in the designs, docs only.

- **F1:** route ids are now per session, lowest free id first, with freed ids reused after a quarantine. A data frame stays at 4 B + payload however often a link reconnects. Ids reach 128 (5 B) only with more than 127 routes bound at once on one link.
- **F2:** `ctr` carries 7 low bits on classic CAN and CRSF, 14 elsewhere, so a classic-CAN authenticated command always keeps its 1 B of data.
- The boundary sizes were measured with generated code. All 30 diagram blocks render, and the draft schema builds and passes lint.
- No `.go`, `.proto`, `go.mod` or `buf` file changed.

Brief: `docs/tasks/DND-R7-design-fix.md`. Finding: `docs/verification/DND-R7-design.md` (F1 required, F2 optional), tied to verify DND-R6 F1 (the LF5 caption).

## Chosen rule and why

| Option (from verify) | Chosen? | Why |
|---|---|---|
| Per-session id space, reset with the bind | **yes** | Ids then depend only on how many routes are bound at once, not on history. Late frames from an old session are already stopped by new keys (secured links) or by the closed connection (streams). |
| Reuse of freed ids | **yes, within a session** | An id is reused only after `UnbindAck`, plus a quarantine of 2 × the retry timeout on reordering faces. Without reuse, route churn inside one long session would also push ids past 127. |
| A cap with explicit rebind | no | `Hello.max_routes` already caps concurrent routes. With reuse a cap is not needed for size, and a rebind would add traffic. |
| Keep `nextRoute` across sessions | **dropped** | It was the cause of F1. The address book no longer keeps route-id state. |

**Residual:** on an `INSECURE` datagram link (UDP with `Insecure(reason)`), a frame delayed across a reconnect can hit a reused id. Such a link accepts unauthenticated frames from anyone anyway, so the residual adds no new attack. It is stated in wire §4.

## Measured (draft schema, generated Go, scratch probe)

```text
route_id     1, payload 62: 66 B (+4)
route_id   127, payload 62: 66 B (+4)
route_id   128, payload 62: 67 B (+5)
route_id 16383, payload 62: 67 B (+5)
route_id 16384, payload 62: 68 B (+6)
classic CAN command ctr     1, 1 B data + 3 B MAC: 8 B
classic CAN command ctr   127, 1 B data + 3 B MAC: 8 B
classic CAN command ctr   128, 1 B data + 3 B MAC: 9 B
classic CAN command ctr 16383, 1 B data + 3 B MAC: 9 B
classic CAN command ctr 16384, 1 B data + 3 B MAC: 10 B
secured, route 3, ctr 127 (7-bit), payload 62, tag 8: 76 B
secured, route 3, ctr 16383 (14-bit), payload 62, tag 8: 77 B
```

## Files changed

- `docs/design/link-context-wire.md`:
  - schema comments for `route_id`, `ctr` and `Bind.route_id`;
  - the presence rule for `ctr` (absent means low bits 0; `sealed` marks a secured frame);
  - the cost table: 4 B with the stated id/payload conditions, a secured row of 12–15 B, and the CAN command row "1 B of data, always";
  - §4: new **Id rules** and **Counter rules**;
  - §8: new tests `TestLinkFrameRouteIdBoundary` (127 → +4, 128 → +5), `TestRouteIdsStaySmallAcrossReconnects`, `TestRouteIdReuseQuarantine` and `TestCounterLowBits` (127/128/255/256, wrap rebuild, gap ≥ 64 fails the tag);
  - §9: `nextRoute` removed.
- `docs/design/close-reconnect.md`:
  - `nextRoute` removed from the address-book entry and the close order;
  - the route-id row of the reconnect table rewritten, with a note on what changed;
  - tests `TestLink_LateFrameFromOldSessionDropped` (old frame on a reused id fails the tag) and `TestLink_IdsStaySmallAcrossReconnects`.
- `docs/diagrams/link-flows.md`:
  - LF5 rebinds as `route_id 1`, adds a "late frame on a reused id" branch, and has a new caption. This also fixes verify DND-R6 F1, the LF5 caption that contradicted "all link state lost".
  - LF6: a late frame fails the tag, because session 7's keys are gone.
- `docs/diagrams/robotics.md`: the S7 caption says only the counter's low 7 bits are sent.
- `docs/diagrams/render-check.txt`: rerun.

## Commands → key output

Offline; scratch only.

- **Probe:** generated Go for the draft schema (DND-R7 scratch), `proto.Size` → the table above.
- **Draft schema:** extracted from the doc → `buf build` ok, `buf lint` (STANDARD) ok. buf 1.75.0 is the binary built from the module cache in DND-R7.
- **mmdc** (mermaid-cli 12.2.0 / Mermaid 12.3.0): every file exit 0, 30/30 blocks.
- **Doc link check** (same validator, `docs/`, excluding `research/ideation/` and `.obsidian/`): 42 Markdown files, 740 links, 0 broken.
- **Code check:** no `.go`, `.proto`, `go.mod`, `go.sum` or buf file is newer than the DND-R5-fix gate log.

## Network contacts

None.

## Not verified

- The rebuild-from-low-bits behaviour and the quarantine timing exist only as a design and planned tests.
- The 2 × retry-timeout quarantine is an estimate; the right value depends on the face's reorder window.

## Decisions for the owner

None new. The CAN FD recommendation for authenticated commands stands, now with the counter rule that keeps classic CAN at 1 B of data.

## Proposed mb learnings

- 2026-11-09 [code] [medium] Small per-link ids only stay small if their space is per session and freed ids are reused (with a quarantine on reordering links); "never reuse, carry the next id across reconnects" silently grows varints over a link's life (dndm DND-R7-design-fix).
- 2026-11-09 [code] [low] Send only the low bits of an AEAD counter on tiny frames and rebuild the full counter at the receiver (SecOC-style); a varint counter otherwise grows by a byte at 128 (dndm DND-R7-design-fix).

## Next action

Commit DND-R2…R5 per review's plan once verify's DND-R5-fix spot-check lands and Leader says so.
