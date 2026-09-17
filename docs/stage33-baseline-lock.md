# Stage 33 — Baseline Lock and Time Gate

Companion: `stage33-execution-and-evidence.md`, `stage33-certification.md`.

## 1. Time gate (verified 2026-09-17, ~23:59 UTC)

```text
Current UTC            : 2026-09-17 (system clock)
Observation window     : 2026-09-17T16:55:20Z → 2026-10-17T16:55:20Z
Elapsed                : ~0.3 of 30 days — WINDOW NOT ELAPSED
Track A (v4.1.0)       : G0 gate = BLOCKED — promotion NOT evaluated,
                         v4.1.0 NOT created, window NOT closed
Track B (v4.2.0)       : parallel work only
```

The Stage 33 long-form charter's own rule governs: "Before that timestamp:
do not perform promotion; do not create v4.1.0; do not declare the
observation window closed; do not mark Track A complete."

## 2. Freeze

| Item | Value |
| --- | --- |
| HEAD / branch | master @ `edcffb6` at freeze; `VERSION=4.2.0-rc1`; tree clean |
| Candidate | `v4.2.0-rc1` object `bd0d9636` → commit `bb56cf3`, on origin, unchanged |
| Observation baseline | `v4.1.0-rc2` object `e8416b58` → commit `bc674af1`, unchanged |
| Rollback baseline | `v4.0.0-rc2` → `5cd008be`, unchanged |
| Withdrawn | `v4.1.0` — absent, not recreated |
| CI auth | `gh` unauthenticated → remote workflow state unverifiable |

## 3. Track separation

* Track A outcome this stage: **BLOCKED (time gate)** — see certification.
* Track B outcome: CONTROLLED-PUBLICATION-READY (unchanged scope) with
  F-32-2 closed, F-32-3 dispositioned, F-32-4 closed.
* No observation evidence was transferred between lines; no historical
  claim was rewritten.
