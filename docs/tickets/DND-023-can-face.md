---
title: "CAN face for the link-context core"
date: "2026-11-09"
scope: "local-only planning; implementation not started"
source_revision: "c7b5b61f79ea07d3359d8963ac81b967e3e544ec"
id: "DND-023"
status: "blocked"
priority: "P2"
milestone: "M4 feasibility"
kind: "implementation / missing feature"
dependencies: ["DND-022", "DND-015"]
---

# DND-023 — CAN face for the link-context core

## State and permission

Status: **blocked**, until DND-015 names a CAN target (controller, classic CAN or CAN FD) and DND-022 exists. Filed in DND-R8 because research-2 §8.1 (A2) names a "CAN face ticket".

Decision (2026-11-08): on classic CAN, authenticate command routes only; authenticated data goes on CAN FD. Applies once a target is chosen.

## Scope

- Route id carried in the CAN identifier (11 or 29 bit), so a frame carries only the `LinkFrame` payload or sealed fields. This is design decision 1 (DND-R6-fix).
- Classic CAN, measured with the draft schema:
  - telemetry gets 6 B of data per frame;
  - an authenticated command (7-bit counter + 3 B MAC) gets **1 B**.
  - So authenticated commands need CAN FD in practice; the 2026-11-08 decision applies once the target is chosen.
- CAN FD: zone key (PSK) profile, 8 B tag, about 51 B of data.
- **Failed-MAC counters with lockout and an alarm are mandatory** (A2): a 3 B MAC gives 2^-24 per try, about 2.3 h expected per forged command at 2,000 frames/s (inferred).
- Fragmentation for larger messages (the `frag` field), with one reassembly buffer per route.

## Non-goals

Choosing the target (DND-015), key provisioning (DND-009), gateways to other buses (DND-014).

## Acceptance

- [ ] Frame layout tests: telemetry 6 B and command 1 B on classic CAN, about 51 B on CAN FD, at counters 127/128.
- [ ] Failed-MAC lockout: after N failures the route locks and an alarm counter rises; recovery needs a new handshake.
- [ ] Hardware runs require explicit approval; the first tests use a virtual CAN or fake face.

## Dated log

- 2026-11-09: DND-R8. Filed from research-2 §8.1 A2 and the 2026-11-08 CAN decision. Blocked on DND-015's target. No code.

[Ticket index](index.md) · [Design](../design/link-context-wire.md) · [DND-022](DND-022-link-context-core.md)
