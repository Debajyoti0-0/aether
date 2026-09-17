# Stage 32 Certification — Parallel Work During the Observation Window

Companion: `stage32-baseline-lock.md` (time gate),
`stage32-release-and-infrastructure-evidence.md` (INT-1, CI),
`stage32a-truth-table.md` (claims vs binary).

## 1. Gate matrix

| Gate | Description | Status | Evidence |
| --- | --- | --- | --- |
| G0 | Time/repository/tag integrity | PASS | window NOT elapsed; tags unchanged; tree clean |
| G1 | Observation closure | **BLOCKED — WINDOW NOT ELAPSED** (by rule) | 0.3/30 days at execution |
| G2 | 4.1.0 promotion criteria | **NOT EVALUATED — BLOCKED** (Criterion A fails on time) | baseline-lock §1 |
| G3 | Crash-matrix reliability | **PASS — INT-1 CLOSED** | Category-1 harness defect; documented tolerance; integrity asserts strict; **5/5** consecutive green |
| G4 | Long-tail CLI coverage | PASS — bounded | release surface VERIFIED; remainder DEFERRED-WITH-OWNER (owner, Stage 33+) |
| G5 | CI recovery | **BLOCKED-WITH-OWNER** | gh unauthenticated; origin/master stale at `fc062e0`; exact remedy documented |
| G6 | Keyless signing | **PARTIALLY CLOSED → CI-implemented, CI-unexecuted** | sign step + upload in release.yml (validated YAML); real run requires CI revival (F-32-2/F-32-3 recorded) |
| G7 | Provenance | OPEN — F-30-3 (expiry 2027-03-31) | unchanged; requires CI |
| G8 | Live integrations | BLOCKED | LIVE-1 unchanged; no authorized access |
| G9 | Cross-platform runtime | compile PASS (5/5 incl. darwin/arm64); runtime windows/amd64 only | unchanged distinctions |
| G10 | 4.2.0-rc1 candidate health | PASS | separate line; healthy RC; explicit non-inheritance of the window |
| G11 | Final promotion/retention | **RETAIN/BLOCKED — window not elapsed** | no v4.1.0 created; no decision fabricated |
| G521–G540 (32a) | Truth table / claims / readiness | PASS | 32a doc; READY FOR CONTROLLED PUBLICATION (with standing debt) |

## 2. Blocker and debt register (delta from Stage 31)

| ID | Item | Status |
| --- | --- | --- |
| INT-1 | Crash-matrix harness flake | **CLOSED** (5/5 green; assertions preserved) |
| F-32-2 | windows-sign job downloads an artifact name nothing uploads | OPEN (HIGH) — needs a CI run to fix+validate |
| F-32-3 | Release workflow end-to-end execution unverifiable without gh auth | OPEN (HIGH) — BLOCKED-WITH-OWNER |
| F-32-4 | VERIFY.md header references stale 4.0.0-rc2 | OPEN (LOW) — regenerate per release |
| F-30-2 | Signature claim vs pipeline | CLOSED (Stage 31) + CI step added (this stage, unexecuted) |
| F-30-1/F-30-3/LIVE-1/DEBT-1/D-29-1/D-29-2 | unchanged from Stage 31 (DEBT-1 now bounded with named scope) |

## 3. Observation decision

* Observed tag: `v4.1.0-rc2` (`bc674af1`). Window: OPEN, closes
  2026-10-17T16:55:20Z. Not closed early; not transferred.
* New P0/P1 this stage: **0**. Log appended (append-only).
* What must be re-run after the window closes: end-of-window unit +
  integration suites against a fresh `bin/aether-rc2` built from
  `bc674af1`, crash-matrix included (now reliably green), plus the
  promotion-criteria evaluation (Stage 33).

## 4. Final verdict

**`RETAIN/BLOCKED — v4.1.0-rc2 (promotion gate blocked: window not
elapsed)`; Stage 32 parallel work COMPLETE**: CI signing implemented
(unexecuted), INT-1 closed, DEBT-1 bounded, truth table produced —
`v4.2.0-rc1` READY FOR CONTROLLED PUBLICATION with standing debt. No
promotion, no new tags, no window manipulation.

## 5. Stage 33 handoff

Stage 33 begins on/after 2026-10-17T16:55:20Z and is decision-only for
the 4.1.0 line: verify `v4.1.0-rc2` → `bc674af1` unchanged; confirm the
observation log holds zero P0/P1 and no promotion-blocking P2; clean-room
build `bin/aether-rc2` from `bc674af1`; run the full unit + integration
suites (crash-matrix included — expected green post-INT-1) plus the
release-surface matrix; then PROMOTE (version-bump commit from
`bc674af1`, annotated `v4.1.0`, push, verify ls-remote, `v4.0.0-rc2`
rollback baseline intact) or RETAIN with the specific reason. In
parallel, the 4.2.0 GA path requires reviving CI (`gh auth login`;
`git push origin master`) to execute the now-real race/provenance/signing
workflows, closing F-32-2 (windows-sign artifact-name mismatch) against a
real run, regenerating VERIFY.md for the published version, and
dispositioning LIVE-1 when authorized access exists.
