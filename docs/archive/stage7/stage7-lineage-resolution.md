# Stage 7 Lineage Resolution (Gate G26)

**Date:** 2026-09-12
**Decision:** World A — Stage 6 authoritative, Stage 5 unmerged external work

---

## 1. Conflict Summary

| Aspect | Stage 5 Claim | Stage 6 Claim | This Repository |
|--------|---------------|---------------|-----------------|
| Baseline commit | `d250d52` | `990426b` / `b3ed72f` | `990426b` (Stage 3), `b3ed72f` (Stage 6) |
| Version | `3.6.0-stage5` | `3.4.0-stage3` baseline, `3.5.0-rc1` target | `3.4.0-stage3` |
| Artifacts | sbom, manifest, checksums, conformance, interop, evidence, crash-matrix | "N/A — no Stage 5 artifacts exist" | No Stage 5 artifacts |
| Deferred items | 15 | 32 (from Stage 2/3) | 32 |

---

## 2. Evidence

```bash
# d250d52 not in this repository
git cat-file -t d250d52 2>&1
# fatal: Not a valid object name d250d52

git branch -a --contains d250d52 2>&1
# error: malformed object name d250d52

# Common ancestor check
git merge-base d250d52 HEAD 2>&1
# fatal: Not a valid object name d250d52
```

**Conclusion:** `d250d52` does not exist in this repository's object database. The Stage 5 baseline commit is not reachable from any ref in this clone.

---

## 3. Lineage Determination

**World A** — Stage 6 is authoritative for `C:\Users\Debajyoti0-0\OneDrive\Documents\aether`.

- Stage 3 complete at `990426b` (3.4.0-stage3)
- Stage 6 executed on top, produced `b3ed72f` (docs) + `d6fb93e`/`54e84b5` (code fixes)
- Forensic review at `b69a152` confirmed actual state
- Stage 5 report describes a **different fork/branch/worktree** never merged here

**World B rejected** — Would require re-executing Stage 6 on a non-existent commit `d250d52`.

---

## 4. Stage 5 Disposition

Stage 5 work is **archived as unmerged-external**:

- Commit reference: `d250d52` (external, not in this repo)
- Artifacts: Not present in this repository
- Deferred items: Superseded by Stage 6's 32-item register (Stage 2/3 consolidated)
- Gate accounting: Stage 6's "DESIGNED" gate results supersede Stage 5's PASS claims

---

## 5. Version-Truth Rule (G26.6)

**No version regression permitted across stages.**

- Stage 3: `3.4.0-stage3`
- Stage 6 target: `3.5.0-rc1` (Staged RC)
- Stage 7 target: `3.8.0-stage7` (Signed RC)
- Stage 8 target: `4.0.0` (Release)

The claimed Stage 5 version `3.6.0-stage5` is **not part of this lineage** and does not constrain versioning.

---

## 6. Deferred-Item ID Mapping (G26.4)

Stage 5's 15 items map to Stage 6's 32 items as follows:

| Stage 5 ID | Stage 6 ID(s) | Note |
|------------|---------------|------|
| 1 (SIGKILL crash matrix) | S2-13, S3-1 | Consolidated |
| 2 (Kerberos conformance) | S3-4 | Direct |
| 3 (XML-DSig C14N) | S3-5 | Direct |
| 4 (PRT broker grant) | S3-6 | Direct |
| 5 (IMDSv2 per-cloud) | S3-7 | Direct |
| 6 (Idempotency keys) | S3-8 | Direct |
| 7 (OCSP/CRL) | S3-7 (revocation) | Merged |
| 8 (Per-IP rate limiting) | S3-8 (partial) | Different scope |
| 9 (Per-event signatures) | S3-9 | Direct |
| 10 (Rollback lifecycle) | S3-10 | Direct |
| 11 (RequestID/OperatorID on Evidence) | S3-11 | Direct |
| 12 (Full provenance model) | S2-8, S3-12 | Consolidated |
| 13 (Planner determinism) | S2-5, S3-13 | Duplicate |
| 14 (Replay env capture) | S2-7, S3-14 | Duplicate |
| 15 (OneDrive sync race) | S3-15 (RESOLVED) | Fixed in Stage 3 |

**Total unique items after consolidation: 32** (Stage 6 register).

---

## 7. Gate Renumbering (G26.5)

| Stage | Gate Range | Status |
|-------|------------|--------|
| Stage 5 | G0–G13 | Frozen, superseded |
| Stage 6 | G14–G25 | Frozen, "DESIGNED" ≠ PASS |
| Stage 7 | **G26–G40** | Active (this execution) |

---

## 8. Repo Relocation Plan (G26.7)

**Target:** `C:\dev\aether` (off OneDrive)

**Rationale:** Stage 6 §7 documents repeated OneDrive cloud-tombstone incidents destroying working files (`internal/planner` deleted multiple times). This is an active supply-chain/storage risk.

**Plan:**
1. Create `C:\dev\aether`
2. `git clone` current repo to target (or `robocopy` + `git init`)
3. Verify `git fsck --full`, remotes, submodules, LFS, hooks
4. Update CI/CD secrets if path-dependent
5. Delete OneDrive copy after verification

---

## 9. G26 Acceptance

| Criterion | Status | Evidence |
|-----------|--------|----------|
| G26.1 Authoritative lineage frozen | **PASS** | World A declared |
| G26.2 Stage 5 archived as unmerged-external | **PASS** | This document |
| G26.3 World B not required | **PASS** | d250d52 absent |
| G26.4 Deferred ID mapping | **PASS** | Section 6 table |
| G26.5 Gate renumbering | **PASS** | Section 7 table |
| G26.6 Version-truth rule declared | **PASS** | Section 5 |
| G26.7 Repo relocation plan | **PASS** | Section 8 |

**Gate G26: PASS** — Stage 7 execution authorized.