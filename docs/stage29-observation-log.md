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

| 2026-09-18 | stage35 | INFO | F-34-3 closed | corrupt vault -> typed ErrVaultCorrupt, zero panic text, exit 1; regression tests | CLOSED |
| 2026-09-18 | stage35 | INFO | F-34-4 closed | vault bound to workspace identity; substitution (same-passphrase, empty-vault classes) rejected with identity-named error | CLOSED |
| 2026-09-18 | stage35 | event | test harness | torn-write bitflip test made layout-independent (diff-based live-data location) after identity binding shifted page layout | CLOSED |
| 2026-09-18 | stage35b | INFO | false-cert closure | Stages 20-26 retroactively dispositioned: 6 CLOSED-BY-SUCCESSOR, 7 FALSIFIED-AND-CORRECTED, 0 STILL-OPEN | CLOSED |

| 2026-09-18 | stage36 | INFO | boundary assurance | F-34-4 remediation re-attacked from alternate paths: call-graph (1 vault path, binding at store layer), empty-state substitution REJECTED, rename/clone matrices deterministic, 60-mutation corruption campaign (0 panics, 0 false VERIFIED) | CLOSED |
| 2026-09-18 | stage36 | event | security boundary | Plaintext-meta rebind attack exercised (bbolt rewrite -> VERIFIED): confirmed exactly at the documented Stage 35 boundary. G29 decision: ACCEPTED ARCHITECTURAL BOUNDARY; passphrase-sealed binding deferred with rationale. | CLOSED (documented) |
| 2026-09-18 | stage36 | INFO | DEBT-1 | 20 flags across 5 high-value commands swept; empty-name workspace delete verified fail-closed before destructive action | CLOSED |
| 2026-09-18 | stage36 | INFO | CI revival | gh still unauthenticated; BLOCKED-WITH-OWNER stands (remedy: gh auth login + push master, 36 commits reviewed) | BLOCKED-WITH-OWNER |

| 2026-09-17 | stage37a | INFO | time gate | Stage 37 invoked pre-gate (~3h elapsed of 30d); converted to 37a per charter rule; promotion re-deferred to on/after 2026-10-17T16:55:20Z | DEFERRED |
| 2026-09-17 | stage37a | INFO | observation audit | Independent row-level recount: 0 P0, 0 P1, 1 P2 (closed), 1 P3 (accepted) across 27 events | CLOSED |
| 2026-09-17 | stage37a | INFO | DEBT-1 | Batch 2: plan/rollback/relay/ztna/watch swept (16 flags), no anomalies; cumulative 10 commands fully swept | CLOSED |

| 2026-09-18 | stage39 | INFO | strict completion | Stages 10–26 dispositions re-verified from raw commands on the current binary: 26/26 rows evidenced (24 VERIFIED, 2 DOWNGRADED at source: Stage 10 CI execution and B7 Release Validate → BLOCKED-WITH-OWNER per red ci.yml/release.yml runs); 0 UNSUPPORTED, 0 FAILED; all 4 waivers re-verified with owner+expiry; 3 release tags unchanged; 21/21 fuzz targets 0 crashes; window continues | CLOSED |

Running totals (updated as events are appended): P0=0, P1=0, P2=1 (fix
scheduled, discovered at window open, not introduced during the window),
P3=1 (closed), events=28 (24 closed, 2 deferred, 2 blocked-with-owner).

| 2026-09-18 | stage40 | INFO | Stage 38 completion + 5 corrections | Campaigns 4-6 executed from raw commands on current builds (tag bb56cf3 + master 5cc7756): 0 new P0, 0 new P1; F-40-1 (P2, transport mismatch NEW both builds), F-40-2/F-40-3 (P3), F-40-4 (Info); Stage 34 four-fix lineage reproduced on-tag and REMEDIATED on-master; corrections C1-C5 landed (C1 matrix reference, C2 Validate BLOCKED-WITH-OWNER, C3 ldflags citation, C4 fuzz 23->21 across 15 docs, C5 F-30-3 + LIVE-1 standalone waivers); 21/21 fuzz targets 5.81M execs 0 crashes; build/vet/unit/integration/lint/vulncheck all green; 3 release tags unchanged; window continues | CLOSED |

| 2026-09-18 | stage42 | INFO | External forensic verification + production-readiness closure | External report (15 C/H/M/L + 4 INFO, tested against a stale 4.0.0-rc1/3.4.0-stage3 clone) re-verified against the current tag build bb56cf3: 19 CONFIRMED / 1 DISMISSED-STALE (L6) / 1 DEFERRED-WITH-OWNER (M6 design); ALL 17 code confirmations FIXED (C1 ALPN http/1.1-only, H1 revoke --operator, H2 connect CA fallback, H3 replay workspace threading, H4 doctor Windows, H5 RL whitelist, M1 RiskScore wired, M2 whitelist narrowed, M3 documented, M4 atomic key-file commit, M5 15s handshake bound, L1 non-zero exit, L2 passphrase wired, L4 query channel removed, L5 60s client, S42-1 revoke MkdirAll, CLI-0 9 regression tests); F-40-1 (the h2/h1 transport mismatch) CONFIRMED and FIXED; 0 new P0, 0 new P1 during this stage; 29/29 pkgs + race + integration + lint(0) + vulncheck(0) + fuzz(21 targets ~9.1M execs 0 crashes) green; 3 release tags unchanged; window continues | CLOSED |

Running totals (updated as events are appended): P0=0, P1=0, P2=2 (F-40-1
FIXED in Stage 42; the original P2 closed), P3=1 (closed), events=29
(26 closed, 1 deferred-with-owner, 2 blocked-with-owner).

## Note on window integrity

The window opened 2026-09-17 and closes 2026-10-17. No entry in this log
may be back-dated; the end-of-window integration and unit runs (WS4) must
execute on or after the end date against `bin/aether-rc2`. Until then the
GA decision is RETAIN by definition.
