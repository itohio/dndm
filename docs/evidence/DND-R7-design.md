# DND-R7-design report (dndm, 2026-11-09)

**Result:** both designs written, docs only. The draft `link.proto` compiles and passes buf STANDARD lint (checked in `_scratch`). Its frame sizes were measured with generated Go code: a DATA frame is **4 B + payload**, against 86–94 B today. No `.go`, `.proto`, `go.mod` or `buf` file in the repo changed.

Brief: `docs/tasks/DND-R7-design.md`.

## Files written

- `dndm/docs/design/link-context-wire.md`: draft schema, per-frame cost, binding rules, v1 compatibility, generator needs, tooling plan, byte-count tests.
- `dndm/docs/design/close-reconnect.md`: as-is flow, defects R1–R5, connection generations, links and binds on reconnect, interaction with bind, failing tests.
- This report. One pointer line each in `docs/index.md`, `docs/roadmap.md`, and the DND-008 and DND-020 tickets.

## Commands → key output

Offline throughout: `GOTOOLCHAIN=local GOPROXY=off`, set per command. Scratch only: `~/src/tmp/dndm/protocheck/`.

- **buf from the local module cache.** `go build github.com/bufbuild/buf/cmd/buf` in a scratch module requiring `github.com/bufbuild/buf v1.75.0` (`GOFLAGS=-mod=mod` for that command only) → binary, `buf --version` 1.75.0. Nothing was installed and nothing downloaded.
- **Schema check.** The `proto` block was extracted from link-context-wire.md into a scratch buf v2 workspace.
  - `buf build` → exit 0.
  - `buf lint` with STANDARD (minus PACKAGE_VERSION_SUFFIX, as the repo excepts) → exit 0, after renaming enum values to the `BIND_STATUS_`/`CLOSE_REASON_` prefixes.
  - With the repo's extra COMMENTS rule, the lint wants a leading comment on every message and enum value. Those are added at implementation time.
- **Sizes.** `protoc-gen-go` built from the cache → `link.pb.go`. Then a scratch probe with `proto.Size`:

```text
payload    26: DATA    30 (+4)  +seq(200)    33 (+7)  sealed tag8 ctr200    41 (+15)  no route_id    28 (+2)
payload    62: DATA    66 (+4)  +seq(200)    69 (+7)  sealed tag8 ctr200    77 (+15)  no route_id    64 (+2)
payload   127: DATA   131 (+4)  +seq(200)   134 (+7)  sealed tag8 ctr200   143 (+16)  no route_id   129 (+2)
payload   128: DATA   133 (+5)  +seq(200)   136 (+8)  sealed tag8 ctr200   144 (+16)  no route_id   131 (+3)
payload 16383: DATA 16388 (+5)  +seq(200) 16391 (+8)  sealed tag8 ctr200 16400 (+17)  no route_id 16386 (+3)
payload 16384: DATA 16390 (+6)  +seq(200) 16393 (+9)  sealed tag8 ctr200 16401 (+17)  no route_id 16388 (+4)
BIND for ManipulatorState (once per link): 66 B
HELLO: 41 B
```

  It matches the cost table in link-context-wire §3. EasyRobot ManipulatorState: 148 B → 66 B. ManipulatorTarget: 120 B → 30 B. Today's numbers are from verify's DND-research.
- **Tooling facts** (link-context-wire §7):
  - go.mod has `google.golang.org/protobuf v1.36.10`; the generated code says `protoc-gen-go v1.36.5`.
  - buf config is v1 with an unused well-known-types dependency (no `google/protobuf` imports).
  - The `buf` CLI is not installed on this machine.
  - The local module cache has protobuf up to v1.36.12 (stable) and buf v1.75.0 source.
- **Doc link check** (same validator as earlier rounds, excluding `research/ideation/`): 38 Markdown files, 702 relative links, 0 broken.

## Network contacts

None.

## Not verified

- Every claim in close-reconnect §2 (R1–R5) is read from the code. None is reproduced: the task is docs-only. The failing tests in §6 will settle them, and R1 (duplicate delivery on simultaneous dial) is the main inference.
- Generator support for the schema subset (nanopb options, protobuf-c, protobuf-go-lite) is from their documented feature sets, marked `#verify`. No generator other than protoc-gen-go was run.
- The upstream "latest" buf and protobuf versions were not checked (offline). The target is "latest stable at update time".
- RP2040 RNG quality for `Hello.session`.

## Decisions for the owner

1. **DATA frame encoding**: plain `LinkFrame` protobuf, 4 B overhead (recommended), or the raw `varint route_id ‖ payload` mode at 1–2 B. The raw mode is not decodable by generated code alone (link-context-wire §3.1). Recommend LinkFrame everywhere, with the route id lifted into the CAN identifier on CAN, and raw mode only if a measured link proves 2–3 B matter.
2. **Tooling fetch**: allow pinning buf and protoc-gen-go as Go tools (go.mod `tool` directives; a module download) for the v2 config and regeneration. Recommend yes, as its own change after verify accepts DND-R5.
3. **Legacy v1 fallback**: a link configured with a secure profile refuses v1 unless `Insecure("legacy v1 peer")`. Recommend yes; it closes the version-strip downgrade.
4. **Tie-break for simultaneous connections**: keep the connection dialled by the lower identity. Recommend yes; it is symmetric and needs no extra round trip.

## Proposed mb learnings

- 2026-11-09 [code] [medium] Check draft .proto files offline by building buf and protoc-gen-go from the local Go module cache in a scratch module (`go build`, not `go install`); then measure real encodings with `proto.Size` instead of hand arithmetic (dndm DND-R7).
- 2026-11-09 [code] [medium] Pure protobuf framing has a floor: a bytes payload costs tag + length (2 B) and a small id another 2 B. "~2 B overhead" needs either face-level addressing (CAN id) or a non-protobuf prefix; decide explicitly (dndm DND-R7).
- 2026-11-09 [code] [low] A connection registry whose close callback clears unconditionally needs a generation token (compare-and-clear) before it can ever replace a connection (dndm DND-R7).

## Next action

Apply ux's UX-3 diagram review when it arrives. After verify accepts DND-R5 and the owner answers decisions 1–2: write the close-reconnect failing tests (§6), then the link codec, starting with its byte-count test.
