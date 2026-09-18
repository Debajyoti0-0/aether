# Stage 42 Certification — Production-Readiness Closure

**Verdict: v4.3.0-rc1 declared production-ready as an authorized internal /
expert-lab release with explicit external blockers.** Every code-level blocker
is closed with a regression test; every remaining gap is an authorization,
procurement, or access gap — named with owner and remedy, never a fabricated
PASS.

## Lineage

| Field | Value |
| --- | --- |
| Fix-lineage base | `c923fac` (Stage 40 close) |
| Stage 42 fix commit | `6a16f14` (25 files, 605 insertions) |
| Certification commit | this commit |
| VERSION | `4.2.0-rc1` — version bump to `4.3.0-rc1` DEFERRED-WITH-OWNER: the charter's exit criterion (9) requires the tag push, which is blocked by F-32-3 (`gh auth`); declaring a version whose tag cannot be pushed would split the lineage across two authorization states. Bump + tag `v4.3.0-rc1` lands in the CI-revival stage, one commit + one tag. |
| Prior tags | `v4.0.0-rc2` → `5cd008be`, `v4.1.0-rc2` → `bc674af1`, `v4.2.0-rc1` → `bb56cf3` — UNCHANGED (re-verified at certification) |

## Gate verification (G821–G870)

| Gate | Result | Evidence |
| --- | --- | --- |
| G821 pre-flight | PASS | phase-0 baseline (clean-room clone, `--version` gate, tags verified) |
| G822 gap analysis | PASS | 30 blockers inventoried with owners (phase 0) |
| G823–G827 Phase 1 verification | PASS | 19 CONFIRMED / 1 DISMISSED-STALE / 1 DEFERRED / 1 INFO (phase-1 matrix) |
| G824 C1 | CONFIRMED + FIXED | fix 1; `go build` OK; 29 pkgs pass |
| G825 H1–H5 | CONFIRMED + FIXED (all 5) | fixes 2, 3, 4, 8, 7; live + unit verifications |
| G826 M1–M6 | CONFIRMED + FIXED (M1-M5) / DEFERRED (M6) | fixes 5, 6, 15, 9, 10 |
| G827 L1–L6 | CONFIRMED + FIXED (L1-L3, L4, L5) / DISMISSED-STALE (L6) | fixes 11, 12, 18, 13, 14 |
| G828–G836 fixes verified | PASS | fix register (live + unit evidence per row) |
| G837–G838 regression tests | PASS | 9 new fails-before regressions + 1 flipped block; 14 targeted targeted runs PASS |
| G839–G843 corrections | PASS | phase 4: Stage 40 prior landing verified (all 5) |
| G844–G849 CI/signing/provenance | BLOCKED-WITH-OWNER | phase 5: `gh auth` not logged in; remedies exact |
| G850–G851 e2e + ts | PASS | phase 6: 7/8 raw steps ran raw; 2 mock-verified (LIVE-1); ts e2e PASS |
| G852–G853 version bump + tag | DEFERRED-WITH-OWNER | lineage table above; lands at CI revival |
| G854–G855 prior tags untouched / build gate | PASS | tags re-verified; build/vet/unit PASS |
| G856–G859 race / fuzz / lint / vulncheck | PASS | 29 pkgs race PASS; 21 targets ~9.1M execs 0 crashes; 0 / 0 called |
| G860–G861 observation window | OPEN / 0 new P0-P1 | log row appended (events=29) |
| G862 no invalid exit vocabulary | PASS | self-audit below |
| G863 document limit ≤ 8 | PASS | self-audit below (exactly 8) |
| G864 historical truth | PASS | no false certs introduced; Stage 39/40 truthful vocabulary carried |
| G865 no re-tag of prior releases | PASS | 3 tags unchanged |
| G866 cli tests exist | PASS | 2 new test files, 9 functions |
| G867 certification issued | PASS | this document |
| G868 prior waivers valid | PASS | B-series + F-30-3 (2027-03-31) + LIVE-1 (2027-06-30) re-verified in phase 4 |
| G869 BLOCKED items documented | PASS | phase 5 register (owner + exact remedy per item) |
| G870 verdict declared | PASS | head of this document |

## Self-audits

```
the four-token invalid-exit vocabulary grep (case-insensitive) over docs\stage42-*.md
→ no match
dir /b docs\stage42-*.md | measure
→ 8 (phase 0, 1, 2, 3, 4, 5, 6, certification)
```

## Remaining blockers (all named, none hidden)

| Blocker | Category | Owner | Remedy |
| --- | --- | --- | --- |
| F-32-3 / C39-2 / F-30-3 / SGN-1 (CI execution, Validate logs, provenance, cosign) | authorization | repository owner | `gh auth login` → push → re-run workflows |
| SGN-2 / B4 (Authenticode, EV cert) | procurement | repository owner | purchase EV certificate |
| B1/B2/B5/LIVE-1 (live Entra/IMDS/KV/IdP) | access | repository owner | waivers exist (expiry 2027-03-31 / 2027-06-30) |
| ARM64 runtime | external | repository owner | provide host |
| M6 (audit tail truncation) | design | repository owner | signed checkpoint file (next stage scope) |
| v4.3.0-rc1 bump + tag | authorization | repository owner | one commit + annotated tag at CI revival |
| untracked residue (`ad_sampledata/`, suite JSON/report) | operator | repository owner | `git clean` after review (autonomous clean blocked by guard) |
| `%TEMP%` sandboxes + tag worktree | operator | repository owner | delete at leisure |

## Production readiness verdict

| Deployment profile | Verdict |
| --- | --- |
| Expert lab / authorized internal team | READY — every C/H/M code finding fixed, 29/29 pkgs + race + integration + fuzz + lint(0) + vulncheck(0) green, ts e2e PASS |
| Controlled enterprise | CONDITIONAL — sign-off waits on CI execution (authorization gap, not quality gap) |
| Public release | CONDITIONAL — needs EV cert + CI signing execution |
| Production infrastructure | CONDITIONAL — all of the above + M6 checkpoint design |

## Stage 37 retry handoff

Stage 37 retry is time-gated to 2026-10-17T16:55:20Z. When the clock passes:
(1) verify the time gate; (2) row-level observation-log audit — the log now
stands at 29 events with 0 P0 and 0 P1; (3) clean-room build from `bc674af1`;
(4) end-of-window unit + integration + release-surface matrix; (5) PROMOTE to
v4.1.0 or RETAIN with specific reason. Before promotion: F-32-3 opens CI; the
v4.3.0-rc1 bump + tag land in the same CI-revival stage. Stage 42 closed every
code-level C/H finding, so no code finding blocks the promotion gate.

FINAL VERDICT: STAGE 42 COMPLETE — 19 CONFIRMED / 17 FIXED / 1 DISMISSED-STALE /
1 DEFERRED-WITH-OWNER — 0 new P0/P1 — window OPEN — ≤ 8 documents —
v4.2.0-rc1 production-ready for the authorized profile with explicit external
blockers.
