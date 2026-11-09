# Link-context wire in protobuf (design, DND-R7)

Status: **design draft, no code**. Task: `docs/tasks/DND-R7-design.md`. Owner decisions it follows ([decisions](../../../docs/decisions.md), 2026-11-08):

- the link-context model is adopted;
- protobuf schemas are the language-neutral wire contract (C++/embedded/TS use generated code);
- zones are bound per link type, crossing only through bridges with a grant;
- network links get loud `Insecure()` opt-outs;
- security profiles are never negotiated down;
- buf and protobuf tooling get updated.

**Decisions on this design** (Leader, 2026-11-09, under the owner's autonomy instruction; `docs/tasks/DND-R6-fix.md`):

1. Plain protobuf `LinkFrame` data frames everywhere (4 B + payload), with the route id in the CAN identifier on CAN links: **yes**. The raw mode of §3.1 (b) is not used.
2. Pinning buf and protoc-gen-go as Go tools: **not approved** (it needs a network download). It stays pending for the owner, so §7's update plan cannot start.
3. Legacy v1 fallback only via `Insecure("legacy v1 peer")` on a link configured as secure: **yes** (§5).
4. Simultaneous connections keep the one dialled by the lower identity: **yes** ([close-reconnect](close-reconnect.md#31-tie-break-for-simultaneous-connections-decided)).

Words follow the [diagram glossary](../diagrams/README.md#glossary): *link* is a connection over one face, *stage* is one step on the way to a face, and *bridge* is only a zone crossing with a grant.

Diagrams: [link flows](../diagrams/link-flows.md), [core](../diagrams/core.md).

## 1. Problem in numbers

Today every message frame is preamble (16 B: magic, total size, header size, message size), then a `types.core.Header` (timestamp, type, **route as text**), then the message. Verify measured EasyRobot's real routes ([DND-research](../../../docs/verification/DND-research.md)):

| Route | Payload | Frame today | Overhead |
|---|---|---|---|
| `types.control.ManipulatorState@easyrobot.manipulator.raw` | 62 B | 148 B | **86 B** |
| `types.control.ManipulatorTarget@easyrobot.manipulator.raw.target` | 26 B | 120 B | **94 B** |
| same state route, hashed (27-char id) | 62 B | 119 B | 57 B |

Most of it is the route name, which contains the fully qualified protobuf type name, repeated in every frame. The link-context model sends the name **once per link** (BIND) and then only a small number.

## 2. Schema (draft)

A new file `proto/types/link/link.proto` (package `types.link`). Today's `types.core` messages stay for legacy links (§5). Field numbers 1–15 cost a 1-byte tag, so they are reserved for the data path. Control messages use 16+.

```proto
syntax = "proto3";
package types.link;
option go_package = "github.com/itohio/dndm/types/link";

// One frame on a link-context link. Exactly one of the groups below is set:
//   DATA:    route_id != 0, payload (or ctr + sealed on a secured link), seq when the route buffers.
//   CONTROL: route_id == 0 and exactly one control field (16+).
// Faces delimit frames themselves: UDP datagram, COBS on serial, CAN frame,
// or a varint length prefix on byte streams (TCP).
message LinkFrame {
  // ---- data path (1-byte tags) ----
  uint32 route_id = 1;   // per link, per sending side, per session; 0 = control frame; kept below 128 (§4)
  bytes  payload  = 2;   // the application message, serialized protobuf
  uint32 seq      = 3;   // per route, starts at 1; 0 = absent (latest routes omit it)
  uint32 frag     = 4;   // fragment index << 1 | last, only on faces with a small MTU
  uint32 ctr      = 5;   // AEAD counter, low 7 or 14 bits per link profile (§4); 0 is sent as absent
  bytes  sealed   = 6;   // ciphertext || tag; replaces payload on secured links
  // 7..15 reserved for future data-path fields

  // ---- control (2-byte tags) ----
  Hello     hello      = 16;
  Handshake handshake  = 17;
  Announce  announce   = 18;
  Bind      bind       = 19;
  BindAck   bind_ack   = 20;
  Unbind    unbind     = 21;
  UnbindAck unbind_ack = 22;
  Close     close      = 23;
  // 24..31 reserved: repair/fetch (DND-006), capability report (DND-013)
}

enum Profile {
  PROFILE_UNSPECIFIED = 0;
  PROFILE_ONBOARD     = 1;  // trusted on-board wire, zone fixed by build config, no crypto
  PROFILE_ZONE_PSK    = 2;  // zone pre-shared key -> link keys (small radios, CAN FD, CRSF)
  PROFILE_STATIC_DH   = 3;  // ephemeral + static X25519, static key bound to the zone anchor
  PROFILE_GROUP_KEY   = 4;  // broadcast: zone group key, sender slots
  PROFILE_INSECURE    = 5;  // network link opened with Insecure("reason"); advertised as such
}

message Hello {
  uint32  version    = 1;  // link protocol version, 2 for this design
  Profile profile    = 2;  // fixed per link by config; never negotiated down
  bytes   nonce      = 3;  // fresh per session
  uint32  max_frame  = 4;  // largest frame this side accepts
  uint32  max_routes = 5;  // route table cap per direction (A8)
  uint64  session    = 6;  // random per session; echoed in Close and used by the address book (close-reconnect.md)
}

message Handshake {        // profile-specific, opaque to the core
  uint32 step  = 1;
  bytes  data  = 2;        // ephemeral key, proof, signed static key id ...
}

enum Role {
  ROLE_UNSPECIFIED = 0;
  ROLE_INTENT      = 1;    // "I can send this route"
  ROLE_INTEREST    = 2;    // "I want this route"
}

message Announce {         // replaces types.core Intent/Interest on v2 links
  Role   role     = 1;
  string path     = 2;     // zone-local path, e.g. "easyrobot.manipulator.raw"
  string type     = 3;     // full protobuf message name, e.g. "types.control.ManipulatorState"
  bool   register = 4;     // false = withdraw
}

message Bind {             // sent by the side that will send DATA on route_id
  uint32 route_id = 1;     // allocated by the sender: lowest free id, per session (§4)
  string path     = 2;
  string type     = 3;     // full name, not a short hash (verify A6d: a 4-byte hash is forgeable)
  uint32 stages   = 4;     // bit mask: latest, buffered, history, fragment, e2e-sign, e2e-seal
  bytes  policy   = 5;     // stage parameters (depth, deadline ...), schema per stage
}

enum BindStatus {
  BIND_STATUS_UNSPECIFIED = 0;
  BIND_STATUS_OK           = 1;
  BIND_STATUS_DENIED       = 2;  // route not permitted in this zone, or no grant
  BIND_STATUS_UNKNOWN_TYPE = 3;
  BIND_STATUS_BAD_POLICY   = 4;
  BIND_STATUS_TABLE_FULL   = 5;
}

message BindAck   { uint32 route_id = 1; BindStatus status = 2; }
message Unbind    { uint32 route_id = 1; }
message UnbindAck { uint32 route_id = 1; }

enum CloseReason {
  CLOSE_REASON_UNSPECIFIED = 0;
  CLOSE_REASON_NORMAL           = 1;
  CLOSE_REASON_PROFILE_MISMATCH = 2;
  CLOSE_REASON_AUTH_FAILURES    = 3;  // too many failed tags
  CLOSE_REASON_DUPLICATE        = 4;  // the other connection between this pair wins (close-reconnect.md)
  CLOSE_REASON_PROTOCOL         = 5;
}

message Close { CloseReason reason = 1; string text = 2; uint64 session = 3; }
```

Design choices:

- **Flat fields, no `oneof`, no proto3 `optional`.** The DATA path is never nested, so it pays no extra tag and length. Presence comes from zero values: `route_id` 0 means control, and `seq` starts at 1. `ctr` needs no presence, because `sealed` already marks a secured frame, so an absent `ctr` simply means low bits 0. Every generator in §6 handles this subset.
- **Direction and DND-020.** The sender of DATA allocates the ids, so each side has its own id space. A route used in both directions gets two ids, one per sender. That is the DND-020 fix expressed in the schema.
- **Announce stays separate from Bind.** Announce mirrors today's Intent/Interest (who has, who wants). Bind is sent only by the data sender, once both sides want the route.

## 3. Per-frame cost

Protobuf wire arithmetic (tag = 1 B for fields 1–15; varint length 1 B below 128, 2 B below 16 384):

| Frame | Encoding | Overhead | ManipulatorState (62 B) | ManipulatorTarget (26 B) |
|---|---|---|---|---|
| Today, plain route | preamble + Header + msg | 86–94 B | 148 B | 120 B |
| DATA on UDP / COBS serial (latest) | `route_id` (2) + `payload` tag+len (2) | **4 B** while `route_id` < 128 and the payload < 128 B; +1 B for payloads ≥ 128 B; +1 B for ids 128–16 383, which happen only with more than 127 routes bound at once on one link (§4) | 66 B | 30 B |
| same on TCP | + varint length prefix (1–2) | 5–7 B | 67 B | 31 B |
| buffered route | + `seq` (2–3) | +2–3 B | 68–69 B | 32–33 B |
| secured link, tag 8 B | `route_id` (2) + `ctr` (0–3: absent when its low bits are 0, 2 B with 7 bits, up to 3 B with 14 bits, §4) + `sealed` tag+len (2) + 8 B tag | 12–15 B (14 B typical) | 74–77 B | 38–41 B |
| CAN (route id in the CAN identifier) | `payload` tag+len only | 2 B: 6 B of data left in a classic CAN frame (8 B) | — | — |
| CAN command, authenticated, 3 B MAC | `ctr` (0–2, low 7 bits on classic CAN) + `sealed` tag+len (2) + 3 B MAC | at most 7 B: **1 B of data left** in classic CAN, always, because `ctr` never exceeds 127 there; about 51 B in CAN FD (64 B) with an 8 B tag | — | — |

So a data frame is **4 B of protobuf overhead plus the payload**, against 86–94 B today, about 95 % less for EasyRobot's routes. The id and counter rules in §4 keep it at 4 B across reconnects. The CAN and boundary rows were measured with generated code from the draft schema; at the boundaries, route_id 127 gives +4 and 128 gives +5, and ctr 127 fits a classic CAN command while ctr 128 needs 9 B. With protobuf framing, an authenticated command on classic CAN carries only 1 B of data, so authenticated commands need CAN FD in practice. The owner's CAN rule ("authenticate command routes only, authenticated data on CAN FD") still holds; it bites harder than research-2's binary-framing estimate of 4 B of data.

### 3.1 The ~2 B option (decided: not used)

The brief asks for "~2 B". Pure protobuf cannot reach that, because a `bytes` field always costs a tag and a length (2 B), and the route id costs another 2 B. Two ways to get to 1–2 B:

- **(a) Lift the route id into the face header** where the face has an addressing field, such as the CAN identifier. The frame is then `payload` only: 2 B. This stays protobuf.
- **(b) Raw DATA mode:** the frame is `varint route_id ‖ payload bytes`, the length taken from the face's delimiter. That is 1 B for ids below 128. Control frames stay full `LinkFrame` protobuf, marked by `route_id` 0. The varint is protobuf's own encoding, but a DATA frame would no longer be a parseable protobuf message, so generated code alone cannot decode it. Each runtime needs a 10-line reader.

Recommendation, **adopted** (decision 1): `LinkFrame` everywhere (4 B), plus (a) on CAN. Option (b) is not used. It would come back only as a new decision, if a measured link proves the 2–3 B matter.

### 3.2 Go hot path

- Control frames use generated code (`proto.Unmarshal`).
- DATA frames are parsed with `google.golang.org/protobuf/encoding/protowire` (ConsumeTag/ConsumeVarint/ConsumeBytes), which returns sub-slices of the frame buffer. So the payload is not copied before the application's `proto.Unmarshal`.
- Encoding uses the reverse order: compute `proto.Size(msg)`, append the tags and length, then `MarshalAppend` the message into the same buffer. That is one buffer and no copy.
- This is the same wire format as the schema, so it is not a second contract.

## 4. Binding rules (zone, security, ids)

**When BIND is allowed.** A link drops every frame except `hello`/`handshake` until its handshake has completed and the profile is confirmed (verify A1). On `PROFILE_ONBOARD` the handshake is just the Hello exchange.

**Zone per link type** (decided "zone binding per link type"):

| Face | Profile | How the zone is fixed | BIND check |
|---|---|---|---|
| I²C, SPI, on-board UART, CAN inside one robot | ONBOARD | build config: link L belongs to zone Z | route must exist in Z's table |
| CAN FD, CRSF, small radios, exposed serial | ZONE_PSK | link keys derived from Z's PSK and both nonces. A wrong zone key is caught **at the handshake**: the peer's proof does not verify, and the link closes before any BIND. Frame tags are a second line of defence | same, plus failed-tag counter → `CLOSE_REASON_AUTH_FAILURES` |
| Wi-Fi/TCP between capable nodes | STATIC_DH | peer's static key pinned or signed by Z's anchor; zone id in the transcript. A key from another zone fails at the handshake | same, plus per-device identity for grants |
| UDP broadcast | GROUP_KEY | Z's group key; route tables **per sender slot** (A3). No handshake, so a wrong key shows only as tag failures at data time | per sender slot |
| any network face with `Insecure("reason")` | INSECURE | build config, like ONBOARD, but logged loudly and advertised | as ONBOARD; command routes may refuse frames from INSECURE/ONBOARD links (verify A5: "trusted origin" flag on the route) |

**Bind handling** in the receiving core:

1. Resolve `path` and `type` in the link's zone table. Never consult another zone's table; a bridge is the only crossing.
2. Check the type against the local registration (full name compare).
3. Check the grant if the route is bridged.
4. Check the table cap and the BIND rate limit (A8).
5. Answer with `BindAck`.

DATA for an id without a `BIND_STATUS_OK` is dropped and counted.

**Id rules** (verify A3; revised after verify DND-R7 F1 so ids stay below 128):

- **Ids belong to a session.** Each new session (new HELLO) starts with an empty id table, and the sender allocates the **lowest free id** from 1. Nothing is carried over between sessions; the address book keeps no route-id state.
- **Freed ids are reused, safely.** An id is free again only after `UnbindAck`. On faces that can reorder (UDP), it also has to sit out a quarantine of 2 × the link's retry timeout before reuse, so a late DATA frame for the old route cannot land on the new one. When an id is in quarantine, the sender takes the next free id.
- **The result.** An id reaches 128, and so costs 5 B instead of 4 B, only if more than 127 routes are bound at the same time on one link, which `Hello.max_routes` normally caps lower (64 on MCUs). It does not depend on the number of reconnects or route changes.
- **Late frames from an old session** cannot be decoded as a new route:
  - on secured links (zone key, static-DH, group key), the new session has new keys, so the old frame fails its tag;
  - on stream faces (TCP, serial), the old connection ended before the new session began;
  - on OnBoard wires, a device reset loses frames in flight.
  - Residual: on an `INSECURE` datagram link (UDP with `Insecure(reason)`), a frame delayed across a reconnect could hit a reused id. That link accepts unauthenticated frames from anyone anyway, so this adds no new attack.

**Counter rules** (verify DND-R7 F2):

- The AEAD counter is per direction, per session, and starts at 1. Fresh keys per session make restarting safe.
- `ctr` carries only its low bits: **7 bits** on profiles where every byte counts (classic CAN, CRSF), **14 bits** elsewhere. The varint is therefore at most 1 or 2 B.
- The receiver rebuilds the full counter from the highest one it has accepted. It takes the value with the same low bits that is **closest** to it, either ahead (frames lost in between) or behind (a late frame that the replay window can still accept), as SecOC and DTLS do. The nonce always uses the full counter.
- Because the rebuilt value may be ahead or behind, it is unambiguous only within half the window: up to 63 frames lost or late at 7 bits, up to 8 191 at 14 bits. With a larger gap the rebuilt counter is wrong, the tags fail, and after N failures the link closes and runs a new handshake. A mis-rebuilt counter can only fail the tag; it cannot be accepted.

**Control reliability on lossy faces:**

- `Bind`, `Unbind` and `Close` are retried until acknowledged, with a per-link timeout (`Close` is best effort).
- Duplicates are idempotent: the same `route_id` with the same path and type is acknowledged again.

## 5. Compatibility with today's peers

- **Today's frames** start with the magic `0xFADABEDA` (or `0xCEBAFE4A`, headerless). Link-context frames are never sent before both sides agree, so a v1 peer never sees one.
- **Negotiation, inside the v1 handshake** (TCP/mesh only):
  - Add `uint32 link_version = 7` to `types.p2p.Handshake`. A v1 peer ignores the unknown field.
  - If INITIAL and FINAL both carry `link_version >= 2`, both sides switch the connection to v2 frames (varint length prefix + `LinkFrame`) right after FINAL. They start with `hello`.
  - Otherwise the connection stays v1.
- **Downgrade rule** (decision 3). An attacker can strip `link_version` from an unauthenticated v1 handshake. So a link configured with a secure profile **refuses to stay v1**; staying v1 needs `Insecure("legacy v1 peer")`, which is loud. Legacy v1 stays host-only during the migration (verify A5).
- **New faces** (MCU serial, CAN, CRSF, UDP broadcast) speak v2 only. They never had v1 peers.
- **API.** `Publish`/`Subscribe`/`Producer`/`Consumer` do not change. The remote endpoint picks the v1 or v2 codec per connection.

## 6. What each generator needs from the schema

| Runtime | Generator | Needs | Status in this schema |
|---|---|---|---|
| Go (host) | protoc-gen-go | — | fine; hot path via `protowire` (§3.2) |
| TinyGo | protobuf-go-lite (aperturerobotics) | proto3 without reflection; avoid `Any`, maps, WKTs | none used `#verify` generator feature list |
| C on MCUs | nanopb | an `.options` file with `max_size` for every string/bytes (`path` 128, `type` 96, `payload` per face MTU) or callbacks; proto3 subset | flat fields, no oneof/optional; `payload` as a callback field for zero-copy decode `#verify` |
| C (Linux/SBC) | protobuf-c | proto3 subset | same subset `#verify` with the protobuf-c version chosen |
| TypeScript | protobuf-es | full proto3 | fine |
| C++ (nncase etc.) | protoc C++ or EmbeddedProto | — | fine |

Schema rules for all generators:

- use `uint32` for ids and counters (no zig-zag);
- no maps, `Any` or well-known types on link messages;
- enums start with an `_UNSPECIFIED = 0`;
- field numbers 1–15 only for per-frame fields.

The `.options` file for nanopb lives next to `link.proto` and is part of the contract.

## 7. Tooling: current and target

| Item | Current (observed) | Target |
|---|---|---|
| Go runtime | `google.golang.org/protobuf v1.36.10` (go.mod) | latest stable at update time; local module cache has up to v1.36.12, v1.37.0-rc.1 not used |
| Generated code | `protoc-gen-go v1.36.5` (`types/core/header.pb.go` header), protoc "(unknown)" | regenerate with the same protoc-gen-go version as the runtime |
| buf config | `proto/buf.yaml` v1, `proto/buf.gen.yaml` v1 (plugin `go`, `paths=source_relative`), `buf.lock` pins `buf.build/protocolbuffers/wellknowntypes` | buf v2 config at repo root (`modules: [{path: proto}]`), `buf.gen.yaml` v2 |
| Well-known types dep | listed but **no `.proto` imports `google/protobuf/*`** | remove the dep and the lock entry |
| buf CLI | **not installed** on this machine (`buf: command not found`); `make proto` runs `buf generate` | pin it as a Go tool (`tool` directive in go.mod, Go 1.24+) or a pinned binary; needs owner approval (download) |
| Other generators | none | add when the matching target exists: protobuf-es (TS), nanopb (MCU), protobuf-go-lite (TinyGo); each in its own output dir |

**Update plan** (each step its own change, behind the gates):

1. Owner approves fetching buf and protoc-gen-go, a module download (**pending**: Leader did not approve it in DND-R6-fix).
2. Convert the config to v2 and drop the unused WKT dep. Regenerate with no schema change; the generated diff must be version comments only.
3. Bump `google.golang.org/protobuf` and regenerate. Gates, wire probe byte-identical.
4. Add `types/link/link.proto` plus `.options` and generate Go. Lint with `buf lint` and `buf breaking` against the previous tag.
5. Add other generators only when a target needs them (DND-015).

## 8. Byte-count test (to write when code is allowed)

`codec/link_frame_test.go`, failing first (the encoder doesn't exist yet):

- **TestLinkFrameOverhead**: encode DATA for payloads of 26 B and 62 B (the sizes of EasyRobot's ManipulatorTarget and ManipulatorState above) and 127/128/16 383/16 384 B (varint boundaries), and assert:
  - overhead = 4 B below 128 B, 5 B up to 16 383 B, 6 B above (UDP/serial framing);
  - +1/+2 B with the TCP length prefix;
  - the v1 frame for the same route names is 86/94 B, so the saving is measured in the same test.
- **TestLinkFrameRouteIdBoundary**: route_id 127 → +4 B, route_id 128 → +5 B (payload 62 B; measured 66 and 67 B with the draft schema).
- **TestRouteIdsStaySmallAcrossReconnects**: bind 10 routes, then reconnect 50 times (each a new session) and unbind/rebind routes in between. Every id stays ≤ 10, and every DATA frame stays at 4 B + payload. Verify's F1 case (ids passing 127 after about 13 reconnects) must fail under the old rule.
- **TestRouteIdReuseQuarantine**: on a reordering fake face, a freed id is not reused before `UnbindAck` plus the quarantine, and a late frame for the old route is dropped, not delivered on the new one.
- **TestCounterLowBits**: classic CAN profile with `ctr` at 7 bits, counters 127, 128, 255 and 256. The command frame is ≤ 8 B with 1 B of data every time (measured: a raw ctr 128 would need 9 B). The receiver rebuilds the full counter across the 127→128 wrap, and after a gap of 64 frames or more the rebuilt counter is wrong, so the tag fails (the link resyncs with a new handshake).
- **TestLinkFrameRoundTrip**: DATA, DATA+seq, sealed, and every control message decode back equal. Decoding DATA with `protowire` and with generated `proto.Unmarshal` gives the same fields; this guards against a second, hand-written contract.
- **TestLinkFrameNoCopy**: the decoded payload aliases the frame buffer (pointer range check).
- A scratch cross-check with real EasyRobot messages (read-only `replace`, as verify did) reports the exact frames. EasyRobot can't be imported by dndm tests, because the dependency points the other way.

## 9. Open questions

- `session` persistence across reboots on MCUs without flash wear-levelling. A fresh random session per boot is enough, because route ids and counters are per session.
- Where `stages`/`policy` schemas live: per stage message, defined with DND-006.
- `session` size and randomness on RP2040, whose ROSC RNG quality is `#verify` (research-2).
- Fragment field encoding versus a separate `fragment` stage header: decide with the CRSF/CAN target (DND-015).

[Overview](../index.md) · [close-reconnect](close-reconnect.md) · [Diagrams](../diagrams/README.md)
