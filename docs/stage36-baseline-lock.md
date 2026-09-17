# Stage 36 — Baseline Lock

Companion: `stage36-boundary-assurance.md` (adversarial results),
`stage36-certification.md` (gates, verdict, handoff).

## 1. Baseline

| Item | Value |
| --- | --- |
| HEAD / branch | master @ `76a2027` at stage start; tree clean |
| Candidate | `v4.2.0-rc1` object `bd0d9636` → commit `bb56cf3`, on origin, immutable |
| Observation baseline | `v4.1.0-rc2` object `e8416b58` → commit `bc674af1` — window OPEN (closes 2026-10-17T16:55:20Z), not closed, not transferred |
| Rollback baseline | `v4.0.0-rc2` → `5cd008be` — unchanged |
| Withdrawn | `v4.1.0` — absent |
| Unpublished | 36 commits on master ahead of `origin/master` (`fc062e0`) — all classified stage work (Stage 34 §2 methodology) |

## 2. CI revival (G622–G627)

```text
gh auth status → "You are not logged into any GitHub hosts."
```

**BLOCKED-WITH-OWNER** (unchanged). Remedy, exact and operator-only:

```bash
gh auth login            # interactive OIDC/device flow
git push origin master   # 36 reviewed commits, classified safe (Stage 34 §2)
```

After push: `ci.yml` (race, lint) and the tag-triggered `release.yml`
(signing → provenance → verify jobs) become executable; `gh run list`
provides the run URLs that convert IMPLEMENTED → EXECUTED. Signing and
provenance remain **UNEXECUTED** this stage; no run is fabricated.

## 3. Track A

BLOCKED — time gate not elapsed. The observation window remains attached
to `v4.1.0-rc2` (`bc674af1`); Stage 35b's disposition of Stages 20–26
stands unchanged (verified: false claims still false, successor evidence
still successor evidence, waivers still waivers).
