# Process History — pruned

**Pruned:** 2026-09-20 · commit that removed these paths: see `git log --grep "prune process history"`.

The following process-history artifacts were intentionally removed from the working tree during repository purification and remain **fully recoverable from git history** (checkout any commit prior to the pruning commit):

- `artifacts/rebase/` — World-B baseline, object-database forensics, hygiene records
- `artifacts/stage44/` — operational qualification evidence (regression lock, ops scenarios, soak, CLI matrix)
- `artifacts/stage47/` — cross-platform runtime evidence, GA gate matrix
- `artifacts/stage48/` — blocker-closure reports, hard-blocker docs
- `artifacts/stage49/` — operator intake, external blocker status
- `artifacts/cleanup/` — purification phases 0–3 records
- `artifacts/final-release-status.md`

**Still live (the release-relevant minimum):**

- `artifacts/stage45/` — signed release payload, SBOM, provenance, checksums, manifest, signatures
- `artifacts/stage46/independent-audit-report.md` — the document the external auditor signs (B-1)
- `artifacts/stage48/auditor-handoff/`, `publish-runbook.md`, `board-dossier.md` — handoff package (B-1/B-3/B-4)
- `RELEASING.md` — operator runbook and resume prompt

Historical references in `auditor-handoff.md` and `board-dossier.md` that name pruned paths resolve via git history at any pre-pruning commit.
