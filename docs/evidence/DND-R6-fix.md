# DND-R6-fix report (dndm, 2026-11-09)

**Result:** done, docs only.

- All of ux's UX-3 fixes are applied: the 15 diagrams marked "fix" are fixed and LF4 is split into 4a/4b, so the set is now 30 blocks.
- Leader's four decisions are recorded in both design docs.
- 30/30 blocks render offline, and the doc link check finds 0 broken links.
- No `.go`, `.proto`, `go.mod` or `buf` file changed.

Brief: `docs/tasks/DND-R6-fix.md`; review: `docs/ux/UX-3-dndm-diagrams.md`.

## What changed

**Decisions recorded** (link-context-wire.md header; close-reconnect.md header):

| # | Decision | Where recorded |
|---|---|---|
| 1 | Plain protobuf `LinkFrame` (4 B + payload) everywhere; route id in the CAN identifier on CAN: yes | wire §3.1 retitled "decided: not used" for raw mode |
| 2 | buf/protoc-gen-go as Go tools: not approved, pending owner | wire §7 plan step 1 marked pending |
| 3 | Legacy v1 only via `Insecure("legacy v1 peer")`: yes | wire §5 |
| 4 | Lower identity wins the tie-break: yes | close-reconnect §3.1 retitled "(decided)" |

**UX-3 fixes**, in `dndm/docs/diagrams/`:

| Finding | Fix |
|---|---|
| G1 arrow meanings | Every wire crossing is `==>` (core 2, 4, 6; MP5; S1, S2, S7). Dotted is control only: MP3's relay and S7's telemetry are solid now, and S7's caption no longer redefines dotted. Refusals are `--x` (MP5, S3). Core 5's frame fields are side by side (`~~~`), not a flow. The README says no caption changes an arrow's meaning. Sequence boxes are neutral grey; MP4's attacker is in a red "Untrusted" box, and both of its replies are dashed. |
| G2 one word, one meaning | New **glossary** in README: route, intent/interest, match, face, link, session, route id, zone, bridge, gateway, stage, profile. "Link" is used only for connections; the in-process pairing is "match (Linker)" (core 1, 3, 4, LF2/LF3). "Bridge" is used only for zone crossings: S7 has a "CAN gateway (same zone, not a bridge)", and NATS is "NATS gateway" in S3 and S8. The design doc's `Bind.layers` field is renamed `stages`, with "stage" everywhere. Both design docs point to the glossary. |
| G3 refusals and bridges | The bridge sits between the zone frames in core 7, MP5 and S3. Refusals go into the bridge: core 7 `lt --x bridge`, MP5 `host --x br`, S3 `dash --x bridge` (no grant for r2). |
| G4 colours | Secure AEAD and other stages are amber (core 8, S8). New "allowed but flagged" style (amber, red dashed border) for `Insecure(reason)`, drawn as the alternative path, not feeding into Secure. Frame fields are white (core 5). Devices are frames or neutral boxes (core 6, S7 joint MCUs as frames with blue controllers). `---` is labelled ("uses", "watched by") and added to the legend. |
| G5 ticket ids in captions | Plain point first, id in parentheses (core 4, MP1). |
| G6 reading order | "Start here" list of 9 diagrams at the top of the README. |
| G7 renderer | README and render-check.txt say Obsidian's Mermaid 11.x and GitHub are untested. |
| LF4 split | **4a** is the handshake per link profile (OnBoard / zone key / static-DH, plus profile mismatch). **4b** is the bind outcomes (ok / denied → data dropped / table full). The wrong zone key is now a **handshake failure** (the proof does not verify, CLOSE before any BIND), matching the updated design table (wire §4). Only broadcast links, which have no handshake, catch it at data time. S5 points to 4a/4b. |
| LF3 two ideas | The route-close lines moved into LF2, so LF3 shows only connection loss and redial and points to the close-reconnect design. |
| Target frame sizes | Core 5 target shows `route_id` 2 B + payload tag/length 2 B = 4 B, with as-is 86–94 B (header 70–78 B). Core 6, S1 and S2 use 4 B / about 15 B labels; S7 uses the route id in the CAN identifier. |
| core 3 | InterestRouter now collects from every endpoint; the mesh Handshaker label says it wraps a remote endpoint; `remote ---|uses| conn`. |
| core 4 | Connection hexagon added between the node frames: announcements dotted through it, MESSAGE frames `==>`. |
| S1 | `flowchart TB`, one UART hexagon used both ways, Wi-Fi hexagon between SBC and host. |
| S9 | "OnBoard wire: no check" is a plain box, not a diamond. |

Kept as is (verdict K): core 2 (merged its two identical edges, as suggested), LF1 (caption explains "address book" and "seed node"), LF5, LF6, MP1, MP2, MP6, S4, S6.

## New finding while updating S7

Measured with generated code from the draft schema (scratch probe), with the route id in the CAN identifier:

| Frame | Result |
|---|---|
| unauthenticated telemetry | 6 B of data in an 8 B classic CAN frame |
| authenticated command (counter + 3 B MAC) | **1 B of data** in classic CAN; about 51 B in CAN FD with an 8 B tag |

Research-2's binary-framing estimate was 4 B of data for the command. With protobuf framing, authenticated commands need CAN FD in practice. This is recorded in wire §3 and the S7 caption. The owner's CAN rule still holds; this is input for when a target is chosen.

```text
CAN telemetry, payload 6 B, no route_id: 8
CAN command, 1 B data + 3 B MAC sealed, ctr 5, no route_id: 8
CAN FD command, 20 B data + 8 B tag, ctr 300: 33
```

## Commands → key output

Offline; scratch only.

- **Render check:** `mmdc` (mermaid-cli 12.2.0, Mermaid 12.3.0, local npx cache) on all five files. Every file exit 0, 30/30 blocks: README 1, core 8, link-flows 7, message-paths 6, robotics 8 ([render-check.txt](../diagrams/render-check.txt)). I looked at PNG renders of core 4, S1, S3 and MP5: refusals end at the bridge, and S1 reads top to bottom.
- **Draft schema** after the `layers` → `stages` rename: `buf build` and `buf lint` (STANDARD) with the buf 1.75.0 built from the module cache in DND-R7 → ok.
- **Doc link check** (same validator, excluding `research/ideation/`): 39 Markdown files, 720 links, 0 broken. One anchor broke after retitling close-reconnect §3.1 and was fixed.
- **Code check** (`find -newer` the DND-R5 final gate log, run at the end of this task): no `.go`, `.proto`, `go.mod`, `go.sum` or buf file is newer.

## Network contacts

None.

## Not verified

- Rendering in Obsidian (bundled Mermaid 11.x) and on GitHub (G7).
- A second rating of the three major findings, which ux suggested (verify or owner).
- The CAN numbers come from the draft schema; the real schema may differ.

## Decisions for the owner

- Pending from DND-R7: allow fetching buf and protoc-gen-go as Go tools (Leader did not approve it). Recommend yes, as its own change after DND-R5 is accepted and committed.
- New input, not a decision yet: with protobuf framing, authenticated commands on classic CAN leave 1 B of data. Recommend requiring CAN FD for authenticated command routes when a CAN target is chosen.

## Proposed mb learnings

- 2026-11-09 [docs] [medium] After a clarity review, put a glossary next to the legend and make captions never redefine arrow styles; most diagram fixes were the legend's own rules applied consistently (dndm DND-R6-fix).
- 2026-11-09 [code] [medium] Measure protobuf framing on tiny links before promising budgets: on classic CAN, tag+length plus a counter and a 3 B MAC leave 1 B of data (dndm DND-R6-fix).

## Next action

Once verify accepts DND-R5, commit DND-R2…R5 when Leader says. Then write the close-reconnect failing tests and the frame encoder, starting with its byte-count test.
