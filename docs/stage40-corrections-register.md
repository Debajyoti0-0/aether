# Stage 40 Corrections Register

Baseline: master @ `5cc7756`; VERSION = `4.2.0-rc1`; tag build @ `bb56cf3`.
All five Stage 39 correction items landed with before/after evidence.
Tags unchanged.

## C1 - Stage 33b disposition matrices updated at source (G764)

State verified: both DOWNGRADED rows already carry the corrected statuses
in `docs/stage33b-charter-closure-matrix.md` (Stage 39 updated them at
source):

- Row 10 CI pipeline execution: `BLOCKED-WITH-OWNER *(downgraded from
  CLOSED-BY-SUCCESSOR by Stage 39)*` (latest ci.yml run failure
  2026-09-16; F-32-3 stands).
- Row 10 Release Validate (B7): `BLOCKED-WITH-OWNER (CI) / local-verified`
  (release.yml #35253502855 failed at Validate on the v4.2.0-rc1 ref;
  local Validate steps green).

Landed this stage: the disposition-record reference was added
(`Corrected by Stage 39; disposition record:
docs/stage39-certification.md (DOWNGRADED ROWS).` below the Stage 39
recount paragraph). Diff: `git diff docs/stage33b-charter-closure-matrix.md`
(+1 line).

## C2 - release.yml Validate failure diagnosed (G765)

Raw: `gh auth status` -> `You are not logged into any GitHub hosts. To log
in, run: gh auth login`. Therefore `gh run view 35253502855 --log` cannot
return run logs; classification: BLOCKED-WITH-OWNER.

Remedy (exact operator action): `gh auth login && gh run view 35253502855
--log`. The delta between local green and CI red is not diagnosable
without CI logs.

Local reproduction of the exact Validate steps from release.yml
(`go vet ./...`, `go test -count=1 ./...`,
`go test -tags=integration -count=1 ./test/integration/...`): all green
this stage (raw results in docs/stage38-campaign-5-integration.md). Delta
is CI-environment-specific until logs open.

## C3 - ldflags documentation corrected (G766/G767)

Before (authoritative scan of all *.md/*.yml outside archive):
`grep -rn "main\.Version|-X main"` -> the single wrong citation in the
tree is `CHANGELOG.md:255`: `-ldflags "-X main.version=…"`. Verified clean
before landing: `.goreleaser.yml:30`, `dist/config.yaml`,
`.github/workflows/release.yml`, Makefile, scripts, and all live build
paths already bind
`-X github.com/Debajyoti0-0/aether/internal/version.Version`. VERIFY.md
does not exist; README.md does not document the ldflags binding.

After: `CHANGELOG.md:255` now reads `-X
github.com/Debajyoti0-0/aether/internal/version.Version=…` with the
correction note pointing here (diff +1 line).

Correct pattern (verified): `go build -ldflags "-X
github.com/Debajyoti0-0/aether/internal/version.Version=4.2.0-rc1 -X
github.com/Debajyoti0-0/aether/internal/version.Commit=5cc7756" -o
bin/aether-40 ./cmd/aether` -> `aether-40 --version` prints
`aether version 4.2.0-rc1` (G767 PASS; both stage binaries were built this
way). NOTE: the prompt's `-X main.Version=…` silently no-ops (version
lives in internal/version.Version) - same no-op class as the CHANGELOG
citation.

Historical evidence blocks: none of the historical docs cite the wrong
binding (scan verified); only CHANGELOG.md did. No stage certification
rewriting needed.

## C4 - Fuzz count corrected 23 -> 21 (G768/G769)

Authoritative count (raw): 21 unique fuzz targets across 7 fuzz_test.go
files (api 4, engine/exec 2, engine/token 3, protocol/msoapx 4,
protocol/saml 2, protocol/wstrust 4, workspace 2).

Before: 16 wrong references across 15 historical docs (stage28 through
stage36: certifications, baseline locks, qualification/observation,
production QA, execution/trust evidence). The two stage39 references are
the discrepancy notes themselves and were left as-is.

After: all 16 references corrected inline (23 targets -> 21 targets,
23/23 -> 21/21, preserving em-dashes and line endings byte-for-byte) and
each affected doc carries the footer: `Corrected 23 → 21 in Stage 40; the
count discrepancy is documented in docs/stage40-corrections-register.md.`
Residual scan: 0 wrong references remain.

Verified by running all 21 named targets (raw output):
21/21 targets x 10s, cumulative 5,812,358 execs, 0 crashes
(G769 PASS; per-target execs captured in the sweep output).

## C5 - F-30-3 and LIVE-1 promoted to standalone waivers (G770/G771)

- `docs/stage40-f-30-3-waiver.md` - Owner: Repository owner (Debajyoti
  Haldar); Risk: no SLSA provenance; Impact: supply-chain attestation
  absent; Compensating controls: checksums + SPDX SBOM + annotated-tag
  traceability + pinned in-repo pipeline; Expiry: 2027-03-31; Approval
  present. Closure procedure included.
- `docs/stage40-live-1-waiver.md` - Owner: Repository owner (Debajyoti
  Haldar); Risk: no live Entra/IMDS/IdP validation; Impact: mock-verified
  only; Compensating controls: mock success/negative matrices + offline
  provider error-class matrix (re-proven Stage 40 Campaign 4.5) + 21-target
  protocol fuzz (0 crashes, re-proven Stage 40); Expiry: 2027-06-30;
  Approval present. Closure procedure included.

Both carry all required fields; on-disk waiver-file rule now applies
uniformly to B4, B5, F-30-3, LIVE-1.

## Environment notes

- Suite residue `ad_sampledata/` was created during this stage's suite run
  (tests write fixtures into the repo CWD - register F-40-3). Removal
  requires operator approval; it is untracked and excluded from all counts.
- Sandbox `%TEMP%\aether-s40` (binaries, mocks, sandboxes) is outside the
  repo and is cleaned up at the operator's discretion.
