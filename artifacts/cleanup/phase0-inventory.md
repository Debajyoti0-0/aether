# Phase 0 — Full Directory Inventory (read-only)

**Date:** 2026-09-20 · HEAD `9ebebf0` · clean tree · 0 tags · 0 remotes

## Totals

| Metric | Value |
|---|---|
| Tracked files | 346 |
| All files (excl. `.git`) | 1,137 |
| Repo working tree | 166 MB |
| `.git` | 99 MB |

## Size by top-level directory

| Dir | Size | Purpose hypothesis |
|---|---|---|
| `artifacts/` | 88 MB | Release evidence + handoffs; **88 MB is `stage45/dist/` (the 6 signed release binaries) — protected** |
| `bin/` | 16 MB | Tracked dev-built binary (`aether.exe`) — see Phase 1 |
| `.kilo/` | 58 MB | **Untracked** AI-tool state incl. 3 nested worktree repos with duplicated source + old binaries |
| `internal/` | 1.3 MB | Product source |
| `ad_sampledata/` | 3.5 MB | Untracked+ignored operator engagement data (BloodHound-style lab set) |
| `docs/` | 256 KB | Product docs + threat model + historical stage evidence |
| `pkg/` | 60 KB | Public packages (plugins) |
| `test/` | 24 KB | Integration tests |
| `deploy/` | 17 KB | Packaging (debian) |
| `scripts/` | 13 KB | Build/tooling scripts |
| `.github/` | 8 KB | CI (`ci.yml` + `ga-smoke.yml`) |
| `cmd/` | 1 KB | Entrypoint |
| `.vscode/` | 1 KB | Untracked editor state |

## Top files > 1 MB

| File | Size | Classification |
|---|---|---|
| `.kilo/worktrees/*/bin/aether.exe` ×3 | ~53 MB total | Untracked tool-state duplicates |
| `bin/aether.exe` | 15.7 MB | **Quarantine candidate** (Phase 1) |
| `artifacts/stage45/dist/*` ×6 | ~89 MB total | **KEEP — signed release payload** |
| `ad_sampledata/*` | 3.5 MB | KEEP (operator data, ignored) |

## Untracked / ignored

- `ad_sampledata/` — ignored (`.gitignore`), operator engagement data.
- `.kilo/` — untracked tool state (contains its own nested git worktrees: `leaf-gate`, `onyx-crepe`, `sincere-plume` — full source copies + built binaries).
- `.vscode/settings.json` — untracked editor state.
- **Zero untracked non-ignored files.**

## Gate 0: PASS — inventory complete, nothing modified.
