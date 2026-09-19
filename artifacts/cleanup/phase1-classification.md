# Phase 1 — Classification

Every path classed into exactly one bucket. Group-level classification with per-file detail for all candidates; `KEEP-PRODUCTION` groups are listed by directory (346 tracked files; every directory's full contents inherit the bucket unless a specific file is called out).

## KEEP — PRODUCTION (never touch)

| Group | Evidence / rationale |
|---|---|
| `cmd/`, `internal/`, `pkg/`, `go.mod`, `go.sum` | The product; builds green (28/28 pkgs) |
| `*_test.go` (276 files), `test/`, all `testdata/` | Regression/security protection — Prime Directive |
| `docs/` | README links `stage3-threat-model.md` + `PLATFORM.md`; rest is stage evidence → see KEEP-DEV |
| `.github/workflows/ci.yml`, `.github/workflows/ga-smoke.yml` | CI + B-3 closure mechanism |
| `Makefile`, `scripts/`, `deploy/`, `.golangci.yml` | Build + release infrastructure |
| `README.md`, `CHANGELOG.md`, `VERSION`, `LICENSE`, `SECURITY.md`, `RELEASING.md` | Contract, history, legal, disclosure, operator runbook |
| `artifacts/**` (all) | **Prime Directive: release evidence, signed payload, handoffs, governance chain** |
| `.gitignore` | Hygiene |

## KEEP — DEVELOPMENT

| Item | Rationale |
|---|---|
| `.vscode/settings.json` (untracked, 1 KB) | Local editor state; harmless; not committed |
| `ad_sampledata/` (untracked+ignored, 3.5 MB) | Operator engagement data; deliberately outside VCS |

## QUARANTINE CANDIDATES (tracked — move to `artifacts/cleanup/<TS>/quarantined/`, reversible)

| # | Path | Size | Last modified | Signals (≥2 independent) | Rationale |
|---|---|---|---|---|---|
| Q1 | `AETHER_STAGE1_SAFETY_WIRING_IMPLEMENTATION_REPORT.md` | 10 KB | 2026-09-10 | (1) superseded by `docs/stage1-evidence.md`; (2) zero refs from README/RELEASING/CI/build | Root-level campaign report → historical |
| Q2 | `AETHER_STAGE2_ARCHITECTURE_BASELINE.md` | 4.7 KB | 2026-09-10 | same two signals | same |
| Q3 | `AETHER_STAGE2_STORAGE_AND_SPINE_IMPLEMENTATION_REPORT.md` | 13 KB | 2026-09-10 | same two signals | same |
| Q4 | `AETHER_STAGE3_TEAMSERVER_ARCHITECTURE_BASELINE.md` | 4.6 KB | 2026-09-10 | same two signals | same |
| Q5 | `AETHER_STAGE3_TEAMSERVER_V2_IMPLEMENTATION_REPORT.md` | 13 KB | 2026-09-11 | same two signals | same |
| Q6 | `AETHER_v3.2.0_FORENSIC_BASELINE_RCA_REPORT.md` | 79 KB | 2026-09-10 | same two signals | Root-level forensic RCA of a *foreign-lineage* version (v3.2.0); superseded by `docs/development-cycle-forensic-review.md` |
| Q7 | `Aetherv1.0.0—Complete-Engineering-Blueprint` | 44 KB | 2026-09-02 | (1) pre-campaign design doc for v1.0.0; (2) zero refs from any tracked doc/CI/build; (3) extension-less filename breaks tooling | Historical blueprint |
| Q8 | `bin/aether.exe` | 15.7 MB | committed | (1) **repo's own RCA flagged its removal as deferred fix F21** ("committed with no provenance, signature, or checksum"); (2) `make build` regenerates it — no checkout dependency; (3) the release payload is `artifacts/stage45/dist/` (signed, checksummed) — `bin/` duplicates it unsigned | Dev-built binary committed to VCS |

**Reference repair required if Q1–Q6 move:** `docs/stage1-evidence.md`, `docs/stage2-evidence.md`, `docs/stage3-teamserver-evidence.md` reference the root reports → update to quarantine paths (mechanical edit, no content change).

## DELETE CANDIDATES (untracked machine-local — local disk only, not repo content)

| # | Path | Size | Signals | Rationale |
|---|---|---|---|---|
| D1 | `.kilo/` | 58 MB | (1) untracked tool state; (2) contains 3 full worktree clones (`leaf-gate`, `onyx-crepe`, `sincere-plume`) duplicating source + stale binaries; (3) branches already merged into master | AI-tool worktrees; machine-local; frees 58 MB. Reversible: they are worktrees of this same history |

## INVESTIGATE

None — every candidate resolved with two or more independent signals.

## Gate 1: PASS — all 346 tracked files + all untracked paths classified; nothing modified.
