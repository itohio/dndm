# DND-R5 evidence (working tree, not committed)

Pin `c7b5b61f79ea07d3359d8963ac81b967e3e544ec`; go1.27.2 darwin/arm64 (Apple M5); `GOTOOLCHAIN=local GOPROXY=off`. "Pin" runs use `tmp/dndm/pin-r4`: the pinned tree plus the DND-R4 step-0 `Dropped()` stub plus the test file named. Task: Leader DND-R5 (msg m-6418e80e17ee). Verify's DND-R4 report: [`docs/verification/DND-R4.md`](../../../../docs/verification/DND-R4.md).

## What changed

| Item | Ticket | Tests (written first) | Before | After |
|---|---|---|---|---|
| Residual risk A3/A4 recorded | [DND-005](../../tickets/DND-005-caller-scoped-correlation.md#residual-risk-verify-dnd-r4-probes-a3a4), [DND-009](../../tickets/DND-009-identity-auth-and-profile-boundary.md) | — | — | docs |
| Later owner auth/zone statements copied verbatim | DND-009 | — | — | 5 quotes, script-checked against `decisions.md` |
| Streaming correlation (acceptance 3) | DND-005 | `TestCallerStreamingRepliesKeepCallIdentity` | pin: 3/3 FAIL (call b got `a2`, `a3`; `Dropped` 0) | ok ×30 `-race` |
| Pluggable nonce source | DND-005 | `TestCallerPredictedNonceForgery` (counter / random / crypto) | random and crypto 3/3 FAIL: "a predicted-nonce forgery was accepted" (counter case passes: the forgery lands, as intended) | all ok ×30 `-race`; whole package ok with each build default |
| F21 ticket | [DND-020](../../tickets/DND-020-remote-linker-route-direction.md) | sketch only | — | no code |
| RC6 interest close vs in-flight Send | [DND-007](../../tickets/DND-007-bounded-dispatch-and-closure.md#implementation-evidence) | `closure_test.go` (4 tests) | pin: panic `send on closed channel` 3/3; fan-in race 3/3 | ok ×30 `-race`, ×100 without |
| Stream frame writes, `contextRW` ownership, dialing flag, dial context | [DND-008](../../tickets/DND-008-stream-and-peer-lifecycle-races.md#implementation-evidence) | `network/stream/ownership_test.go`, `endpoint/mesh/addrbook_test.go`, `network/net/net_test.go` | all fail at the pin (3/3; the addrbook test with `-race`) | ok ×30 `-race` |

Logs: [nonce-before](nonce-before.txt), [nonce-after](nonce-after.txt), [streaming-before-pin](streaming-before-pin.txt), [rc6-before](rc6-before.txt), [rc6-after](rc6-after.txt), [stream-before](stream-before.txt), [stream-after](stream-after.txt), [addrbook-before](addrbook-before.txt), [addrbook-after](addrbook-after.txt), [dial-before](dial-before.txt), [dial-after](dial-after.txt).

## Nonce source: selection and overhead

- **Static selection, two ways.**
  - Build tag picks the default: a plain Go build uses `CryptoNonce`; TinyGo or `-tags dndm_nonce_counter` uses `CounterNonce`; `-tags dndm_nonce_random` uses `RandomNonce`.
  - `WithCallerNonceSource(src)` overrides it per Caller. It is an interface value, called once per `Call`/`HandleReplies`.
- **Per call** ([bench](nonce-bench.txt), 5 runs): 0 B/op and 0 allocs/op for all three.

  | Source | Best ns/op | Median ns/op |
  |---|---|---|
  | counter | 2.05 | 2.17 |
  | random (math/rand/v2) | 4.73 | 5.00 |
  | crypto (crypto/rand) | 61.7 | 63.5 |

- **Code size** ([size](nonce-size.txt)): a scratch binary that only calls `NewCaller` links only the default source's `Next`. Stripped sizes are 7,768,098 B (crypto), 7,768,082 B (counter) and 7,768,050 B (random), so unused sources cost nothing. crypto/rand is linked anyway (the counter's random start).
- Not measured: TinyGo builds (no TinyGo toolchain here).

## Gates

| Gate | Command | Result | Log |
|---|---|---|---|
| Build, vet, gofmt | `go build ./...`; `go vet ./...`; `go vet -tags dndm_nonce_counter` and `-tags dndm_nonce_random ./x/bus`; `gofmt -l .` | exit 0; exit 0; exit 0 ×2; empty | [gate-build-vet.txt](gate-build-vet.txt) |
| Race | `go test -race -count=3 -timeout 900s -v ./...` | 8 packages ok (new: `network/net` has a test now) | [gate-race-count3.txt](gate-race-count3.txt) |
| Race reports | `race-dedupe.py` | 0 reports | [gate-race-unique.txt](gate-race-unique.txt) |
| No race ×10 | `go test -count=10 -timeout 900s ./...` | 8 packages ok | [gate-norace-count10.txt](gate-norace-count10.txt) |
| x/bus ×20 | `-count=20` without and with `-race` | ok both, 0 race reports | [xbus-flake-count20.txt](xbus-flake-count20.txt) |
| Exported API | `go doc -all`, 11 packages, pin vs tree | additive only. New in DND-R5: `bus.NonceSource`, `CounterNonce`/`NewCounterNonce`, `RandomNonce`, `CryptoNonce`, `WithCallerNonceSource`, and `(*dndm.Link).Close`, which used to be promoted from `Base` with the same signature | [api-diff.txt](api-diff.txt) |
| Wire format | fixed-header frame probe, pin vs tree | byte-identical | [wire-probe-base.txt](wire-probe-base.txt), [wire-probe-current.txt](wire-probe-current.txt) |
| EasyRobot | 4-target `go build -o /dev/null` with `GOFLAGS=-mod=readonly` (EasyRobot `abb5c9c`) | exit 1, same as at the pin: `cmd/manipulator` builds; opencv4 missing for two; TinyGo-only for one. EasyRobot tree: only `?? docs/` | [easyrobot-build.txt](easyrobot-build.txt) |

**Incident, fixed.** A scratch size probe needed `GOFLAGS=-mod=mod`. That variable stayed exported in the persistent shell, so the API diff (`go doc`, at 21:58:29Z) and the first EasyRobot build (21:58:46Z) ran with it:

- dndm `go.sum` gained one `golang/protobuf … /go.mod` line.
- EasyRobot `go.mod` moved `spf13/cobra` from indirect to direct.

Both files were restored byte-for-byte from `HEAD`; each was clean before (DND-R4 gates; verify's DND-R4 status check), and the mtimes match the two commands. The API diff and the EasyRobot build were then rerun with `GOFLAGS=-mod=readonly`. Afterwards `git status` shows dndm `go.mod`/`go.sum` unchanged and EasyRobot `?? docs/` only. The test gates before the API diff ran with `-mod=mod` but wrote nothing (mtimes). A final race run with `-mod=readonly` is in [gate-final-readonly.txt](gate-final-readonly.txt).

**One unexplained failure.** That final run failed `TestContainer_Publish` once, in 0.03 s; the log kept only the summary, not the assertion. It did not come back:

| Run | Repetitions | Failures |
|---|---|---|
| `TestContainer_Publish` alone, `-race -count=20` | ×10 | 0 |
| Full suite, `-race -count=3` | ×8 | 0 |
| Full suite, `-race -count=10` | ×6 | 0 |
| `TestContainer_Publish`, 8 parallel processes × 300 runs, working tree | 2,400 | 0 |
| Same, at the pin | 2,400 | 0 |
| Root package, 3 processes × 80 | 240 | 0 |

[INFERENCE, static] The test asserts a mock expectation, `MockIntent.Interest`, that only the fan-out's notify goroutine satisfies, and it waits for that goroutine with two 1 ms sleeps. That code is unchanged since the pin. Not fixed: there is no repro and no assertion text. Next time, rerun with `-v` output kept.

## Behaviour changes vs the pin (new in DND-R5)

| Area | Before | After |
|---|---|---|
| Caller nonces | counter from a random start (DND-R4) | build default: crypto/rand per call (TinyGo: counter); per-Caller override |
| `LocalIntent.Send` when the link is removed while it waits | panic if the interest closed, otherwise blocked | returns `ErrNoInterest` |
| `Link(nil)`, `Link.Close` | returned with Sends possibly still in flight; `Link.Close` returned early when another goroutine was closing | wait until no Send uses the old channel (bounded: waiting Sends are released) |
| `LocalInterest` close | closed `msgC` first | closes `msgC` after its other close callbacks |
| `FanInInterest` close | channel could close under a runner | runners stop first |
| `Stream.Write` | concurrent calls could interleave in the writer | serialised |
| `StreamContext` read after a cancelled read | bytes for the cancelled read were lost; its buffer was written after it returned | bytes go to the next read; abandoned buffers are not touched |
| `StreamContext.Write` cancelled | `ErrNotEnoughBytes`; buffer back in the pool while being written | context error; buffer not reused |
| `net.Node.Dial` | ignored ctx | honours ctx |

## Not done / not verified

- DND-008 close/reconnect coordination (acceptance item 3): no fixture or design yet.
- `DialOpt` has no fields, so there are no options to honour.
- Serial dial context: not changed; testing it needs hardware.
- TinyGo build of the counter default.
- DND-020 is not implemented (as instructed).
- DND-006/009/010/011 have no design code (as instructed).

[DND-005](../../tickets/DND-005-caller-scoped-correlation.md) · [DND-007](../../tickets/DND-007-bounded-dispatch-and-closure.md) · [DND-008](../../tickets/DND-008-stream-and-peer-lifecycle-races.md) · [DND-020](../../tickets/DND-020-remote-linker-route-direction.md) · [Ticket index](../../tickets/index.md)
