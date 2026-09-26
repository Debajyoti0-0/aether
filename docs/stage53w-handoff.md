# Stage 53W — Handoff

Captured (UTC): 2026-09-26.

## 1. Trunk state

| Item | Value |
| --- | --- |
| Branch | `reconciliation/stage53r` |
| HEAD | `4364ab4` (matrix fix); docs commit lands on top |
| `master` (local) | `c75732a404d054aae730685916b467ffbc1b9096` |
| `master` (origin) | `c75732a404d054aae730685916b467ffbc1b9096` |
| `VERSION` | `5.0.0-alpha2` |
| v5 tags | none |
| v4 tags | 11, preserved |

`master` was not promoted, moved, reset, or force-pushed. Stage 54 §38 holds.

Commits this stage: `4364ab4` (flag-matrix completeness). Preceded by Stage 53V's
`f5a81c9`, `c624f55`, `9c23a23`, `e47226a`.

## 2. Workdir discipline

Every command in this stage ran with `workdir C:\dev\aether`, and repository
identity (`git rev-parse --show-toplevel`, `--abbrev-ref HEAD`, `rev-parse HEAD`)
was confirmed at pre-flight and re-verified at exit. The Stage 53V hazard still
applies: PowerShell's `Set-Location` does **not** change the process working
directory that .NET file APIs resolve against, so absolute paths are required for
any `[System.IO.*]` call.

## 3. Matrices — now verified, and more complete than reported

Verified against the shipped binary's own `--help` output, not against the
generator that produced them:

| Artifact | Binary | Matrix | Missing | Extra |
| --- | ---: | ---: | ---: | ---: |
| Command nodes | 155 | 155 | 0 | 0 |
| Flag bindings | 1101 | 1101 | 0 | 0 |
| Flag duplicates | — | 0 | — | — |

**The "945 flags" figure propagated since Stage 53V was wrong.** The binary
exposes 1101 flag bindings; the matrix held 945 because the in-process walk never
initialised cobra's auto-added `--help` (155 commands) and `--version` (root).
Both are now included — `--help` matters because it is the flag that returns
`flag.ErrHelp`, the documented cause of cobra skipping argument validation.

Nodes reconcile with Stage 53S's 152 exactly: 155 − root − help − completion = 152.

All six matrices still regenerate byte-identically (SHA-256 drift 0 of 6).

Hazard class documented in `docs/stage53w-newrootcommand-hazard.md`. The rule:
**derive CLI trees from the same entry point the binary uses, and diff against the
binary before trusting the output.**

## 4. Security review (G53R-26) — executed, found a P1

13 items executed with raw evidence: 10 PASS, 1 FAIL, 1 FLAKY, 1 PARTIAL.
Full detail in `docs/stage53w-security-review.md`.

| ID | Sev | Disposition | Summary |
| --- | --- | --- | --- |
| SEC-53W-01 | **P1** | **DEFERRED** (fix required) | LDAP LDAPS **and** StartTLS hardcode `InsecureSkipVerify: true`. No CA pool, no override flag on any of the 14 commands. `Bind` sends credentials over an unauthenticated channel. |
| SEC-53W-02 | P2 | ACCEPTED (quarantined) | Untracked `artifacts/stage54/{security-results,browser-runs}.json` assert figures that do not reproduce (34 packages vs 35/52; 10 fuzz targets vs 9). Not adopted. |
| SEC-53W-03 | P3 | DEFERRED (RCA-INCOMPLETE) | One unexplained race-suite failure, not reproduced in 10 subsequent full runs. Output was not captured — a disclosure, not a clean result. |

**SEC-53W-01 blocks Stage 54 production qualification.** It is an internal,
fixable defect with a five-step remediation in the review document. It was not
fixed here because flipping the TLS default would break every existing
`--tls`/`--starttls` call against lab-issued certificates — a posture decision
with real blast radius that needs its owner, not a drive-by change.

**G53R-26 = `BLOCKED-WITH-OWNER`.**

## 5. Browser scope — resolved by documented reduction

`docs/stage53w-browser-scope.md`. **Option A**, no code change.

- Chromium/Chrome and Edge: **QUALIFIED** (CDP).
- Firefox 156.0.1: installed, **NOT PERFORMED** — the harness is CDP-only and
  Firefox does not implement CDP. Installing it was never the fix.
- Safari: **NOT PERFORMED**, cannot run on Windows.

The reduction is defensible because the code under test is engine-neutral —
verified, not assumed: no WebGL/WebGPU, no `SharedArrayBuffer`/`Atomics`, no
Workers, therefore no COOP/COEP cross-origin isolation requirement. The surfaces
are Canvas 2D, WebAssembly, fetch and DOM.

That is an architectural argument, not a test result, and the document says so.

Deferred: WebDriver BiDi or Playwright harness (Stage 55/56), after Stage 52b
settles the dashboard.

## 6. Stage 54 readiness

**Ready to execute.** Its first deliverable set — the command/flag/argument
matrices — is now verified complete against the binary instead of silently short.

**The 66-gate matrix remains unperformed.** No promotion decision is possible.

Carried-forward constraints for whoever runs it:

- Start from `reconciliation/stage53r`, not `master`.
- Do not `git add -A`: a blanket add stages 191 files / 71.2 MB including ~108 MB
  of stale stage-47 binaries.
- Do not adopt `artifacts/stage54/security-results.json` or `browser-runs.json`
  (SEC-53W-02).
- G3206 stays `BLOCKED-WITH-OWNER`. Samba/FreeIPA/Certipy/mock-CA substitutions
  are prohibited.
- Firefox/Safari are `NOT PERFORMED`, not passing.
- Static analysis covers 3 linters; 668 findings outstanding
  (`docs/stage53v-linter-scope.md`).

## 7. Stage 52b — starting point

**Start from `reconciliation/stage53r` @ the Stage 53W docs commit.** Clean tree,
all gates green, matrices verified.

Unchanged work: graph operation persistence, checkpoint/replay via
`/api/graph/at/{seq}`, evidence-backed time travel, HTMX, fonts, and the 50,000-node
performance target.

Two facts Stage 52b should inherit rather than re-derive:

- **WebGL is deliberately not implemented.** `internal/web/static/graph.js:3-18`
  documents the measured reasoning: layout runs in `graph.wasm`, the 2D canvas
  draw pass is adequate for engagement-scale graphs, and a WebGL path that
  silently falls back to a software rasteriser is slower and less predictable.
  The 50,000-node claim is recorded **UNVERIFIED**.
- **No Workers, no `SharedArrayBuffer`, no `Atomics`** in the current build, so
  there is no cross-origin isolation requirement to preserve. Adding workers will
  change the browser support surface and interacts with the Firefox/Safari deferral.

G3607 (Samba/AD lab) remains separate from G3206 and is still never started.

## 8. Stage 47 readiness

**`BLOCKED-WITH-OWNER` on G3206** — requires genuine Windows Server 2022 AD DS, a
KDC, and Enterprise AD CS. Operator script:
`scripts/ad-lab/adcs-windows-setup.ps1`.

`internal/protocol/ms-wcce/` does not exist on this branch; the MS-WCCE work is
stranded, not lost, and no port was performed this stage.

SEC-53W-01 lands on the same owner's desk.

## 9. Recommended order

1. **SEC-53W-01** — the P1. It gates Stage 54 and it is internal.
2. **Stage 52b** — unblocked, independent of Stage 53T's status.
3. **Stage 54** — execute the 66-gate matrix; promotion stays blocked until it passes.
4. **Stage 47** — when the operator provisions real AD DS + AD CS.
5. **Static analysis** — 668 findings, `gosec` G101/G404 and `staticcheck` first.
6. **Browser harness rewrite** — Stage 55/56.

## 10. Uncommitted work

Nothing uncommitted from this stage's code or documents. Substantial untracked
evidence remains on disk by design and was not staged or deleted: ~230 MB across
`artifacts/stage47`, `stage53s`, `stage53t`, `stage54` and the stage 4–7 backfill
directories, the six pre-existing unverified `docs/stage54-*.md` files, and the two
quarantined result JSONs from SEC-53W-02. Classification is Stage 54 §46 work.
