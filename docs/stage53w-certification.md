# Stage 53W — Certification

**Exit status: `BLOCKED-WITH-OWNER`**

Reason: the G53R-26 security review was executed and found a **P1** — LDAP TLS
certificate verification is unconditionally disabled with no operator override,
on the path that carries bind credentials. A review that surfaces a P1 has not
produced a clean result, and an internal fixable defect must not be waived for
convenience.

- Repository: `C:\dev\aether`
- Branch: `reconciliation/stage53r` @ `4364ab4`
- `master`: `c75732a404d054aae730685916b467ffbc1b9096` (local and origin, unmoved)
- `VERSION`: `5.0.0-alpha2` · v5 tags: none

## Workstreams

| WS | Requirement | Status | Evidence |
| --- | --- | --- | --- |
| WS1 | Verify corrected matrices against the binary | **DONE — verified, and found incomplete** | `artifacts/stage53w/matrix-verification.txt` |
| WS2 | G53R-26 security review | **DONE — executed; P1 found** | `docs/stage53w-security-review.md` |
| WS3 | Browser cross-platform | **DONE — Option A, documented** | `docs/stage53w-browser-scope.md` |
| WS4 | `NewRootCommand` hazard | **DONE — fix already applied, now documented** | `docs/stage53w-newrootcommand-hazard.md` |
| WS5 | Finalize Stage 53T | **DONE — BLOCKED-WITH-OWNER** | `docs/stage53t-certification.md` |
| WS6 | Handoff | **DONE** | `docs/stage53w-handoff.md` |
| WS7 | Master not promoted | **VERIFIED** | below |
| WS8 | Certification | this document | — |

## Matrix verification — the "945 flags" figure was wrong

Verified against the shipped binary's own `--help` output, not against the
generator that produced the matrices.

| Artifact | Binary | Matrix | Missing | Extra |
| --- | ---: | ---: | ---: | ---: |
| Command nodes | 155 | 155 | **0** | **0** |
| Flag bindings | 1101 | 1101 | **0** | **0** |
| Flag duplicates | — | 0 | — | — |

Node reconciliation with Stage 53S's 152 is exact: 155 − root − help − completion.

**F-53W-01 (fixed).** Verification found the matrix still short by 156 bindings:
`--help` on all 155 commands and `--version` on the root. Both are attached by
cobra during execution, so the in-process walk never saw them. `--help` is not
cosmetic — it returns `flag.ErrHelp`, the documented cause of cobra skipping
argument validation, so the matrix must carry it to qualify that path. Adding
`InitDefaultHelpFlag`/`InitDefaultVersionFlag` triggers `mergePersistentFlags`,
which emitted 6 persistent flags twice; the persistent loop now reclassifies
rather than duplicates.

Result: `flag-matrix.json` = 1101 entries, exactly matching the binary. The
"945 flags" figure propagated since Stage 53V understated the real surface by 14%.

**F-53W-02 (fixed, in Stage 53V).** `command-inventory.json` shelled out to a
gitignored, stale `aether.exe` and discarded exec errors, emitting a plausible
1-command inventory. Now walks the live in-process tree.

All six matrices regenerate byte-identically (drift 0 of 6).

## Security review (G53R-26) — 13 items executed

10 PASS · 1 FAIL · 1 FLAKY · 1 PARTIAL. Findings, all dispositioned:

| ID | Sev | Disposition | Summary |
| --- | --- | --- | --- |
| SEC-53W-01 | **P1** | **DEFERRED — fix required** | LDAP LDAPS + StartTLS hardcode `InsecureSkipVerify: true`; no CA pool, no override on any of 14 commands; `Bind` sends credentials over an unauthenticated channel |
| SEC-53W-02 | P2 | ACCEPTED — quarantined | Untracked `security-results.json` / `browser-runs.json` assert figures that do not reproduce (34 packages vs 35/52; 10 fuzz targets vs 9). Not adopted, not committed |
| SEC-53W-03 | P3 | DEFERRED — RCA-INCOMPLETE | One unexplained race failure; 10 subsequent full runs clean; output not captured |

**SEC-53W-01 blocks Stage 54.** Five-step remediation in the review document.
Not fixed here because flipping the TLS default would break every existing
`--tls`/`--starttls` call against lab-issued certificates — a posture decision
needing its owner, and the charter carries a hard no-regression rule.

**G53R-26 = `BLOCKED-WITH-OWNER`.**

## Browser cross-platform — Option A, no code change

`docs/stage53w-browser-scope.md`. Chromium/Edge **QUALIFIED**; Firefox 156.0.1
installed but **NOT PERFORMED**; Safari **NOT PERFORMED**.

The reduction is justified by measurement, not assertion: no WebGL/WebGPU, no
`SharedArrayBuffer`/`Atomics`, no Workers, therefore no COOP/COEP requirement. The
exercised surfaces are Canvas 2D, WebAssembly, fetch and DOM. That is an
architectural argument, not a test result, and the document states so.

Disposition: **`WAIVED-WITH-OWNER`** for Stage 53T closure, scoped strictly to
cross-browser qualification. It does not cover browser stability, cleanup, or
process ownership, which remain separately gated on Chromium evidence.

## Two inaccuracies in the entry gate, corrected

1. **"The generators were fixed to call `Execute()`."** They were not, and calling
   `Execute()` is not a fix — it would *run* the root command, whose `RunE`
   prints help. The actual fix moved `loadModules()` into `NewRootCommand()`.
2. **"The generators are lazy during `Execute`."** Only cobra's
   `help`/`completion` commands and `--help`/`--version` flags are lazy. The
   `ad` and `ldap` subtrees were never attached at all, because only `Execute()`
   called `loadModules()`.

Both distinctions matter: one is a workaround that executes the program being
inspected, the other is a misdiagnosis that would send a fixer to the wrong place.

## Disclosures

- **SEC-53W-03's evidence gap is mine.** The failing race run's output was
  consumed by a summary expression and discarded, so the failing package and
  stack are unrecoverable. A clean re-run does not substitute for the lost
  evidence, and this is recorded rather than papered over.
- **The brief's premise that "0 issues" was near-meaningless was correct**, and
  Stage 53V's measured response (3 linters, 668 enumerated findings) stands. The
  lint result is still scoped to `govet`, `ineffassign`, `misspell`.

## Gate matrix

| Gate | Requirement | Result |
| --- | --- | --- |
| G4081–G4083 | Identity, baseline, positions | PASS |
| G4084–G4086 | Matrices, generators, harness present | PASS |
| G4087 | Firefox presence | PASS — 156.0.1 confirmed via `Get-AppxPackage` |
| G4088–G4089 | Linter config, tools | PASS |
| G4090 | Baseline build/vet/test/race | PASS |
| G4091–G4095 | Matrix verification | **PASS after fix** — 155/1101, 0/0 |
| G4096–G4097 | Checklist defined and executed | PASS — 13/13 |
| G4098 | Findings dispositioned | PASS — 3/3 |
| G4099 | G53R-26 status | **BLOCKED-WITH-OWNER** (P1) |
| G4100–G4101 | Browser decision + scope doc | PASS — Option A |
| G4104 | Hazard documented | PASS |
| G4105 | Fix applied | **already applied in Stage 53V** (`c624f55`) |
| G4106 | Stage 53T updated | PASS |
| G4107 | Handoff | PASS |
| G4108 | Master not promoted | PASS — `c75732a` local and origin |
| G4109 | Certification | this document |
| G4110–G4114 | Build, vet, unit, race, govulncheck | PASS |
| G4115 | No v5 tag | PASS |
| G4117 | No Stage 47 code | PASS — no `internal/protocol/ms-wcce/` |
| G4118 | ≤ 6 documents | PASS — 5 created |
| G4119 | Working tree clean | PASS |
| G4120 | No regression | PASS |

## Stage 53T status

**Before:** `BLOCKED-WITH-OWNER` (G53R-26 + browser cross-platform).
**After:** **`BLOCKED-WITH-OWNER`** — browser cross-platform is resolved by
documented scope reduction; **G53R-26 remains, now with a specific P1 rather than
an unperformed review.**

This is progress of a real kind: the blocker is no longer "nobody did the review",
it is "the review was done and here is the specific defect".

## Promotion

`master` remains at `c75732a` on local and origin. Promotion deferred to Stage 54
§38. Not performed, not authorized, not attempted.

## Stage 54 status

**The 66-gate matrix remains unperformed.** Stage 54 is ready to execute — its
matrix deliverables are verified complete rather than silently short — but no
qualification decision is possible and no production claim may be made.

## Documents produced

1. `docs/stage53w-security-review.md`
2. `docs/stage53w-browser-scope.md`
3. `docs/stage53w-newrootcommand-hazard.md`
4. `docs/stage53w-handoff.md`
5. `docs/stage53w-certification.md` (this document)
6. `docs/stage53t-certification.md` (closure appended)

Supporting evidence in `artifacts/stage53w/` does not count against the limit.
