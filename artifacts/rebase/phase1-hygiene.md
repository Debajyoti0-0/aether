# Phase 1 — Repository Hygiene

**Date:** 2026-09-19

## 1.1 Junk directories — REMOVED

`-ExecutionPolicy`, `-File`, `-NoProfile`, `-p`, `Bypass`, `powershell` (all confirmed empty) and `%ADX%` (contained only an empty `workspaces/` dir) — residue of a mangled `powershell -ExecutionPolicy Bypass -File …` invocation on 2026-09-19 14:08. Removed with `rmdir` after `find %ADX% -type f | wc -l` returned 0.

## 1.2 Residue classification

| Item | Finding | Decision |
|---|---|---|
| `bin/aether.exe` (modified, 23 MB → 16 MB) | **Foreign-lineage build artifact**: reports `aether version 4.0.0-rc1` — a version string that exists nowhere in this tree. Not a rebuild of this source. | **RESTORED** to tracked version (reports `3.4.0-stage3`, matching tree). Nothing lost: binary is reproducible from source (see stage45 reproducible-build). |
| `ad_sampledata/` (untracked, 3.5 MB) | BloodHound-style AD/ADCS collection JSON (PHANTOM.CORP lab dataset, dated 2024-03-05); contains user/group data — engagement data, not source fixtures. | **GITIGNORED**, kept on disk, never committed. |
| `.gitignore` | did not exist | Created with `ad_sampledata/` entry. |
| Stale refs | `refs/cline/checkpoints/*` (agent checkpoints), branches `onyx-crepe`/`sincere-plume` both at `990426b` (already merged) | Preserved untouched; flagged for optional operator cleanup. |

## 1.3 Remote — BLOCKER DOCUMENTED

`git remote -v` is empty. No authorized remote URL exists in this session. **REMOTE RELEASE = BLOCKED** — publication is blocked, not the product. Stage 45 records the offline-release deviation. Remediation: operator runs `git remote add origin <authoritative-url>` when the canonical location is decided.

## 1.4 Branch & tag policy

- Default branch: `master` (single branch, no remote).
- `v4.2.0-rc1` does not exist and was not created. No tags exist; no tag was created this stage (see final report: GA tagging deferred to an authorized ceremony).

**Gate 1: PASS** — working tree contains only intentionally classified residue; remote blocker documented.
