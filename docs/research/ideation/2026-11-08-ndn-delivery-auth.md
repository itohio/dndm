---
title: "NDN prior art for dndm delivery classes, zones and authentication"
date: "2026-11-08"
scope: "local-only research and ideation; no code"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
task: "DND-R4-research (roster)"
owner: "ideation"
status: "draft for owner accept/park"
---

# NDN prior art for dndm: delivery classes, zones and authentication

Roster task DND-R4-research. These are **options and recommendations, not decisions**. Only the owner accepts or parks them; dndm turns accepted ones into designs and tickets. No code, build, test or experiment ran for this note.

Evidence labels used below: **observed** (I read the file or page this session), **reported** (a paper, spec or vendor says it; not reproduced), **inferred** (my reasoning from named facts), **hypothesis** (not established). Budgets and sizes not taken from a source carry `#verify`.

## 1. Frame

**Decision served:** turn the owner's 2026-11-08 decisions on DND-006, DND-009 and DND-010 (verbatim in `docs/roster.md`, "Owner decisions on dndm") into design inputs before any code.

**Owner requirements (mandatory, from those words):**

- R1 `latest` is the default class.
- R2 A buffered/reliable class exists, for dropped links, lost UDP packets, network errors and bursty links that can't send right away. Buffers are probably configurable.
- R3 Durable history exists: a consumer can ask for already-produced data, so the producer doesn't have to produce it again. History can live anywhere, including partial copies near consumers. Recovering missing parts may or may not need a DHT.
- R4 Zones exist, closer to NATS accounts. NDN permissions are the reference ("the inspiration for dndm").
- R5 Authentication is needed. Take the first steps now; full auth comes "when we are ready".
- R6 When a device doesn't need every part, the implementation must fit embedded devices: super light, modular, expandable.
- R7 Packet broadcast gets built (DND-010).

**Non-goals here:** code, ticket IDs, roadmap edits, picking a board.

**Conflicts with the existing DND-R2 memos, which the owner's words override:**

| Memo recommendation | Owner decision | Consequence |
|---|---|---|
| DND-006 Option A: "Durable commands ... refused" | R3: durable history is needed | Split two kinds of durability: durable **data history** (new dndm scope) and durable **commands/effects** (DND-012, which stays on NATS/application storage). See card C. |
| DND-009 Option C: no keys or crypto in dndm; NATS gateway between zones | R4 + R5: zones in dndm, first auth steps | Keep C's gateway for crossing zones. Add a zone model plus auth seams to dndm itself. See card D. |
| DND-010 Option A: no-go | R7: build broadcast | Broadcast on an untrusted segment needs per-sender authentication (DND-010 memo, Option C). Sequence trusted-segment broadcast first, then signatures. See cards A and D. |

## 2. What dndm has today (observed at the pin)

These files are unmodified in the working tree (`git status` was clean for them; HEAD `c7b5b61`).

| Fact | Source |
|---|---|
| A route is `Type@path` (plain) or `prefix#base64(hash)` (hashed). The hash "allows for Object-capability security model where only those who know exact path and exact type can send and decode" | `route.go:41-45,72-74,125-126` |
| The hash is unkeyed SHA-1 of `Type@path`, so anyone who knows type and path can compute it | DND-009 memo, `route.go:113-116` (reported by that memo; I didn't reread those lines) |
| `Header` has `timestamp`, `type`, `want_result`, `bytes signature = 5`, `string route = 6`. **No per-message sequence, producer id or epoch** | `proto/types/core/header.proto:24-31` |
| `Handshake` has `me`, `you`, intents, interests and an unused `bytes public_key = 6` | `proto/types/p2p/peering.proto:16-23` |
| `Intent` carries `route`, `hops`, `ttl`, `register` | `proto/types/core/intents.proto:11-16` |
| The design docs name NDN as an inspiration: "applications talk about *what data* they want, not *where* it lives" | `DESIGN.md:5`, `README.md:7`, `x/SPEC.md:55` |
| Selecting a forwarding endpoint uses a lexical `HasPrefix` | DND-009 memo, `peer.go:76-77` |
| Every route gets the same unstated policy: the producer blocks while the subscriber silently drops. One global queue size, default 1 | DND-006 memo table |

**The gap that matters most (inferred):** dndm names **routes**, not **individual messages**. A message has no stable name of its own: there is no producer id, epoch or sequence number. Without that, a receiver can't detect a loss, ask for a retransmission, deduplicate a relayed copy, fetch history by name, or check that a signature covers a specific message. NDN, SVS, DeftT and DDS all give each sample an identity. Section 5 builds on this.

## 3. Prior art: extraction

Each row gives the mechanism, the setup, what was shown, and what limits the transfer to dndm.

### 3.1 NDN security: signed Data, key locators and trust schemas

| Work | Mechanism | Shown / status | Transfer to dndm |
|---|---|---|---|
| NDN Packet Format v0.3, Signature ([spec](https://docs.named-data.net/NDN-packet-spec/current/signature.html), read 2026-11-08) | Every Data packet carries `SignatureInfo` (type, optional `KeyLocator`, `SignatureTime`) and `SignatureValue` over Name…Content. Signature types: DigestSha256 (integrity only), RSA, **ECDSA P-256 (mandatory to support, ≤72 B)**, **HMAC-SHA256 (32 B)**, **Ed25519 (64 B)**. A KeyLocator is a name of the certificate Data, or a key digest. A signed Interest adds `SignatureNonce`, `SignatureTime` or `SignatureSeqNum` against replay | Spec (observed) | dndm already reserves `Header.signature`. What's missing: a signed portion that covers **route + per-message name + payload**, a key locator field, and a replay field. HMAC is the cheap MCU option inside a zone that shares a key; Ed25519 is the option for per-sender provenance. |
| NDN 101, Trust Model ([page](https://101.named-data.net/security/trust-model/), read 2026-11-08) | Identity = a hierarchical name; certificates are Data signed by a parent; consumers fetch the chain through KeyLocators up to a local trust anchor. **Policy = trust schema**: rules "what data name ⇐ which key name" with wildcards and captured variables, enforced automatically by libraries. Works offline, with no identity provider | Docs (observed); grounded in Yu et al. 2015 ([paper](https://named-data.net/wp-content/uploads/2015/11/schematizing_trust_ndn.pdf), not read in full) | dndm routes are already hierarchical paths. A schema rule can tie "who may publish `zone/robot1/lidar/*`" to "keys named `zone/robot1/KEY/*`", with no ACL lists per peer. That is the NDN permission model the owner refers to. |
| Light VerSec (LVS), python-ndn ([docs](https://python-ndn.readthedocs.io/en/latest/src/lvs/lvs.html), read 2026-11-08) | Rules `#rule: name-pattern & {constraints} <= #signer-rule`; the text is **compiled to a binary model**; a `Checker(model).check(data_name, key_name)` answers allowed or not. A pattern variable carries across the signing chain, so `post/author` must be signed by that same `author` | Docs (observed). Also in NDNts `@ndn/lvs` ([docs](https://ndnts-docs.ndn.today/typedoc/modules/_ndn_lvs.html)) and the ndnd Go stack ([repo doc](https://github.com/named-data/ndnd/blob/main/docs/security-util.md), search result only) | The binary-model split fits embedded use: compile on the host, ship a small rule table, run only a matcher on the device. Its size on an MCU is `#verify`. A Go reference exists in ndnd (not read). |
| VerSec / DCT / DeftT ([draft-nichols-iotops-defined-trust-transport-06](https://datatracker.ietf.org/doc/draft-nichols-iotops-defined-trust-transport/06), 2024-06, expired I-D; read 2026-11-08) | A **trust domain** = members provisioned only with their own identity bundle plus a schema **certificate signed by the domain trust anchor**. The domain id = SHA-256 thumbprint of the schema cert. Validation happens at the *subscriber* against schema rules. **Broadcast/multicast many-to-many** over UDP or IPv6 link-local multicast; reliability through **syncps** set reconciliation (IBLT) of named Publications with lifetimes. Sigmgrs: EdDSA, AEAD (group key from an elected keymaker), AEADSGN. Expired publications dropped; timestamp + expiry window prevents replay | Draft, not standard. Reported: "minimum expected capability is that of an ESP32"; "~300 bytes per certificate"; EdDSA signing 15–21 µs + per-KB on the tested platforms (Table 4) | **The closest prior art to dndm's goals:** brokerless pub/sub, broadcast, embedded, schema-based authorisation, no CA. Two direct transfers: (a) zone id = thumbprint of the zone's signed rule set; (b) reliability by syncing *named* items. Difference: DeftT is many-to-many collection sync with lifetimes. dndm also needs local zero-copy fan-out and typed protobuf routes. |
| NAC and NAC-ABE ([Zhang et al., MILCOM 2018](https://users.cs.fiu.edu/~afanasyev/assets/papers/zhang2018nac.pdf), read 2026-11-08) | Confidentiality by **naming keys**: an access manager publishes a KEK per data-name *granularity* and a KDK encrypted per authorised decryptor; producers encrypt content with a symmetric CK wrapped by the KEK. Keys are ordinary named Data, so caches and repos can hold them. Revocation = short-lived keys plus rotation; data already cached stays readable. NAC-ABE replaces per-decryptor RSA with CP-ABE attributes | Paper (observed); packet sizes reported (KDK ≈1.7 kbit RSA, ≈3.6 kbit ABE) | Encryption is **later** (the owner asked for authentication, not confidentiality). The transferable idea: **encrypt at production, so history copies held anywhere (R3) stay protected at rest.** ABE is heavy for MCUs (inferred; the paper calls the crypto expensive for IoT). |

### 3.2 NATS accounts, for aligning zones

| Work | Mechanism | Transfer |
|---|---|---|
| NATS accounts ([docs](https://docs.nats.io/learn/security/accounts-and-multitenancy), read 2026-11-08) | "An account is an isolated tenant with its own subject space"; the same subject name in two accounts is two different subjects; the boundary is "absolute"; crossing only via **export/import** | dndm **zone** ≈ account: its own route namespace; nothing crosses except routes explicitly exported by one zone and imported by another. |
| NATS decentralized JWT auth ([docs](https://docs.nats.io/learn/security/decentralized-auth), read 2026-11-08) | Chain **operator → account → user**, each an Ed25519 NKey; JWTs signed by the parent. At connect the server sends a nonce; the client signs it with its seed (proof of possession); the server verifies user→account→operator. Permissions travel in the JWT, set by **scoped signing keys** (role templates, e.g. pub `orders.>`, sub `_INBOX.>`). Revocation entries in the account JWT; expiry on credentials | Maps one-to-one onto NDN's anchor → intermediate → leaf, but checks *connections*, not *data*. For dndm: the nonce proof-of-possession handshake is the smallest real auth step (card D, step 1). Scoped keys ≈ trust-schema rules expressed as role templates. |

**Zones: NATS and NDN compared (inferred synthesis)**

| Concept | NATS | NDN / DeftT | Proposed dndm |
|---|---|---|---|
| Root of trust | operator key | trust anchor; DeftT schema cert signed by the anchor | zone anchor key |
| Tenant / domain | account (subject space) | name prefix + schema; DeftT domain id = schema thumbprint | **zone id** = hash of the zone's signed rule set (or anchor key) |
| Member identity | user NKey, JWT | certificate Data named under the prefix | peer key; id = key fingerprint |
| Permission | pub/sub subject allow/deny in JWT or scoped key | trust schema: data-name pattern ⇐ key-name pattern | route-pattern rules: publish/subscribe ⇐ key pattern; compiled table |
| Enforcement point | server, at connect and per publish | every consumer, per Data packet | **both:** link admission (handshake) **and**, later, per-message verification for broadcast/relayed data |
| Crossing | export/import | none in DeftT (relays forward valid publications between subnets) | explicit export/import route list; NATS gateway (DND-014) for crossing into NATS |
| Revocation | revocation list in the account JWT; expiry | short-lived certs and keys | short-lived peer certs + zone revocation list (later) |

The key difference (inferred): NATS trusts the **broker connection**. NDN trusts the **data**. dndm needs both. Point-to-point TCP/serial links can rely on link admission. Broadcast, relays and history copies "stored anywhere" (R3, R7) need data-level signatures, because the sender of a copy is not its producer.

### 3.3 Sync, repos and caching (for buffered and durable delivery)

| Work | Mechanism | Transfer |
|---|---|---|
| State Vector Sync v3 ([spec](https://named-data.github.io/StateVectorSync/Specification.html), updated 2026-07-17, read 2026-11-08) | Each producer names its publications `/<node>/<group>/t=<bootstrap>/seq=<n>`. A **state vector** of (node, bootstrap time, latest seq) is multicast on every new publication and periodically (default 30 s ±10 %), with suppression (200 ms) to avoid floods. A receiver that sees a higher seq **fetches the gap by name**. Bootstrap time handles a node restart that lost its counter | **Directly fits the buffered-reliable class over lossy broadcast:** loss detection = a vector compare; repair = fetch the missing names from whoever holds them. Needs per-message names (§2 gap). Steady-state cost: one vector message per period per group plus one per publication. |
| PSync ([repo](https://github.com/named-data/PSync), read 2026-11-08) | IBLT of producer state; **partial sync**: a consumer sends a Bloom filter of the subscribed producers | Subset subscription without a full vector; IBLT difference cost ∝ difference size. Heavier than SVS for small groups (inferred). |
| DeftT syncps (draft above) | IBLT cState broadcast; members answer with the publications others lack; ≤ one cAdd per member per cState; total traffic "strictly upper bounded by the application-level publication rate" (reported) | A bounded-traffic reliable broadcast that needs no receiver acks to producers. |
| NDN storage ([NDN 101](https://101.named-data.net/storage/), read 2026-11-08) | Three tiers: application storage, **repos** (general-purpose, *untrusted* by the application, rendezvous when peers aren't online together), **content store** (opportunistic cache in each forwarder, no guarantees). Data is immutable; names are never reused | **Durable history = immutable named messages held by any tier.** Integrity comes from the producer signature, so the holder need not be trusted. Repo protocol: [repo-ng spec](https://redmine.named-data.net/projects/repo-ng/wiki/Repo_Protocol_Specification) (search result only). |
| Zenoh storages and queryables ([abstractions](https://zenoh.io/docs/manual/abstractions/), read 2026-11-08) | A **storage** subscribes to a key expression, stores the values, and answers queries with the latest per key. A **queryable** is a computation registered at a key expression. A hybrid logical clock timestamps each value | A model already in production for "history anywhere": storages placed near consumers. A queryable maps to "ask a service for a result". Zenoh-pico targets constrained devices (vendor claim; footprint not read, `#verify`). |
| ROS 2 / DDS QoS ([docs](https://docs.ros.org/en/rolling/ROS-Framework/interfaces/topics/About-Quality-of-Service-Settings.html), read 2026-11-08) | History KEEP_LAST(N)/KEEP_ALL; Reliability BEST_EFFORT/RELIABLE; Durability VOLATILE/TRANSIENT_LOCAL ("publisher responsible for persisting samples for late-joining subscriptions"); Deadline; **Lifespan** (expired samples dropped); **request-vs-offered compatibility** (a subscriber requests a minimum, a publisher offers a maximum; no match means no connection plus an event) | **The policy vocabulary for R1–R3**, already familiar from robotics (dndm cites ROS as an inspiration). Request-vs-offered is a ready rule for DND-013 negotiation. |
| DTN Bundle Protocol v7 ([RFC 9171](https://www.rfc-editor.org/rfc/rfc9171), read 2026-11-08) | Store-carry-forward overlay; copes when "sender and receiver are not concurrently present"; uses scheduled/opportunistic contacts; bundle lifetime and expiration; fragmentation | Prior art for **bursty links** (R2): queue until a contact window, expire by lifetime. Full BP is far too much for dndm; take only "hold with lifetime, flush on contact" (inferred). |
| NATS JetStream KV ([docs](https://docs.nats.io/learn/key-value/), read 2026-11-08) | A bucket is a stream; `get` = the last message on a subject; history/revisions; TTL | Shows that "latest" and "history" are two views of one log. Also the existing **command** durability path (DND-012), which should stay there. |
| Kademlia DHT ([Maymounkov & Mazières 2002](https://link.springer.com/chapter/10.1007/3-540-45748-8_5), search result only) | XOR-metric routing; O(log n) lookup of which nodes hold a key | Needed only when holders are many, membership is open or unknown, and no broadcast domain exists. For a robot LAN or zone of tens of nodes, holders can advertise coverage directly (card C). Hypothesis: a DHT is unnecessary in dndm's first durable module. |
| NDN-Lite ([repo](https://github.com/named-data-iot/ndn-lite), read 2026-11-08) | A C11 NDN stack for IoT, with **app-support modules** (`access-control`, `ndn-trust-schema`, `pub-sub`, `repo`, `security-bootstrapping`, `service-discovery`) separate from encode/forwarder/security backends (micro-ecc); ported to RIOT and nRF52840 | Evidence that the **modular split** (core codec + forwarder; optional trust schema, repo, pub/sub) works on MCUs. Memory figures: the NDN-Lite pub/sub TR ([PDF](https://named-data.net/wp-content/uploads/2021/01/NDN-TR-0071-ndn-lite-pubsub.pdf)) reports latency and memory overhead, but the fetch redirected and I didn't read the numbers, so `#verify`. |

## 4. Mapping onto dndm (inferred)

| NDN / prior-art concept | dndm today | Mapping |
|---|---|---|
| Name prefix | route path (`easyrobot.manipulator.raw`, EasyRobot `cmd/manipulator/main.go:240`, `getIntentPath`; cited by the DND-009 memo) | Keep it. Segment separator is still open (DND-009 residual). Exact-segment match is required regardless. |
| Data type | protobuf type in `Type@path` | Keep. NDN carries type in the content; dndm's typed routes are stronger. |
| Data packet (named, signed, immutable) | message on a route with no per-message name | **Add a message name:** `route + producer + epoch + seq` (card A). |
| Interest (pull one Data) | none; dndm Interest is a long-lived subscription | Add **fetch-by-name** for history and repair only (cards B, C). Live traffic stays push. |
| Sync (SVS/PSync) | Intent/Interest advertisement and readiness notify | Add a per-route **state vector** message for reliable and broadcast routes (card B). |
| Content store / repo | none | `history` module with a pluggable store (card C). |
| KeyLocator + signature | `Header.signature` reserved, unused | Fill when signing lands; add a key-locator field (card D). |
| Trust schema | none; route hash claims object-capability but is computable | Zone rule table: route patterns ⇐ key patterns (card D, later step). If hashed routes stay as a capability, they need a **keyed** hash (HMAC with a zone secret); an unkeyed hash is not a capability (inferred). |
| Trust domain / account | none | `zone` (card D). |
| Broadcast face | none (DND-010) | UDP broadcast/multicast face carrying named messages plus state vectors (cards A, B). |

## 5. Deconstruction

**Actors:** producers (sensors, services), consumers (controllers, loggers), relays/bridges, history holders (producer itself, a near-consumer cache, a dedicated store), zone authority (owner-run anchor), gateway to NATS (DND-014), the owner (sets zones and keys).

**Mechanisms needed by R1–R7:**

1. **Message identity** (route, producer, epoch, seq). Needed by R2, R3, R5 (signing a specific message), R7 (dedup on broadcast).
2. **Delivery policy per route**: history depth, reliability, durability, lifespan, overflow, flush mode. Needed by R1–R3.
3. **Gap detection and repair**: state vector compare, then fetch by name. Needed by R2 and R7.
4. **History store and lookup**: put/get by name or range; coverage advertisement. Needed by R3.
5. **Zone boundary**: zone id, exact-segment routes, export/import list. Needed by R4.
6. **Authentication**: link proof-of-possession now or soon; per-message signatures later. Needed by R5 and R7.
7. **Module boundaries**: core vs optional packages; compile-time exclusion. Needed by R6.

**Constraints:** additive wire changes only, unless the owner accepts a break (proto3 additive fields are ignored by old peers; inferred from proto3 semantics, not tested on dndm); EasyRobot importers must still build (DND-R2 rule); bounded memory in every class (DND-006 acceptance); no hidden durable command semantics (DND-012).

**Assumptions to challenge:**

- "Reliable" means acks to the producer. With SVS/syncps, reliability can be **receiver-driven repair** from any holder, with no producer acks.
- History needs a DHT. At zone scale, coverage advertisement may be enough (card C).
- Auth requires certificates and a CA. NATS and DeftT both show anchor-signed chains without a public CA; step 1 needs only pinned keys.
- Broadcast saves work end to end. It saves sender transmissions on one segment. Receivers still process everything; Wi-Fi multicast rates are low (DND-010 memo; DeftT draft notes 802.11 multicast inefficiency).
- A durable "ask for result" equals a durable command. A result fetched by name is **idempotent data**. A command causes an effect. They need different authorities (DND-012).

**Failure modes:** unbounded holder memory; replay of old signed messages (needs an epoch/time window, as in DeftT and SVS bootstrap time); a seq reset after reboot without an epoch, so the consumer sees "old" seqs; storms of state vectors (needs suppression); a forged producer id on broadcast before signatures exist; history copies outliving revocation (NAC notes cached data stays readable).

## 6. Options (diverge)

Four mechanisms for R2/R3, simplest first:

1. **Per-subscriber bounded queue only:** the DND-006 memo's `reliable`, blocking with a deadline. It handles a slow consumer, not link loss, bursts or history.
2. **Sender-side retransmit ring plus NACK** (DDS RELIABLE + KEEP_LAST). It handles loss and short outages per link. History is limited to ring depth at the producer.
3. **Named messages plus state-vector sync plus fetch-by-name from any holder** (SVS/DeftT style). It handles loss, broadcast, relays and history near consumers.
4. **Content-addressed segments plus DHT** (IPFS/Kademlia style). It handles large blobs and open membership, at high complexity.

Three mechanisms for R4/R5:

- **a.** Link admission only (NATS-like).
- **b.** Per-message signatures plus trust schema (NDN-like).
- **c.** a now, b later, sharing one key and identity model.

## 7. Candidate cards (converge)

### A. Per-message names: one primitive under DND-006, 009, 010 and 012

```text
Idea and purpose: personal utility + OSS. Give every message a name:
  route + producer id + epoch + seq (+ optional timestamp). One primitive serves
  loss detection, dedup, repair, history lookup, broadcast and signatures.
Serves: R2, R3, R5, R7. Mandatory groundwork; doesn't replace any class decision.
Mechanism: additive Header fields (e.g. producer, epoch, seq); seq per
  (producer, route); epoch = boot/bootstrap time as in SVS so a reboot that loses
  the counter can't collide. Local in-process fan-out can skip the fields (zero
  cost) and stamp them only at the remote/packet boundary.
Reuse/prior art: NDN naming (version/segment conventions); SVS
  /<node>/<group>/t=<bootstrap>/seq=<n>; DeftT publications with timestamps and
  lifetimes; DDS sample identity.
Evidence: observed: Header lacks these fields (header.proto:24-31).
  Reported: SVS/DeftT use exactly this. Inferred: additive proto3 fields are
  wire-compatible with old peers.
Must be true: producer id is stable per zone (not the self-declared `me` string
  once auth exists); seq width and epoch survive reboot on an MCU without flash
  wear (epoch from RTC or a boot counter #verify).
Falsifier: an old peer rejects frames that carry the new fields; or per-message
  overhead exceeds an agreed share of small sensor messages (e.g. >10 % of a
  32 B payload #verify).
Next test (proposed, needs dndm code permission): add fields behind a version
  flag; round-trip old<->new peers over the existing TCP remote endpoint in an
  in-memory test; measure encoded size delta. ~0.5 day #verify. Pass: old peer
  decodes, new peer detects an injected gap.
Unknowns: does any EasyRobot importer depend on Header layout? (dndm checks the
  importers); route-scoped vs producer-scoped seq when one producer has many routes.
Recommendation: do this first; it unblocks B, C, D-step-2 and DND-010.
Decision: pending (owner).
```

### B. Three delivery classes as policies: `latest` (default), `buffered` (configurable), `history`

```text
Idea and purpose: DND-006 as a per-route policy object with DDS-style fields and
  request-vs-offered matching, not a fixed enum:
    latest   = KEEP_LAST(1), best effort, drop-oldest, counted drops (R1)
    buffered = KEEP_LAST(N) or byte/age-bounded ring, gap detection via card A,
               repair via NACK (unicast) or state vector + fetch (broadcast),
               flush mode {immediate | batch window | on link-up}, lifespan
               expiry, overflow {drop-oldest | block-with-deadline | reject} (R2)
    history  = buffered + a history store (card C) answering fetch-by-name/range (R3)
Serves: R1-R3; owner's "perhaps configurable".
Mechanism: the producer offers a policy, the subscriber requests a minimum; link
  only if compatible, else an explicit "incompatible policy" event (DDS rule;
  advertised through DND-013). Queues stay bounded in messages AND bytes.
Reuse/prior art: ROS 2/DDS QoS (history, reliability, durability, lifespan,
  compatibility); DTN "hold until contact, expire by lifetime" for bursty links;
  SVS for broadcast repair.
Evidence: observed: options.go per-route options are empty (DND-006 memo).
  Reported: DDS semantics. Hypothesis: three classes cover EasyRobot's state
  (100 ms) and command routes.
Must be true: the policy fits in a few bytes on the wire for DND-013;
  repair traffic stays bounded (one outstanding repair per gap, suppression).
Falsifier: a real EasyRobot route needs semantics outside the three (e.g.
  ordered-exactly-once effects) -> that belongs to DND-012/NATS, not here.
Next test (proposed): paper transition table per class for loss, burst, link
  drop 5 s, reconnect, slow consumer, producer restart (epoch change); then a
  deterministic in-memory lossy face test after permission. ~1 day #verify.
Unknowns: drop-oldest vs drop-newest for large frames (DND-006 residual);
  default N for `buffered`; whether block-with-deadline is ever allowed on a
  remote read loop (DND-007 head-of-line issue).
Recommendation: adopt the policy object; `latest` default as decided; ship
  `buffered` with unicast NACK first, broadcast repair with DND-010.
Decision: pending (owner). DND-006 memo's "durable refused" is superseded by R3.
```

### C. Durable history without a DHT first: holders advertise coverage inside a zone

```text
Idea and purpose: R3. Messages on `history` routes are immutable named data
  (card A). Any node may hold any subset: the producer (ring/flash), a store near
  consumers, a dedicated store. A consumer asks "route R, producer P, seq a..b"
  or "latest k". Whoever holds it answers; the producer is the fallback, so it
  needn't produce again.
Serves: R3 including partial copies; resolves "may or may not require DHT".
Mechanism: holders advertise a compact coverage summary per (route, producer):
  seq ranges, or an IBLT/Bloom digest for large sets, piggybacked on the
  state-vector message of card B. A consumer picks holders from the summaries;
  repair of a missing part = fetch those names from another holder. Integrity
  needs no trusted holder once messages are signed (card D step 2). Until then,
  holders are trusted-zone members only.
  Large blobs (maps, model weights): content-addressed segments plus a signed
  manifest (hash list). Streams stay seq-named.
  DHT: a later module, behind a revisit trigger: holder count or open membership
  makes zone-wide advertisement too costly, or history must cross segments with
  no relay.
Reuse/prior art: NDN repos + content store (untrusted storage, immutable names);
  Zenoh storages + queryables; SVS/PSync/DeftT for coverage digests; Kademlia
  only for the deferred module.
Evidence: reported: NDN/Zenoh run this model. Hypothesis: zone sizes are tens of
  nodes #verify with the owner.
Must be true: durable history is DATA, not COMMANDS. Commands keep DND-012's
  NATS/application authority; replaying history never re-executes an effect.
Falsifier: the owner's "consumer asks, service needn't produce" turns out to mean
  parameterised computations whose inputs can't be named deterministically; then a
  queryable/memoisation design (result named by input digest) is needed instead.
Next test (proposed): owner names 2-3 concrete history use cases (e.g. lidar scan
  replay, map tiles, last-known state on join); paper check that each is a
  seq-range or hash fetch. 30 min with the owner.
Unknowns: retention and eviction per store; flash endurance on devices;
  erasure/revocation for copies held elsewhere (NAC: cached data stays readable).
Recommendation: build the store interface + producer-local ring + one host store;
  defer DHT. Needs a new ticket (dndm/product allocates the ID); keep DND-012 for
  commands.
Decision: pending (owner).
```

### D. Zones like NATS accounts, permissions like NDN trust schemas; the smallest first auth step

```text
Idea and purpose: R4 + R5. A zone = an isolated route namespace with an id, an
  anchor key and a rule table; routes cross only through an explicit export/import
  list or the NATS gateway (DND-014). Auth arrives in steps that share one
  identity model.
Serves: R4, R5, and per-sender auth required by R7 on untrusted segments.
Mechanism, staged:
  Step 0 (first steps now, no crypto): Zone type + zone id in Handshake; exact
    path-segment matching; default-deny route allowlist per zone through the
    existing beforeLink hook; an Authenticator interface (null impl) at link
    admission and a Signer/Verifier interface (null impl) at the message
    boundary; key-locator + epoch/seq fields reserved (card A). Advertised as
    "unauthenticated" via DND-013. Behaviour change limited to DND-017 rules.
  Step 1 (when the owner says ready): static Ed25519 key per peer; peer id =
    key fingerprint; Handshake.public_key filled; challenge-response nonce
    signature (as NATS connect) proves possession; per-zone pinned key list.
    Same for serial and TCP (no TLS needed). Links admitted only for pinned keys.
  Step 2: zone anchor signs peer certs with role/route-pattern claims (NATS
    scoped keys ~ NDN schema rules); compiled rule table checked on both
    subscribe/publish admission; per-message signatures (Ed25519 or zone HMAC)
    with key locator for broadcast, relays and history copies.
  Step 3 (later, optional): encryption at production (NAC idea; AEAD group key
    as DeftT) so history held anywhere stays confidential.
Reuse/prior art: NATS accounts, nonce proof, scoped signing keys, revocation;
  NDN signature/KeyLocator; LVS compiled binary model; DeftT domain id =
  schema-cert thumbprint, subscriber-side validation; fpv-arena's key-fingerprint
  ids (ideation portfolio note, reported).
Evidence: observed: public_key and signature fields exist unused
  (peering.proto:22, header.proto:29); handshake trusts `me` (DND-009 memo).
  Reported: Ed25519 sig 64 B, P-256 <=72 B, HMAC 32 B (NDN spec); DeftT ~300 B per
  cert, ESP32 as minimum target.
Must be true: Step 0 adds no runtime cost when the null authenticator is used;
  Step 1 crypto fits the smallest named target (verify-only builds possible on
  devices that only consume) #verify in DND-015.
Falsifier: Step 1 handshake latency or code size breaks the RP2040/ESP32 serial
  drivers' budget #verify; or the owner's adversary model includes compromised
  in-zone processes (then link auth alone is not enough; jump to Step 2).
Next test (proposed): DND-009 paper cases (profile.a vs profile.ab, spoofed me,
  forged route hash, revoked key) traced through Step 0 and Step 1. ~0.5 day.
  Pass: every case has a named decision point and deny result.
Unknowns: who holds the zone anchor key (owner? per-robot?); revocation latency
  target; whether hashed routes stay (then keyed hash with the zone secret).
Recommendation: implement Step 0 now as "first steps"; keep DND-009 Option C's
  gateway for crossing into NATS; Step 1 is the first real auth when the owner
  approves.
Decision: pending (owner).
```

### E. Modular layering that fits embedded

```text
Idea and purpose: R6. A core that compiles alone on a device; everything else is
  an optional package the linker drops when unused.
Mechanism: core interfaces only; modules depend on core, never on each other
  except as listed below.
Reuse/prior art: NDN-Lite (codec/forwarder core; app-support modules for trust
  schema, repo, pub/sub, access control); DeftT (sigmgr, distributors and faces as
  plug-in modules); LVS (compile on host, ship binary rules).
Evidence: inferred; dndm's current protobuf reflection use on TinyGo is unknown
  (DND-015).
Must be true: core avoids reflection-heavy paths on device builds or they work
  under TinyGo #verify; module selection by import, not runtime flags.
Falsifier: the core binary for the smallest target exceeds an owner-set flash/RAM
  ceiling with null modules.
Next test (proposed, DND-015 permission): build the core + serial face + latest
  class for one named target with TinyGo; record flash/RAM. Cost unknown until the
  target is named.
Recommendation: adopt the layering below as the design constraint for B-D.
Decision: pending (owner).
```

| Layer | Package role (proposed) | On MCU | Notes |
|---|---|---|---|
| Core | route + exact-segment match; message name fields; bounded codec (DND-001); `latest` class ring; Face interface; null Authenticator/Signer | yes | budget per subscription = N × max message size, fixed at compile or init time #verify |
| Faces | serial, TCP, UDP unicast, UDP broadcast/multicast | serial/UDP yes | DND-010/011; one face per transport |
| `buffered` | retransmit ring, NACK, flush modes, lifespan | optional | bytes and message caps mandatory |
| `sync` | per-route state vector + suppression timers | optional | needed by broadcast repair and history coverage |
| `history` | Store interface; RAM ring; flash log; host file/DB store | ring/flash optional; DB host-only | flash wear #verify |
| `auth/ed25519` | proof-of-possession handshake; verify-only build | optional | sizes #verify |
| `auth/hmac` | zone shared-key message MAC | optional | cheapest per-message integrity |
| `zone` | zone id, allowlist, export/import, compiled rule table | allowlist on MCU; rule compiler host-only | LVS-style split |
| `bridge/nats` | DND-014 gateway | host-only | crosses into NATS accounts |
| `dht` | Kademlia-style holder lookup | host-only, deferred | revisit trigger in card C |

**Budget anchors.** These are reported or assumed, **not measured for dndm**: ESP32 320 KB SRAM (DeftT draft); RP2040 264 KB SRAM `#verify` (not from a source read this session); Ed25519 signature 64 B, ECDSA P-256 ≤ 72 B, HMAC-SHA256 32 B (NDN spec); certificate ~300 B (DeftT); EdDSA verify 41–58 µs + per-KB on DeftT's test machines, which are not MCUs. MCU timing is `#verify`.

## 8. Ranking

| Card | Value | Effort | Confidence | Why |
|---|---|---|---|---|
| A message names | high: unblocks B, C, D2, DND-010 | low–medium | medium–high | One additive primitive; strong prior-art convergence |
| B policy classes | high: R1–R2 directly | medium | medium | DDS vocabulary is proven; repair cost needs bounding |
| D auth steps 0–1 | high: R4–R5 | low (step 0), medium (step 1) | medium | Step 0 is mostly DND-017 work; step 1 is a known NATS-like pattern |
| C history | high: R3 | medium–high | low–medium | Use cases not yet named; DHT question open |
| E layering | high: R6 constraint | low as a design rule | medium | Depends on TinyGo/protobuf facts (DND-015) |

Suggested order: A → (B, D step 0) → DND-010 broadcast face (trusted segment) → C → D step 1. E constrains all of them.

## 9. Knowledge map

### Known knowns

| Fact | Evidence / limit |
|---|---|
| dndm messages have no per-message identity | observed `header.proto:24-31` at `c7b5b61` |
| Auth fields reserved but unused | observed `peering.proto:22`, `header.proto:29`; unused per DND-009 memo |
| The route hash is claimed as a capability, but the hash is unkeyed | observed comment `route.go:41-45`; unkeyed per DND-009 memo |
| NDN mandates P-256 ECDSA support; Ed25519 and HMAC also defined | NDN spec, read 2026-11-08 |
| NATS isolates accounts absolutely; cross only by export/import; Ed25519 nonce proof at connect | NATS docs, read 2026-11-08 |
| SVS v3 repairs loss by state vector + fetch by name, with bootstrap time for restarts | SVS spec 2026-07-17 |
| DeftT does broadcast + schema-based auth + set reconciliation, targeting ESP32 | expired I-D -06 (reported, not reproduced) |
| DDS/ROS 2 define history/reliability/durability/lifespan with request-vs-offered matching | ROS 2 docs |

### Known unknowns → owning ticket

| Question | Decision affected | Cheapest answer | Ticket |
|---|---|---|---|
| Message-name fields: width, scope (per route or per producer), epoch source on MCUs | card A | in-memory old/new peer test + size delta | DND-006 (fields), DND-013 (advertise) |
| Class set and defaults; drop-oldest vs drop-newest; default N | card B | paper transition table, then lossy-face test | DND-006 → DND-007 |
| Repair traffic bounds on broadcast (suppression timers) | cards B, A | simulation with SVS defaults (30 s / 200 ms) as a starting point | DND-010 → DND-011 |
| Concrete history use cases; retention; where stores sit | card C | 30 min with the owner | **new ticket (durable data history)**; DND-012 for the command boundary |
| Is a DHT ever needed at the owner's scale? | card C | holder/zone size estimate from the owner | new history ticket (revisit trigger) |
| Adversary model: untrusted LAN, hostile profiles, compromised in-zone process | card D step choice | owner answer | DND-009 |
| Who holds the zone anchor key; revocation latency | card D step 2 | owner answer | DND-009 → DND-017 |
| Segment separator `.` vs `/` and migration | cards A, D | grep importers | DND-009 (residual) |
| Ed25519/HMAC code size and time on the named target | cards D, E | build-only measurement after target is named | DND-015 → DND-016 |
| TinyGo + protobuf reflection compatibility | card E | build-only check | DND-015 |
| Policy and auth advertisement on the wire; mismatch handling | cards B, D | descriptor examples | DND-013 |
| NDN-Lite memory figures (the fetch failed) | card E calibration | re-fetch the TR | DND-015 (reference only) |

### Unknown knowns (unconsulted knowledge, concealed assumptions)

- **The owner's earlier Python broadcast robot** (DND-010 residual, not located) may already contain answers on rates, loss handling and buffer sizes. Ask the owner. → DND-010.
- **Hidden assumption: "durable" means one thing.** The owner's R3 is about data; DND-012 is about commands. Treating them as one would either build a broker in dndm or block R3. → DND-012 + new history ticket.
- **Hidden assumption: reliable = acks.** Receiver-driven repair (SVS/syncps) avoids producer ack state on MCUs. → DND-006.
- **Hidden assumption: the producer id is `me`.** Until step 1, `me` is self-declared; history and dedup keyed on it can be poisoned in a mixed-trust zone. → DND-009.
- **Hidden assumption: clocks agree.** Lifespan, replay windows and epoch need a time source; MCUs without RTC need boot counters. → DND-006 / DND-015.
- **Existing knowledge in the portfolio:** fpv-arena already uses P-256 device keys with SPKI-fingerprint ids and pinned links (`docs/notes/2026-11-07-roster-and-fpv-arena.md`). Abugaga uses NATS accounts per roster (ABU-050). One identity convention could serve all three. → DND-009 (cross-project note to Leader).

### Unknown unknowns (probes)

These are proposed, not performed:

- **Pre-mortem: buffers and stores.** "Six months later the robot stalls": a `buffered` route fills on a dead link and back-pressures a control loop. Probe: link-down fault test with every overflow policy (DND-007).
- **Epoch reset storm:** many MCUs reboot together and their epochs collide (same RTC-less boot counter). Probe: simulated simultaneous reboots (DND-006/015).
- **Broadcast on Wi-Fi:** multicast at basic rates starves other traffic. Probe: authorised capture on the target network (DND-016; needs network permission).
- **History poisoning before signatures:** a trusted-zone member injects a fake seq range that others cache. Probe: paper case in DND-009; mitigated only by step 2.
- **Revocation vs held copies:** revoked key, history still served from caches. Probe: paper case (DND-009/017).

Coverage limit: I read the specs, docs and papers listed in section 3. I didn't read NDN-Lite's memory numbers, the zenoh-pico footprint, the ndnd Go source, the PSync/SVS papers in full, or any dndm source beyond the files in section 2.

## 10. Handoff

- **Owner:** accept or park cards A–E, and confirm the durable split (history data vs commands) and the auth staging (step 0 now, step 1 when ready).
- **dndm peer:** turn accepted cards into designs in DND-006/009/010 and a new history ticket. Check whether the EasyRobot importers depend on the Header layout. Fix the segment separator.
- **product:** allocate the new ticket ID, if dndm's convention needs product.
- **Nothing here authorises code, network tests or board work.**

## Sources

All accessed 2026-11-08, read-only:

- NDN Packet Format v0.3, Signature: <https://docs.named-data.net/NDN-packet-spec/current/signature.html>
- NDN 101, Trust Model: <https://101.named-data.net/security/trust-model/>
- NDN 101, Storage: <https://101.named-data.net/storage/>
- Yu et al., Schematizing Trust in NDN (ICN 2015): <https://named-data.net/wp-content/uploads/2015/11/schematizing_trust_ndn.pdf> (search result, not read)
- python-ndn, Light VerSec: <https://python-ndn.readthedocs.io/en/latest/src/lvs/lvs.html>
- NDNts @ndn/lvs: <https://ndnts-docs.ndn.today/typedoc/modules/_ndn_lvs.html> (search result)
- Nichols, Jacobson, King, DeftT draft-06: <https://datatracker.ietf.org/doc/draft-nichols-iotops-defined-trust-transport/06>
- Zhang et al., NAC: Automating Access Control via Named Data (MILCOM 2018): <https://users.cs.fiu.edu/~afanasyev/assets/papers/zhang2018nac.pdf>
- SVS v3 specification: <https://named-data.github.io/StateVectorSync/Specification.html>
- PSync: <https://github.com/named-data/PSync>
- NDN-Lite: <https://github.com/named-data-iot/ndn-lite>; pub/sub TR (fetch failed): <https://named-data.net/wp-content/uploads/2021/01/NDN-TR-0071-ndn-lite-pubsub.pdf>
- NATS accounts: <https://docs.nats.io/learn/security/accounts-and-multitenancy>
- NATS decentralized auth: <https://docs.nats.io/learn/security/decentralized-auth>
- NATS key-value: <https://docs.nats.io/learn/key-value/>
- ROS 2 QoS: <https://docs.ros.org/en/rolling/ROS-Framework/interfaces/topics/About-Quality-of-Service-Settings.html>
- Zenoh abstractions: <https://zenoh.io/docs/manual/abstractions/>
- RFC 9171 Bundle Protocol v7: <https://www.rfc-editor.org/rfc/rfc9171>
- Kademlia (2002): <https://link.springer.com/chapter/10.1007/3-540-45748-8_5> (search result)
- repo-ng protocol: <https://redmine.named-data.net/projects/repo-ng/wiki/Repo_Protocol_Specification> (search result)

Local: `docs/roster.md` (owner decisions), `dndm/docs/roadmap.md`, tickets DND-006/009/010/011/012/013/015, `dndm/docs/research/known-unknowns.md`, `dndm/route.go`, `dndm/proto/types/core/{header,intents}.proto`, `dndm/proto/types/p2p/peering.proto`, `dndm/{README,DESIGN}.md`, `dndm/x/SPEC.md`.
