---
title: "Decide authenticated identity and profile isolation"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-009"
status: "backlog"
priority: "P0"
milestone: "M0 decisions"
kind: "research / missing feature"
dependencies: []
implementation_evidence: []
---

# DND-009 — Decide authenticated identity and profile isolation

## State and permission

Status: **backlog**. Kind: **research / missing feature**.
Milestone: **M0 decisions**. Priority: **P0** — URI claims, route hashes and lexical prefixes do not authorize access; a bridge must not expose untrusted profiles.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Requires explicit approval before the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: None (dependency-free; still not authorized to execute).

## Decision (2026-11-08)

Requirement (2026-11-08): zones are needed, modelled more like NATS accounts. Permissions should follow how named data networking (NDN), dndm's original inspiration, handles them. Authentication is needed: implement first steps towards it now and add full auth later. The implementation must fit an embedded device when not all parts are needed (very light, modular, expandable).

Status unchanged: the design waits for the research note (`docs/research/ideation/2026-11-08-ndn-delivery-auth.md`). No code until then. Design questions noted by dndm (DND-R4):

- Zones as NATS-account-like namespaces: a zone owns a name prefix, with explicit imports/exports between zones?
- NDN trust schemas: name-based rules for which key may sign which names. Where is the schema kept and distributed, and who issues keys?
- "First steps toward auth" without enforcing it yet: carry identity and signature fields end to end (the protos already have unused `public_key`/`signature` fields), verify when a key is known, log or count unsigned data, keep a policy switch off by default?
- Modular and embedded: auth as an optional layer (build tag or interface) so a minimal build has no crypto dependency. Which primitive fits small MCUs (Ed25519 vs HMAC per link)?
- Exact path-segment matching to replace the lexical `HasPrefix` (`profile.a` must not match `profile.ab`): independent of auth, so can it land earlier?
- **Inherited residual risk from DND-005 (DND-R4 verification, probes A3/A4):** reply nonces are correlation, not authentication. With the counter source, one observed reply reveals that Caller's next nonce (`n+1`), so anyone who sees the response path can forge an accepted reply. With any source, anyone on the request path can copy the nonce and answer first. The per-call random sources (DND-R5) close only A3. A4 needs what this ticket designs: source authentication of the replier (for example, replies signed or carried in a channel keyed to the service's identity) or per-route permissions. The design must not assume nonces stop injection; its threat model should test reply injection on the request and response paths.

### Further requirements (2026-11-08)

- Auth, zones and API shape: zero-knowledge proofs, cryptographic channels, signatures, replay-attack mitigation and Diffie–Hellman cryptography are priorities. An existing NATS data-layer package sets up a cryptographic channel between publisher and subscriber, assuming the producer's public key is shared out of band (exporting/importing subjects between NATS accounts makes the keys known, so a common symmetric key can be derived); dndm could do the same. dndm must stay modular and layered: developers add layers as needed (reliability, durability, encryption, etc.). The API should need as little boilerplate and mental load as possible, inspired by the HTTP middleware or Python decorator patterns.
- Zone enforcement: the whole dndm protocol must be zone-aware and must not allow unpermitted zone crossing. How to do that in a middleware pattern is an open question.
- Overhead and embedded: the design must be memory- and performance-aware. Each layer adds compute, memory and call-stack overhead, which matters for embedded work. Zones should not be sent in every message, sync or link, nor stored with every name: that bloats messages and memory. A cryptographic zone would hold at least 64 bits of public key, which is costly on ESP32/RP2040 memory and I2C/SPI/CAN/ELRS bandwidth. Find a design that is robust and embedded-proof without compromising apps where large footprints are acceptable.
- C++: a dndm C++ version will be needed at some point, as a minimal reference implementation for very small devices or where C++ is required (e.g. nncase).
- Adversarial probing: zero-knowledge proofs and signatures only where they pay for themselves, and the design must be probed for vulnerabilities, e.g. whether a link can be guessed and messages sent without authorization, as with classic TCP/IP and MAC spoofing attacks. This applies to both the research and the verification work.

Note: the earlier zone-prefix-on-every-name framing is superseded by the overhead requirement. Only its invariant remains, as a constraint to test against: no unpermitted zone crossing, enforced where no layer can bypass it.

## Outcome

A decision-ready threat model and identity/authorization design, including the option to keep raw dndm strictly trusted-local.

## Scope

Distinguish stable peer, process activation, profile, trust zone and call identities. Compare authenticated transport versus a trusted terminating gateway; define least privilege, exact namespace ownership, revocation, encryption and credential lifecycle. Include sandbox denial boundaries without importing any credentials.

## Non-goals

No credentials, crypto implementation, live penetration tests, broad work connectors or authorization inferred from a route name.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the canonical 2026-11-08 static report, sections 1–2, 4–5 and Limits; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [route.go](../../route.go) lines 100–119 — Unkeyed route hash is an opaque identifier, not authority.
- [peer.go](../../peer.go) lines 76–77 — HasPrefix matches lexical prefixes, not isolated namespaces.
- [endpoint/mesh/handshake.go](../../endpoint/mesh/handshake.go) lines 203–217 — Handshake accepts advertised identity text.

## Unknowns and decisions

Which adversaries are in scope: untrusted LAN nodes, compromised local processes or hostile profiles? Select the trust boundary and key authority before implementation.

## Proposed reproduction or decision fixture — NOT RUN

Construct paper cases for profile.a versus profile.ab, spoofed URI, forged route hash, revoked activation, reply-path escalation and sandbox wildcard subscriptions. Trace authentication and authorization at each hop without acquiring secrets.

## Decision memo (2026-11-08)

**Question.** Which trust boundary does dndm enforce: none beyond a trusted host or LAN, authenticated peer links, or a terminating gateway? And who issues and revokes identities?

**What the code does at the pin** (read from source, nothing executed):

- Identity is self-declared. The handshake takes the peer's `Me` string as its new identity with no credential check (`endpoint/mesh/handshake.go:203–217`). Inbound connections start with an empty path (`network/net/net.go:67`).
- The wire has unused auth fields: `Handshake.public_key` (`proto/types/p2p/peering.proto:22`) and `Header.signature` (`proto/types/core/header.proto:29`). Neither is referenced by non-generated Go code (`git grep` at the pin).
- Route selection is a lexical prefix: `strings.HasPrefix(route.Path(), peer.path)` (`peer.go:76–77`), used to choose forwarding endpoints (`container.go:124,135`; `endpoint/mesh/mesh.go:56`). `profile.a` matches `profile.ab`, and a peer that declares an empty path matches every route.
- The hashed route is unkeyed SHA-1 of `Type@path` (`route.go:113–116`), so anyone who knows the type and path can compute it. Parsing accepts any hash string (`route.go:159`).
- Remote intent and interest registrations are accepted for any route string, with no authorization step (`endpoint/remote/messages.go:212–233,277–299`). A `beforeLink` hook exists but every caller passes `nil` (`linker.go:63–68`; `endpoint/remote/remote.go:89`; `endpoint/direct/direct.go:62`).
- Links are plain `net.Dial`/`net.Listen` with no TLS (`network/net/net.go:38,48`).

**Option A: trusted-local only, no cross-zone path in dndm.** Raw dndm is declared unauthenticated and limited to loopback, unix sockets, serial or an isolated segment. Each profile gets its own router/process. Apps that need to cross zones use NATS themselves.
- Code: docs, plus DND-017 enforcement: refuse non-loopback listen unless an explicit trusted-LAN option is set; exact path-segment matching where an empty path matches nothing (`peer.go:77`); a default-deny route allowlist through `beforeLink` and the registration handlers. Cost: small.
- Risk: any host on a "trusted" LAN can impersonate a peer. Safe only while that assumption holds.
- Compatibility: no wire change; additive options. Behaviour change: binding `0.0.0.0` (`examples/embedded/embedded/main.go:209`) needs the opt-in. EasyRobot's dot-separated paths (peer `easyrobot.manipulator`, route `easyrobot.manipulator.raw`; `EasyRobot/cmd/manipulator/main.go:240,258`) still match if `.` is a segment separator.

**Option B: authenticated dndm links.** mTLS over `net.Conn`, or Noise with static keys, binding the handshake `Me` to the authenticated key, plus a route ACL per peer.
- Code: `network/net/net.go`, `endpoint/mesh/handshake.go`, `endpoint/remote/messages.go`, credential configuration, and a key issuance/revocation authority. Cost: large.
- Risk: home-made protocol mistakes if Noise is hand-wired. Serial links need a non-TLS scheme [INFERENCE]. Embedded CPU/RAM cost is unmeasured.
- Compatibility: wire change, because TLS wraps the stream and authenticated peers cannot talk to old ones. New public options. EasyRobot serial links would need keys or an exemption.

**Option C: A inside each zone, plus a NATS-terminating gateway across zones.** Same local rules as A. The only sanctioned crossing is a gateway process (the [DND-014](DND-014-scoped-nats-bridge.md) bridge) that authenticates to NATS with credentials per profile (operator/account/user JWTs, accounts as the isolation boundary) and forwards only an exact allowlist of routes.
- Code: A's DND-017 work, plus allowlist enforcement in the bridge. Cost: medium, most of it in DND-014.
- Risk: the gateway is a choke point. A compromised process inside a zone is not contained.
- Compatibility: as A. No dndm wire change.

| | A trusted-local | B authenticated links | C local + NATS gateway |
|---|---|---|---|
| Spoofed peer on the LAN | not stopped | stopped | not stopped inside a zone; stopped at the crossing |
| Profile prefix confusion | fixed (exact segments) | fixed | fixed |
| Custom crypto/keys in dndm | none | yes | none (NATS creds) |
| Wire change | none | yes | none |
| Cross-zone traffic | outside dndm | direct | through the gateway |
| Size | small | large | medium |

**Recommendation: C.** NATS is already the first orchestration transport, and its account/JWT model provides authenticated, revocable isolation without dndm carrying keys or crypto. C keeps the wire format and EasyRobot's serial setup unchanged and makes DND-017 a small, testable set of allow/deny rules. Exact-segment matching and the default-deny allowlist fix real routing gaps whichever option is chosen. Defer B until something must link dndm peers directly across an untrusted network; until then leave `public_key` and `signature` reserved and unused rather than half-implemented.

**What it unblocks.** These tickets list DND-009 in their frontmatter `dependencies:`: [DND-010](DND-010-packet-broadcast-decision.md), [DND-012](DND-012-durability-and-command-boundary.md), [DND-013](DND-013-capability-negotiation-contract.md), [DND-017](DND-017-enforce-profile-gateway-boundary.md). Transitively: DND-011, DND-014 and DND-015.

**Residual unknowns.**

- The accepted adversary set: untrusted LAN hosts, hostile profiles, compromised local processes.
- Whether serial/USB links count as trusted.
- The segment separator (`.` or `/`) and the migration of existing paths.
- Whether Abugaga needs dndm directly across hosts. That would force B.
- NATS credential revocation latency and who operates the NATS operator key.
- Enforcement reliability depends on the lifecycle races (F14) and on DND-008.

## Acceptance — unmet until evidence exists

- [ ] Every case names the trusted decision point and deny behavior.
- [ ] Profile/zone isolation survives reconnect, revocation and reply forwarding; lexical matching never grants authority.
- [ ] Decision records trusted-local-only limitations or the chosen authenticated remote design plus key lifecycle.
- [ ] On secured link-context profiles (ZONE_PSK, STATIC_DH, GROUP_KEY), an unauthenticated `Close` that arrives before the handshake completes is not accepted: the link stays as it was and the frame is dropped and counted, like every other frame before the handshake (design link-context-wire §4). Test failing first: a forged `Close` before the handshake must not tear down a link that is mid-handshake. Background: `network/linkctx` today honours `Close` in any state, which is harmless on ONBOARD (no authentication exists there) but would let anyone on a secured face reset links (DND-022 verification, optional item 2).

## Verification procedure — future, not executed

Static threat-model review followed, only with permission, by DND-017 negative fixtures using synthetic identities; no current secure isolation is claimed.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium research decision: one threat model and gateway/direct alternative comparison; enforcement size cannot be fixed beforehand.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and scope amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R2. Decision memo added (options, recommendation). Decision pending; status unchanged.
- 2026-11-09: acceptance item added from the DND-022 verification (optional item 2). On secured profiles, an unauthenticated pre-handshake `Close` must not be accepted. Today `linkctx.Link.Receive` accepts `Close` in any state; ONBOARD-only, so no change there until secured profiles land with this ticket.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
