# Stage 40 Baseline Lock

| Field | Value |
| --- | --- |
| Locked at | Start of Stage 40 execution (2026-09-18, execution window 13:47-15:30 +0530) |
| Master HEAD | `5cc7756` (verified `git rev-parse` at lock and re-verified at close; parent of the Stage 40 commit `db1f567`) |
| VERSION | `4.2.0-rc1` |
| Tracked tree | Clean at lock (verified `git status --porcelain`; untracked tooling dir `.kilo/` only) |
| Tag `v4.0.0-rc2` | `5cd008be1e3e20b039214a911626a6a4426c7838` (unchanged; verified at lock and at close) |
| Tag `v4.1.0-rc2` | `bc674af1ab9ac49c8ec154fda103e9e558457301` (unchanged; verified at lock and at close) |
| Tag `v4.1.0` | absent (verified `git tag -l v4.1.0` empty) |
| Tag `v4.2.0-rc1` | annotated `bb56cf3c56d750b2190f46e09cd74e59e709bf64`, object `bd0d9636004c51954466d7c809d1bff337f7d37e` (unchanged; verified at lock and at close) |
| Tag mutations | none (no tag created, moved, or deleted) |
| Working tree at close | Clean of Stage 40 content (26 files committed in `db1f567`; untracked: `.kilo/` tooling dir, `ad_sampledata/` suite residue awaiting operator removal approval - both excluded by convention) |

Verification commands (raw, executed at lock and at close):
`git rev-parse HEAD`, `git status --porcelain`, `git rev-list -n1
v4.0.0-rc2`, `git rev-list -n1 v4.1.0-rc2`, `git rev-list -n1 v4.2.0-rc1`,
`git tag -l v4.1.0`, `git tag -l v4.2.0-rc1 --format="%(objecttype)
%(objectname)"`. Outputs recorded verbatim in
docs/stage38-certification.md (Baseline lock section).
