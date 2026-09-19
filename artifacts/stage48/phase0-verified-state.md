# Stage 48 — Phase 0: Verified State

**Date:** 2026-09-19 · probed from scratch

| Check | Result |
|---|---|
| HEAD | `d0f2d2e` (stage47 evidence commit) |
| Branch / tree | master, clean |
| Tags / remotes | 0 / 0 |
| VERSION | `3.4.0-stage3` |
| Stage 44–47 evidence | all present (`rebase/`, `stage44/`, `stage45/`, `stage46/`, `stage47/`) |
| `artifacts/stage45/dist/` | 6 binaries + SBOM + provenance + signatures + checksums + manifest |
| Docker | available (enabled Stage 47 Linux runtime evidence) |
| `gh` CLI | **not authenticated** — no GitHub account in this environment |
| CI workflows | `ci.yml` exists locally; **unrunnable without a remote** |

**Gate 0: PASS** — baseline matches the Stage 47 report exactly.

No evidence file references foreign-lineage objects (`9a03334`, `bb56cf3`, `v4.2.0-rc1`) except as explicitly rejected history (`rebase/object-database-report.md`).

## Final product regression (re-confirmed this stage)

- `go build ./...` + `go vet ./...` — clean
- `go test -count=1 ./...` — **28/28 packages green, 0 failures**
- `go test -race ./internal/api/ ./internal/cli/ ./internal/workspace/` — green
- `govulncheck ./...` — no affecting vulnerabilities
