# Stage 34 — Baseline Lock

Companion: `stage34-execution-and-trust-evidence.md`, `stage34-certification.md`.

## 1. Repository and remote state

| Item | Value |
| --- | --- |
| HEAD / branch | master @ `822fd96`+; tree clean; `VERSION=4.2.0-rc1` |
| Candidate | `v4.2.0-rc1` object `bd0d9636` → commit `bb56cf3`, on origin, immutable |
| Observation baseline | `v4.1.0-rc2` object `e8416b58` → commit `bc674af1` — window attached, OPEN |
| Rollback baseline | `v4.0.0-rc2` → `5cd008be` — unchanged |
| Withdrawn | `v4.1.0` — absent |
| Remote | origin HEAD `fc062e0`; **27 commits unpublished** (origin/master..HEAD) |
| CI auth | `gh` unauthenticated — remote runs unverifiable |

## 2. Unpublished-commit classification (G1 prerequisite — reviewed, not pushed)

All 27 commits in `origin/master..HEAD` were individually reviewed. Every
one is deliberate stage work of this engagement: FIX-4/FIX-5 implementation
and defect fixes, stage evidence documents, release-infrastructure fixes
(signing step, windows-sign alignment, SLSA step, vacuous guard), waiver
filings, INT-1 closure, and the version/binary release commits. **No
unrelated, experimental, or credential-bearing commits exist.** The push
is therefore safe for the operator to execute verbatim:

```bash
gh auth login          # interactive; operator-only
git push origin master # publishes exactly the 27 reviewed commits
```

## 3. Time gate

Observation window: 2026-09-17T16:55:20Z → 2026-10-17T16:55:20Z.
Elapsed at Stage 34 execution: <1 day. **Track A: BLOCKED — time gate
not elapsed** (final; no promotion work performed). All Stage 34 work is
Track B parallel evidence or trust-chain implementation.
