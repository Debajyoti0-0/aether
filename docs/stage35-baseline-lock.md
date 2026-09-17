# Stage 35 — Baseline Lock and Finding Reproduction

Companion: `stage35-remediation-evidence.md` (design + substitution
matrix + panic remediation), `stage35-certification.md` (verdict).
Also this stage: `stage35b-false-cert-closure-matrix.md` (Stages 20–26
retrospective disposition, per the 35b charter).

## 1. Baseline

| Item | Value |
| --- | --- |
| HEAD / branch | master @ `445d299` at stage start; remediation commits `822fd96`…; tree clean |
| Candidate | `v4.2.0-rc1` object `bd0d9636` → commit `bb56cf3`, on origin, immutable |
| Observation baseline | `v4.1.0-rc2` object `e8416b58` → commit `bc674af1` — window OPEN, attached, closes 2026-10-17T16:55:20Z |
| Rollback baseline | `v4.0.0-rc2` → `5cd008be` — unchanged |
| Withdrawn | `v4.1.0` — absent |
| Track A | BLOCKED — time gate not elapsed (no promotion work) |

## 2. G1 — F-34-4 vulnerability reproduction (mandatory, on current tree)

```
workspace wa (2 audit entries) ─┐
                                ├ same passphrase
workspace wb (1 audit entry)  ──┘
cp wb/vault.db wa/vault.db
aether audit verify --workspace wa
→ exit 0, "Audit trail integrity: VERIFIED (0 entries, 0 tampered)"   ← VULNERABLE
```

Reproduced exactly as Stage 34 recorded. No code/env/fixture drift —
the finding is real on the current tree.

## 3. Root cause (confirmed by source)

`OpenVault` opened the bolt database with no workspace-identity check:
any vault encrypted for the same record-sealing passphrase opened in any
workspace directory. The audit hash chain detects record *modification*
but nothing bound the vault *as a whole* to the workspace it belongs to.

## 4. Stage 34 findings lifecycle position

| Finding | Stage 34 | Stage 35 start | Stage 35 end |
| --- | --- | --- | --- |
| F-34-3 (panic on corrupt vault) | OPEN (P2) | reproduced (exit 2 + raw panic text) | **CLOSED** (typed `ErrVaultCorrupt`, zero panic text) |
| F-34-4 (vault substitution) | OPEN (P2) | reproduced (VERIFIED 0 entries) | **CLOSED** (identity binding; substitution rejected) |
| F-34-1/F-34-2 | FIXED | regression-tested, intact | intact (regressions re-run) |
