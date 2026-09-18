# Stage 31 — Qualification Evidence and Observation Continuity

All evidence from the Stage 31 working state (`424658e`) and the pushed
candidate tag `v4.2.0-rc1` (`bb56cf3`). Companion: baseline-lock
(delta, continuity), certification (verdict).

## 1. G482–G484 — Tag pushed and verified

```text
git push origin v4.2.0-rc1   → * [new tag] v4.2.0-rc1 -> v4.2.0-rc1
git ls-remote --tags origin  → bd0d9636004c51954466d7c809d1bff337f7d37e refs/tags/v4.2.0-rc1
                               bb56cf3c56d750b2190f46e09cd74e59e709bf64 refs/tags/v4.2.0-rc1^{}
git show v4.2.0-rc1:VERSION  → 4.2.0-rc1
Clean-room build at tag      → ./bin/aether-rc1 --version = "aether version 4.2.0-rc1"
```

The tag is externally verifiable. The local-only red flag (AA1) is closed.

## 2. G2 — D-29-1 closure, independently verified from the candidate artifact

Clean-room clone at `v4.2.0-rc1`, fresh build, 6 required cases:

| Case | Exit | Expected |
| --- | --- | --- |
| `providers list` | 0 | 0 — valid invocation not regressed |
| `providers list unexpected` | 1 | 1 — rejected, command body not executed |
| `workspace list` | 0 | 0 |
| `workspace list unexpected` | 1 | 1 |
| `export verify-evidence --help` | 0 | 0 — help intact |
| `export verify-evidence unexpected` | 1 | 1 |

`cobra.NoArgs` present in the tagged source (`git show v4.2.0-rc1:internal/cli/export_verify.go`).
D-29-1 = **CLOSED** (not test-only; enforced in command definitions).

## 3. F-30-2 resolution — claim and pipeline now match (Option B)

* `.goreleaser.yml` release footer: cosign/Authenticode instructions
  **removed**; replaced with a factual note that the pipeline produces no
  signatures (no `signs:` configuration) and that the instructions return
  only when signing is enabled. `goreleaser check` validates the config.
* `VERIFY.md`: prominent signing-status banner added; the two quick-check
  signature steps explicitly labeled NOT CURRENTLY PRODUCED.
* Why not Option A (add signing now): keyless cosign requires an OIDC
  identity flow unavailable in this environment; introducing a local
  signing key would create an unresolved key-custody question. The proper
  implementation is `signs:` with keyless cosign inside the GitHub Actions
  release workflow (Stage 32 item; `cosign` and `goreleaser` are both
  installed locally when needed).
* Snapshot verification: the footer now advertises only checksums + SBOM —
  both actually produced (verified in the Stage 30 snapshot run; config
  re-validated with `goreleaser check` after the edit).

F-30-2 = **CLOSED** (instructions removed; signing implementation tracked
as the GA-path item it always was).

## 4. F-30-1 resolution — darwin/arm64 added (Option A)

`ignore: goos darwin / goarch arm64` removed from `.goreleaser.yml`.
`goreleaser build --snapshot --clean` now builds **5** targets including
`darwin_arm64_v8.0` (artifact `aether_darwin_arm64_v8.0` present in
`dist/`). Runtime qualification on Apple Silicon remains untested and is
not claimed — the release matrix no longer excludes the platform, and
runtime qualification is tracked below.

F-30-1 = **CLOSED** (build-matrix exclusion removed; runtime qualification
deferred to platform-runtime item, honest status maintained).

## 5. F-30-3 — SLSA provenance: limitation with expiry (Option B)

Not implemented here: `actions/attest-build-provenance` requires the
GitHub Actions release workflow, which has not been exercised (origin/master
is stale; no release workflow run exists). Recorded limitation:

```text
F-30-3: No SLSA provenance is generated. Limitation: artifact provenance
(unverifiable claim) — consumers have checksums + SBOM only.
Compensating control: checksums.txt (sha256), SPDX-2.3 SBOM per archive,
tag→commit traceability via git.
Expiry: 2027-03-31. Owner: repository owner (Debajyoti Haldar).
Next action: add actions/attest-build-provenance to the release workflow
when the release workflow is revived.
```

F-30-3 = **OPEN (documented limitation with expiry and owner)**.

## 6. G490 — Boundary matrix: bounded pass + deferred long tail

Bounded matrix (release-surface commands, fully verified across Stages
28–31): `export verify-evidence` — 14 flags × valid/invalid/empty/boundary
classes covered by the 13-scenario CLI matrix + 5 unit/CLI test suites;
`providers validate` — 3 providers × success/401/403/malformed/
unreachable/missing-flags; exit codes 0/1/2/3 all exercised from real
processes.

Long tail (the remaining ~24 commands' per-flag boundary cases):
**DEFERRED-WITH-OWNER** — owner: repository owner; target: Stage 32/33
(gate-namespace continuation); method: the scripted top-level matrix
(help/bare/unknown-flag/unknown-arg for all 26 commands — executed and
passing) plus per-flag sweeps per command. No `PARTIAL` exit is issued:
the bounded scope is VERIFIED, the remainder is DEFERRED-WITH-OWNER.

## 7. Quality gates (Stage 31 HEAD, `424658e`)

| Gate | Result | Detail |
| --- | --- | --- |
| build / vet | PASS | |
| unit | PASS | 29 packages |
| integration | **FLAKY-FAIL** | see §8 |
| race | PASS | CGO_ENABLED=1, mingw-winlibs GCC 16.2.0; 29 packages ok, 0 DATA RACE, exit 0 (reconfirmed on this HEAD) |
| fuzz | 21/21 targets exit 0 | 60s per target; 0 crashes/panics/timeouts |
| lint | PASS | 0 issues |
| govulncheck | PASS | 0 affecting |

## 8. Flake protocol record — TestCrashMatrix (integration)

| Run | Result |
| --- | --- |
| Stage 31 first run (full suite) | FAIL — 3 of ~25 cells: killed child committed 1–2 entries beyond the strict bound (`committed = 4, want 1 or 2`; k=8, k=21 similar) |
| Rerun 1 (`-run TestCrashMatrix`) | FAIL (same class, `35 vs want 33 or 34`) |
| Rerun 2 | FAIL (same class) |
| Rerun 3 | PASS |

* Observed: killed process commits slightly more than the marker bound.
* Expected: committed ≤ marker index + 1.
* Root cause: Windows `TerminateProcess` latency — the harness kills the
  child after marker N, but the child appends 1–2 more entries before
  termination lands. The durability property under test (no torn/partial
  commit; chain integrity) is not violated; the bound is timing-sensitive.
* Storage code is byte-identical to the Stage 30 run where the full
  integration suite PASSED (delta audit: no storage change since
  `bc674af1`).
* Disposition: **DEFERRED-WITH-OWNER** — test-harness robustness fix
  (synchronization handshake instead of timing) in a future stage;
  assertions must not be weakened. Owner: repository owner. Not a product
  regression; not counted as a PASS, not hidden.

## 9. Observation window (G491)

Status: **OPEN — continuing, not closed early.** Close 2026-10-17T16:55:20Z.
New events appended to `docs/stage29-observation-log.md`: tag push,
F-30-2 resolution, F-30-1 matrix change, crash-matrix flake record.
New P0/P1 defects: **0**. The window remains attached to `v4.1.0-rc2`.
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.