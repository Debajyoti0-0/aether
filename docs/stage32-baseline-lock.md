# Stage 32 — Baseline Lock and Time Gate

Companion: `stage32-release-and-infrastructure-evidence.md`,
`stage32a-truth-table.md`, `stage32-certification.md`.

## 1. Time gate (verified 2026-09-17, ~23:40 UTC)

```text
Current UTC            : 2026-09-17 (system clock)
Observation window     : 2026-09-17T16:55:20Z → 2026-10-17T16:55:20Z
Elapsed                : ~0.3 of 30 days — WINDOW NOT ELAPSED
Consequence            : OBS-1 = BLOCKED — promotion gate NOT evaluated;
                         no v4.1.0 creation; no window closure; parallel
                         work only (per the Stage 32 critical timing rule)
```

## 2. Freeze

| Item | Value |
| --- | --- |
| HEAD / branch | master @ `45c56bf` (Stage 32 fixes); tree clean |
| VERSION | `4.2.0-rc1` |
| Candidate tag | `v4.2.0-rc1` object `bd0d9636` → commit `bb56cf3` — **on origin** (unchanged this stage) |
| Observation baseline | `v4.1.0-rc2` object `e8416b58` → commit `bc674af1` — unchanged, window attached |
| Rollback baseline | `v4.0.0-rc2` → `5cd008be` — unchanged |
| Withdrawn | `v4.1.0` — absent, not recreated |
| Remote | `origin` = `https://github.com/Debajyoti0-0/aether`; remote HEAD `fc062e0` (stale — origin/master has not received stages 19–32) |
| CI auth | `gh` 2.98.0 installed, **unauthenticated** — remote workflow state unverifiable from this host |

## 3. Lineage rules honored

* `v4.1.0-rc2` window NOT transferred to `v4.2.0-rc1` (Stage 31 Outcome B stands).
* No tag created, moved, or mutated this stage.
* Promotion from `v4.2.0-rc1`'s commit for the 4.1.0 line: never contemplated;
  any future `v4.1.0` comes from `bc674af1` per the Stage 29/31 decisions.
* `4.2.0-rc1` remains a separate candidate line (health audit: see
  release-and-infrastructure-evidence §5).
