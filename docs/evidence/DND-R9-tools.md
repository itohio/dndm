# DND-R9-tools report (dndm, 2026-11-09)

**Result:** verified (unit, race and wire-probe boundary). All four steps of link-context-wire §7 are done, committed locally as 0bb4604 (tools), e160c3c (protobuf bump), 954bc81 (codec) and the docs commit that carries this report:
1. buf v1.75.0 and protoc-gen-go are pinned Go tools in separate tool modules.
2. The buf config is v2 at the repo root, with the WKT dep dropped. The generated diff is empty.
3. protobuf is bumped to v1.36.12 and the code regenerated. The raw descriptors are byte-identical and the wire probe is byte-identical.
4. DND-022's `link.proto` and `.options` are added; `buf lint` and `buf breaking` are clean. A protowire DATA codec makes four byte-count tests pass. They failed first as a build failure (the API did not exist yet), not as a red assertion; see the step-4 table.

Two deviations need a decision:
- **EasyRobot:** its read-only build now fails until EasyRobot raises its own protobuf requirement to v1.36.12, a one-line change.
- **buf.build:** `buf config migrate` contacted buf.build once, which the approval did not cover.

Brief: `docs/tasks/DND-R9-tools.md`. Leader message m-66f7f06aebab. Owner: "download buf and protoc-gen-go". State: HEAD `0062746`, branch `main`, local only.

## Files written

**Tools**
- `tools/go.mod`, `tools/go.sum`: protoc-gen-go v1.36.12 as a `tool`.
- `tools/buf/go.mod`, `tools/buf/go.sum`: buf v1.75.0 as a `tool`.

**buf config**
- `buf.yaml`, `buf.gen.yaml`: v2, at the repo root.
- `examples/embedded/proto/buf.yaml`, `buf.gen.yaml`: v2, a standalone module.
- Deleted: `proto/buf.yaml`, `proto/buf.gen.yaml`, `proto/buf.lock`, `examples/embedded/proto/buf.lock`.

**Make targets**
- `Makefile`: `proto` and the new `proto-lint` run buf via `go tool`.
- `examples/embedded/Makefile`: `proto` runs buf via `go tool`.

**protobuf runtime**
- `go.mod`, `go.sum`: `google.golang.org/protobuf` v1.36.10 → v1.36.12. No other change.

**Generated code**
- The 7 existing `.pb.go` files, regenerated with protoc-gen-go v1.36.12.
- New: `types/link/link.pb.go`.

**Schema and codec**
- `proto/types/link/link.proto`, `proto/types/link/link.options`.
- `codec/link_frame.go`, `codec/link_frame_test.go`.

**Docs**
- `docs/design/link-context-wire.md`: status, decision 2, §7 table and plan, §8 correction.
- `docs/tickets/DND-022-link-context-core.md`: in progress, first acceptance item, log.
- `docs/tickets/index.md`, `docs/roadmap.md`.
- This report and `docs/evidence/dnd-r9/`.

## Step 1: tools pinned (network, owner-approved)

**Decision: separate tool modules, not dndm's `go.mod`.**
- **buf's dependencies stay out:** a `tool` line in dndm's `go.mod` would put buf's 89 modules into dndm's module graph and `go.sum`. Every consumer, EasyRobot included, would then resolve them.
- **The modules are invisible to library users:** a nested module is outside `./...` and outside dndm's module zip.
- **Why two modules, not one:** buf v1.75.0 requires `google.golang.org/protobuf v1.36.13-0.20261016172727-e9ee2a76ab8e`, an unreleased pseudo-version. In one module, protoc-gen-go would resolve to that pseudo-version instead of the runtime's v1.36.12.
- **Result:** `tools/go.mod` pins protoc-gen-go at exactly the runtime version (go 1.24.1). `tools/buf/go.mod` holds buf (go 1.26.7, buf's own minimum).

**Usage**
- `go tool -modfile=tools/buf/go.mod buf …`, or `make proto` / `make proto-lint`.
- buf.gen.yaml runs the plugin as `local: ["go", "tool", "-modfile=tools/go.mod", "protoc-gen-go"]`.

**"Latest stable"**
- `go get -tool …@latest` resolved protoc-gen-go **v1.36.12** and buf **v1.75.0** (2026-11-09 09:24Z).
- v1.37.0-rc.1 exists in the cache. It is a release candidate and was not used.

**Network contacts** (each `GOPROXY=https://proxy.golang.org GOFLAGS=-mod=mod` set on that one command; `GOSUMDB` at its default `sum.golang.org`; [log](dnd-r9/network.txt)):

| Time (UTC) | Command | Hosts (requests) | Modules |
|---|---|---|---|
| 09:24:21 | `go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@latest` in `tools/` | proxy.golang.org (8) | protobuf v1.36.12 (zip already cached; checksum from the cached sumdb tiles) |
| 09:24:30 | `go get -tool github.com/bufbuild/buf/cmd/buf@latest` in `tmp/dndm/r9/bufprobe` | proxy.golang.org (238), sum.golang.org (100) | buf v1.75.0 and 88 dependencies (list in the log) |
| 09:25:03 | `go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5` in `tools/` | proxy.golang.org (16), sum.golang.org (8) | protobuf v1.36.5, the generator of the committed code, for step 2 |
| ~09:26 | `buf config migrate` (scratch copy) | **buf.build** (BSR) | resolved the old `buf.lock` WKT dep; 4 new files in `~/.cache/buf/v3/commits` |

- **The buf.build contact was not in the approval.** `buf config migrate` resolved the v1 `buf.lock` dependency against the BSR. I found this afterwards from the new cache files. No data was uploaded; it was a commit lookup for `buf.build/protocolbuffers/wellknowntypes`.
- **Guard since then:** I removed the dependency, and every later buf command ran with `HTTPS_PROXY=http://127.0.0.1:9`. A later `buf breaking` against the v1 tree tried the BSR again and failed on the guard.
- **Committed config needs no network:** it has no deps and no `buf.lock`.

`tools/buf/go.mod` was built offline from the cache (`GOPROXY=off`) after the scratch probe. Its `go.sum` holds the sumdb-verified hashes.

## Step 2: buf v2 config, WKT dep dropped ([log](dnd-r9/step2.txt))

- **Config:** root `buf.yaml` with `modules: [{path: proto}]` and no deps. The lint and breaking rules are the same as `buf config migrate`'s v1-equivalent output; `STANDARD` replaces v1's `DEFAULT`.
- **Generation:** root `buf.gen.yaml` uses a managed `go_package_prefix` (as v1 did) and the local plugin via `go tool`.
- **Embedded example:** it keeps its own v2 module, with a different prefix and output directory.
- **Regeneration with protoc-gen-go v1.36.5:** I deleted all `.pb.go` files and regenerated. **Generated diff: none**, not even the version comments (same generator; protoc stays "(unknown)" under buf).
- **Lint:** `buf lint` found 39 findings in the legacy v1 files: enum value prefixes and comments in `header.proto` and `peering.proto`, plus the package directory of `test/message.proto`.
  - **Not fixed:** renaming those values would change the generated Go API that EasyRobot uses.
  - **Instead:** `ignore_only` names those 4 rules and 3 files only. New files get every rule.
- **Breaking:** clean against an image of HEAD's protos.

## Step 3: protobuf bumped to v1.36.12 ([gates](dnd-r9/step3-gates.txt), [descriptors](dnd-r9/step3-descriptors.txt))

- **Bump:** `go get google.golang.org/protobuf@v1.36.12` ran offline from the cache (`GOPROXY=off`). The `go.mod` diff is one line; `go.sum` gains 2 lines.
- **Pin:** `tools/go.mod` moved to protoc-gen-go v1.36.12 (offline), and the code was regenerated.
- **Generated diff (7 files, +148/−235), two kinds of line only:**
  - the version comment;
  - the raw descriptor is now a `const` string literal instead of `string([]byte{0x…})`.
- **Raw descriptors:** extracted from both versions' Go literals, they are **byte-identical** in all 7 files.
- **API:** `go doc -all` for `types/core`, `types/p2p` and `types/test` is identical ([api-diff](dnd-r9/api-diff.txt)).
- **Gates:** gofmt clean; build ok; vet ok with all 3 tag sets; `go test -race -count=3 ./...` 9/9 ok; x/bus ok with both nonce tags; wire probe byte-identical.

### EasyRobot ([log](dnd-r9/easyrobot-build.txt))

EasyRobot points at dndm with `replace github.com/itohio/dndm => ../dndm` and requires protobuf v1.36.10. dndm now requires v1.36.12, so EasyRobot's `-mod=readonly` build stops with "updates to go.mod needed":

| Target | At the pin / DND-R8 | dndm now, EasyRobot as is | Scratch copy of EasyRobot with only `protobuf v1.36.12` |
|---|---|---|---|
| `cmd/manipulator` | 0 | **1** (go.mod needs update) | 0 |
| `cmd/display/destination` | 1 (opencv4) | 1 (go.mod) | 1 (opencv4) |
| `drivers/lidar` | 1 (opencv4) | 1 (go.mod) | 1 (opencv4) |
| `drivers/manipulator` | 1 (build constraints) | 1 (build constraints) | 1 (build constraints) |

- **Brief conflict:** step 3's "EasyRobot read-only build unchanged" does not hold without that EasyRobot change. EasyRobot is read-only for me.
- **The fix:** with EasyRobot's one-line bump (plus 2 `go.sum` lines, offline from the cache), every target matches the pin.
- **EasyRobot's tree:** untouched; it has the same other-peer edits as before.

## Step 4: link.proto and byte-count tests

**Schema checks** ([log](dnd-r9/step4-buf.txt))
- `proto/types/link/link.proto` is design §2's schema with the same fields, numbers and enums. Fields 7–15 and 24–31 are now `reserved`, so they cannot be taken by accident.
- I added a comment to every message, enum and enum value, because the COMMENTS lint requires them.
- `buf lint`: exit 0. `buf breaking` against an image of the step-3 tree: exit 0, since adding a file is not a break.
- `types/link/link.pb.go` is generated with protoc-gen-go v1.36.12, Go package `link`.

**nanopb options** (`link.options`)
- `payload` and `sealed` are callbacks, for zero-copy decode.
- `path` 128, `type` 96, `nonce` 32, `data` 128, `policy` 32, `text` 64.
- **Not run through nanopb:** there is no MCU target yet (DND-015).

**DATA codec** (`codec/link_frame.go`, design §3.2)
- **Decoding** with `protowire`: the payload is a sub-slice of the frame.
- **Encoding** emits fields in field-number order with zero values omitted, so the bytes equal the generated code's.
- **Functions:**
  - `AppendLinkData(b, LinkData)`;
  - `AppendLinkMessage(b, routeID, seq, msg)` uses `proto.Size` then `MarshalAppend` into the same buffer;
  - `DecodeLinkData(frame)`: route 0 means a control frame, which the caller decodes with `proto.Unmarshal`;
  - `AppendLinkLength`, `LinkDataSize`.
- **Errors:**
  - DATA mixed with a control field: `ErrBadFrame`;
  - a truncated frame, or a data-path field with the wrong wire type: `ErrBadFrame`;
  - route 0, or payload and sealed together, on encode: `ErrBadArgument`.

**Tests** (`codec/link_frame_test.go`)

| Test | Before ([log](dnd-r9/step4-before.txt)) | After ([log](dnd-r9/step4-after.txt)) |
|---|---|---|
| `TestLinkFrameOverhead`: payloads 26/62/127/128/16 383/16 384 → overhead 4/4/4/5/5/6 B; TCP prefix 1/1/2/2/3/3 B; v1 frames of the same EasyRobot routes measured at 86 and 94 B of overhead | build failed: `undefined: AppendLinkData`, `LinkData`, `AppendLinkLength`, `AppendLinkMessage`, `DecodeLinkData` | ok ×10 `-race` |
| `TestLinkFrameRouteIdBoundary`: id 1/127 → +4, 128/16 383 → +5, 16 384 → +6 | same build failure | ok |
| `TestLinkFrameRoundTrip`: DATA, +seq, sealed, sealed with ctr 0, fragment, empty, message. protowire decode = generated `Unmarshal`, and generated `Marshal` = protowire bytes. 8 control messages decode as route 0 and round-trip. Mixed frame, truncated frame and bad encoder input are rejected | same build failure | ok |
| `TestLinkFrameNoCopy`: the decoded payload aliases the frame | same build failure | ok |

- **Accidental broken-encoder run:** a mistaken edit first wrote `route_id` twice and dropped `seq`. Overhead and RouteIdBoundary then failed with +6/+7/+8/+10 B instead of +4/+5/+6, so the tests catch a broken encoder.
- **Measured in the test:** the EasyRobot state route goes from 148 B (v1) to 66 B; the target route from 120 B to 30 B.
- **Allocations** (throwaway test, deleted): with a preallocated buffer, `AppendLinkMessage`, `AppendLinkData` and `DecodeLinkData` each make 0 allocations per frame.
- **Design §8 correction:** it said "+1/+2 B with the TCP length prefix". The prefix is 3 B for frames of 16 384 B or more, so payloads of 16 379 B and up. Corrected in §8.

**Not in this round** (they need the link state machine; DND-022's remaining acceptance):
- `TestRouteIdsStaySmallAcrossReconnects`, `TestRouteIdReuseQuarantine`, `TestCounterLowBits`;
- A1, A3, A5 and A8;
- the EasyRobot cross-check with real messages.

## Final gates ([build/vet](dnd-r9/gate-build-vet.txt), [race](dnd-r9/gate-race-count3.txt))

- gofmt empty; `go build ./...` ok; `go vet` ok with the default, `dndm_nonce_counter` and `dndm_nonce_random` tags.
- `go test -race -count=3 ./...`: 9/9 packages ok; race-dedupe: 0 reports. `go test -count=10 ./...`: ok. x/bus `-race` with each nonce tag: ok.
- Wire probe: byte-identical to base.
- **API diff vs HEAD:**
  - `codec` gains `LinkData`, `AppendLinkData`, `AppendLinkMessage`, `AppendLinkLength`, `LinkDataSize` and `DecodeLinkData`;
  - new package `types/link`;
  - everything else is identical ([api-diff](dnd-r9/api-diff.txt)).
- `go.mod`: the protobuf line only. No `toolchain` line, and no tool lines in dndm's `go.mod`.

## Roadmap link to DND-commit.md

`docs/roadmap.md` (committed) links `docs/evidence/DND-commit.md`, which is still untracked. **Choice: commit it, not drop it.** It is the gated report of DND-commit/-2 that the roadmap cites. It goes in this round's docs commit.

## Proposed commits (after verify and review accept)

1. `chore(tools): pin buf v1.75.0 and protoc-gen-go as Go tools; buf v2 config at the root, WKT dep dropped [DND-R9-tools]` (was `build(...)`; rule 13 types, review L4)
   - Files: `tools/**`, buf configs, Makefiles.
   - `tools/go.mod` at protoc-gen-go **v1.36.5** in this commit, so the regenerated code (byte-identical) matches the pinned generator. The step-2 snapshot is saved in `tmp/dndm/r9/step2/`.
2. `chore(deps): protobuf v1.36.12; regenerate with protoc-gen-go v1.36.12 [DND-R9-tools]` (was `build(...)`)
   - Files: `go.mod`, `go.sum`, `tools/go.*`, the 7 `.pb.go` files.
   - Lands together with EasyRobot's bump, or after the owner accepts the gap.
3. `feat(codec): link.proto, nanopb options and the protowire DATA codec with byte-count tests [DND-022]`
   - Files: `proto/types/link/**`, `types/link/**`, `codec/link_frame*.go`.
4. `docs: DND-R9-tools: design §7/§8, DND-022, index, roadmap, report, DND-commit.md [DND-R9-tools]`

Each commit is gated on its staged tree, as for DND-R8.

## Not verified

- nanopb, protobuf-c and TinyGo generators against `link.proto` and `.options`: not run (no targets yet).
- `go mod tidy` for dndm: it cannot run offline because `gopkg.in/check.v1`, a test dependency of yaml.v3, is not in the cache. That is the same at HEAD. So `go.sum` keeps the two now-unused protobuf v1.36.10 lines. `tools/` and `tools/buf/` are tidy (`go mod tidy -diff` exit 0).
- EasyRobot with the bump: built only in a scratch copy, not in EasyRobot's tree (read-only).
- The `go tool -modfile` plugin path has only been run on this machine (macOS, go1.27.2).

## Decisions for the owner (via Leader)

1. **protobuf v1.36.12 needs EasyRobot to bump too.** Options:
   - (a) Land dndm's bump together with a one-line EasyRobot `go.mod`/`go.sum` bump, made by EasyRobot's owner peer. **Recommended:** it is a minimal change, and the scratch build matches the pin on all 4 targets.
   - (b) Keep the runtime at v1.36.10 and pin protoc-gen-go v1.36.10, deferring the bump. This is a 5-minute change, and EasyRobot needs nothing.
2. **The unapproved buf.build contact** (one commit lookup during `buf config migrate`). Disclosed; nothing to decide unless the owner wants buf's BSR access blocked in a config. For now it is guarded per command.

## Proposed mb learnings

- 2026-11-09 [tooling] [high] `buf config migrate` contacts the BSR to rewrite `buf.lock` deps (v1 shake256 commit ids → v2 b5 digests). Drop unused deps before migrating, or run buf with `HTTPS_PROXY=http://127.0.0.1:9`, if network access is limited to module downloads (dndm DND-R9).
- 2026-11-09 [go] [medium] Pin heavy CLI tools such as buf in their own nested `tools/<name>/go.mod` with `go tool -modfile=…`. Nested modules stay out of the library's graph and module zip. A tool that needs a pre-release of a shared dependency then cannot drag a sibling tool (protoc-gen-go) off the runtime's version (dndm DND-R9).
- 2026-11-09 [go] [medium] Bumping a dependency in a library that a sibling repo consumes via `replace => ../lib` breaks that repo's `-mod=readonly` build until it raises the same requirement. Check the consumer before calling a bump "unchanged" (dndm DND-R9, EasyRobot).

## Next action

Verify and review DND-R9-tools. Leader routes decision 1 to the owner, and to EasyRobot's peer for option (a). Commit as proposed after acceptance and Leader's go. Then the rest of DND-022: the link state machine, the id allocator and the counter rebuild, with tests failing first.

## DND-R9-fix (review R-DND-R9, Leader decisions L1–L4), 2026-11-09

**Result:**
- L1–L4 are done.
- L1 and the new two-control reject case each had a test that failed first.
- L2's wire bytes are unchanged, and `buf breaking` against the last committed state is clean.
- `make proto-breaking` runs offline.
- All gates pass. Not committed.

Brief: Leader message m-59e5d86db73f.

| Item | Change | Test | Before | After |
|---|---|---|---|---|
| **L1** (required) | `DecodeLinkData` returns `ErrBadFrame` for a DATA frame carrying both `payload` and `sealed` | `TestLinkFrameRoundTrip/payload_and_sealed_in_one_frame`: review's probe frame (route 5, `payload "plain"`, `sealed "sealed"`) | FAIL 3/3, `got <nil>` ([log](dnd-r9/l1-before.txt)) | ok ×10 `-race` |
| **L2** | `LinkFrame` fields 16–23 are now `oneof control`, with the same numbers and names. The decoder also rejects a frame with **more than one** control field, because generated decoding would silently keep the last one | `TestLinkFrameRoundTrip/two_control_messages_in_one_frame` (Hello then Close on the wire); the control round trips moved to the oneof API | FAIL 3/3, `got <nil>` ([log](dnd-r9/l2-before.txt)) | ok ×10 `-race` |
| **L3** | `make proto-breaking` runs `buf breaking --against '$(PROTO_BASE)'`, default `.git#ref=HEAD`, offline | scratch clone whose last commit is this tree ([log](dnd-r9/l3-proto-breaking.txt)) | — | unchanged tree: exit 0. Moving `Hello.max_frame` from field 4 to 9: "Previously present field 4 … was deleted", exit 2 |
| **L4** | Commits 1–2 use `chore(tools)` / `chore(deps)` (see "Proposed commits") | — | — | — |

**L2 wire bytes** ([log](dnd-r9/l2-wire.txt)): throwaway programs marshalled the same 8 control frames and 1 DATA frame with the pre-L2 generated code and with the oneof code. `cmp` reports them identical.

**How I handled `buf breaking` for L2:**
- Against an image of the pre-L2 draft, buf reports 8 findings, one per control field: field "moved from outside to inside a oneof". That is expected: oneof membership is part of the generated API.
- I reset the baseline instead. The draft was never committed or shipped (DND-022 in progress), and `link.proto` does not exist at HEAD or in commits 1–2.
- So the check that matters, against the step-3 image (= commit 2's tree), is clean: exit 0.
- From commit 3 on, `make proto-breaking` guards the schema against the last commit.

**`make proto-breaking` before commit 1:** it cannot run in the real repo until commit 1 lands. HEAD (`0062746`) still has the v1 config under `proto/`, so against `.git#ref=HEAD` the imports in `peering.proto` do not resolve (shown in the L3 log for `HEAD~1` of the scratch clone). The gate plan:
- commit 1: `buf generate` reproduces HEAD's `.pb.go` (no diff);
- commits 2–3: `make proto-breaking` plus the Go gates.

**Other edits:**
- design §2: schema block, the oneof design choice, and the nanopb row in §6;
- `link.options` note: nanopb generates `which_control`;
- DND-022 log.

Codec API unchanged since the first DND-R9 report. `types/link` now has `LinkFrame.Control` and `LinkFrame_Hello` etc., in place of the plain control fields.

**Gates** ([build/vet](dnd-r9/fix-gate-build-vet.txt), [race](dnd-r9/fix-gate-race-count3.txt)):
- gofmt empty; build ok; vet ok with the default, counter and random tags.
- `TestLinkFrame*` ×10 `-race` ok; `go test -count=10 ./...` ok; x/bus `-race` with each nonce tag ok.
- `go test -race -count=3 ./...`: 9/9 ok, 0 race reports.
- `buf lint` exit 0; `buf breaking` vs the step-3 image exit 0; `make proto` regenerates `link.pb.go` byte-identical.
- Wire probe byte-identical.
- buf commands ran with the network blocked.

**Network:** none.

**Commit plan:** unchanged apart from L4. Commit 2, the protobuf bump, lands together with EasyRobot's one-line bump (decision 1a); Leader coordinates the timing. L5 (`go mod tidy` when a download is approved) is left as is.

**Next action:** verify checks the whole round. Commit 1 → 4 after Leader's go, with commit 2 timed with EasyRobot.

## Commits (DND-R9-commit, Leader message m-1a0382c640d6), 2026-11-09

Each commit was gated on its exported staged tree, offline, with buf's network blocked. Gate logs are in `tmp/dndm/commit-gates/r9-c*.txt`.

| # | Hash | Message | Per-commit checks |
|---|---|---|---|
| 1 | 0bb4604 | `chore(tools): pin buf v1.75.0 and protoc-gen-go as Go tools; buf v2 config at the root, WKT dep dropped [DND-R9-tools]` | Go gate (build, vet, gofmt, `-race` 9/9). `buf lint` 0. `buf generate` reproduces all 7 committed `.pb.go` files byte for byte with protoc-gen-go v1.36.5. `tools/go.sum` holds only the v1.36.5 lines: a full tidy needs go-cmp, which is not in the cache |
| 2 | e160c3c | `chore(deps): protobuf v1.36.12; regenerate with protoc-gen-go v1.36.12 [DND-R9-tools]` | Go gate. `buf lint` 0. `buf breaking --against <repo>/.git#ref=HEAD` 0. `buf generate` reproduces the 7 staged files (v1.36.12) |
| 3 | 954bc81 | `feat(codec): link.proto, nanopb options and a zero-alloc protowire DATA codec with byte-count tests [DND-022]` | Go gate. `buf lint` 0. `buf breaking` vs the previous commit 0. `buf generate` reproduces `types/link/link.pb.go`. `TestLinkFrame*` ×10 `-race` ok. The commit body says the byte-count tests first failed as a build failure (verify's minor point) |
| 4 | (this commit) | `docs: DND-R9-tools — design §7/§8, DND-022, index, roadmap, report, DND-commit.md [DND-R9-tools]` | doc link check |

- **EasyRobot:** at commit 2, EasyRobot's `go.mod` still had protobuf v1.36.10. EasyRobot's bump is in progress in parallel (Leader). Until it lands, EasyRobot's `-mod=readonly` build stops with "updates to go.mod needed".
- **Network:** none.
