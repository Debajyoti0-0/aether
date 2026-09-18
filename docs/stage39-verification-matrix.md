# Stage 39 — Strict Completion Verification Matrix: Stages 10–26

Baseline: master @ `77acc53` (HEAD); `VERSION=4.2.0-rc1`; tree clean (only
untracked tooling dir `.kilo/`, excluded from all counts). Binary tested:
`bin/aether-39.exe`, built from current master with the corrected ldflags
binding `-X github.com/Debajyoti0-0/aether/internal/version.Version=4.2.0-rc1`
→ `aether version 4.2.0-rc1`.

Source matrices: `docs/stage33b-charter-closure-matrix.md` (Stages 10–18,
13 rows) and `docs/stage35b-false-cert-closure-matrix.md` (Stages 20–26,
13 rows). Every row is re-verified below with a raw command executed at
Stage 39; no disposition is taken on cited-document evidence alone.

Status vocabulary: VERIFIED / DOWNGRADED / UNSUPPORTED / FAILED.
**UNSUPPORTED 0 · FAILED 0.**

Environment notes (raw): `gh` installed but unauthenticated (`gh auth
login` required); `ENTRA_LAB_TENANT` unset; `AZURE_DEV_VM` unset;
observation window OPEN (closes 2026-10-17T16:55:20Z).

## Matrix A — Stage 33b rows (Stages 10–18)

| # | Stage | Charter item | Disposition (Stage 33b) | Verification command (raw) | Raw result (Stage 39) | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 10 | CI pipeline execution | CLOSED-BY-SUCCESSOR | `Invoke-RestMethod .../actions/workflows/ci.yml/runs?per_page=1` (unauthenticated API fallback; gh itself BLOCKED: `gh auth login` required) | ci \| completed \| **failure** \| 2026-09-16T11:26:40Z | **DOWNGRADED** → BLOCKED-WITH-OWNER | F-32-3 lineage (stage32/33-certification §F-32-3); latest ci.yml run is RED, not green |
| 2 | 10 | EV cert | PERMANENTLY-WAIVED | `Test-Path docs/stage13-phase2-b4-waiver.md` + `Select-String 'Owner\|Expiry'` | file exists; Expiry **2027-03-31**; Owner **Repository owner (Debajyoti Haldar)**; Risk + Compensating controls sections present | VERIFIED | waiver file on disk |
| 3 | 10 | HSM/KMS custody | PERMANENTLY-WAIVED | `Test-Path docs/stage13-b5-waiver.md` + `Select-String 'Owner\|Expiry'` | file exists; Expiry **2027-03-31**; Owner **Repository owner (Debajyoti Haldar)**; Risk + Compensating controls present | VERIFIED | waiver file on disk |
| 4 | 10 | Release Validate | CLOSED-BY-SUCCESSOR | release.yml run 35253502855 job query (v4.2.0-rc1, 2026-09-17T17:33:50Z) | job **"Validate (go vet, build, test)" → failure**; downstream jobs skipped; local B7 qualification doc present with PASS rows | **DOWNGRADED** → BLOCKED-WITH-OWNER (CI) / local-verified | stage13-phase2-b7-ci-qualification.md; release.yml #35253502855 |
| 5 | 11 | Live Entra validation | PERMANENTLY-WAIVED (access-dependent) | `[ -n "$ENTRA_LAB_TENANT" ]` | `ENTRA_LAB_TENANT: UNSET` → BLOCKED-WITH-OWNER; waiver register entry stage35b §waiver-verification: LIVE-1 expiry 2027-06-30, owner repository owner (inline in the 35b matrix; no standalone waiver file) | VERIFIED (waiver-register-verified) | stage35b-false-cert-closure-matrix.md §waiver table; stage35-certification.md:22 |
| 6 | 11 | Live IMDS validation | PERMANENTLY-WAIVED (access-dependent) | `[ -n "$AZURE_DEV_VM" ]` | `AZURE_DEV_VM: UNSET` → BLOCKED-WITH-OWNER; same LIVE-1 register entry (expiry 2027-06-30, owner repository owner) | VERIFIED (waiver-register-verified) | stage35b §waiver table; stage35-certification.md:22 |
| 7 | 12 | Tag freeze | CLOSED-BY-SUCCESSOR | `git rev-list -n1 v4.0.0-rc2` + `git for-each-ref refs/tags` | `5cd008be1e3e20b039214a911626a6a4426c7838`; tag object `779cdb6f`, annotated, peeled matches | VERIFIED | git output; stage39 matrix row 11 |

| 8 | 13 | Tag repair | CLOSED-BY-SUCCESSOR | `git show v4.2.0-rc1:VERSION` | `4.2.0-rc1` (clean; no BOM) — content-proof standard holds on the current tag | VERIFIED | git output |
| 9 | 14 | Tag repair | CLOSED-BY-SUCCESSOR | `git for-each-ref` all release tags | v4.0.0-rc2 `779cdb6f`→`5cd008be`; v4.1.0-rc2 `e8416b58`→`bc674af1`; v4.2.0-rc1 `bd0d9636`→`bb56cf3` — all annotated, all peeled-commit match | VERIFIED | git output |
| 10 | 15 | Tag audit | CLOSED-BY-SUCCESSOR | `git ls-remote origin v4.2.0-rc1` | `bd0d9636004c51954466d7c809d1bff337f7d37e refs/tags/v4.2.0-rc1` (remote == local) | VERIFIED | git ls-remote output |
| 11 | 17 | `v4.0.0-rc2` created | CLOSED-BY-SUCCESSOR | `git rev-list -n1 v4.0.0-rc2` | `5cd008be1e3e20b039214a911626a6a4426c7838` (unchanged at every freeze) | VERIFIED | git output |
| 12 | 18 | GA rule decision | CLOSED-BY-SUCCESSOR | `Select-String docs/stage19-terminal-decision.md 'terminal\|rc2'` | "**Option B — `4.0.0-rc2` TERMINAL**"; Stage 29 posture RETAIN Production-Limited confirmed | VERIFIED | stage19-terminal-decision.md:8; stage29-decision.md:18 |
| 13 | 16 | Tag repair | CLOSED-BY-SUCCESSOR | `git cat-file -t v4.0.0-rc2` + Stage 17 repair lineage | `tag` (annotated); Stage 17 repair lineage cited by 33b; v4.0.0-rc2 intact | VERIFIED | git output; stage17-tag-repair lineage |

## Matrix B — Stage 35b rows (Stages 20–26)

| # | Stage | False claim | Disposition (Stage 35b) | Verification command (raw) | Raw result (Stage 39) | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 14 | 20 | Observability | CLOSED-BY-SUCCESSOR | PKI bootstrap (`serve cert init`) + `serve --metrics-addr 127.0.0.1:9090` + GET `/metrics`,`/healthz`,`/readyz` | **/metrics 200** · **/healthz 200** · **/readyz 503 fail-closed** | VERIFIED | live HTTP output this stage |
| 15 | 20 | Revocation | CLOSED-BY-SUCCESSOR | `export verify-evidence --revocation=ocsp --issuer testdata/ca.pem --cert testdata/leaf.pem --responder .../ocsp-good.der` | **Status: GOOD / EVIDENCE ACCEPTED / exit 0**; CRL-file GOOD exit 0; ocsp-revoked → **REVOKED exit 2**; crl-revoked → **REVOKED exit 2** (exit-code contract now enforced — Stage 27 defect fixed) | VERIFIED | raw CLI output this stage |
| 16 | 20 | ARM64 | CLOSED-BY-SUCCESSOR | `$env:GOOS='linux'; $env:GOARCH='arm64'; go build ./...` (and darwin/arm64) | linux/arm64 **PASS**; darwin/arm64 **PASS** (compile verified; runtime honestly still UNVERIFIED) | VERIFIED (with standing runtime caveat) | go build output |
| 17 | 20 | IdP validation | CLOSED-BY-SUCCESSOR | `providers validate okta --domain http://127.0.0.1:8081 --token test-token-not-real` against `testdata/okta-200.py` mock | **Token validation: OK / OIDC discovery: OK / Okta validation: OK / exit 0** | VERIFIED | raw CLI output this stage |


| 18 | 20 | Tag `v4.1.0` (planned) | FALSIFIED-AND-CORRECTED | `git tag -l v4.1.0` + `git show v4.1.0:internal/cli/export_verify.go` | tag list **empty**; content-proof **fails**: `fatal: invalid object name 'v4.1.0'` — withdrawal holds | VERIFIED | git output (Stage 27 cited) |
| 19 | 22 | FIX-1/2/3 interim-status claim (retired label) | CLOSED-BY-SUCCESSOR | `--version` (FIX-1) + live `/metrics`,`/healthz`,`/readyz` (FIX-2/3) | version **4.2.0-rc1** (correct binding); `/metrics` 200, `/healthz` 200, `/readyz` 503 fail-closed; the Stage 22 interim label is absent from the Stage 28+ status discipline | VERIFIED | raw output this stage |
| 20 | 23 | `CERTIFIED 4.1.0` | FALSIFIED-AND-CORRECTED | `git tag -l v4.1.0` | **empty** — certification preserved as false, tag withdrawn | VERIFIED | git output (Stage 27 cited) |
| 21 | 23 | Tag "to be created" | FALSIFIED-AND-CORRECTED | `git show v4.1.0:VERSION` | fails (`invalid object name`) — claim moot; Stage 28 content-proof standard is the replacement procedure | VERIFIED | git output (Stage 27/28 cited) |
| 22 | 24 | `CERTIFIED 4.1.0` (wrong content) | FALSIFIED-AND-CORRECTED | `git show v4.1.0:internal/cli/export_verify.go` | `fatal: invalid object name 'v4.1.0'` — the wrongly-certified content no longer exists under any v4.1.0 ref | VERIFIED | git output (Stage 27 RCA cited) |
| 23 | 25 | `PUBLISH 4.1.0` | FALSIFIED-AND-CORRECTED | Stage 27 withdrawal + stage29-decision.md posture check | withdrawal holds (tag absent); current posture **RETAIN 4.1.0-rc2 Production-Limited**, controlled publication only | VERIFIED | stage27-certification.md; stage29-decision.md:18 |
| 24 | 25 | FIX-4/FIX-5 `PASS` | CLOSED-BY-SUCCESSOR | full success+negative revocation matrix + okta validate (rows 15/17) | OCSP/CRL good→exit 0, revoked→exit 2; okta validate exit 0 — success AND negative paths green on the current binary | VERIFIED | raw CLI output this stage (FIX-5 success path exercised via the okta mock fixture; live IdP remains under LIVE-1) |
| 25 | 26 | `4.1.0 STABLE` | FALSIFIED-AND-CORRECTED | stage29-decision.md + observation log check | **RETAIN Production-Limited** (not STABLE); real observation window OPEN, hard time gate 2026-10-17T16:55:20Z | VERIFIED | stage29-decision.md; stage29-observation-log.md |
| 26 | 26 | "Production observation complete" | FALSIFIED-AND-CORRECTED | `Get-Content docs/stage29-observation-log.md -Tail 6` | log ends with the window-integrity rule (closes 2026-10-17; "Until then the GA decision is RETAIN by definition") — no promotion before close | VERIFIED | observation log tail |

## DOWNGRADED rows (2)

1. **Stage 10 CI pipeline execution**: CLOSED-BY-SUCCESSOR →
   **BLOCKED-WITH-OWNER**. The 33b row's own fine print already said CI
   *execution* is F-32-3 BLOCKED-WITH-OWNER; Stage 39 confirms with a raw
   run query (latest ci.yml run = failure, 2026-09-16). gh remains
   unauthenticated. Matrix updated at source.
2. **Stage 10 Release Validate (B7)**: CLOSED-BY-SUCCESSOR →
   **BLOCKED-WITH-OWNER (CI) / local-verified**. Local B7 qualification
   PASS rows exist (stage13-phase2-b7-ci-qualification.md), but the
   release.yml run on the v4.2.0-rc1 ref failed at the Validate job
   (#35253502855, 2026-09-17T17:33:50Z, inside the observation window —
   an observation event, not a charter re-run). Matrix updated at source.

## Count reconciliation

* Stage 33b declares 13 rows → 13 rows verified above.
* Stage 35b declares 13 rows → 13 rows verified above. (35b's
  retrospective block reprints a 6+7 split of its own 13 rows; the
  matrix table itself is authoritative: 13 rows.)
* Total: **26 rows** — VERIFIED 24 · DOWNGRADED 2 · UNSUPPORTED 0 ·
  FAILED 0.

## Supplementary findings (not matrix rows)

* **F-39-1 — CI quality-gate regression**: latest `ci.yml` run
  (2026-09-16) is failure; release.yml's three latest runs (2026-09-17)
  all failed at "Validate (go vet, build, test)" on the v4.2.0-rc1 ref.
  Local equivalents are green this stage (build/vet/unit/integration/
  fuzz/lint/vulncheck). F-32-3 (gh unauthenticated) stands as the
  blocking remediation.
* **G741 count discrepancy**: Stage 38 charter target "23 fuzz targets"
  vs. repo reality **21 unique fuzz targets** (across 7 fuzz_test.go
  files). All 21 executed: ~10s each, cumulative ≈10.9M execs, 0 crashes.
* **ldflags binding**: the prompt's `-X main.Version=…` silently no-ops
  (version lives in `internal/version.Version`); documented so future
  stages build with the correct binding.

