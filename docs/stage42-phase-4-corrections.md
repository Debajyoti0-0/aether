# Stage 42 — Phase 4 Corrections (Stage 40 prior-landing verification)

The Stage 42 charter's Phase 4 re-lands the five Stage 40 corrections. All five
LANDED in my Stage 40 execution (commit `c923fac`, 2026-09-18). This phase closes
as VERIFICATION of the prior landing — no duplicate edits, no duplicate documents.

| # | Correction | Where landed (Stage 40) | Re-verified now (Stage 42, now) | Status |
| --- | --- | --- | --- | --- |
| 1 | Stage 33b matrices: two DOWNGRADED rows corrected at source + "Corrected by Stage 39/40" note | `docs/stage33b-charter-closure-matrix.md` (reference line added; both rows updated to truthful `BLOCKED-WITH-OWNER` vocabulary) | `docs/stage39-certification.md` + `docs/stage40-corrections-register.md` present in tree; closure matrix reference line present | LANDED + VERIFIED |
| 2 | `release.yml` Validate failure diagnosis | `docs/stage40-corrections-register.md`: run #35253502855 = `BLOCKED-WITH-OWNER` (`gh auth status` not logged in; local Validate green) | `gh auth status` re-run in Phase 5 of this stage — still not logged in; the BLOCKED-WITH-OWNER disposition carries forward with the same remedy | CARRIES FORWARD |
| 3 | ldflags documentation | `CHANGELOG.md:255` fixed to `-X github.com/Debajyoti0-0/aether/internal/version.Version` (only wrong citation in tree) | re-verified: no `main.Version` / `-X main.` citations in `docs/`, `.github/`, README, VERIFY, `.goreleaser.yml` in this tree | LANDED + VERIFIED |
| 4 | Fuzz count 23 → 21 | 16 references across 15 docs reworded in Stage 40; authoritative count 21 verified: 21/21 targets, 0 crashes | re-verified now on the tag build: `AUTHORITATIVE_FUZZ_TARGETS=21`, `FUZZ_RUN=21`, `FIZZ_CRASHES=0` (~9.1M execs across the sweep) | LANDED + VERIFIED |
| 5 | Standalone waiver files | `docs/stage40-f-30-3-waiver.md` (expiry 2027-03-31), `docs/stage40-live-1-waiver.md` (expiry 2027-06-30) — owner, risk, impact, compensating control, expiry, approval | both present in tree with all six fields | LANDED + VERIFIED |

## Duplicate-document rule

The charter names Phase 4 evidence as `docs/stage42-phase-4-corrections.md`
(this file). No second set of Stage 40 documents was duplicated: the register,
waivers, and matrix corrections exist at their Stage 40 paths and are verified —
duplicating them would create two sources of truth for the same citations.

## Verification commands

```
dir /b docs\stage40-corrections-register.md docs\stage40-f-30-3-waiver.md docs\stage40-live-1-waiver.md  → all present
rg "main.Version|-X main\." docs/ .github/ README.md VERIFY.md (PS Select-String)  → no match
Select-String 'func Fuzz' --include *_test.go | count  → 21
```
