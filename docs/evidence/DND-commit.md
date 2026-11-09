# DND-commit report (dndm, 2026-11-09)

**Result:** 13 local commits on `main` (c7b5b61..31321d3): the 10 code commits from review R-DND-R5's plan and 3 docs commits for verified rounds. Each commit was gated as it was made. Nothing was pushed. Author: Andrius Mikonis <andrius.mikonis@gmail.com>, from the workspace include config.

Task: Leader message m-919189e493e1 (GO after verify V-DND-R5 with its fix round, and review R-DND-R5 with its recheck).

## Commits

| # | Hash | Message |
|---|---|---|
| 1 | 5e2532a | fix(codec): read whole frames with a body cap; malformed input wraps ErrBadFrame [DND-001] |
| 2 | fff99f7 | fix(net): Dial honours its context [DND-008] |
| 3 | c93ba0c | fix(mesh): addrbook AddConn under lock; dialing flag and retry log without races [DND-004, DND-008] |
| 4 | 1f85174 | fix(core): Base owns close callbacks; synchronous ordered Close; close-vs-send and router readiness [DND-018, DND-007] |
| 5 | 7d088f7 | fix(stream): contextRW owns buffers and never closes its request channel; serialised frame writes; idempotent Close [DND-019, DND-008, DND-018] |
| 6 | 214f490 | fix(remote): Close no longer waits on its own handler [DND-019] |
| 7 | 9d6515f | fix(mesh): gate publish on handshake readiness; Container registers routes when ready [DND-004] |
| 8 | 464a02d | fix(core,remote): remote interests have one reader; remote wrappers implement RemoteIntent/RemoteInterest [DND-003] |
| 9 | 3907a77 | docs(api): message ownership contract for Send and C [DND-002] |
| 10 | 12b4916 | feat(x/bus): caller-scoped nonces with a pluggable nonce source [DND-005] |
| 11 | 225a73c | docs: baseline build/vet/race evidence at c7b5b61 [R1-04] |
| 12 | 69662b9 | docs(diagrams): Mermaid diagrams of the core idea, link flows, message paths and robot scenarios [DND-R6-diagrams, DND-R6-fix] |
| 13 | 31321d3 | docs: tickets, roadmap and DND-R2…R5 evidence [DND-R2, DND-R3, DND-R4, DND-R5, DND-R5-fix] |

## Deviations from review's plan

- **Order.** Review's #5 (mesh readiness) failed its gate when committed right after core. The handshake tests need the stream fix (RC5 race) and the remote Close fix, and without them the mesh package hung to the 900 s timeout. Following review's rule ("if a split cannot be gated green, …"), I moved stream (#6) and remote (#7) ahead of it instead of merging them, so each ticket still has its own commit. Final order: codec, net, addrbook, core, stream, remote, mesh, DND-003, DND-002, DND-005.
- **Files from the fix round placed per review:**
  - F1 and `scratch_test.go` → stream;
  - F2 and `ready_close_test.go` → mesh;
  - F3 → core: `x/bus/errors.go`, the `busErr` hunks of `producer.go`, `bus_test.go`, `errclosed_test.go`;
  - the nonce-source fix and `nonce_source_test.go` → x/bus nonces.
- **Hunk splits:**
  - `endpoint.go`: Base/lifecycle → core; `ReadyEndpoint` → mesh.
  - `container.go`: Close → core; readiness hunks → mesh.
  - `dndm.go`: error checks → core; RemoteInterest skip → DND-003.
  - `intent.go`, `interest.go`: the ownership doc hunks → DND-002; the rest → core.
  - `producer.go`: busErr → core; doc → DND-002.
- **Docs commits:**
  - Review's #12 (ideation) and the design half of #13 are held, as instructed.
  - Diagrams are committed in their DND-R6-fix version. The DND-R7-design-fix edits (flows 5/6, the S7 caption, the render-check header) stay in the working tree.
  - Review's #14 (tickets/evidence) is committed last; it includes `DND-R5-fix` and `dnd-r5-fix/`.

## Gates

Each code commit's tree was exported from the index (`git checkout-index`) to `tmp/dndm/gate` and checked offline (`GOFLAGS=-mod=readonly`) with `go build ./...`, `go vet ./...`, `gofmt -l .` and `go test -race -count=1 ./...`.

- **Commits 1–3:** the root package and x/bus fail with exactly the tests that fail at the pin c7b5b61. I ran the pin itself twice: root (RC1–RC4) and x/bus fail there too. Every package a commit touches passes, and no test outside the pin's failing set fails.
- **Commit 4 onwards:** root passes.
- **From commit 5:** the whole suite passes, except one pin-era x/bus flake (`TestStreamProducerConsumer`) seen once at commit 7 before the DND-003 fix.
- **Commit 10:** also with `-tags dndm_nonce_counter` and `-tags dndm_nonce_random`.
- Each commit body states its own gate result.
- **Final check at HEAD** (`git archive HEAD`): build, vet and gofmt clean; `go test -race -count=3 ./...` 8/8 packages ok, 0 races.
- **Docs commits:** no Go change. A link check over the tracked docs at each commit found 0 broken links, but some links point to held files (below).
- Gate logs: `tmp/dndm/commit-gates/`.

## Left uncommitted

| Path | Why |
|---|---|
| `docs/design/link-context-wire.md`, `docs/design/close-reconnect.md`, `docs/evidence/DND-R7-design.md`, `docs/evidence/DND-R7-design-fix.md` | held: verify's DND-R7-design-fix recheck is pending |
| `docs/diagrams/link-flows.md`, `robotics.md`, `render-check.txt` (modified) | the DND-R7-design-fix edits to the committed diagrams; held with the design round |
| `docs/research/ideation/*` | held: ideation owes one citation fix |
| `docs/evidence/DND-commit.md` | this report |

13 links in committed docs point to held files: design docs, `evidence/DND-R7-design.md` and the ideation notes. They resolve in the working tree and will resolve in history once those files are committed.

## Network contacts

None.

## Next action

Once verify rechecks DND-R7-design-fix, commit the design docs, their evidence and the held diagram edits, and commit the ideation notes after their citation fix. Then write the DND-008 close/reconnect failing tests and the frame encoder.
