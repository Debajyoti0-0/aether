# Phase 0 — Baseline Re-Verification (World B)

**Date:** 2026-09-19
**Repository:** `C:\Users\Debajyoti0-0\OneDrive\Documents\aether`
**Probed from scratch this session; nothing taken from prior reports on trust.**

## Verified State

| Check | Result |
|---|---|
| HEAD | `0dae3ae0292c3a03aefd51b6735decd072b6b4d1` (master) |
| VERSION (file) | `3.4.0-stage3` |
| `git show HEAD:VERSION` | `3.4.0-stage3` |
| Tags | **0** (`git tag --list` empty) |
| Remotes | **none** (`git remote -v` empty) |
| `artifacts/` before this session | **absent** |
| Working tree | `M bin/aether.exe`, `?? ad_sampledata/`, plus 6 empty junk dirs + `%ADX%/` (all untracked) |
| Full history | 56 commits, root `37c0c09` (2026-09-10, F13 version single-sourcing) |

## Claim Consistency (prompt vs. reality)

| Claim | Verified |
|---|---|
| HEAD = `0dae3ae` | ✓ |
| VERSION = `3.4.0-stage3` | ✓ |
| zero tags | ✓ |
| no remote | ✓ |
| no `artifacts/` | ✓ (before this session) |
| revocation is connection-time file-based, no OCSP | ✓ — README Stage 3 "Known limitations (truthful)": "revocation is connection-time file-based (no OCSP)" |
| persistence = sealed local workspaces (AES-256-GCM + Argon2id) | ✓ — README Stage 2 + commit `567aaff` (salt.bin + HMAC tag, keyless only via explicit KEYLESS marker) |
| adjudicating commits present | ✓ `b69a152` (forensic review, 2 files / 449 lines), `0dae3ae` (Stage 7 G0/G26, 2 files / 260 lines) |

## Lineage Decision

**WORLD B — this repository is the record of truth.**

Discarded for this repository (foreign lineage, no objects exist here):
- Stage 40-R through 43 reports (HEAD `9a03334`, tag `v4.2.0-rc1` = `bb56cf3`, `artifacts/stage43/`, OCSP staging responder, LAN dashboard `172.20.10.3:18446`, gate matrix 35/38, readiness 92.1%).
- No release gate, waiver, or evidence from that narrative may be imported.

Gate 0: **PASS** — baseline matches reality and the World B decision is recorded.

## Note on evidence-directory consolidation

The Stage 44-R prompt specifies `artifacts/stage44r/`; the re-based GA prompt specifies `artifacts/rebase/` + `stage44/45/46/`. To avoid a split evidence package, all forensic (44-R) evidence is consolidated under `artifacts/rebase/`. No evidence was duplicated or omitted.
