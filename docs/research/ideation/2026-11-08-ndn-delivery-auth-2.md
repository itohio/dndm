---
title: "dndm layers, zones and links under embedded overhead limits (research 2)"
date: "2026-11-08"
scope: "local-only research and ideation; no code"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec (+ DND-R2..R4 working tree)"
task: "DND-R4-research-2"
status: "draft for owner accept/park"
supersedes_parts_of: "2026-11-08-ndn-delivery-auth.md cards A and D"
---

# dndm layers, zones and links under embedded overhead limits

Follow-up to [the first research note](2026-11-08-ndn-delivery-auth.md). It answers five later owner requirements (2026-11-08): crypto/DH/ZKP/signatures/replay and data-layer-sdk; zone-aware protocol with no unpermitted crossing; memory/compute/call-stack overhead with no zone bytes on every message; a C++ reference core later; adversarial probing. It also covers the later decision on a compile-time nonce generator.

**These are options and recommendations, not decisions.** No code, build, test or measurement ran. Evidence labels: **observed** (I read the code or page this session), **reported** (a spec, paper or vendor says so), **inferred** (my reasoning, gap named), `#verify` (an unmeasured number or guess).

> **Revised 2026-11-09 (DND-research-fix)** after independent verification of this note and the owner's protobuf decision (2026-11-09: the protobuf schemas are the language-neutral wire contract; no hand-written binary spec):
> - §4.2, §4.4, §6, card F and decision 6 now express the link-context model in protobuf. The schema of record is dndm's design [`docs/design/link-context-wire.md`](../../design/link-context-wire.md) (DND-R7) §2: message `types.link.LinkFrame`.
> - Today's overhead is the independently measured 86–94 B, not the earlier ~70 B estimate.
> - §8.1 adds residual risks A1–A8 found by the verification attack, each with an owning ticket.
> - S2's failsafe watchdog location is corrected.
>
> The earlier hand-written binary layout is withdrawn, not kept as an alternative.

## 1. Frame

**Requirements (2026-11-08, mandatory):**

| ID | Requirement |
|---|---|
| Z1 | The whole dndm protocol is zone aware and does not allow unpermitted zone crossing |
| Z2 | No zone bytes in every message, sync or link, nor stored with every name: 64 bits added to every name and message is too much where memory and I²C/SPI/CAN/ELRS bandwidth are at a premium (ESP32, RP2040) |
| Z3 | Abstract dndm so it is bullet-, idiot- and embedded-proof, without compromising apps where 1 GB for a hello world is acceptable |
| Z4 | Layers the developer adds (reliability, durability, encryption …), with as little boilerplate and mental load as possible; HTTP middleware / Python decorator as inspiration. Be aware of compute, memory and call-stack overhead per layer |
| Z5 | Cryptographic channels, signatures and replay-attack mitigations; DH cryptography is important. The data-layer-sdk model: the producer's public key is known out of band, and a common symmetric key is derived |
| Z6 | A C++ reference core later (very small devices, nncase), so the core and wire format must be language-neutral |
| Z7 | Probe each option for attacks: link guessing, header spoofing, TCP-style injection |
| Z8 | Compile-time pluggable nonce generator: counter from a random start / random / crypto-random |

**Decision served:** what dndm's wire core, layer API, zone mechanism and security profiles look like before dndm turns DND-006/009/010 into designs.

**Facts about today's dndm wire** (observed):

- A stream frame = 16-byte preamble (4 B magic, 4 B body length, 4 B header length, 4 B message length) + a protobuf `Header` + the protobuf message (`codec/codec.go:3-5,37-40,126-140`).
- The `Header` carries `receive_timestamp` (local-only, not sent), `timestamp`, `type`, `want_result`, an unused `bytes signature`, and `string route` (`proto/types/core/header.proto:24-31`). So **every message repeats the full route string** (`Type@path`).
- **Measured during independent verification** (scratch encoder over the dndm working tree, offline). This replaces my earlier ~70 B estimate, which was low:

  | Robotics-library route | Route id | Payload | Frame | Overhead |
  |---|---|---|---|---|
  | `types.control.ManipulatorState@acmerobot.manipulator.raw` | 56 chars | 62 B | 148 B | **86 B** |
  | `types.control.ManipulatorTarget@acmerobot.manipulator.raw.target` | 64 | 26 B | 120 B | **94 B** |
  | `types.control.ManipulatorConfig@acmerobot.manipulator.dh.config` | 63 | 0 B | 93 B | **93 B** |
  | the state route with `NewHashedRoute` | 27 | 62 B | 119 B | 57 B |

  The reason: the plain route id includes the **fully qualified protobuf type name** (`types.control.ManipulatorState@…`). Removing that per-message name is the same change that removes any zone bytes.

---

## 2. The abstraction (abstract skill)

**Purpose:** a model of dndm that makes per-message overhead near zero on small links without taking features from large nodes.

**Model: names are resolved once per link; frames carry short handles; zone and security are properties of the link context, never fields of a frame.**

1. A **zone** is a namespace plus an authority (a key or config). A node has one route table per zone.
2. A **link** joins exactly one local zone to one peer. It is bound to its zone **once**, at setup: by build-time configuration on trusted wires, or by an authenticated handshake whose keys depend on the zone credential.
3. A **route bind** is a control exchange on a link. It maps a full name (zone-local path + type) to a **short per-link route id** (1 byte for the first 128 routes). The access check (grant) runs here, once.
4. A **frame** carries only the route id, the payload (or a counter + sealed payload on secured links), and a seq or fragment field when a layer needs it. All of these are fields of one protobuf message, `types.link.LinkFrame` (§4.2). The receiver resolves the id in the link's table. An id the table doesn't know is dropped.
5. **Crossing zones** requires a **bridge**: an object holding route handles in both zones, created only from an explicit grant (export in A, import in B). Frames never pass from one zone's link to another's; the bridge re-publishes.

**Grounding (independent cases with the same structure):**
- MAVLink 2 signing: per-link 8-bit link ID, 48-bit timestamp, 48-bit truncated signature; the key is shared out of band (mavlink.io, observed).
- AUTOSAR SecOC on CAN: a truncated freshness value + truncated MAC per PDU, with keys and freshness managed outside the frame (AUTOSAR PRS, search-result excerpts: reported).
- EDHOC (RFC 9528): an authenticated DH handshake of 101 bytes total with static-DH keys referenced by kid; it establishes a context so later messages carry only small ids (observed table).
- DeftT (first note): the trust domain is fixed by provisioning, not carried per publication.
- Inside dndm, the `Intent`/`Interest` advertisements already form a per-link registration step (`proto/types/core/intents.proto`, observed).

**Boundary:** the model is cheap because the link context identifies the sender and zone. It breaks where the frame's sender is not its producer:
- multi-hop relays;
- broadcast on a shared medium;
- history copies served by third parties.

There, the cost moves to an **end-to-end layer** (per-producer signatures or AEAD, larger tags) on only the routes that need it.

**What the model loses:** global message names on the wire, so a frame can't be understood without its link context. Debugging and capture tools need the bind table.

**Falsifier:** a deployment where most traffic is relayed or broadcast and every route needs producer authenticity. Then per-message cost approaches the E2E layer's cost and the link-context saving disappears. **Validation pending:** compute bytes per message for the four scenarios in §9 with real route counts.

---

## 3. Revisiting the first note's cards

| First-note card | Status under the new constraints | Change |
|---|---|---|
| **A. name every message** (route + producer + epoch + seq on the wire) | **Superseded on the wire; kept at the boundaries** | No names in data frames. The producer is implied by the link (point to point) or by a 1-byte sender slot (broadcast). The epoch is implied by the session key (a new handshake = a new epoch). `seq` (varint) only where a delivery layer needs it. Global names `(route-hash, producer-key-id, epoch, seq)` exist only in route-bind control frames, history stores and E2E-signed records. |
| **B. three delivery classes** | Kept | Become route layers (§4.3). `latest` adds 0 B; `buffered` adds a seq varint and a repair channel; `history` adds a store and a fetch control message. |
| **C. history without DHT first** | Kept | Store index keyed by global names; fetch/repair on a link uses `(route-id, seq)` translated at the store. Signatures needed only for copies served by non-producers (E2E layer). |
| **D. zones like NATS accounts + staged auth** | **Step 0 revised** | The zone is a link property and a route-table scope, never a field. Step 0 = build/config-time link→zone binding + per-link route-bind table + grant check + unforgeable route handles + no raw-send API. Step 1 = handshake profile (PSK or static-DH) making the zone cryptographic. Step 2 = E2E layer for relays/broadcast/history. |
| **E. embedded core + optional packages** | Kept, sharpened | Compile-time composition; **protobuf schemas as the language-neutral contract** for frames and control messages (owner decision; generated code per runtime); stage-array execution for constant call depth (§4.4); a C++ core as a later ticket. |

---

## 4. Layered architecture

### 4.1 Stack

```text
 Application      Publish[T]/Subscribe[T] on a route handle           (typed; protobuf/nanopb)
 ───────────────────────────────────────────────────────────────────────────────────────────
 Route layers     latest | buffered | history | fragment | e2e-sign | e2e-seal     (per route, opt-in)
 Core (always)    zone route table · route bind · dispatcher · grant check · handles
 Link layers      secure(aead, tag) | handshake(profile) | replay window          (per link, opt-in)
 Face             serial(COBS/CRC) | tcp | udp | udp-broadcast | can | i2c | spi | crsf   (one per link)
```

**Invariant (Z1):** the core dispatcher is the only path between faces and routes. Layers transform frames inside a link or a route pipeline, but **cannot address a different link or zone**: they get no link or zone handles, only `next(frame)`. Crossing is possible only through a bridge (§5.4).

### 4.2 Core wire format: protobuf (language-neutral through generated code)

**The `.proto` schema is the wire contract**, for data frames and link control alike (owner decision). The draft is dndm's [`docs/design/link-context-wire.md`](../../design/link-context-wire.md) §2, a new `proto/types/link/link.proto`. This note does not duplicate it; the summary:

- **`LinkFrame`** has flat fields: no `oneof`, no proto3 `optional`.
  - The data path uses fields 1–15 (1-byte tags): `route_id = 1` (0 = control frame), `payload = 2` (the application's serialized protobuf message), `seq = 3`, `frag = 4`, `ctr = 5`, and `sealed = 6` (ciphertext ‖ tag, which replaces `payload` on secured links).
  - Control messages use fields 16+: `Hello` (version, profile, nonce, max frame, max routes), `Handshake` (opaque per profile), `Announce` (replaces today's Intent/Interest), `Bind`, `BindAck`, `Unbind`, `UnbindAck`, `Close`.
- **Bind messages carry the names and permissions**, once per route per link.
  - `Bind{route_id, path, type, stages, policy}` names the zone-local path and the **full** protobuf type name. A short hash would be forgeable (finding A6d), and names cost nothing here because they are sent once.
  - `BindAck{route_id, status}` returns ok / denied (no grant or not permitted in this zone) / unknown type / bad policy / table full.
  - The zone is **not** in the bind: the link already fixes it.
- **Data frames carry a small `route_id`:** a varint, 1 B below 128. Ids belong to a session: each new session starts with an empty table, and the data sender takes the **lowest free id** from 1. A freed id is reused after `UnbindAck`, plus a quarantine of 2 × the retry timeout on reordering faces. The address book keeps no route-id state. So an id reaches 128 (5 B instead of 4 B) only with more than 127 routes bound at once on one link (design §4 "Id rules", revised after DND-R7 verification finding F1).
- **Faces delimit frames themselves:** UDP datagram, COBS on serial, CAN frame, or a varint length prefix on TCP. On CAN links the route id moves into the CAN identifier, so the frame is `payload` only. This is decided (owner decision, link-context design) and still protobuf.
- **Generated code everywhere:**
  - Go: protoc-gen-go, with the hot path parsed by `protowire` (the same wire, no copy);
  - TinyGo: protobuf-go-lite;
  - C on MCUs: nanopb, with an `.options` file of `max_size` limits that is part of the contract;
  - C on Linux: protobuf-c;
  - C++: protoc C++ or EmbeddedProto;
  - TypeScript: protobuf-es.

  Schema rules (design §6): `uint32` ids and counters, no maps/`Any`/well-known types, `_UNSPECIFIED = 0` enums, fields 1–15 for per-frame fields only.

**Per-message cost in protobuf** (wire arithmetic: tag 1 B for fields 1–15; length varint 1 B below 128, 2 B below 16 384; matches design §3):

| Frame | Encoding | Overhead |
|---|---|---|
| DATA, `latest` route (UDP, COBS serial) | `route_id` tag + 1-byte id (2) + `payload` tag + length (2) | **4 B** (5 B for payloads ≥ 128 B) |
| DATA on TCP | + varint length prefix | 5–7 B |
| `buffered`/`history` route | + `seq` (2–3) | +2–3 B |
| secured link, tag *t* | `route_id` (2) + `ctr` (0–3: absent when its low bits are 0; it carries only 7 or 14 low bits, design §4 "Counter rules") + `sealed` tag + length (2) + *t* | 4–7 B + *t* (12–15 B with an 8 B tag, 14 B typical; design §3) |
| CAN, route id in the CAN identifier | `payload` tag + length | 2 B |

Against today's measured 86–94 B, that is about 95 % less for the robotics library's routes. It is 2–3 B more than the withdrawn binary layout. dndm's design §3.1 records the decision to accept those bytes rather than leave protobuf.

### 4.3 Route layers (examples)

| Layer | Adds on wire | Adds state | Notes |
|---|---|---|---|
| `latest` (default, decided) | 0 B | one slot per subscriber (message size) | drop-oldest, counted drops |
| `buffered(depth, bytes, age, flush)` | `seq` 2–3 B | sender ring + receiver gap state | gap → repair control message (unicast NACK) or state-vector repair (broadcast); field range 24..31 is reserved in `LinkFrame` (DND-006) |
| `history(store)` | `seq` 2–3 B | store index | fetch control message `(route-id, seq range)` (reserved field range) |
| `fragment` | `frag` 2 B | one reassembly buffer per route | for CAN/CRSF/I²C small MTUs |
| `e2e-sign(ed25519)` | 64 B sig + key id, inside the payload's own protobuf message | producer key table | relays, broadcast, history copies; host-class nodes |
| `e2e-seal(aead, producer key)` | tag 8–16 B + `seq` | per-producer key | the owner's data-layer-sdk model (§7), done with AD and replay checks |

### 4.4 Middleware/decorator API with minimal boilerplate (sketch, not final)

**Go (host and TinyGo):**

```go
// One link: a face, its zone, and its security profile. Zone is a value, not a string per message.
lab := dndm.Zone("lab", dndm.ZonePSK(labKey))            // or dndm.ZoneAnchor(anchorPub) for static-DH
n := dndm.NewNode(
    dndm.Link(serial.Face("/dev/ttyACM0"), lab, dndm.OnBoard()),               // trusted wire, declared
    dndm.Link(udp.Face(":7400"), lab, dndm.Secure(aead.ChaCha20Poly1305, dndm.Tag(8))),
)

// Routes: decorators read like middleware; defaults are safe (latest, zone of the link that binds it).
scan := dndm.Publish[*pb.Scan](n, lab, "lidar/scan")                              // latest
pose := dndm.Publish[*pb.Pose](n, lab, "pose", dndm.History(store.Ring(1024)))
cmd  := dndm.Subscribe[*pb.Target](n, lab, "arm/target",
    dndm.Buffered(dndm.Depth(8), dndm.Deadline(50*time.Millisecond)))

// A user-written layer is one small value; ordering is the argument order.
type metrics struct{ dndm.PassThrough }
func (m *metrics) Out(f *dndm.Frame, next dndm.Next) error { m.n++; return next(f) }
```

**C++ (reference core later; policy-based, zero-cost when a layer is absent).** Frames and control messages use the generated `types.link` code (nanopb, EmbeddedProto or protobuf-c); the C++ core hand-writes no wire layout:

```cpp
using Lab   = dndm::Zone<dndm::ZonePsk>;
using Uart  = dndm::Link<dndm::face::Serial<Cobs, Crc16>, Lab, dndm::OnBoard>;
using Radio = dndm::Link<dndm::face::Udp, Lab, dndm::Secure<dndm::Ascon128, dndm::Tag<8>>>;
dndm::Node<Uart, Radio> node{lab_key};
auto scan = node.publish<Scan>("lidar/scan");                       // Scan = generated message type
auto cmd  = node.subscribe<Target, dndm::Buffered<8>>("arm/target");
```

**Execution: decorators in the API, a flat stage array inside.** Nested closures grow the stack by one frame per layer. Instead, the builder flattens the layers into a fixed array of stage functions run in a loop, so call depth is constant: `Send → stage[i] → face.Write`. On TinyGo/C++ with layers chosen at compile time, the array is a constant and unused stages aren't linked (inferred; verify with a size build under DND-015).

**Idiot-proof defaults (Z3):**
- A **network face** (TCP/UDP/broadcast) refuses to start without `Secure(...)` unless the code says `dndm.Insecure("reason")`, which logs loudly and advertises "unauthenticated" in DND-013.
- An **on-board face** (UART/I²C/SPI) must be declared `OnBoard()` to run without AEAD.
- There is **no API that sends on a link without a route handle**, and handles come only from `Publish/Subscribe` in a zone or from a bridge grant.
- **Nonce generators** (Z8) are chosen at compile time for **correlation** nonces only. AEAD nonces are always per-direction counters under per-session keys (§8, nonce row). The two are never mixed.

---

## 5. Zone enforcement with ~zero per-message cost

### 5.1 Options compared

| Option | How the zone is fixed | Per-message bytes | Per-link setup | Bypass resistance | Fits |
|---|---|---|---|---|---|
| **Z-a Zone field in every frame** (superseded framing) | 8 B zone id/key per frame | +8 B | 0 | Only as strong as the check at each hop; a buggy layer can rewrite it | Nothing small; rejected by Z2 |
| **Z-b Build-time link→zone binding** (trusted wire) | Link declared in code/config to belong to zone Z; route table per zone | 0 | 0 | Enforced by the core (no cross-zone handles); **no defence against a device on the wire** | I²C, SPI, on-board UART, CAN inside one robot |
| **Z-c Zone-PSK handshake** | HKDF(zone PSK, both nonces, link profile) → link keys; wrong zone = wrong key = every frame fails | 0 (the AEAD tag pays for integrity anyway) | 2 msgs ≈ 2 × (2 + 16) B `#verify`; 1 HKDF | Strong against outsiders; any zone member can impersonate any other member (shared secret) | RP2040/no-ECC MCUs, CAN FD, CRSF, small radios |
| **Z-d Static-DH handshake bound to a zone anchor** (EDHOC/Noise-KK-like) | Keys from ephemeral+static X25519; peer's static key pinned or signed by the zone anchor; transcript includes the zone id | 0 | ≈ 101 B total in EDHOC's static-DH-kid example (reported); 2 X25519 per side | Strong; per-device identity and revocation; forward secrecy | ESP32-C5, SBC, host; RP2040 if ~tens of ms setup is fine |
| **Z-e Short per-link route ids** (orthogonal; used with all the above) | Ids exist only in a link's table, which belongs to one zone | 0 extra (replaces the route string) | 1 `Bind` per route per link (~10 B + path + full type name, once) | An id has no meaning outside its link, so no cross-zone confusion through ids | Everything |
| **Z-f Group key per zone for broadcast** | Zone group key (from PSK or distributed over Z-d links); a sender slot per member | + `seq` (2–3 B) + the slot (a protobuf field, ≥ 2 B; `LinkFrame` has no slot field yet: open, DND-010) | key distribution | Strong against outsiders; **no source authentication between members** without E2E signatures | UDP broadcast/multicast, shared buses |

**Recommendation:**
- Z-e everywhere, plus the face-appropriate binding: Z-b for on-board wires, Z-c for small radios/CAN FD/CRSF, Z-d for IP links between capable nodes, Z-f for broadcast.
- This meets Z2: **0 zone bytes per message on every option except the rejected Z-a**.

### 5.2 Why layers can't bypass it

The **trusted computing base** is the core: the dispatcher, the per-zone route tables and the handle mint.
- Layers are stages with `next()` only.
- Faces deliver bytes to the core of their own link only.
- A frame received on link L is resolved in L's table; there is no API to look up another zone's table.
- Route handles are opaque, unforgeable, in-process objects (Go: unexported struct pointers; C++: non-copyable types created by the node).
- **Test to write:** a malicious layer that rewrites `RouteId` to every value 0–255. Pass = it can only reach routes bound on its own link (same zone), never another zone. A paper case now; a fixture in DND-017.

### 5.3 Zone in names and storage (Z2)

- No zone in route names, route hashes or history keys: each zone has its own table or store, so the zone is the **container**, not a field.
- On a multi-zone node, memory cost is one table header per zone (~16 B `#verify`), not 8 B per name.

### 5.4 Crossing with explicit grants

```go
dndm.Bridge(n, dndm.Grant{From: lab, To: fleet, Route: "pose", As: "robots/r1/pose", Layers: dndm.Latest()})
```

- A bridge exists only if a grant names both zones. Grants live in config signed by both zone authorities (later), or in local config (step 0).
- The bridge subscribes in A and publishes in B: one copy, a re-encode, a new route id on B's links.
- It refuses wildcard grants unless an option explicitly allows them.
- This is also where the NATS gateway (DND-014) sits.

---

## 6. Costs

All numbers are `#verify` unless a source is given. "Inferred" values are arithmetic from cited figures.

### 6.1 Per layer / option

| Item | Wire per message | Wire per link setup | RAM per link | RAM per route | Flash | CPU | Call depth | Copies |
|---|---|---|---|---|---|---|---|---|
| Current dndm frame | **86–94 B** measured for the robotics library's plain routes; 57 B hashed (§1) | handshake strings | buffers per conn | route strings | Go runtime + protobuf | protobuf marshal of header | many (channels, goroutines) | encode per remote |
| Core DATA (`LinkFrame`) | **4 B** (5 B for payloads ≥ 128 B; 2 B on CAN) | 0 | table header ~16 B + ids | entry ~16 B (id, type ref, flags/ptr) `#verify` | generated code + core ~2–4 KB C++ `#verify` (nanopb size per design §6) | id lookup (array index); `protowire` parse, no copy | +1 (dispatcher) | 0 (encode the payload into the frame buffer) |
| `Bind` | — | ~10 B + path + full type name per route per link (once) | — | — | generated | full-name compare | — | — |
| Face serial (COBS + CRC16) | +~3 B (delimiter, COBS 1 per 254, CRC 2) | 0 | 1 rx frame buffer | — | ~0.5 KB `#verify` | ~1 pass | +1 | 1 (COBS) |
| Link `Secure(AEAD, tag t)` | +t (4/8/16) + `ctr` field 0–3 B (7 or 14 low bits; the receiver rebuilds the full counter, as SecOC/DTLS do; design §4 "Counter rules"); `sealed` replaces `payload` (same 2 B tag + length) | — | 2 keys (2×16–32 B) + 2 counters + 64-bit replay bitmap ≈ 100 B | 0 | ChaCha20-Poly1305 or Ascon ~1.5–3 KB `#verify` | per byte: SW ChaCha/Ascon on M0+ `#verify`; AES-GCM with ESP32-C5 HW AES (GHASH SW) `#verify` | +1 | 0 (in place) |
| Handshake PSK (Z-c) | — | ~36 B `#verify` | transient ~100 B | — | HKDF-SHA256 ~1–2 KB, or ESP32-C5 HW SHA | 2–4 HMAC | — | — |
| Handshake static-DH (Z-d) | — | ~101 B (EDHOC static DH, kid; reported) | transient ~0.5–1 KB `#verify` | — | X25519 + HKDF ~4–8 KB `#verify` | 2 X25519 per side: Cortex-M0 3,589,850 cycles per scalar mult (Düll et al. 2015, reported) → ~27 ms at 133 MHz, ~54 ms per side (inferred for RP2040, `#verify`) | — | — |
| `latest` | 0 | — | — | 1 slot × max message | ~0.3 KB | 0 | +1 | 0 (immutable after Send, DND-002) |
| `buffered(N)` | `seq` 2–3 B (+ repair control messages) | — | — | ring N × max message + gap bitmap | ~1 KB `#verify` | ring ops | +1 | 1 into ring |
| `fragment` | `frag` 2 B per fragment | — | — | 1 reassembly buffer | ~0.5 KB | — | +1 | 1 into reassembly |
| `e2e-sign(ed25519)` | +65 B | key distribution | key table | — | Ed25519 ~5–10 KB `#verify` | verify/sign ms on MCU `#verify`; µs on host (DeftT table, first note) | +1 | 0 |
| Broadcast group key (Z-f) | + slot field (≥ 2 B, open) + `seq` 2–3 B + `ctr` + tag | key distribution | per-sender replay state (bitmap per sender) | — | as AEAD | as AEAD | +1 | 0 |

### 6.2 Per link type

| Link | Usable payload per frame | Recommended profile | Per-message overhead (core + security) | Notes |
|---|---|---|---|---|
| **Classic CAN** (8 B data) | 8 B | Z-b inside one robot; authenticated **commands need CAN FD** (decided after the DND-R6-fix measurement) | 2 B (route id in the CAN identifier, `payload` tag + length) → 6 B data. Authenticated command: `ctr` 0–2 (7 low bits) + `sealed` 2 + 3 B MAC ≤ 7 B → **1 B data** (design §3) | SecOC-like truncation trades forgery resistance for space (AUTOSAR PRS, reported). With a 3 B MAC, forgery needs lockout (§8.1 A2) |
| **CAN FD** (64 B) | 64 B | Z-c PSK + 8 B tag | route id in the CAN identifier: `ctr` 0–3 (14 low bits) + `sealed` 2 + 8 B tag = 10–13 B → about 51 B data (design §3) | |
| **I²C** | transaction-sized | Z-b only (on-board) | 4 B `LinkFrame`; whether to lift the route id into the register/command byte, as on CAN, is open `#verify` (DND-015) | Master-polled; crypto on a probe-able on-board bus buys little (inferred) |
| **SPI** | transaction-sized | Z-b only | as I²C | Same as I²C |
| **ELRS/CRSF** | 60 B payload, 64 B frame (observed in an FPV simulator's CRSF codec) | Z-c PSK, tag 4 B; `latest` only | `route_id` 2 + `ctr` 0–2 (7 low bits) + `sealed` 2 + 4 B tag = 8–10 B → 50–52 B app data | **RC channels stay native CRSF under the FPV simulator's rig failsafe watchdog;** dndm carries telemetry/aux only. ELRS's own link protection is outside dndm (`#verify` what ELRS provides) |
| **UART/USB serial** | any | Z-b on-board, else Z-c | 4 B + ~3 B COBS/CRC (or the AEAD tag instead of the CRC) | Today's robotics-library serial drivers |
| **UDP/Wi-Fi** (ESP32-C5) | ~1400 B `#verify` | Z-d (+ Z-f for broadcast) | 4–7 B + 8–16 B tag (14 B typical with 8 B) | ESP32-C5 has AES, SHA, HMAC, ECC and ECDSA accelerators (datasheet/TRM search excerpts, reported). Which curves the ECC unit supports is `#verify`; P-256 may be the HW-friendly choice there |
| **TCP (host)** | stream | Z-d; optionally E2E | 4–7 B + length prefix (1–2 B) + 16 B tag | Hosts can enable every layer: Z3's "1 GB hello world" side |

**Target resources** (reported): RP2040 = dual Cortex-M0+ @133 MHz, 264 KB SRAM, no crypto accelerator (datasheet snippets); its RNG is ROSC-based, and its quality for keys is `#verify`. ESP32-C5 = HP RISC-V up to 240 MHz with crypto accelerators; SRAM size `#verify`.

---

## 7. data-layer-sdk channel notes

**Observed** in `github.com/Synternet/data-layer-sdk`, MIT, at `main` and tag `v0.2.0` (the version pinned by a downstream consumer):

| Fact | Source |
|---|---|
| Ed25519 identity per service; identity = base58(public key) | `pkg/service/service.go` (Configure) |
| `makeMsg` signs **only the payload**; adds NATS headers `identity`, `signature` (base64), `timestamp` (decimal ns) | `service.go` makeMsg, both versions |
| `Verify` with an empty `KnownPublicKeys` and missing identity/signature **returns nil (accepts unsigned)**; with no known keys it trusts the self-asserted identity | `service.go` Verify (main) |
| `KnownPublicKeys` = allowlist of producer keys | `pkg/options/options.go` |
| **No DH key agreement or payload encryption** found in `main` or `v0.2.0` service/options code | the files read; repo file list. The derived common symmetric key described in requirement Z5 is not in this public code (it may live elsewhere; `#verify` with the owner) |

**Lessons for dndm:**

1. **Bind the signature or MAC to the route, sequence/time and payload** (AEAD associated data). Signing the payload alone allows replay and relabelling onto another subject; the timestamp header is unsigned.
2. **No silent downgrade.** If a route or link requires auth, unsigned frames are rejected. MAVLink documents the same hazard: "all packets that do not meet the … unsigned packet acceptance rules must be rejected (otherwise there is no benefit gained from signing)".
3. **Authenticity is not authorisation.** A valid signature from an unknown key must fail closed when a key list exists; key lists are per zone.
4. **Binary, not text headers:** ~150+ B of text headers per message is a host-only luxury (inferred from base58 + base64 + decimal field sizes).
5. **The owner's out-of-band producer-key model** maps to Noise-style "K"/"KK" patterns: the subscriber knows the producer's static key. Add an ephemeral DH for forward secrecy, as EDHOC does. In dndm it is the `e2e-seal` route layer (producer-to-subscriber across relays/history) or the Z-d link handshake (per hop).

---

## 8. Threat model per option

Options: **O1** trusted wire (Z-b, no crypto) · **O2** zone PSK + link AEAD (Z-c) · **O3** static-DH handshake + link AEAD (Z-d) · **O4** broadcast group key (Z-f) · **O5** E2E producer signatures/seal on top of any.

| Threat | O1 trusted wire | O2 PSK AEAD | O3 static-DH AEAD | O4 group key | O5 E2E |
|---|---|---|---|---|---|
| **Link/route-id guessing** (send on a guessed id) | Succeeds for anyone on the wire. Ids are tiny and public, never secrets. **Residual: high** if the wire is reachable | Fails: frames without the key fail the tag. Residual: forgery chance 2^-8t per try for tag t bytes (4 B → 2^-32), so rate-limit/close after N failures | Same as O2 | Outsiders fail; **any member can send on any id** | Unauthorised producer fails at the subscriber |
| **Session hijack / TCP-style injection** | Succeeds | Injected frames fail; an attacker can still kill the transport (RST/flood) → DoS only | Same + per-device keys | Outsiders fail | Content forgery fails end to end |
| **Frame injection** | Succeeds | Fails (AEAD) | Fails | Members can inject as others (no source auth) | Fails for signed routes |
| **Header spoofing** (kind, route id, seq, flags) | Succeeds | Fails: all header bytes are AEAD associated data | Fails | Members only | Covered if AD includes route + seq |
| **Replay** | Succeeds | Rejected by the per-direction counter + 64-bit window; counters restart safely because every handshake makes new keys | Same | Needs a per-sender epoch persisted across reboot (flash counter; MAVLink keeps a persistent timestamp) — **residual: replay across reboot if the epoch isn't persisted** | Seq/time inside the signed AD |
| **Downgrade** to an unauthenticated layer | n/a | Prevented only if the profile is fixed by config per link/zone and never offered down; the handshake transcript includes the profile. **Residual: misconfiguration** (an `Insecure()` left in) | Same | Same | A route marked "signed required" rejects unsigned |
| **Zone confusion** (frames or routes leak across zones) | Prevented by the core if layers can't address other links (§5.2). **Residual: a core bug** | + wrong zone = wrong key | + the transcript binds the zone id | Group key per zone | Producer key lists per zone |
| **Nonce prediction** (forge a reply by predicting a correlation nonce; DND-005 finding) | Possible with a counter generator: use random/crypto-random on hosts (owner's Z8). Residual on MCUs with counters: in-zone forgery | Outsiders can't inject, so prediction doesn't help them; insiders could → crypto-random where several processes share a link | Same | Members could | Signed replies |
| **AEAD nonce reuse** (catastrophic for GCM/ChaCha) | n/a | Avoided: counters under fresh per-session keys; **never use the pluggable random generator for AEAD nonces** | Same | **Highest risk:** a shared key across many senders → sender-slot + epoch + counter nonce construction mandatory | n/a |
| **Compromised member** | Total | Can impersonate any zone member (shared PSK) → rotate the zone key | Contained to its own identity; revoke its static key | Can forge any member's broadcast | Contained to its producer key |
| **DoS / resource exhaustion** | Bounded by DND-001/007 caps | + failed-tag counters, close the link after N | + handshake rate-limit (X25519 cost on MCUs) | + per-sender replay-state caps | Signature verify cost per frame → only on routes that need it |

### 8.1 Residual risks added after the verification attack (A1–A8)

These come from the independent verification attack on the link-context model. dndm's link-context design (DND-R7) already adopts several of the mitigations; the ticket column names who owns the rest.

| # | Attack | Residual and mitigation | Owner ticket |
|---|---|---|---|
| A1 | **Route-id guessing or injection before bind** (DATA or BIND on a link whose handshake hasn't finished) | Without a rule, a pre-handshake window on O2/O3 faces behaves like O1. **Rule:** a link drops every frame except `hello`/`handshake` until its handshake completes and the profile is confirmed (design §4). Cap HELLO/HS processing per peer and per second, because X25519 is costly on MCUs | link-context wire core (dndm design DND-R7 → its implementation ticket); DND-004 (handshake replay); DND-015 for MCU costs |
| A2 | **Route-id guessing after bind** on classic CAN | A 3 B MAC gives 2^-24 per try: at ~2,000 frames/s, about 2.3 h expected per forged command (inferred). **Failed-MAC counters with lockout and an alarm are mandatory on CAN**, not optional; authenticated commands need CAN FD (decided) | CAN face ticket (after DND-015 names targets); DND-016 measurement |
| A3 | **Rebind hijack and id aliasing**: re-bind an id to another route, or reuse an id so late frames land on the new route | O1: anyone on the wire can rebind. O4 broadcast: route tables must be **per sender slot**, or one member's BIND silently retargets other members' subscribers. All profiles (design §4 "Id rules", revised): ids belong to a session and start from 1 (lowest free id); a freed id is reused only after `UnbindAck`, plus a quarantine of 2 × the retry timeout on reordering faces; no route-id state across sessions. Late frames from an old session fail the new keys on secured links, can't arrive on stream faces, and are lost on OnBoard resets. **Residual:** an `INSECURE` datagram link can mis-decode a delayed frame onto a reused id, which adds nothing to what that link already accepts. `Bind`/`Unbind` are retried until acknowledged and are idempotent | link-context core ticket (to be filed, N5); DND-008 (lifecycle, close-reconnect design); DND-010 → DND-011 (per-slot tables for broadcast) |
| A4 | **Replay across link resets** | O2/O3 are safe when both sides contribute fresh nonces to the key derivation. **Z-c (zone PSK) has no forward secrecy:** a leaked zone PSK decrypts every recorded past frame of every link in that zone. Z-d has forward secrecy. Choose Z-d wherever the target can afford X25519 | DND-009 (profile choice per target); DND-015 (X25519 cost) |
| A5 | **Downgrade to the trusted-wire profile (O1); a zone is only as strong as its weakest link** | A zone's route table is shared by all its links, so frames injected on one O1 link (a UART declared `OnBoard()` but really exposed: a debug header, USB-serial, a transparent telemetry radio) are forwarded over O2/O3 links. The core can't detect physical exposure. Mitigations: (a) a per-route **"trusted origin" flag**, so command routes refuse frames that entered via ONBOARD/INSECURE links; or (b) separate zones for O1 wires, joined through a bridge with a grant. The **legacy v1 protobuf face** falls under the same rule. Staying v1 on a secure link needs `Insecure("legacy v1 peer")` (decided: owner decision, design decision 3). v1 stays host-only during migration (design §5 text, not an owner decision) | DND-009 (trusted-origin flag); DND-017 (profile gateway boundary); migration in the link-context core ticket (to be filed, N5) |
| A6 | **Zone confusion at a bridge** | (a) A bridge re-encodes, so an `e2e-sign` route must pass the producer's signature through untouched, or B's consumers trust the bridge instead of the producer. (b) Request/reply needs grants in both directions, and repair/fetch control messages must be checked against the grant, or a history fetch through the bridge reaches beyond the granted route. (c) The bridge holds handles in both zones (a confused deputy): it needs the §5.2 malicious-layer test. (d) Grants are on **path + full type name** (`Bind.type` is the full name, not a 4-byte hash) | DND-014 (scoped NATS bridge and bridges in general); DND-017 fixtures; DND-006 (repair/fetch) |
| A7 | **Sender-slot collision in group mode (O4)** | Two devices on one slot (misprovisioned, or a malicious member copying another's slot and counter) reuse an **AEAD nonce under the shared group key**. With GCM or ChaCha20-Poly1305 that leaks keystream and allows universal forgery by anyone who sees both frames, outsiders included. Mitigate with slot assignment bound to provisioning **and a nonce-misuse-resistant AEAD for group mode** (AES-GCM-SIV or a SIV-style construction) | DND-010 → DND-011 (broadcast); DND-015 (AEAD per target) |
| A8 | **Table and handle exhaustion** (BIND flood) | On O1 and O4, anyone (or any member) can fill the route table. Per-link route-table cap advertised in `Hello.max_routes`, a BIND rate limit, and `BIND_STATUS_TABLE_FULL` (design §2, §4) | link-context core ticket; DND-001/DND-007 (bounds) |

**Adversarial probes for verification** (requested by the owner):
- link-id sweep on O1 and O2;
- header bit-flip fuzzing;
- replay of captured frames after reboot (O2/O4);
- forced profile mismatch;
- malicious-layer RouteId rewrite (§5.2);
- correlation-nonce prediction on a counter build.

Added after the verification review:
- **A1:** DATA/BIND sent before the handshake completes → dropped and counted;
- **A3:** rebind of a live id, a late frame after unbind and rebind, and a BIND from one broadcast sender slot targeting another's table → no retargeting;
- **A5:** a frame injected on an ONBOARD link reaching a command route that requires trusted origin → refused, and a legacy v1 peer on a secure-configured link without `Insecure` → refused;
- **A7:** two senders on the same group slot → detected; with a SIV-style AEAD, no keystream leak.

---

## 9. Robotics scenarios for Mermaid diagrams (dndm draws, diagram review follows)

| # | Scenario | Diagram type | What it must show |
|---|---|---|---|
| S1 | **Lidar → SLAM → planner → motor controller** across RP2040 (lidar, serial), SBC (SLAM/planner), host (map viewer over Wi-Fi) | flowchart LR with subgraphs per device | Links and faces (UART on-board Z-b; Wi-Fi Z-d); one zone `robot1`; route layers per arrow (scan: latest; pose: history; target: buffered); the local motor failsafe outside dndm; per-hop overhead labels (UART 4 B + COBS/CRC; Wi-Fi 12–15 B, 14 B typical, with an 8 B tag) |
| S2 | **Teleop over ELRS with failsafe** | flowchart + small timing note | Pilot → gateway → rig → ELRS TX module → RX → drive controller as native CRSF RC (not dndm); dndm carrying telemetry/aux in CRSF frames (60 B payload, Z-c, latest). **The failsafe watchdog sits on the rig, before the ELRS TX module** (150 ms stale + 100 ms hold, then neutral frames with drive-enable low, or no RC frames in `stop` mode; the FPV simulator's rig), **not on the robot**. The robot's receiver has its own failsafe, which applies when RC frames stop (`stop` mode, rig panic, radio loss). What happens when dndm telemetry stops: nothing unsafe |
| S3 | **Multi-robot zones with a bridge** | flowchart with three zone subgraphs | Zones `r1`, `r2`, `base`; a bridge node with explicit grants (`r1:pose → base:robots/r1/pose`); a denied crossing attempt (`r2:arm/target` from `base` without a grant) drawn as a blocked edge; the NATS gateway at `base` |
| S4 | **Sensor history replay** | sequenceDiagram | Producer ring on the MCU, near-consumer history store on the SBC, consumer joining late: fetch range → partial hit → missing part from another holder → producer fallback; E2E signature check on copies served by non-producers |
| S5 | **Link bring-up** | sequenceDiagram | `Hello` → `Handshake` (PSK or static-DH) → keys → `Bind` for each route (path + full type name → id, grant check) → DATA as `LinkFrame{route_id, payload}`; frames other than hello/handshake before the handshake completes are dropped (§8.1 A1); failure branches: wrong zone (handshake proof fails, link closes before any Bind), unknown route (`BindAck` denied), profile mismatch (`Close`) |
| S6 | **Wi-Fi broadcast of features with repair** | sequenceDiagram or flowchart | Group-key frames with sender slot + seq; one receiver misses seq 41; state-vector/NACK repair; per-sender replay windows |
| S7 | **CAN actuator bus** | flowchart + frame layout | Route ids mapped to CAN ids (frame = `payload` only, 2 B); telemetry unauthenticated on the internal bus (Z-b); authenticated command routes on CAN FD (`ctr` + `sealed`, failed-MAC lockout, §8.1 A2); the gateway to the SBC as a bridge |
| S8 | **Build-time composition** | flowchart TB (layer stack) | The same app code compiled for RP2040 (core + serial + latest), ESP32-C5 (core + udp + secure + buffered), host (all + e2e + history + NATS bridge); which stages are linked in each |
| S9 | **Attack paths** | flowchart | Attacker on the serial wire, on Wi-Fi, inside a zone; where each is stopped per profile (O1–O5) and what remains |

---

## 10. Candidate cards

### F. Link-context wire core in protobuf: names bound once per link, 4-byte data frames

```text
Idea and purpose: personal utility + OSS. Replace per-message route strings (86–94 B of
  overhead today) with per-link route ids bound once; data frames are protobuf
  `types.link.LinkFrame{route_id, payload}`: 4 B + payload.
Serves: Z2, Z6, DND-001/006/010/013 groundwork. Mandatory: no zone/name bytes per message;
  protobuf schemas are the wire contract (owner).
Mechanism: §4.2; schema of record dndm docs/design/link-context-wire.md §2 (LinkFrame,
  Hello, Handshake, Announce, Bind/BindAck, Unbind/UnbindAck, Close); generated code per
  runtime (protoc-gen-go, protobuf-go-lite, nanopb, protobuf-c, EmbeddedProto,
  protobuf-es).
Reuse/prior art: MAVLink 2 link IDs, SecOC, EDHOC connection identifiers, HPACK-like
  indexing (inferred analogy).
Evidence: independently measured 86–94 B overhead per frame for a robotics library's plain routes (57 B
  hashed), because the route id includes the fully qualified proto type name; protobuf
  wire arithmetic gives 4 B for LinkFrame DATA (design §3).
Must be true: every face can carry link control (route_id 0); reconnect rebinds (ids
  are per link session, lowest free id, reused after UnbindAck + quarantine, design §4).
Falsifier: a measured scenario where Bind churn (many short-lived routes) costs more
  than the per-message saving.
Next test: design §8 byte-count and round-trip tests (TestLinkFrameOverhead,
  TestLinkFrameRoundTrip) when code is allowed.
Unknowns: nanopb zero-copy payload decode via callback #verify; buf/protoc-gen-go
  tooling update needs a download (pending owner approval).
Recommendation: adopt (adopted; schema in dndm's design). Legacy v1 framing stays a
  host-only face during migration and only via Insecure("legacy v1 peer") on links
  configured as secure (§8.1 A5).
Decision: owner adopted the link-context model; protobuf framing decided.
```

### G. Zone = link context + per-zone table; crossing only through grant-bound bridges

```text
Idea and purpose: Z1 + Z2. The zone is fixed per link (build-time, PSK or static-DH
  handshake) and per route table; layers can't address other links; a bridge with a
  grant is the only crossing.
Serves: Z1, Z2, DND-009/017/014.
Mechanism: §5. Face-appropriate binding: Z-b on-board, Z-c small radios/CAN FD/CRSF,
  Z-d IP between capable nodes, Z-f broadcast.
Reuse/prior art: NATS accounts + export/import (first note); DeftT trust domains;
  EDHOC; Noise KK.
Evidence: inferred design; costs in §6.
Must be true: the core is small enough to audit; handles are unforgeable in Go and C++.
Falsifier: the malicious-layer test (§5.2) reaches another zone's route.
Next test: paper trace of the §8 probes for each option, then DND-017 fixtures.
  ~0.5 day paper.
Recommendation: adopt; Step 0 = Z-b + tables + handles + grants (no crypto); Step 1 =
  Z-c and Z-d profiles.
Decision: pending (owner).
```

### H. Compile-time layer composition with a decorator API and a flat stage array

```text
Idea and purpose: Z4, Z3. Middleware-style options per link and route; the builder
  flattens them into a fixed stage array (constant call depth); unused layers aren't
  linked; insecure network faces need an explicit, loud opt-out.
Serves: Z3, Z4, Z6, Z8.
Mechanism: §4.4 (Go options/generics; C++ policy templates).
Evidence: inferred; stage-array call depth and dead-code elimination to be measured.
Must be true: TinyGo removes unused layer packages; C++ templates inline stages.
Falsifier: a size build where an unused layer still costs >1 KB flash, or stack per
  Send grows with layer count.
Next test: TinyGo + C++ size/stack builds for 0, 2, 5 layers on the DND-015 target.
Recommendation: adopt.
Decision: pending (owner).
```

### I. Security profiles chosen per face, never negotiated down

```text
Idea and purpose: Z5, Z7. Five profiles O1-O5 (§8), chosen in config per link/route;
  handshake transcripts include the profile; unsigned/unauthenticated frames never
  accepted where a profile requires auth; AEAD nonces = counters under fresh session
  keys; correlation nonces from the owner's compile-time generator.
Serves: Z5, Z7, Z8, DND-009/010.
Mechanism: §5.1, §6, §8; data-layer-sdk lessons §7.
Evidence: MAVLink signing rules, EDHOC sizes, Curve25519 MCU cycles (reported).
Must be true: one AEAD per target that is constant-time and small (ChaCha20-Poly1305
  or Ascon on RP2040; AES-GCM with HW on ESP32-C5) #verify; key provisioning per zone
  is practical for the owner (PSK file / anchor key).
Falsifier: X25519 handshake time or code size breaks a target's budget (then that
  target uses PSK only).
Next test: microbenchmarks on RP2040 and ESP32-C5 with owner approval (board use):
  AEAD cycles/byte, X25519 ms, flash/RAM. DND-016.
Unknowns: ZKP use (pairing by short code, e.g. a PAKE; later); ELRS's own protection.
Recommendation: adopt O1-O4 now as designs; O5 for host-class and history routes.
Decision: pending (owner).
```

---

## 11. Knowledge map

### Known knowns

| Fact | Evidence |
|---|---|
| Today's frame: 16 B preamble + protobuf header with the route string per message | observed `codec/codec.go:3-5,37-40`; `header.proto:24-31` |
| Today's overhead for a robotics library's routes: 86–94 B plain, 57 B hashed (route id includes the fully qualified proto type name) | measured during independent verification |
| LinkFrame DATA = 4 B + payload in protobuf | protobuf wire arithmetic; dndm `docs/design/link-context-wire.md` §3 |
| CRSF frame ≤ 64 B, payload ≤ 60 B | observed in an FPV simulator's CRSF codec |
| MAVLink 2 signing: 13 B (link id 1, timestamp 6, truncated SHA-256 6), persistent timestamp rules, explicit unsigned-acceptance rules | mavlink.io, read 2026-11-08 |
| EDHOC static-DH kid handshake = 37 + 45 + 19 = 101 B | RFC 9528 §1.2 Table 1 |
| X25519 on Cortex-M0: 3,589,850 cycles per scalar mult | Düll et al. 2015 |
| data-layer-sdk signs payload only; accepts unsigned with no known keys; no DH **in the files read** (`service.go`, options, message, service_pub/sub at main and v0.2.0; not a repo-wide search) | observed code, main + v0.2.0 |
| RP2040: dual M0+ @133 MHz, 264 KB SRAM | datasheet search excerpts |
| ESP32-C5: AES/SHA/HMAC/ECC/ECDSA accelerators | datasheet/TRM search excerpts |

### Known unknowns → owning ticket

| Question | Ticket |
|---|---|
| Bytes per message for the new core on real links, measured | design §8 tests (link-context core ticket); DND-016 (measurement) |
| Bind churn, reconnect rebind, id exhaustion behaviour | link-context core ticket; DND-008 (lifecycle, `docs/design/close-reconnect.md`) |
| Residual risks A1–A8 (§8.1) | owner tickets per row in §8.1 |
| AEAD choice per target, cycles/byte, flash; X25519/P-256 times on ESP32-C5 (HW ECC curves) and RP2040 | DND-015, DND-016 |
| Zone key provisioning (PSK files, anchor keys, rotation) | DND-009 → DND-017 |
| Broadcast epoch persistence (flash wear) and sender-slot assignment | DND-010 → DND-011 |
| Handle unforgeability and the "no raw send" API in Go and C++ | DND-017; **new ticket "C++ reference core"** (Z6) |
| Policy/profile advertisement and mismatch handling | DND-013 |
| Correlation nonce generator interface (counter/random/crypto) | DND-005 follow-up |
| CAN/CAN FD and CRSF faces | new face tickets after DND-015 names targets |
| Where the owner's data-layer channel encryption lives | ask the owner (DND-009) |

### Unknown knowns (assumptions, knowledge elsewhere)

- **"Zones must be cryptographic to be enforced."** No: on on-board wires, enforcement is the core's table/handle structure (Z-b). Crypto defends against devices on the wire, which on I²C/SPI means someone with a probe on the board. → DND-009.
- **"Route ids could be secret capabilities."** They are 1-byte public handles; capability comes from link keys and grants. The old hashed-route capability claim is withdrawn (unkeyed hash, first note). → DND-009.
- **"One nonce generator for everything."** Correlation nonces and AEAD nonces have opposite needs (unpredictable vs unique). → DND-005, DND-009.
- **An FPV simulator already has a 64-byte-frame codec with zero allocations** (CRSF) and **a robotics library has its own CRSF library**. A dndm CRSF face should reuse one of them, not a third. → new face ticket.
- **MAVLink and SecOC are robotics/automotive-proven patterns** for exactly this link-context security. Use their rules (persistent freshness, explicit unsigned policy) rather than inventing. → DND-009/010.

### Unknown unknowns (probes, proposed)

- **Reboot during a handshake on an MCU** without RTC: counter/epoch state. Probe: power-cycle fixture (DND-016, board permission).
- **Bridge loops** (r1 → base → r1). Probe: grant-graph cycle check at bridge creation.
- **Multi-path duplicates** (same frame via CAN and Wi-Fi bridges). Probe: E2E seq dedup on history routes.
- **Timing side channels** in MCU crypto (non-constant-time AES fallback). Probe: use reviewed constant-time libraries only; list them in DND-015.

---

## 12. Decisions for the owner

| # | Decision | Recommendation | Owner decision |
|---|---|---|---|
| 1 | Adopt the link-context model: no zone/name bytes in data frames; per-link route ids bound once (card F) | Yes | **adopted** (now; further hardening to be explored later) |
| 2 | Zone binding per face: build-time (on-board), PSK (small radios/CAN FD/CRSF), static-DH (IP), group key (broadcast); crossing only through grant-bound bridges (card G) | Yes; Step 0 without crypto first | yes |
| 3 | Compile-time layers via a decorator API with flat execution; network faces refuse to run insecure without a loud opt-out (card H) | Yes | yes |
| 4 | Security profiles O1–O5, never negotiated down; AEAD nonces are counters, never the pluggable generator (card I); group mode uses a nonce-misuse-resistant AEAD (§8.1 A7) | Yes | yes (A7 addition new) |
| 5 | Classic CAN: authenticate command routes only, and move authenticated data to CAN FD | Yes | yes; then tightened: authenticated commands need CAN FD (1 B left on classic CAN with protobuf framing) |
| 6 | **Protobuf schemas are the language-neutral wire contract** (`types.link`); C++/embedded via generated code (nanopb, protobuf-c, EmbeddedProto), TinyGo via protobuf-go-lite, TypeScript via protobuf-es; C++ reference core as a later ticket; update buf/protobuf tooling (download pending approval) | Yes | **owner decision** (replaces this note's earlier "fixed binary, no protobuf") |

## Sources

Web (read-only, 2026-11-08):

- data-layer-sdk:
  - repo: <https://github.com/Synternet/data-layer-sdk> (GitHub API via the fetch tool)
  - main: [service_pub.go](https://raw.githubusercontent.com/Synternet/data-layer-sdk/main/pkg/service/service_pub.go), [message.go](https://raw.githubusercontent.com/Synternet/data-layer-sdk/main/pkg/service/message.go), [service_sub.go](https://raw.githubusercontent.com/Synternet/data-layer-sdk/main/pkg/service/service_sub.go), [service.go](https://raw.githubusercontent.com/Synternet/data-layer-sdk/main/pkg/service/service.go), [options.go](https://raw.githubusercontent.com/Synternet/data-layer-sdk/main/pkg/options/options.go)
  - v0.2.0: [service.go](https://raw.githubusercontent.com/Synternet/data-layer-sdk/v0.2.0/pkg/service/service.go)
  - [tags](https://api.github.com/repos/Synternet/data-layer-sdk/tags)
- MAVLink message signing: <https://mavlink.io/en/guide/message_signing.html>
- RFC 9528 EDHOC: <https://www.rfc-editor.org/rfc/rfc9528.txt>
- Düll et al., High-speed Curve25519 on 8/16/32-bit microcontrollers: <https://cryptojedi.org/papers/mu25519-20150417.pdf> (ePrint <https://eprint.iacr.org/2015/343>)
- Search-result excerpts only (not opened):
  - AUTOSAR SecOC protocol: <https://www.autosar.org/fileadmin/standards/R24-11/FO/AUTOSAR_FO_PRS_SecOcProtocol.pdf>
  - ESP32-C5 datasheet/TRM: <https://documentation.espressif.com/esp32-c5_datasheet_en.pdf>
  - RP2040 datasheet: <https://pip.raspberrypi.com/documents/RP-008371-DS-1-rp2040-datasheet.pdf>
  - NIST SP 800-232 (Ascon): <https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-232.ipd.pdf>

Local:

- the owner decisions of 2026-11-08 and 2026-11-09 (restated in §1, §12)
- `codec/codec.go`, `proto/types/core/{header,intents}.proto`, [ticket index](../../tickets/index.md)
- [`docs/design/link-context-wire.md`](../../design/link-context-wire.md) (DND-R7: protobuf schema of record, costs, binding rules)
- the independent verification of this note (measured frame sizes, attack table A1–A8)
- An FPV simulator: failsafe watchdog on the rig; CRSF codec
- [first research note](2026-11-08-ndn-delivery-auth.md)
