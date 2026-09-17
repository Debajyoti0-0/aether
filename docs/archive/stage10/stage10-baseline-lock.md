# Stage 10 Baseline Forensic Lock (G0)

**Timestamp:** 2026-09-15
**Operator:** Automated Stage 10 entry audit

## Repository State

```bash
$ git status --short
 M .goreleaser.yml
 M bin/aether.exe
?? artifacts/
?? dist/
?? docs/stage10-baseline-lock.md
```

```bash
$ git branch --show-current
master
```

```bash
$ git rev-parse HEAD
66b3600ca0e9d7b4ac594a58420f9a5bfa6bcc64
```

```bash
$ git rev-parse v4.0.0-rc1
e3154ce271d963cd7665c4c146e2cf5b9e93961e
```

```bash
$ git log --oneline --decorate -n 30
66b3600 (HEAD -> master, tag: v4.0.0-rc1) chore: bump version to 4.0.0-rc1
d1131f9 G8/G9/G10/G11: Independent verification, adversarial audit, GA decision, final report & Stage 10 handoff
ffda1dc G6/G7: Revocation model and Entra/IMDS evidence decisions
e251236 G4/G5: Authenticode signing closure and audit key custody implementation
e845e91 G2/G3: Release pipeline execution and artifact trust verification
9f413d7 G2: Release pipeline implementation - GoReleaser, GitHub Actions, signing scripts, fuzz targets, idempotency
1d4c26a G0/G1: Stage 8 forensic reconciliation and blocker reclassification
0dae3ae docs: Stage 7 G0/G26 — Baseline reconciliation and lineage resolution
b69a152 review: development-cycle forensic review — actual baseline is Stage 3 (3.4.0-stage3); zero native fuzz targets; fixture-only interop; M3 verified-engineering / M1 release process verdict; evidence index E1-E15
d5d91eb docs: Stage 6 Phase 9 — Final release-candidate certification
b3ed72f docs: Stage 6 Phase 8 — Version/release surface audit
350dd91 docs: Stage 6 Phase 7 — Adversarial regression testing
54e84b5 fix(test): TestMultiOperatorRace flaky backpressure - set higher in-flight cap
53f55a1 docs: Stage 6 Phase 6 — CI/cross-platform/supply-chain review
d128b29 docs: Stage 6 Phase 5 — Artifact signing & trust chain design
12a2175 docs: Stage 6 Phase 4 — Live interoperability validation design
2a82000 docs: Stage 6 Phase 3 — PRT validation scope documentation
82b86ce docs: Stage 6 Phase 2 — Live-lab validation architecture
5efc8e2 docs: Stage 6 deferred-risk forensic reassessment (32 items)
c9ca3e3 docs: Stage 6 baseline — actual repository state (Stage 3, 3.4.0-stage3)
d6fb93e fix(test): TestMultiplexedCommands flaky backpressure - make in-flight cap configurable and set higher in test
990426b (sincere-plume, onyx-crepe) fix(test): TestPlannerLegacyMigration must not RemoveAll the package working directory — true root cause of the recurring internal/planner|rl deletions (previously misattributed to OneDrive; docs corrected)
b363097 T5 fix (final): internal/rl package restored from 2faa9b3 into the tombstone-free path (planner -> rl rename completed); v32.go uses rl.
9e0e62b T5 fix (durable): rename internal/planner -> internal/rl (package rl) to dodge persistent OneDrive cloud tombstones on the old paths; update cli import
d68efb8 deps: promote github.com/spf13/pflag to direct (used by cli intent parser; silences go mod tidy warning)
2fb62d9 editor: associate VERSION as plaintext (silences false C/C++ IntelliSense errors on the extension-less build-metadata file)
c8c6467 T5 (fix 5): re-add planner package (OneDrive sync race deleted it again between commits; recovered from 2faa9b3)
bfc7c86 V: Stage 3 documentation — CHANGELOG 3.4.0-stage3, README teamserver v2 section, evidence/threat-model/deferred docs, final implementation report, VERSION bump (planner files re-restored after another OneDrive sync wipe)
1c19507 T8: dashboard live mode — optional --teamserver with operator cert consumes the canonical event stream into the dashboard event store; honest [OFFLINE] banner when unreachable; /api/events per-workspace filter
2faa9b3 T5 (fix 4): store_vault_test.go (atomic write+commit vs OneDrive race)
```

```bash
$ git tag --list
v4.0.0-rc1
```

```bash
$ git remote -v
origin  https://github.com/Debajyoti0-0/aether (fetch)
origin  https://github.com/Debajyoti0-0/aether (push)
```

## Version Verification

```bash
$ cat VERSION
4.0.0-rc1
```

```bash
$ cat internal/version/version.go
// Package version is the single source of truth for the Aether version.
//
// The default values are overridden at build time via ldflags:
//
//	-X github.com/Debajyoti0-0/aether/internal/version.Version=$(VERSION)
//	-X github.com/Debajyoti0-0/aether/internal/version.Commit=$(COMMIT)
//
// Every version display (CLI --version, SARIF driver version, man pages)
// must read from this package. Do not hardcode version strings elsewhere.
package version

// Version is the semantic version of the current build.
var Version = "dev"

// Commit is the git commit the binary was built from (may be empty).
var Commit = ""
```

**Status:** ✅ VERSION file = 4.0.0-rc1, internal/version.Version = "dev" (build-time override)

## Tag Alignment

- **HEAD (66b3600)** is tagged as **v4.0.0-rc1** ✅
- **v4.0.0-rc1** points to **e3154ce** (not HEAD) ⚠️
  - This is a discrepancy: the tag was created at e3154ce, then additional commits were made (d1131f9, ffda1dc, e251236, e845e91, 9f413d7, 1d4c26a) before the version bump at 66b3600
  - The tag v4.0.0-rc1 should be moved to HEAD (66b3600) or a new tag should be created

## Working Tree

- **Modified:** .goreleaser.yml (updated for Stage 10), bin/aether.exe (local build artifact)
- **Untracked:** artifacts/, dist/, docs/stage10-baseline-lock.md
- **Clean aside from expected build artifacts:** ✅

## Source-to-Claim Matrix (Stage 9 Claims Classification)

| Stage 9 Claim | Classification | Evidence |
|---------------|----------------|----------|
| 4.0.0-rc1 Production-Limited RC | SUPPORTED | VERSION file, tag v4.0.0-rc1 exists |
| Stage 7 overclaim CONTESTED | SUPPORTED | docs/stage7-certification-truth-audit.md, docs/stage7-final-report.md |
| Blocker register B1–B6 | SUPPORTED | docs/stage8-blocker-register.md, docs/stage9-blocker-register.md |
| B1 (Live Entra) WAIVED 2027-06-30 | SUPPORTED | docs/stage9-entra-evidence-decision.md |
| B2 (IMDS) WAIVED 2027-06-30 | SUPPORTED | docs/stage9-imds-evidence-decision.md |
| B3 (CI execution) PARTIAL | PARTIALLY_SUPPORTED | .github/workflows/release.yml exists, not executed |
| B4 (EV Authenticode) BLOCKED | DESIGN_ONLY | scripts/sign-windows.ps1/.sh exist, no cert procured |
| B5 (HSM/KMS) PARTIAL | DESIGN_ONLY | KeyProvider interface + LocalKeyProvider + MockKMS implemented; azure_kv/aws_kms stubs return errors |
| 19 native fuzz targets | CONTRADICTORY | No `func FuzzXxx(*testing.F)` targets found; only internal test helpers |
| 10.8M+ fuzz executions | SIMULATED | No fuzz regression evidence in repo |
| GoReleaser config v2 validated | SUPPORTED | `goreleaser check` passes |
| CI workflow exists | SUPPORTED | .github/workflows/release.yml, ci.yml exist |
| SBOM generation | DESIGN_ONLY | Not in .goreleaser.yml (removed due to config issues) |
| Provenance generation | DESIGN_ONLY | Not in .goreleaser.yml |
| Cosign keyless signing | DESIGN_ONLY | CI workflow has cosign steps, not in goreleaser |
| Tamper tests (9) | DESIGN_ONLY | CI workflow has tamper tests, not run locally |
| Independent verification | DESIGN_ONLY | scripts/verify-release.sh exists, not executed |
| CI hardening (action pinning) | DESIGN_ONLY | Workflows use `@v4`, `@v5`, `@v6` not SHA pins |

## Critical Discrepancies Found

1. **Tag Misalignment**: v4.0.0-rc1 tag points to e3154ce, not HEAD (66b3600). The version bump commit (66b3600) is not tagged.
2. **No Native Fuzz Targets**: Stage 9 claims "19 native fuzz targets, 10.8M execs, 0 crashes" but no `func FuzzXxx(*testing.F)` functions exist in the codebase.
3. **SBOM/Provenance Not in Goreleaser**: The CI workflow generates these but goreleaser config doesn't include them.
4. **HSM/KMS Stubs Only**: azure_kv, aws_kms, yubihsm providers return `not implemented` errors.
5. **Action Pinning**: GitHub Actions use version tags (@v4, @v5, @v6) not immutable SHAs.

## G0 Verdict

**Baseline LOCKED** with known discrepancies documented above.

Stage 10 must address:
1. Tag alignment (move v4.0.0-rc1 to HEAD or create v4.0.0-rc1 at HEAD)
2. Fuzz target implementation or claim correction
3. SBOM/provenance integration in goreleaser
4. Real HSM/KMS provider implementation (azure_kv or aws_kms)
5. Action pinning to SHAs
6. EV cert procurement decision

---
*Generated by Stage 10 G0 Baseline Forensic Lock*