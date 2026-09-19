# Phase 3 — Operator Approval Gate

**Status: PENDING — awaiting written sign-off.** Silence is not approval. No file has been moved or deleted.

## Proposed action list

### Quarantine (git mv — reversible, history preserved)

Destination: `artifacts/cleanup/<TS>/quarantined/`

| # | Path | Bytes |
|---|---|---|
| Q1 | `AETHER_STAGE1_SAFETY_WIRING_IMPLEMENTATION_REPORT.md` | 10,342 |
| Q2 | `AETHER_STAGE2_ARCHITECTURE_BASELINE.md` | 4,729 |
| Q3 | `AETHER_STAGE2_STORAGE_AND_SPINE_IMPLEMENTATION_REPORT.md` | 13,361 |
| Q4 | `AETHER_STAGE3_TEAMSERVER_ARCHITECTURE_BASELINE.md` | 4,565 |
| Q5 | `AETHER_STAGE3_TEAMSERVER_V2_IMPLEMENTATION_REPORT.md` | 12,966 |
| Q6 | `AETHER_v3.2.0_FORENSIC_BASELINE_RCA_REPORT.md` | 78,923 |
| Q7 | `Aetherv1.0.0—Complete-Engineering-Blueprint` | 44,193 |
| Q8 | `bin/aether.exe` | 15,712,742 |

Companion edits (mechanical, no content changes):
- Update `docs/stage1-evidence.md`, `docs/stage2-evidence.md`, `docs/stage3-teamserver-evidence.md` to point at the quarantine paths.
- Append `bin/` to `.gitignore` (prevents the dev binary re-entering VCS; `make build` output stays local).
- Commit as ONE atomic commit: `chore(cleanup): quarantine historical root reports and committed dev binary (F21)`.

Post-quarantine verification: `go build ./...`, `go vet ./...`, `go test ./...` must be green; `bin/aether.exe` regenerated locally via `make build` if the operator wants a runnable binary.

### Local disk delete (not repo content; untracked)

| # | Path | Size |
|---|---|---|
| D1 | `.kilo/` (AI-tool worktrees, machine-local) | 58 MB |

### Delete from VCS

**None.** Everything is quarantine-first per the prime directive.

## Preservation receipt

- `artifacts/stage45/dist/` (6 signed binaries): **untouched** — verified SHA-256 after Phase 4.
- `checksums.txt`, `provenance.json`, `release-manifest.json`, `release.pub`, all `.sig`: **untouched**.
- `auditor-handoff/`, `publish-runbook.md`, `board-dossier.md`, `RELEASING.md`: **untouched**.
- `artifacts/rebase/` … `stage49/`, all tests, CI workflows, source, docs: **untouched**.
- No Prime-Directive item appears in any action list above.

## Sign-off

```text
Decision (APPROVED / APPROVED WITH EDITS / REJECTED):  ______________
Edits (if any):  ______________
Operator (name, role):  ______________
Signature:  ______________
Date:  ______________
```
