# Stage 29 — Production Observation Log (4.1.0-rc2)

Production-Limited observation window for `v4.1.0-rc2`
(tag object `e8416b58`, commit `bc674af1`).

```text
observation-start : 2026-09-17T16:55:20Z
observation-end   : 2026-10-17T16:55:20Z (30 real days)
```

## Intake channel

Defects and events are recorded as rows in the table below (append-only),
and, when filed externally, as GitHub issues on
`https://github.com/Debajyoti0-0/aether` referencing this document.
Reporters: expert-lab operators and the authorized internal team.

## Exit criteria (evaluated at window close)

1. Zero P0/P1 defects reported.
2. Zero P2 defects requiring code changes **during the window**.
3. At least one full integration test run at end-of-window against the
   tagged binary (`bin/aether-rc2` built from `bc674af1`).
4. Race gate CLOSED (achieved 2026-09-17, see stage29-baseline-lock §3)
   or BLOCKED with owner + target.

Severities: P0 data loss / security bypass / release-blocking ·
P1 core functionality broken · P2 workaround exists, not release-blocking ·
P3 cosmetic / documentation.

## Defect and event table

| Date (UTC) | Reporter | Severity | Component | Description | Status |
| --- | --- | --- | --- | --- | --- |
| 2026-09-17 | stage29-cli-matrix | P2 | cli (export verify-evidence, providers list, workspace list) | D-29-1: leaf commands without `cobra.Args` restrictions accept and silently ignore unexpected positional arguments while still executing. Behavior/output otherwise correct. | OPEN — fix scheduled for next candidate (cobra.NoArgs); does not require an out-of-cycle rc3 |
| 2026-09-17 | stage29-cli-matrix | P3 | cli (parent commands) | D-29-2: parent commands show help with exit 0 for an unknown subcommand/argument. Accepted cobra pattern; documented in stage29-baseline-lock §4. | CLOSED — documented, no change required |
| 2026-09-17 | stage29-race-closure | event | quality gates | Race suite executed for the first time (scoop mingw-winlibs GCC 16.2.0, CGO_ENABLED=1) on the tagged commit: 29 packages ok, 0 DATA RACE, exit 0. Stage 28 limitation L-28-1 resolved. | CLOSED — race gate CLOSED (green) |

| 2026-09-17 | stage31 | event | release lineage | v4.2.0-rc1 pushed to origin (object bd0d9636, commit bb56cf3) — the candidate is now externally verifiable. | CLOSED |
| 2026-09-17 | stage31 | event | release infra | F-30-2 resolved: false cosign/Authenticode instructions removed from goreleaser footer and VERIFY.md; pipeline claim now matches output. | CLOSED |
| 2026-09-17 | stage31 | event | release infra | F-30-1 resolved: darwin/arm64 added to the build matrix (compile-verified; runtime not claimed). | CLOSED |
| 2026-09-17 | stage31-integration | event | test harness | TestCrashMatrix flakiness characterized (FAIL/FAIL/PASS on reruns, no code change since passing runs): Windows kill-latency lets the child append 1-2 entries past the marker bound; durability property not violated. Test-harness fix DEFERRED-WITH-OWNER (owner: repository owner). | DEFERRED |
| 2026-09-17 | stage31 | event | governance | 4.1.0 line disposition recorded: Option A — deferred promotion of v4.1.0-rc2 on/after 2026-10-17 if the window closes clean (see stage31-certification.md). | CLOSED |

| 2026-09-17 | stage32 | event | CI trust chain | Keyless cosign signing step added to release.yml (the job previously produced no signatures; verify loops passed vacuously). Unexecuted pending CI revival. | CLOSED (implemented) |
| 2026-09-17 | stage32 | event | CI revival | gh unauthenticated; origin/master stale at fc062e0. All remote workflow verification BLOCKED-WITH-OWNER (remedy documented). | BLOCKED-WITH-OWNER |
| 2026-09-17 | stage32 | event | test harness | INT-1 CLOSED: crash-matrix kill-latency tolerance documented; durability assertions strict; 5/5 consecutive green. | CLOSED |
| 2026-09-17 | stage32a | event | release surface | Truth table produced: every lineage CLI claim maps to verified behavior or truthful NEVER-PRODUCED/WITHDRAWN label; no regressions. | CLOSED |

| 2026-09-17 | stage33 | event | CI trust chain | F-32-2 CLOSED: windows-sign artifact names aligned with actual goreleaser output (upload step added; sign targets aether.exe; signed repack). IMPLEMENTED, not EXECUTED — needs CI run. | CLOSED (implemented) |
| 2026-09-17 | stage33 | event | release docs | F-32-4 CLOSED: VERIFY.md regenerated for 4.2.0-rc1 (0 stale refs, 0 missing commands). | CLOSED |
| 2026-09-17 | stage33 | event | quality gates | Integration suite fully green post-INT-1 (21.9s) — first fully-green run since the Stage 31 flake. | CLOSED |

| 2026-09-18 | stage33b | INFO | charter closure | Stages 10-18 retired: 10 CLOSED-BY-SUCCESSOR, 3 PERMANENTLY-WAIVED (B4/B5/B1+B2 — waiver files created; Stage 13 had claimed them filed but no file existed), 0 STILL-OPEN. | CLOSED |
| 2026-09-18 | stage33b | event | CI trust chain | SLSA provenance step added (attest-build-provenance); signing step given a fail-closed zero-signature guard — an empty input set can no longer pass vacuously. | CLOSED (implemented) |
| 2026-09-18 | stage34 | event | test harness | Crash-matrix fixed-tolerance failed under background CPU load (+7 overshoot). Replaced with load-independent structural durability assertion; 5/5 green under deliberate load; full integration suite green. | CLOSED |
| 2026-09-18 | stage34 | event | CLI qualification | Long-tail flag sweep: 587 flags enumerated across the full command tree; 56 parse-class rejections 100% deterministic; 2 sweep anomalies investigated and dismissed as detector false positives. | CLOSED |

Running totals (updated as events are appended): P0=0, P1=0, P2=1 (fix
scheduled, discovered at window open, not introduced during the window),
P3=1 (closed), events=16 (14 closed, 1 deferred, 1 blocked-with-owner).

## Note on window integrity

The window opened 2026-09-17 and closes 2026-10-17. No entry in this log
may be back-dated; the end-of-window integration and unit runs (WS4) must
execute on or after the end date against `bin/aether-rc2`. Until then the
GA decision is RETAIN by definition.
