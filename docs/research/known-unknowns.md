---
title: "dndm knowledge and unknown map"
date: "2026-11-08"
scope: "local-only planning"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
---

# Knowledge and unknown map

Evidence basis: the canonical 2026-11-08 dndm reuse report, final two-review static state, revision `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. No new source audit or runtime execution. See [pinned findings](pinned-findings.md) for line evidence.

## Known knowns — inherited static facts

- Local fan-out shares protobuf objects; remote wrappers serialize/decode. Shared ownership is not specified by the mere existence of a pointer path.
- Reported framing, registration, competing-reader, correlation, closure and lifecycle defects have concrete static paths. Their runtime frequency is unknown.
- Packet broadcast and a NATS bridge are absent in the inspected revision; UDP-named examples are not evidence to the contrary.
- Advertised identity, raw lexical prefixes and an unkeyed route hash are not authentication or profile isolation.
- The source report's two reviews are complete. Runtime, hardware and comparative benchmark evidence remains empty.

## Known unknowns — decision-ready research versus future implementation

| Question / current gap | Decision owner ticket | Required decision artifact / gate | Dependent implementation |
|---|---|---|---|
| Who owns published objects, pooled bytes and retained subscriber references? | [DND-002](../tickets/DND-002-local-buffer-ownership-contract.md) | Ownership timeline, chosen immutability/copy/lease policy and migration cost | DND-003, DND-007; chosen ownership in any packet/bridge path |
| Which classes block, reject, drop, expire or replay? What counts as receipt/completion? | [DND-006](../tickets/DND-006-delivery-and-overload-classes.md) | Bounded class/transition matrix and explicit unsupported guarantees | DND-007; packet and bridge contracts |
| What frame and queue caps fit the actual workload? | [DND-001](../tickets/DND-001-bounded-stream-framing.md), DND-006 | Configured units/caps, failure policy and later measured pressure | Framing/dispatch changes; DND-016 verifies workload fit |
| Which threats, trust zones and profiles must be isolated? Who issues/revokes authority? | [DND-009](../tickets/DND-009-identity-auth-and-profile-boundary.md) | Threat model and selected gateway/authenticated transport, or enforceable trusted-local restriction | [DND-017](../tickets/DND-017-enforce-profile-gateway-boundary.md), then DND-014 |
| Is packet broadcast worth implementing, on which network and with which loss/security/MTU rules? | [DND-010](../tickets/DND-010-packet-broadcast-decision.md) | Go/no-go plus bounded datagram/reassembly design; no-go is valid | [DND-011](../tickets/DND-011-bounded-packet-transport.md), currently unknown size |
| Where do commands become durable and effects idempotent? | [DND-012](../tickets/DND-012-durability-and-command-boundary.md) | Crash/replay authority table; retain NATS/application/Markdown boundary unless separately justified | DND-014 enforces the selected boundary; no dndm durable store authorized |
| How are supported, unsupported and unverified capabilities negotiated? | [DND-013](../tickets/DND-013-capability-negotiation-contract.md) | Versioned descriptor examples and mismatch/downgrade/expiry behavior | [DND-014](../tickets/DND-014-scoped-nats-bridge.md) |
| Which board/runtime can execute the required workload with margin? | [DND-015](../tickets/DND-015-embedded-target-feasibility.md) | Named target, compatibility/resource worksheet and go/no-go gates | No target port is silently planned or authorized; estimate a later ticket only after decision |
| What are actual latency, drops, allocations, CPU, bytes and power? | [DND-016](../tickets/DND-016-reproducible-transport-measurement.md) | Authorized raw measurements with matched semantics, limits and uncertainty | Empirical ticket cannot close on a protocol alone |

Research tickets bound the question; they do not pretend the answer or implementation is known. DND-011, DND-014 and DND-017 are explicitly blocked. A decision completion does not itself grant code/network/hardware permission. Decision outputs must state rejected alternatives and residual unknowns.

## Unknown knowns — potentially recoverable owner/context evidence

- The earlier Python broadcast robot may contain valuable design history, but it was not located/read by the canonical report. Ask the owner for provenance only if a later ticket needs it; do not infer equivalent Go behavior.
- Actual available devices, target toolchains, payloads, frame rates and network constraints may already be known to the owner. DND-015 must recover that context rather than choose a fashionable board.
- Existing consumers may rely on mutation, nonce-less replies or prefix behavior. A later implementation must map those callers at the pinned/current revision before changing contracts. This handoff intentionally did not repeat the source audit.

No hidden runtime facts were recovered: only the final report, owner amendment and directory structure were consumed here. These are investigation targets, not claims of compatibility.

## Unknown unknowns — residual risk and discovery method

- Schedule-dependent races, cancellation deadlocks and partial I/O combinations beyond reported paths: later deterministic barriers, fuzzing and bounded race-detector runs (DND-001/004/007/008).
- MTU, Wi-Fi broadcast rates, receiver churn, interference and congestion interactions: only after packet go/no-go and explicit network/hardware permission, with named topology and captures (DND-010/011/016).
- Embedded runtime/dependency incompatibility, flash endurance, thermal/power limits and resource-pressure cliffs: later target evidence, not ordinary Go build output (DND-015/016).
- Authorization bypasses across reconnect, stale activations, reply paths and bridge loops: negative fixtures after the threat-model decision (DND-009/017/014).

This quadrant is not empty: finite static reviews cannot establish absence of these failures. No new probes were run during planning. Update this map with dated evidence when an authorized ticket resolves a question; never convert an unrun fixture into a pass.

2026-11-08 (R1-04): the first bounded race-detector run and a DND-001 decoder probe were executed. Results are in the [baseline](../baseline-2026-11-08.md) and the [pinned-findings addendum](pinned-findings.md#addendum-2026-11-08-runtime-baseline). Schedule-dependent races are now observed in core lifecycle code (F14), not only predicted.

[Overview](../index.md) · [Roadmap](../roadmap.md) · [Ticket index](../tickets/index.md)
