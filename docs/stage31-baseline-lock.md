# Stage 31 — Baseline Lock, Candidate Delta Audit, and Observation-Continuity Decision

Companion: `stage31-qualification-and-observation.md` (gate evidence),
`stage31-certification.md` (dispositions, verdict, handoff).

## 1. Freeze (Stage 31 start)

| Item | Value |
| --- | --- |
| HEAD / branch | master @ `bb56cf3` at freeze; `424658e` after the Stage 31 fixes; tree clean |
| Candidate | `v4.2.0-rc1` — annotated tag object `bd0d9636004c51954466d7c809d1bff337f7d37e` → commit `bb56cf3c56d750b2190f46e09cd74e59e709bf64` |
| Candidate remote state | **PUSHED** at Stage 31 start: `origin` `refs/tags/v4.2.0-rc1` = `bd0d9636…{}` → `bb56cf3…` (verified via `git ls-remote`) |
| Observation baseline | `v4.1.0-rc2` (object `e8416b58`, commit `bc674af1`) — immutable; window 2026-09-17T16:55:20Z → 2026-10-17T16:55:20Z, OPEN, continuing |
| Retained fallback | `v4.0.0-rc2` → `5cd008be` — untouched |
| Withdrawn | `v4.1.0` — absent, not recreated |

## 2. Candidate delta audit (`git diff v4.1.0-rc2..v4.2.0-rc1`)

14 files changed, 813 insertions, 12 deletions:

| Area | Changed? | Content | Observation impact |
| --- | --- | --- | --- |
| CLI surface | YES | `cobra.NoArgs` on `export verify-evidence`, `providers list`, `workspace list` (D-29-1 fix, commit `36be959`) + regression test | Low — strictens input rejection only; valid invocations unchanged (verified from the candidate build) |
| Exit contracts | NO | unchanged (0/1/2/3 contract re-verified) | None |
| Security controls | NO | no changes to redaction, revocation, evidence gate | None |
| Protocol behavior | NO | no changes under `internal/protocol` | None |
| Provider behavior | NO | no changes under provider code | None |
| Storage/config | NO | no changes | None |
| Build/release | NO (in tag) | tag contains no build-config change; F-30-2/F-30-1 fixes land **after** the tag (`424658e`) | None for the tag; fixes qualify the line going forward |
| Documentation / VERSION / binary | YES | stage 28–30 records, `VERSION=4.2.0-rc1`, rebuilt tracked binary | None |

The tag's own source delta is exactly the D-29-1 fix. There are no
undocumented changes: every commit between the two tags is a reviewed,
deliberate stage artifact.

## 3. Observation-continuity decision matrix

| Question | Finding | Evidence | Consequence |
| --- | --- | --- | --- |
| Is `v4.1.0-rc2` immutable? | YES | tag object `e8416b58` unchanged local+remote | Window continues |
| Is `v4.2.0-rc1` materially different? | NO — fixes-only successor | delta audit §2 (single CLI-strictness fix + docs) | No full re-observation required |
| Does D-29-1 alter release behavior? | Only by rejecting previously-silently-ignored input | G2 verification (6 cases from candidate build) | Low; no production surface change |
| Does the candidate preserve protocol behavior? | YES | no protocol code in delta; fuzz 21/21 | Evidence carries |
| Does the candidate preserve security behavior? | YES | no security code in delta; failure-injection matrix re-run | Evidence carries |
| Can Stage 29 observation evidence transfer? | To the shared (unchanged) surface, yes | §2 | See outcome below |
| Is a new observation window required? | No full window; the candidate's own promotion must wait for the shared window close + its own end-of-window run | this matrix | — |
| Can `v4.1.0` be promoted later? | YES — deferred promotion (see certification §2) | Stage 29 decision | Decision on/after 2026-10-17 |

**Outcome: B — observation preserved for `v4.1.0-rc2` only.** The window
remains attached to the immutable baseline. `v4.2.0-rc1` is a corrected
successor whose delta is contract-strictening; it does not inherit a
window of its own, but its full executed-gate evidence (Stages 30–31)
covers the delta. After 2026-10-17 the promotion decision applies to
`v4.1.0-rc2`; any `4.2.0` promotion additionally requires its own
end-of-window observation run.
> Corrected 23 → 21 in Stage 40; the count discrepancy is documented in docs/stage40-corrections-register.md.