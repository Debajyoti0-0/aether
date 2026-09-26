# Stage 53V — Handoff

Captured (UTC): 2026-09-26.

## 1. Trunk state

| Item | Value |
| --- | --- |
| Working branch | `reconciliation/stage53r` |
| Branch HEAD | `c624f5512f5bc9e025f03956825109a742ea873a` |
| Verified on origin | YES — `origin/reconciliation/stage53r` = `c624f55` |
| `master` (local) | `c75732a404d054aae730685916b467ffbc1b9096` |
| `master` (origin) | `c75732a404d054aae730685916b467ffbc1b9096` |
| `VERSION` | `5.0.0-alpha2` |
| v5 tags | none |
| v4 tags | 11, preserved on `master` |

`master` was **not** promoted, moved, reset, or force-pushed. Per Stage 54 §38 this
stays frozen until the Stage 54 production matrix passes.

Two commits landed this stage:

- `f5a81c9` — Stage 53U: build repair + `ineffassign` clearance
- `c624f55` — command tree and generated matrices made complete

## 2. Workdir discipline — reaffirmed, with a live example

**Every command that reads or writes the repository must target `C:\dev\aether`,
via the `workdir` parameter or an explicit path.**

This is not theoretical. During this stage, a `Set-Location C:\dev\aether`
followed by `[System.IO.File]::ReadAllBytes("artifacts\...")` failed with
`DirectoryNotFoundException` — .NET resolves relative paths against the *process*
working directory, which was still the OneDrive workspace, not PowerShell's
location. The shell was in the right place; the API was not.

Consequences that actually bit this stage:

- Stage 53U's `go build`/`go vet` results were reported against the OneDrive
  workspace while the session's stated working directory was that workspace and
  the authoritative repository was `C:\dev\aether`. The baseline "PASS" was
  inherited from a run in the wrong tree.
- `git add -A` in the intended repository would have staged 191 files / 71.2 MB,
  including ~108 MB of stale stage-47 binaries and six unverified pre-existing
  `docs/stage54-*.md` files. The Stage 53U commit was scoped to 9 files / 84 KB
  for this reason.

Rule: **verify repository identity before and after any bulk operation**
(`git rev-parse --show-toplevel`, `--abbrev-ref HEAD`, `rev-parse HEAD`).

## 3. Stage 52b — starting point

**Start from `reconciliation/stage53r` @ `c624f55`.** The branch is clean, pushed,
and every quality gate green.

State Stage 52b inherits:

- Command tree and all four generated matrices are now complete and verified
  identical to the shipped binary (155 nodes, 945 flags).
- `go build`, `go vet`, `go test -count=1`, `go test -race`, `golangci-lint`
  (0 issues) and `govulncheck` (0 code-affected) all pass at this commit.
- Browser harness is trustworthy for Chromium and is CDP-only. Firefox/Safari are
  **NOT PERFORMED** — see §5. Treat Chromium-only as the current proven scope;
  do not describe the dashboard as cross-browser.
- 135-case engagement-scope matrix green, with the 6 short-circuited cases closed.

Remaining Stage 52b work, unchanged: signed graph mutations, checkpoint/replay via
`/api/graph/at/{seq}`, WebGL, worker-based WASM layout, 50,000 nodes at average
≥55 FPS for 10 seconds, vendored assets, `dashboard.read` default-deny, and
verifier WASM built with `wasm-pack ... --no-opt`. Checkpoint every 1,000 graph
operations; replay from the nearest checkpoint.

**If Stage 52b changes production-critical components, the Stage 54 gates touching
them must be re-run.** Stage 52b and Stage 54 stay traceable.

## 4. Stage 47 — readiness

**BLOCKED-WITH-OWNER on G3206.** Requires genuine Windows Server 2022 AD DS, a KDC
and Enterprise AD CS. Samba, FreeIPA, Certipy, synthetic CAs and mock KDC
responses are prohibited substitutes and were not used.

- `internal/protocol/ms-wcce/` does not exist on this branch (verified: no such
  directory). The MS-WCCE work is recorded as stranded, not lost.
- G3607 (Samba/AD lab) and G3206 (Windows AD DS + AD CS) remain **separate**
  statuses. Do not merge them.
- Operator remedy script: `scripts/ad-lab/adcs-windows-setup.ps1`.

## 5. Browser cross-platform — corrected finding

Firefox **is** installed (156.0.1, AppX/WindowsApps). Prior stages recorded it as
unavailable because it is invisible to standard-path detection. The real blocker
is that `scripts/browser-verify.mjs` speaks the DevTools protocol only, and
Firefox does not implement CDP. Remedy: WebDriver BiDi or a per-engine driver
abstraction, then qualify Chrome and Firefox. Safari needs macOS.

## 6. Static analysis — corrected scope

`.golangci.yml` now enables `govet`, `ineffassign`, `misspell` (all measured at
zero findings). Ten linters are deliberately not enabled, with **668 measured
findings** enumerated in `docs/stage53v-linter-scope.md`. "0 issues" is a claim
about those three linters only, and must never be restated as comprehensive
static analysis.

## 7. Recommended order

1. **Stage 52b** — unblocked, start from `c624f55`.
2. **Stage 47** — after the operator provisions real Windows AD DS + AD CS (G3206).
3. **Stage 54** — full production qualification; master promotion stays blocked
   until its matrix passes.
4. **Static-analysis remediation** — 668 findings, prioritized in
   `docs/stage53v-linter-scope.md`; `gosec` G101/G404 and `staticcheck` first.

## 8. Uncommitted work

None. The tree is clean at `c624f55`. Substantial untracked evidence remains on
disk by design and is **not** committed: ~230 MB across `artifacts/stage47`,
`stage53s`, `stage53t`, `stage54` and the stage 4–7 backfill directories, plus six
pre-existing unverified `docs/stage54-*.md` files. Nothing there has been deleted
or staged. Classifying it is Stage 54 §46 work.
