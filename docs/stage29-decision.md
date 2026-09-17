# Stage 29 — GA-Path Decision: RETAIN 4.1.0-rc2

Companion documents: `stage29-baseline-lock.md` (freeze, race-gate closure,
CLI matrix), `stage29-observation-log.md` (window, defects).

## GA promotion criteria evaluation

| # | Criterion | State at decision time (2026-09-17) | Met? |
| --- | --- | --- | --- |
| 1 | Observation window elapsed (30 real days) | 0 of 30 days elapsed (opened 2026-09-17T16:55:20Z, closes 2026-10-17) | **NO** |
| 2 | Zero P0/P1 defects | 0 P0, 0 P1 so far | yes (so far) |
| 3 | Zero P2 defects requiring code changes | 1 P2 (D-29-1, cobra arg strictness) discovered at window open, fix scheduled for next candidate | pending |
| 4 | Race gate CLOSED | **CLOSED (green)** — full `-race` suite executed on the tagged commit with CGO + mingw-winlibs GCC 16.2.0; 29 packages ok, 0 DATA RACE, exit 0 | **YES** |
| 5 | End-of-window integration PASS | window not elapsed; run scheduled for on/after 2026-10-17 | pending |

## Decision

**RETAIN `4.1.0-rc2` as Production-Limited.**

Rationale: the observation window has not elapsed — 0 of 30 days at
decision time. Per the Stage 29 hard rule ("if the observation window has
not elapsed, the decision is RETAIN rc2") no promotion tag is created.
Criterion 1 is dispositive; criteria 3 and 5 also remain open. The single
most important change this stage delivered is that criterion 4 moved from
BLOCKED to CLOSED: the race gate is now genuinely executed and green, not
waived, so the only remaining path to promotion is elapsed time plus the
end-of-window test runs (and a disposition on D-29-1).

## Tag actions

| Tag | Action |
| --- | --- |
| `v4.1.0` | **NOT created.** Promotion conditions not met. |
| `v4.1.0-rc2` | unchanged at commit `bc674af1` (tag object `e8416b58`); immutable |
| `v4.0.0-rc2` | unchanged at `5cd008be1e3e20b039214a911626a6a4426c7838` |

Limitation register update: L-28-1 (race gate blocked) is **RESOLVED** —
race suite executed and green on the tagged commit; evidence in
stage29-baseline-lock §3. Remaining limitations: live Entra/IMDS NOT
VERIFIED; live Okta/GitLab/Kubernetes mock-only; D-29-1 P2 fix scheduled.

## Stage 30 handoff

Stage 30 = observation-window close and GA decision. Concretely: (1) on or
after 2026-10-17, verify the window defect table (zero P0/P1 required; D-29-1
must have a disposition — either fixed in a `v4.1.0-rc3`/`4.2.0` candidate
with `cobra.NoArgs` and a regression test, or explicitly accepted as
non-blocking); (2) rebuild `bin/aether-rc2` from `bc674af1` and run the
end-of-window integration + unit suites against it; (3) if all five
promotion criteria then hold, cut `v4.1.0` with an explicit version-bump
commit (`VERSION=4.1.0`) and push, otherwise retain rc2 with the specific
reason. Live Entra/IMDS and live provider qualification remain open items
that gate nothing in the observation window but must be dispositioned
before any public GA claim.

## Verdict

**v4.1.0-rc2 RETAINED — PRODUCTION-LIMITED. Race gate CLOSED. Window open
until 2026-10-17.**
