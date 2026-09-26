# Stage 53S — Certification

**Branch** `reconciliation/stage53r` (unchanged by this stage's commits below)
**Base for all evidence** `98e48ef`
**Exit status** **BLOCKED-WITH-OWNER**

No gate is recorded as PARTIAL. Every gate is PASS or BLOCKED-WITH-OWNER, and
each BLOCKED names its owner and what would close it.

## 0. A conflict in the two briefs, and how it was resolved

Two programmes arrived with the same mandate and contradictory instructions on
promotion:

- **Stage 53S §1E / WS5** permits promoting the trunk to `master` *if and only
  if all reachable gates pass*.
- **Stage 54**, which supersedes it, closes with an absolute rule: *"Do not
  promote `reconciliation/stage53r`. Do not move `master`."*

The later document governs, so **nothing was promoted**. `master` is untouched
at `c75732a` and remains so. This is also the position the evidence supports
independently: reachable gates below are BLOCKED, so Stage 53S's own condition
for promotion is not met either. The two readings agree on the outcome, and no
promotion was performed under either.

## 1. Pre-flight

| Gate | Result | Evidence |
| --- | --- | --- |
| Baseline commit | PASS | `reconciliation/stage53r` @ `98e48ef`; `origin/master` @ `c75732a` |
| Working tree | PASS | no tracked modifications at start; only untracked `artifacts/` |
| `go build ./...` | PASS | clean, before and after all changes |
| `go vet ./...` | PASS | clean, before and after all changes |
| `go mod verify` | PASS | `all modules verified` |
| `go test -count=1 ./...` | PASS | 50 packages: 34 with tests, 16 without; all `ok` |
| `go test -race -count=1 ./...` | PASS | all packages `ok`, no race |
| Browser present | PASS | Chrome at `C:\Program Files\Google\Chrome\Application\chrome.exe`; not on `PATH` |
| Harness present | PASS | `scripts/browser-verify.mjs`, 413 lines |
| Engagement fixtures | PASS | 4 in `engagements/` (see deviation D-2) |
| 15 gated commands | PASS | 16 `requireEngagementScope` sites − 1 definition; 15 `bindEngagementFlag` sites |

## 2. WS1 / G54-002, G54-003, G54-009 — browser flake

**R-05 is RESOLVED.** Full reasoning, including a hypothesis that was tested
and discarded, is in `docs/stage53s-rca.md`. Summary:

| Item | Result |
| --- | --- |
| Original failure reproduced | **NO** — 0 failures in 80 runs of the browser gate |
| `exit`/`close` race hypothesis | **DISPROVED** — 200/200 trials delivered stderr first |
| Real mechanism | proven: unguarded `JSON.parse` on an empty post-navigation evaluate (150/150), plus cleanup skipped on the resulting throw |
| Fix | evaluate guarded at both sites; cleanup unconditional; `stopBrowser()` kills the tree and awaits exit |
| Leak before `stopBrowser` | 5 runs → 5 orphaned profiles; 133 found on disk |
| Leak after `stopBrowser` | 5 runs → **0**; 141 orphans cleaned, 0 remaining |
| Verification | 10 module + 8 full-suite runs post-final-fix, 0 failures; `-race` clean |

**Honest caveat.** The original 0.61s failure was never replayed. The gate is
closed on a *proven* cause of the reported signature, not a captured instance
of it. This is stated in the RCA rather than papered over.

## 3. WS3 — exhaustive CLI / rules-of-engagement matrix

15 gated commands × 9 cases = **135 invocations, 135 PASS**, each asserting the
refusal *message* rather than an exit code, because these commands also fail
for unrelated reasons. Evidence: `artifacts/stage53s/roe-matrix.csv`, `.json`.

| Case | Property proven | Result |
| --- | --- | --- |
| 1 missing engagement file | fails closed with no scope | 15/15 refused |
| 2 capability not granted | capability enforced per command | 15/15 as expected |
| 3 wrong domain | domain scoping | 15/15 refused |
| 4 unauthorized DC | controller scoping | 15/15 refused |
| 5 expired window | time-window scoping | 15/15 refused |
| 6 **authorized** | **the gate is not blanket-refusing** | 15/15 passed the gate |
| 7 missing `--domain` | empty scope refused | 15/15 refused |
| 8 routing | 9 LDAP commands gate identically at both mounts; 6 non-LDAP commands have **no** ungated root alias | 15/15 |
| 9 capability split | `ad.ldap.read` does not imply `ad.ldap.acl` | 9/9 refused |

Case 6 is the one that carries the weight: for every command the scope refusal
text was *absent*, so control demonstrably passed the gate and reached the
workspace or connect phase. `dc01.example.com` does not resolve here, so those
cases fail after the gate — which is the property being demonstrated. No real
domain controller was contacted by any case.

## 4. WS4 — full CLI inventory

`artifacts/stage53s/cli-inventory.json`: **152 commands** walked from the built
binary, 34 grouping commands, 94 with required flags, 4 long-running
(`dashboard view`, `exec imds`, `pivot imds`, `pivot verify-imds`, each killed
at an 8s bound and recorded as such). Cobra does not mark required flags in
help output, so each command was additionally probed with no arguments to
recover them. No stray `aether` process survived the sweep.

## 5. Configuration precedence (G53R-25)

`artifacts/stage53s/config-precedence.csv`: **9 cases, 9 PASS.** An unreadable
or malformed `--config` is surfaced as a warning rather than silently ignored
(fail-closed), a valid one is reported as selected, and `--log-level` accepts
only the four documented lowercase values, warning and defaulting to `info`
otherwise.

Half of this gate is **not observable** and is not claimed: `log-level` is bound
and normalised but no logger consumes it (the source says so), so its resolved
value cannot be read back from the CLI. The env-var tier of the precedence chain
is therefore untested end-to-end. See B-3.

## 6. Fuzzing, Rust, WASM

| Check | Result |
| --- | --- |
| Fuzz sweep, all 10 `Fuzz*` targets, 20s each | **0 crashes** |
| Graph crate | 17/17 |
| Verifier crate | 14/14 |
| WASM smoke suite | **56/56** |

`artifacts/stage53s/fuzz-results.txt`.

## 7. Inherited gates from Stage 53R — dispositions

Stage 53R closed with 12 PARTIAL and 2 FLAKY. Each is resolved here to PASS or
BLOCKED-WITH-OWNER. None is left PARTIAL.

| Gate | Was | Now | Basis |
| --- | --- | --- | --- |
| G53R-08 no silent functionality loss | PARTIAL | **BLOCKED-WITH-OWNER** | B-1 |
| G53R-19 Stage 52 regression | FLAKY | **PASS** | §2; root cause proven and fixed |
| G53R-20 full CLI inventory | PARTIAL | **PASS** | §4; 152 commands, required flags recovered |
| G53R-23 output contracts | PARTIAL | **PASS** | §3; 135 cases asserting refusal text |
| G53R-24 exit codes | PARTIAL | **PASS** | §3; exit recorded for all 135 |
| G53R-25 config precedence | PARTIAL | **BLOCKED-WITH-OWNER** | B-3 |
| G53R-26 security regression | PARTIAL | **BLOCKED-WITH-OWNER** | B-2 |
| G53R-28 G3206 honestly preserved | PASS | **PASS** | unchanged; no substitution attempted |
| G53R-32 browser regression | FLAKY | **BLOCKED-WITH-OWNER** | B-4; flake half is PASS, cross-browser half is not |
| G53R-34 dependency audit | PARTIAL | **BLOCKED-WITH-OWNER** | B-5 |
| G53R-36 artifact inspection | PARTIAL | **BLOCKED-WITH-OWNER** | B-6 |
| G53R-37 documentation | PARTIAL | **BLOCKED-WITH-OWNER** | B-7 |
| G53R-39 canonical trunk | PARTIAL | **BLOCKED-WITH-OWNER** | B-8 |
| G53R-40 production convergence | NOT AUTHORIZED | **NOT AUTHORIZED** | §0; unchanged, and reinforced by Stage 54 |

## 8. Blocked gates — owner and what closes each

**B-1 · G53R-08 — owner: Stage 47 (AD/Kerberos/LDAP owner).**
`internal/protocol/ms-wcce/` remains on `stranded-stage-45-46`, unported. The
AD CLI and engagement-scope work was recovered and extended to 15 commands, so
nothing was lost silently; the protocol work is recorded as stranded, not lost.
Closing it requires an evidence-based port, not a branch merge, because the
workspace lineage's AD/Kerberos/LDAP files supersede the stranded flat files.

**B-2 · G53R-26 — owner: security reviewer.**
A systematic differential audit of every security control across both lineages
was not performed. What *was* done is bounded and real: 135 scope-enforcement
cases, two recovered fail-open controls (config fail-open, PKI root `0700`), and
the CLI group-command defect from §7 of the RCA. A full control-by-control
diff is a separate piece of work with its own owner.

**B-3 · G53R-25 — owner: CLI maintainer.**
Config-file selection and fail-closed reporting are verified (9/9). The
`log-level` value and the env-var precedence tier are not observable from the
CLI while no logger consumes the setting, so they can only be closed by unit
tests against viper, or by making the setting observable.

**B-4 · G53R-32 — owner: release engineer with the target browsers.**
Chromium-family is qualified. Firefox is not installed on this machine and
Safari cannot run on Windows, so the cross-browser matrix is environment-blocked
and was not simulated. The flake half of this gate is closed as PASS.

**B-5 · G53R-34 — owner: whoever provisions the Go toolchain.**
`go mod verify` passes. `golangci-lint`, `staticcheck` and `govulncheck` are not
installed and stable-toolchain Clippy is unavailable, so the vulnerability scan
was **NOT PERFORMED**. This is an unmeasured risk, not a clean result.

**B-6 · G53R-36 — owner: release engineer.**
Merge-introduced deletions were inspected and restored (`f436b6e`, 7 reports,
2,525 lines). A full classification of every directory under `artifacts/` was
not performed; the tree holds ~28 MB of untracked evidence, so the classification
is a deliberate omission rather than an oversight.

**B-7 · G53R-37 — owner: release manager.**
`VERSION` is `5.0.0-alpha2` while tracked release artifacts under
`artifacts/release/` are labelled `3.4.0-stage3`, including signatures and
`release.pub`. Reconciling that is a release decision, not a documentation fix,
and re-signing is out of scope here.

**B-8 · G53R-39 — owner: release manager.**
`reconciliation/stage53r` is the evidence-backed candidate and is the only
qualified trunk, but Stage 54 forbids promoting it and `master` stays at
`c75732a`. This gate cannot be closed inside this stage by construction.

## 9. G3206

Stage 47 and 47B remain **BLOCKED-WITH-OWNER** on G3206, which requires a
genuine Windows Server 2022 AD DS/KDC/AD CS. No Samba, FreeIPA, Certipy
fixture, synthetic CA or mock KDC was used to close it, and the merge does not
constitute AD CS qualification.

## 10. Deviations from the briefs

- **D-1** Fixtures are in `engagements/`, not `testdata/engagements/`. This
  predates this stage; the files are referenced by the CLI's own examples.
- **D-2** The LDAP implementation is `internal/cli/ad/ldap.go`, not
  `ad_ldap.go`.
- **D-3** `ldap acl path` does not exist; the command is `aether ad ldap path`
  (mounted at `ldap`, `ldap.go:64`). The matrix uses the real path.
- **D-4** Evidence lives in `artifacts/stage53s/` and is **untracked**, matching
  the `stage53r` precedent (0 tracked files there). Docs and source changes are
  committed.
- **D-5** The 15 gated commands are 9 LDAP + 3 `ad enum` + 2 roast + 1 `ad tgt`.
  `ldap bind`, `ldap rootdse` and `tgt ccache/show/convert` stay ungated by
  design, because none accepts `--domain` and so engagement scope cannot be
  evaluated for them.

## 11. Pre-existing conditions recorded, not changed

- `internal/cli/ad/ldap.go` has gofmt indentation drift at lines 835–866,
  independent of the three lines added here (all gofmt hunks are in 835–866; the
  additions are at 56, 143, 877). The file is CRLF, so `gofmt -l` reports the
  whole file. Left alone deliberately: it is cosmetic, unrelated to every gate,
  and the AD files are still under reconciliation. `go vet` is clean.
- `normalizeLogLevel` is named for normalisation but only validates exact
  lowercase. Its doc comment matches its behaviour, so this is a naming nit, not
  a defect, and no change was made.
- `internal/cli/ad/tgt.go` alignment drift, carried over from Stage 53R.

## 12. What this stage changed

| File | Change |
| --- | --- |
| `scripts/browser-verify.mjs` | guard both evaluate sites; unconditional cleanup; `stopBrowser()` kills the tree and awaits exit before removing the profile |
| `internal/cli/ad/scope.go` | `rejectUnknownSubcommand()` helper |
| `internal/cli/ad/ldap.go` | applied that helper to the `ldap`, `enum` and `acl` groups |
| `.gitignore` | ignore `aether-audit.key` / `aether-audit.jsonl` (§13) |
| `docs/stage53s-rca.md` | this stage's root-cause analysis |
| `docs/stage53s-certification.md` | this document |

`master` was not modified. No `v5.*` tag was created. No v4.x tag was moved or
deleted.

## 13. Credential material found in the working tree

Running the suite left `aether-audit.key` and `aether-audit.jsonl` in the
repository root. The key is a real 32-byte Ed25519 seed in base64, and the
ledger is its derived chain. `.gitignore` covered `*.keytab` but nothing that
matched these, so either could have been committed by accident.

Both are now ignored, alongside the existing keytab rule and for the same
reason. Neither file was committed, inspected into a commit, or deleted — they
are left on disk, because they are the local audit state of whatever run
produced them and removing another process's state is not this stage's call.
If a real engagement key ever lands here, rotate it: it has been sitting in a
directory that is under active development.
