---
title: "Enforce the selected identity/isolation boundary"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-017"
status: "blocked"
owner: "unassigned"
priority: "P0"
milestone: "M1 isolation enforcement"
kind: "implementation / missing feature"
dependencies: ["DND-008", "DND-009"]
implementation_evidence: []
---

# DND-017 — Enforce the selected identity/isolation boundary

## State and permission

Status: **blocked**. Owner: **unassigned**. Kind: **implementation / missing feature**.
Milestone: **M1 isolation enforcement**. Priority: **P0** — A reviewed threat model is not enforcement; the bridge must wait for an actual denying boundary.

Planning only. This ticket is not permission to implement, install dependencies, run tests/models, connect services, acquire credentials or use hardware. Obtain explicit owner permission for the concrete later work. P0 means prerequisite safety/correctness, not an observed production incident. No ticket is accepted or completed by this handoff.

Dependencies: [DND-008](DND-008-stream-and-peer-lifecycle-races.md), [DND-009](DND-009-identity-auth-and-profile-boundary.md)

## Outcome

The DND-009 selected trusted gateway or authenticated remote path enforces peer/profile/zone permissions and revocation.

## Scope

Implement only the chosen authentication, authorization, identity binding and bounded routing rules; deny prefix confusion, spoofing and reply escalation. For trusted-local-only decisions, enforce that exposure restriction and prohibit raw cross-profile/remote use rather than pretending authentication exists.

## Non-goals

No custom cryptography, credential retrieval, all-hostile-code sandbox guarantee or subject-prefix-as-firewall design.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the [canonical report](../../../docs/notes/2026-11-08-dndm-babyai-reuse.md), sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [route.go](../../route.go) lines 100–119 — Route hash does not prove authority.
- [peer.go](../../peer.go) lines 76–77 — Lexical prefix overlap allows profile-like name confusion.
- [endpoint/mesh/handshake.go](../../endpoint/mesh/handshake.go) lines 203–217 — Advertised identity is not authenticated.

## Unknowns and decisions

Blocked on approved DND-009 threat model and chosen enforcement primitive. Credential issuance, revocation and deployment limits must be specified; no grants exist from planning.

## Proposed reproduction or decision fixture — NOT RUN

After permission, use synthetic identities and isolated fixtures for permitted route, profile.a/profile.ab overlap, forged URI/hash/reply, stale activation, revoked identity and reconnect. Record decisions at every trust boundary.

## Acceptance — unmet until evidence exists

- [ ] Only explicitly bound identities may access scoped routes; prefix similarity never grants access.
- [ ] Revocation/reconnect cannot recover old grants, and reply paths cannot escalate permissions.
- [ ] Chosen trusted-local restrictions are enforceable and documented; any unsupported threat stays visibly out of scope.

## Verification procedure — future, not executed

Future negative authorization fixtures and authorized boundary tests must provide deny/allow evidence before DND-014 starts. Planning, route naming and a security design document alone do not satisfy acceptance.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Unknown until threat model/primitive chosen; size the selected gateway or authenticated transport separately, not as a small hash change.

## Implementation evidence

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and owner amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
