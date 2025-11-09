# DNBus Remediation Plan

## Context

The current implementation of `github.com/itohio/dndm/x/dnbus` diverges from the API described in `SPEC.md` and is effectively unusable:

- `Producer.Send` expects a handler callback and closes the intent after the callback returns, rather than sending a typed message as documented. This prevents simple publish flows and breaks callers that follow the SPEC contract.
- `Consumer.Receive` requires a handler callback instead of returning messages (`(T, error)`), which makes it impossible to write idiomatic receive loops and breaks higher-level wrappers.
- `Caller`, `Service`, and `Module` rely on the incorrect producer/consumer APIs, so builds fail and request/reply flows cannot function.
- There are no unit tests covering these behaviours, so regressions were not detected.

## Goals

1. Realign the exported API with `SPEC.md` while preserving backwards-compatible helpers where sensible.
2. Ensure `Producer`, `Consumer`, `Caller`, `Service`, and `Module` cooperate using the corrected contracts.
3. Provide minimal but meaningful unit-test coverage for the critical flows (publish, subscribe, call, service, module lifecycle).
4. Keep the solution composable, testable, and compliant with repository rules (composition over inheritance, explicit dependencies, small functions).

## Proposed Changes

### Producer

- Replace the handler-based `Send` with the documented signature `Send(ctx context.Context, msg T) error`, deferring to an internal `sendWithContext` helper.
- Implement `SendWithTimeout` by wrapping the context with a timeout before delegating to `Send`.
- Ensure `Send` waits for interest exactly once (using the existing mutex/flag) and does not close the intent automatically.
- Retain `SendDirect` as an advanced helper; it should re-use the same interest-checking path so callers who manage the lifecycle manually still work.

### Consumer

- Change `Receive` to `Receive(ctx context.Context) (T, error)` to return typed messages and propagate `context` cancellation or `io.EOF` when the stream ends.
- Keep `C()` as the typed channel escape hatch.
- Guard against concurrent closure via a mutex; mark the consumer closed when the underlying interest/channel terminates.

### Caller

- Update request sending to use the corrected `Producer.Send` API.
- Replace the map-only correlation store with a small struct that tracks both a nonce-to-channel map and a FIFO queue; this allows:
  - Optional nonce-based routing when a user-supplied extractor is provided.
  - Deterministic FIFO routing when replies do not carry a nonce (maintains current behaviour but correct and race-free).
- Expose functional options so users can provide `SetRequestNonce` and `GetResponseNonce` hooks without inflating the public surface area.
- Ensure `handleResponses` drains consumers until context cancellation, closing all waiting channels safely.

### Service & Module

- Update to rely on the corrected `Consumer.Receive` and `Producer.Send` helpers.
- For `Handle`, keep the goroutine-per-request model from the spec but use `SendDirect` for multi-reply flows after waiting for interest once.

### Tests

Add table-driven unit tests under `x/dnbus` covering:

1. `Producer` interest waiting and send semantics (including timeout path).
2. `Consumer` receive loop, typed channel conversion, and graceful closure.
3. `Caller.Call` happy-path roundtrip using the FIFO correlation fallback.
4. `Service.Handle` sending multiple replies via the reply callback.
5. `Module` lifecycle (adding inputs/outputs, closing).

Tests will use an in-memory router created via `dndm.New()` with no endpoints, leveraging only local routing behaviour. Contexts will be short-lived with deterministic deadlines to keep runtime low.

## Open Questions / Follow-ups

- The nonce-based correlation remains optional due to the lack of standard fields in request/response messages. Document the option hooks and consider adding protobuf mixins in a subsequent update.
- Evaluate whether `SendDirect` should remain exported long term once the API stabilises; keep for now for backwards compatibility.


