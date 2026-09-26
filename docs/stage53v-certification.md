# Stage 53V — Certification

**Exit status: `BLOCKED-WITH-OWNER`**

Stage 53V's own scope is complete: the Stage 53U work is committed, the G53R-25
contradiction is resolved, linter breadth is decided and measured, and every named
blocker is dispositioned. Stage 53T cannot be certified COMPLETE because two
blockers require work that was not performed here, and both are internal and
closable.

- Repository: `C:\dev\aether`
- Branch: `reconciliation/stage53r` @ `9c23a23`
- `master`: `c75732a404d054aae730685916b467ffbc1b9096` (local and origin, unmoved)
- `VERSION`: `5.0.0-alpha2`
- v5 tags: none

## Workstreams

| WS | Requirement | Status | Evidence |
| --- | --- | --- | --- |
| WS1 | Commit the Stage 53U working tree | **DONE** | `f5a81c9` |
| WS2 | Resolve the G53R-25 contradiction | **DONE — PASS** | `artifacts/stage53v/g53r-25-*.log` |
| WS3 | Linter breadth decision | **DONE — Response B, A measured** | `docs/stage53v-linter-scope.md` |
| WS4 | Disposition the named blockers | **DONE** | `artifacts/stage53v/gate-dispositions.md` |
| WS5 | Finalize Stage 53T | **DONE — BLOCKED-WITH-OWNER** | `docs/stage53t-certification.md` |
| WS6 | Handoff | **DONE** | `docs/stage53v-handoff.md` |
| WS7 | Master not promoted | **VERIFIED** | `artifacts/stage53v/master-verification.txt` |
| WS8 | Certification | this document | — |

## Commits

| SHA | Content |
| --- | --- |
| `f5a81c9` | Stage 53U: build repair + `ineffassign` clearance |
| `c624f55` | Command tree and generated matrices made complete |
| `9c23a23` | Linter breadth, measured and recorded |

All three pushed to `origin/reconciliation/stage53r`. `master` untouched.

## Quality gates at `9c23a23`

| Gate | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 ./...` | PASS, 0 failures |
| `go test -race -count=1 ./...` | PASS, 0 races |
| `go mod verify` | PASS |
| `golangci-lint run ./...` | PASS — 0 issues across `govet`, `ineffassign`, `misspell` |
| `govulncheck ./...` | 0 code-affected, 0 in imported packages |

**Scoping statements that must travel with these results.** The lint result covers
three linters, not comprehensive static analysis: ten linters are deliberately
disabled with 668 measured findings outstanding, 240 of them `gosec`. The
vulnerability result is 0 *code-affected*; one module-level finding
(GO-2026-5932, `x/crypto/openpgp`, unimported, no fix published) is recorded as
accepted no-action risk and must not be restated as "no dependency findings".

## Defects found and fixed this stage

**F-53V-01 — repository-root build was broken while recorded as passing.**
`go build ./...` and `go vet ./...` failed with `main redeclared` because two
standalone `package main` generators shared `artifacts/stage54/`. Stage 53T's
baseline recorded both as PASS. Fixed by relocating each generator to its own
package; all six matrices regenerate byte-identically. Corrects a false PASS
claim inherited from the prior stage.

**F-53V-02 — the CLI inventory omitted every security-relevant command.** The
generated matrices held 110 commands against the binary's 152. `NewRootCommand`
is documented as returning the full tree but left module loading to `Execute`,
which the generators never call; cobra's `help` and `completion` are attached
during `Execute` too. All 25 `ad` and 14 `ldap` commands — the entire
engagement-gated surface — were missing from the flag, argument, command-detail and
exit-code matrices. Fixed; generated tree now matches the binary exactly.

**F-53V-03 — the inventory generator failed silently.** It shelled out to a
gitignored, stale `aether.exe` and discarded exec errors, so a failed run parsed
empty output and emitted a plausible 1-command inventory instead of failing.
Rewritten to walk the live in-process tree and to report write errors.

**F-53V-04 — `loadModules` was not idempotent.** Making it reachable from
`NewRootCommand` required a guard: cobra's `AddCommand` appends unconditionally, so
an unguarded second call registers the same command pointers twice and duplicates
every module subcommand in help output and in generated inventories.

**F-53V-05 — evidence encoding.** `artifacts/stage53u/golangci-lint.txt` was
UTF-16LE with BOM, so Git classified it as binary. Converted to UTF-8.

**F-53V-06 — 6 of 135 scope cases proved the wrong thing.** They were marked PASS
on a non-zero exit, but `refused:false` records that cobra's required-flag check
short-circuited before the engagement-scope control ran. Re-run with all four
required flags; all 6 now reach the control and refuse with a specific engagement
error.

## Incorrect claims corrected

Three inherited statements were false and are corrected here:

1. **"Firefox is not installed."** False. Firefox 156.0.1 is installed and
   launchable via a WindowsApps AppX alias, which is why standard-path detection
   missed it. The real blocker is that the harness speaks CDP only and Firefox does
   not implement CDP — an architecture gap, not an environment gap.
2. **"golangci-lint = 0 issues."** True but nearly meaningless as stated: it
   covered one linter with `staticcheck` disabled. Now three linters, with the
   remaining 668 findings enumerated.
3. **"G53R-25 blocked" (Stage 53U).** Stage 53T's PASS was correct. Stage 53U
   carried a stale blocker list forward.

## Two invalid results caught before reporting

Recorded because a plausible log is worse than none:

- The first behavioral run splatted a **string** with `@`, so PowerShell expanded
  `--log-level=error` per character into `unknown command "l"`. Discarded and re-run
  with argument arrays.
- A refusal classifier matched the substring "engagement" inside cobra's own
  `required flag(s) "engagement" not set` and scored flag validation as an
  authorization refusal. Discarded and re-run with all required flags supplied.

## Gate matrix

| Gate | Requirement | Result |
| --- | --- | --- |
| G4021 | Workdir + branch | PASS |
| G4022 | Baseline, VERSION, no v5 tags | PASS |
| G4023 | Branch positions | PASS |
| G4024 | Working tree enumerated | PASS |
| G4025 | Linter config captured | PASS |
| G4026 | Tools available | PASS (both off `PATH`, invoked by full path) |
| G4027 | Gates located | PASS |
| G4028 | Baseline build/vet/test | PASS |
| G4029–G4031 | Commit staged, created, tree clean | PASS |
| G4032–G4035 | G53R-25 run and determined | PASS — status PASS |
| G4036–G4040 | Linter breadth decided and recorded | PASS — Response B, A measured |
| G4041–G4045 | Blockers dispositioned | PASS — 2 blocked with owners |
| G4046 | Stage 53T status updated | PASS |
| G4047 | Handoff written | PASS |
| G4048 | Master not promoted | PASS — `c75732a` local and origin |
| G4049 | Certification issued | this document |
| G4050–G4054 | Build, vet, unit, race, govulncheck | PASS |
| G4055 | No v5 tag | PASS |
| G4056 | No Stage 52b code | PASS |
| G4057 | No Stage 47 code | PASS — no `internal/protocol/ms-wcce/` |
| G4058 | ≤ 4 documents | PASS — 3 created |
| G4059 | Working tree clean at exit | PASS |

### Deviations from the brief, and why

1. **`git add -A` was not used.** It would have staged 191 files / 71.2 MB,
   including ~108 MB of stale stage-47 binaries and six unverified pre-existing
   `docs/stage54-*.md` files. The commits are scoped to intended paths.
2. **Response B was chosen for linter breadth, after measuring Response A.**
   Response A produced 668 findings, so "bounded" was false. Findings were neither
   suppressed nor waived; they are enumerated with an owner.
3. **The Stage 54 generator was repaired rather than accepted.** It was a required
   Stage 54 deliverable and it was silently omitting 45 commands.
4. **Two extra defects were fixed** (F-53V-04, F-53V-05) because the first was
   required to make the second safe, and the second blocked clean evidence.

## Residual blockers

**R-1 · G53R-26 security differential — owner: security reviewer.**
`BLOCKED-WITH-OWNER`. A control-by-control differential across both reconciled
lineages was **not performed**. Existing bounded work: the 135-case scope matrix
(re-verified), 16 `requireEngagementScope` call sites over 15 gated commands with
authorization ordered before workspace open and network access, two recovered
fail-open controls. Remedy: build the control inventory for both lineages, diff
control-by-control, disposition every divergence.

**R-2 · Browser cross-platform — owner: release engineer.**
`BLOCKED-WITH-OWNER`. Chromium qualified. Firefox and Safari **NOT PERFORMED**.
Remedy: adopt WebDriver BiDi (Firefox 156 supports it) or a per-engine driver
abstraction in `scripts/browser-verify.mjs`, then qualify Chrome and Firefox.
Safari requires macOS or a real device and cannot be closed on Windows.

**R-3 · Static-analysis breadth — owner: toolchain owner.**
`BLOCKED-WITH-OWNER`. 668 findings outstanding, including 240 `gosec`
(G101 11 and G404 11 first, then `staticcheck` 41). See
`docs/stage53v-linter-scope.md`.

## Stage 53T status

**Before:** `BLOCKED-WITH-OWNER`.
**After:** **`BLOCKED-WITH-OWNER`** — G53R-26 and browser cross-platform remain.
G53R-20, G53R-23, G53R-24, G53R-25 and G53R-34 are now RESOLVED, and the
previously false PASS claims on `go build`/`go vet` are corrected.

## Promotion

`master` remains at `c75732a` on both local and origin. Promotion is deferred to
Stage 54 §38 and requires the Stage 54 production matrix to pass. Not performed,
not authorized, not attempted.

## Stage 52b

Cleared to begin from `reconciliation/stage53r` @ `9c23a23`. No Stage 52b code was
written. No Stage 47 code was written.

## Documents produced

1. `docs/stage53v-certification.md` (this document)
2. `docs/stage53v-handoff.md`
3. `docs/stage53v-linter-scope.md`
4. `docs/stage53t-certification.md` (closure section appended)

Supporting evidence is under `artifacts/stage53v/` and does not count against the
four-document limit.
