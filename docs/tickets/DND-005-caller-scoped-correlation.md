---
title: "Make reply correlation caller-scoped"
date: "2026-11-08"
scope: "local-only planning and implementation"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-005"
status: "done"
priority: "P0"
milestone: "M1 remote correctness"
kind: "implementation / fixable defect, runtime-confirmed"
dependencies: []
implementation_evidence: ["x/bus/caller_test.go", "x/bus/nonce.go", "commit 12b4916", "commit 7e27991"]
---

# DND-005 — Make reply correlation caller-scoped

## State and permission

Status: **done** (DND-R4 and DND-R5; verified, reviewed, committed locally as 12b4916; nonce nits F11/F13 in 7e27991). Kind: **implementation / fixable defect, runtime-confirmed**. Milestone: **M1 remote correctness**. Priority: **P0**: nonce collisions deliver one caller's reply to another.

**Decision (2026-11-08):** accept option A — random per-caller nonce start; with a nonce hook, foreign/missing-nonce replies are dropped and counted; FIFO only without hooks.

**Decision (2026-11-08, nonce generator, after the predicted-nonce attack A3):** the nonce generator is configurable. Small devices may start from a random number and count up, capable devices use random nonces, more capable ones cryptographic random. Different wrappers select different nonce sources at compile time.

## Residual risk (DND-R4 verification, probes A3/A4)

A nonce correlates a reply with its call; **it is not authentication**.

- **A3, predicted nonce:** with the counter source, one observed reply reveals that Caller's next nonce (`n+1`). Any participant that can see the response path (for example another Caller on it) can inject a forged reply that the pending call accepts. Independent verification forged one 3/3 against the DND-R4 tree. `TestCallerPredictedNonceForgery` keeps this as a test: the forgery lands with `CounterNonce` and is dropped with `RandomNonce` and `CryptoNonce`.
- **A4, copied nonce:** any participant on the request path can copy the nonce from the request and answer before the service (3/3). No nonce source prevents this. It needs source authentication or per-route permissions ([DND-009](DND-009-identity-auth-and-profile-boundary.md)).
- Blind guessing (A1) and replaying old replies (A2) were resisted (500 and 20 attempts).

Consequence: until DND-009 lands, request/response correlation is safe against accidental misdelivery only, within one trust domain. Builds that use the counter source (TinyGo default, `-tags dndm_nonce_counter`) also accept predicted-nonce forgeries from anyone who can see the response path.

Dependencies: none.

## Outcome

Each reply is attributable to one live caller and call; unmatched replies never consume another waiter.

## Scope

Choose private reply paths or caller identity plus call nonce; reject absent/unknown/wrong-scope replies; define restart, late reply, timeout, cancellation and streaming correlation. Defer dispatcher saturation mechanics to DND-007.

## Runtime evidence (DND-R3 probe, throwaway, not a kept test)

Two `Caller`s with nonce hooks share `p.req`/`p.resp` and call one at a time, 10 rounds each. **Corrected in DND-R4 after verification:** each tree got one probe run in DND-R3, and the outcome is bimodal per run at both trees.

| Tree | Per run (independent verification, 16 runs per tree) |
|---|---|
| pin `c7b5b61` | either all 10 second-caller replies are the first caller's stale reply, or all 10 time out; misdelivery in 3 of 16 runs |
| working tree after DND-R3 | same two modes; misdelivery in 3 of 16 runs |

So the misdelivery is pre-existing and its rate is unchanged by DND-R2/R3. The timeout mode is a lost readiness notification for the second producer (fixed in DND-R4, see the log).

Mechanism (read from `x/bus/caller.go` at the pin): every Caller starts at nonce 1 (lines 88–96). Every caller's Consumer receives every reply on the shared path. Caller 2's queued copy of caller 1's reply (nonce 1) matches caller 2's own first waiter (nonce 1) and is delivered (lines 112–128). A reply with an unknown nonce falls back to FIFO and can also land on an unrelated waiter.

## Decision memo (DND-R3)

**Question.** How does a reply identify its caller and call, and what happens to replies without that identity?

| | A: per-caller random nonce base | B: explicit caller ID + call nonce | C: private reply path per caller |
|---|---|---|---|
| How | Each Caller starts its nonce counter at a random 64-bit value (crypto/rand). A reply whose nonce is present but not ours is dropped and counted. | New option carries caller ID and nonce through the hooks; the Service echoes both | Each Caller subscribes `responsePath/<callerID>`; requests carry the reply path; Service replies there |
| Wire / message types | unchanged (same nonce field) | user messages need a caller-ID field | user messages need a reply-path field; Service API reads it |
| Public API | additive (`Dropped()` counter) | new option; hook signatures change or are duplicated | Service and Caller API change |
| Collision risk | negligible, not authentication | none within a process | none |
| Reply fan-out | every caller still receives every reply | same | each caller gets only its own |
| Size | small | medium | medium–large |

**Missing-nonce policy (needed with any option):** (1) when a response-nonce hook is configured, a reply without a nonce is dropped and counted; FIFO only applies when no hook is configured (single-caller use, documented); or (2) keep today's FIFO fallback (wrong under concurrency, as the probe shows).

**Recommendation: A with policy (1).** It fixes the observed cross-delivery without touching wire format, message types or existing single-caller users. C is the scalable follow-up if measurements (DND-016) show reply fan-out to N callers matters.

**What it unblocks:** [DND-007](DND-007-bounded-dispatch-and-closure.md) (depends on DND-005), then DND-008, DND-014 and DND-016 through their dependency chains.

**Failing tests to write first once decided:** `x/bus/caller_test.go`: `TestCallersSharingResponsePathGetOwnReplies` (the probe as a test), `TestCallerDropsReplyWithForeignNonce`, `TestCallerMissingNoncePolicy`; plus a timed-out call whose late reply must not reach the next call.

## Non-goals

No exactly-once execution, complete duplex implementation, or random nonce presented as authentication.

## Pinned evidence

Revision: `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`. Evidence below is inherited from the 2026-11-08 canonical static report, sections 1–2, 4–5 and Limits and review; two independent **static** reviews complete. This handoff did not repeat the source audit. Relative source links open the checkout; line numbers refer to the pin, not guaranteed future HEAD.

- [x/bus/caller.go](../../x/bus/caller.go) lines 88–96 — Each Caller starts at nonce 1.
- [x/bus/caller.go](../../x/bus/caller.go) lines 112–128 — Absent/unrecognised nonce falls back to FIFO.
- [x/bus/caller.go](../../x/bus/caller.go) lines 173–224 — Multiple callback replies are not a full duplex lifecycle.
- [x/bus/caller.go](../../x/bus/caller.go) lines 258–289 — One-item waiter channel can block shared dispatch.

## Unknowns and decisions

Choose reply namespace/envelope migration; existing peers may omit nonce. Decide explicit rejection rather than silent compatibility fallback.

## Proposed reproduction or decision fixture — NOT RUN

Create two callers sharing one response path, both issuing their first request. Deliver reordered replies with nonce 1, unknown/absent nonce, a timed-out reply and a reply from an earlier caller generation.

## Acceptance

- [x] Each accepted reply reaches only the matching live caller/call: `TestCallersSharingResponsePathGetOwnReplies` (deterministic: the service holds both requests and answers caller 1 first), `TestCallerNoncesDifferAcrossCallers`.
- [x] Unknown, missing and expired (late) identities are not delivered and are counted: `TestCallerDropsReplyWithForeignNonce`, `TestCallerMissingNoncePolicy`, `TestCallerLateReplyDoesNotReachNextCall` (`Caller.Dropped()`). Wrong-generation replies are the same case as foreign nonces (a new Caller has a new random start).
- [x] Streaming chunks retain call identity; terminal/cancelled calls cannot consume later replies: `TestCallerStreamingRepliesKeepCallIdentity` (two `HandleReplies` calls on one Caller; call a ends after its first chunk; a2/a3 are dropped and counted, call b gets exactly b1–b3). It fails 3/3 at the pin (b received `a2`, `a3`; `Dropped` 0). It passes with the DND-R4 matching unchanged and ×30 with `-race`.
- [x] Nonce source is pluggable (DND-R5): `TestCallerPredictedNonceForgery`. Before wiring, the random and crypto cases fail 3/3 ("a predicted-nonce forgery was accepted"); after, all three cases pass ×30 with `-race`, and the whole package passes with each build default.
- [x] All of the above fail at the pin (+ the `Dropped()` declaration only) and pass after the fix, with `-race` ×30 (late-reply test ×20).

## Verification procedure — future, not executed

After approval, deterministic correlation fixtures cover collision/reordering/late traffic and document any wire incompatibility; DND-007 separately proves slow-stream isolation.

Record the exact revision, inputs/configuration, actual observations, failures and scope limitations in the evidence/log when later authorized. Research decisions may close only on their stated decision deliverable; empirical and implementation tickets require actual execution evidence. A research recommendation never creates an implementation grant.

## Size assumptions

Medium: caller/service envelope or routing contract and tests; migration effort depends on chosen identity shape.

## Implementation evidence

- `x/bus/caller.go`:
  - each Caller's nonce counter starts at a random non-zero 64-bit value (crypto/rand);
  - `waiterFor`: with a response-nonce hook only an exact nonce match is delivered; a foreign or missing nonce goes to no call. Without a hook, FIFO as before;
  - undelivered replies are counted (`Dropped() uint64`, new);
  - godoc on the hooks: the nonce needs 64 bits, and a fresh request is needed per call.
- Step 0 before the failing run: the `Dropped()` method and counter field only, never incremented.
- The readiness fix in [DND-007](DND-007-bounded-dispatch-and-closure.md) (an `IntentRouter` remembers it was notified and seeds wrappers created later) was needed for the shared-path test to be deterministic. With that fix alone, the test misdelivers 10/10 and never times out. This supports the inference that the probe's timeout mode was a lost readiness notification.
- API: additive (`Caller.Dropped`). Behaviour: callers with a response-nonce hook no longer get FIFO-matched replies; callers without hooks are unchanged.

### Nonce generator (DND-R5)

- `x/bus/nonce.go`: `NonceSource interface { Next() uint64 }`, with three implementations:
  - `NewCounterNonce()`: counts up from a crypto/rand start (atomic add).
  - `RandomNonce{}`: `math/rand/v2` per call. Runtime-seeded, but no cryptographic promise.
  - `&CryptoNonce{}`: crypto/rand per call, with a buffer reused under a mutex so it does not allocate.
- Selection is static, two ways:
  - **Build default by build tag.** A plain Go build uses `CryptoNonce`. TinyGo, or `-tags dndm_nonce_counter`, uses the counter; `-tags dndm_nonce_random` uses `RandomNonce` (files `nonce_default*.go`).
  - **Per Caller:** `WithCallerNonceSource(src)`. The option picks a concrete type at the call site, but the Caller calls it through the interface.
- The Caller never issues 0 or a nonce that is still live (it redraws). The old `randomNonceStart` is gone.
- Overhead:
  - Calls: one interface call per `Call`/`HandleReplies`, 0 allocations for all three sources. Apple M5, go1.27.2, best of 5: counter 2.1 ns, random 4.7 ns, crypto 62 ns.
  - Code size: a scratch binary using `NewCaller` links only the default source's `Next`, so unused sources cost nothing. Stripped sizes differ by ≤ 48 bytes between the three defaults, because crypto/rand is linked anyway for the counter's start.
  - TinyGo was not built here (no toolchain).
- Behaviour change vs DND-R4: plain Go builds now draw every nonce from crypto/rand instead of counting up from a random start. Matching is unchanged.

## Dated log

- 2026-11-08 — Created local planning ticket from the final canonical static report and the 2026-11-08 scope amendment §5. No code changes, reproduction, tests or runtime measurement performed. Acceptance remains unchecked; implementation evidence is empty.
- 2026-11-08: DND-R3. Runtime-confirmed with a throwaway two-caller probe; decision memo added; status backlog → **blocked (decision)**. No code changed.
- 2026-11-08: DND-R4. Option A with policy (1) accepted. Failing tests written first and recorded at the pin, then implemented; status → **in review**. DND-R3 cross-caller comparison corrected after verification (bimodal at both trees).
- 2026-11-08: DND-R5. Residual risk from verification (A3/A4) recorded. Nonce-generator decision implemented: `NonceSource`, three sources, build-tag default, per-Caller option. The forgery test was failing first. The streaming identity test was added (acceptance 3); it fails at the pin.
- 2026-11-09: DND-R5-fix (optional finding F1). A `NonceSource` that keeps returning 0 or a nonce still in use froze the Caller: `registerWaiter` spun under `c.mu`, so even a timed-out call could not unregister. Now `Next` is called outside the lock with at most 8 tries, after which `Call`/`HandleReplies` return the new `bus.ErrNonceSource`. The `NonceSource` contract is documented. `TestCallerRejectsBrokenNonceSource` (zero, repeated): before, it froze and the binary timed out; after, it passes ×30 with `-race`.
- 2026-11-09: status → **done**. Accepted after independent verification and committed locally:
  - 12b4916 `feat(x/bus): caller-scoped nonces with a pluggable nonce source` (DND-commit), including the DND-R5-fix `ErrNonceSource` change;
  - 7e27991: findings F11 (`CryptoNonce` is a value type) and F13.

[Ticket index](index.md) · [Roadmap](../roadmap.md) · [Knowledge map](../research/known-unknowns.md)
