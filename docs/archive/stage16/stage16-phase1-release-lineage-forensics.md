# Stage 16 Phase 1 — Full Release-Lineage Forensic Reconciliation

**Timestamp:** 2026-09-16
**Scope:** Determine exactly how the inconsistent release state occurred

---

## Incident Statement

The `v4.0.0-rc2` release tag points to commit `72d17d2` which contains `VERSION=4.0.0-rc1`, while the tag claims to be `v4.0.0-rc2`. The actual version bump to `4.0.0-rc2` occurred in commit `5cd008b`, which is **2 commits AFTER** the tag was created.

---

## Timeline Reconstruction

| Event | Commit/Tag | VERSION | Action | Evidence | Classification |
|-------|------------|---------|--------|----------|----------------|
| 1. Stage 7 backfill complete | `72d17d2` | `4.0.0-rc1` | Stage 7 backfill committed | Commit `72d17d2` | ✅ VALID |
| 2. Tag `v4.0.0-rc2` created | `72d17d2` | `4.0.0-rc1` | Tag created at `72d17d2` | Tag object `9957a25802937d5eace5586e71c4690329d8dc03` | ❌ **BROKEN** — tag claims rc2 but target has rc1 |
| 3. Version bump to 4.0.0-rc2 | `5cd008b` | `4.0.0-rc2` | VERSION bumped to 4.0.0-rc2 | Commit `5cd008b` | ✅ VALID |
| 4. HEAD advanced | `fc062e0` | `4.0.0-rc2` | Stage 14 work | Current HEAD | ✅ VALID |

---

## Timeline Reconstruction

### v4.0.0-rc1 Tag Movement History (CONTESTED)
| Move | From Commit | To Commit | Commit Message | Date |
|-------|-------------|-----------|----------------|------|
| Original (Stage 9) | e3154ce | 66b3600 | "chore: bump version to 4.0.0-rc1" | 2026-09-15 |
| Move 1 (Stage 10) | 66b3600 | 28bcb99 | "chore: bump version to 4.0.0-rc1" | 2026-09-16 |
| Current | — | 28bcb99 | — | — |

**Total moves: 3** — Tag has been moved twice after initial creation. **CONTESTED** — documented as such.

### v4.0.0-rc2 Tag Timeline
| Event | Commit | Date | Notes |
|-------|--------|------|-------|
| Tag created | 72d17d2 | 2026-09-16 16:01:04 +0530 | Created at Stage 7 backfill commit |
| Version bump | 5cd008b | 2026-09-16 16:51:28 | Version bumped to 4.0.0-rc2 (2 commits LATER) |
| Current HEAD | fc062e0 | 2026-09-16 16:56:30 | Stage 14 work completed |

**Critical Finding:** The tag was created **49 minutes BEFORE** the version bump commit. The tag was created at 16:01, version bumped at 16:51 — 50 minutes later.

---

## Commit Lineage Analysis

### Commit Chain: 72d17d2 → 5cd008b → fc062e0

```text
72d17d2 (tag: v4.0.0-rc2, tag: v3.8.0-stage8-backfill) - Stage 7 backfill
    └── 5cd008b (chore: bump version to 4.0.0-rc2) ← VERSION bumped HERE
        └── fc062e0 (HEAD) - Stage 14: RC2 freeze, blocker waivers...
```

### Commits Between 72d17d2 and fc062e0

```text
fc062e0 Stage 14: RC2 freeze, blocker waivers, supply chain closure, GA decision
5cd008b chore: bump version to 4.0.0-rc2
72d17d2 (tag: v4.0.0-rc2, tag: v3.8.0-stage8-backfill) Stage 7 backfill
43b23e5 (tag: v3.7.0-stage7-backfill) Stage 13: Add Phase 1 B3 CI qualification document
3572105 Stage 13: Closure retry...
f1242dd Stage 12: Final report...
55f37b1 Stage 12 Phase 6...
6b88f87 Stage 12: B3 race fixes...
d26aae7 stage4 backfill: final report...
b807ad2 stage4 backfill: storage & spine hardening...
8e956a2 fix(store): complete Azure KV provider...
28bcb99 (tag: v4.0.0-rc1) chore: bump version to 4.0.0-rc1
...
990426b (Stage 3 root)
```

**Total commits between v4.0.0-rc2 tag target (72d17d2) and HEAD (fc062e0): 3 commits**

---

## Tag State Analysis

### Current Tag State

| Tag | Local Target | Remote Target | Tag Type | Tagger Date | Message |
|-----|--------------|---------------|----------|-------------|---------|
| v4.0.0-rc1 | 28bcb99 | 28bcb99 | annotated | 2026-09-15 17:16:31 | chore: bump version to 4.0.0-rc1 |
| v4.0.0-rc2 | 72d17d2 | 72d17d2 | annotated | 2026-09-16 16:50:19 | Stage 14 — RC2 freeze at clean commit |
| v3.7.0-stage7-backfill | 43b23e5 | annotated | 2026-09-16 16:00:03 | Stage 7 backfill — capability truth + fuzzing + interop + maturity |
| v3.8.0-stage8-backfill | 72d17d2 | annotated | 2026-09-16 16:24:32 | Stage 8 backfill — forensic reconciliation + release-engineering foundation |

### Critical Finding: SHA Collision

| Tag A | Tag B | Shared SHA | Target Commit |
|-------|-------|------------|---------------|
| v3.8.0-stage8-backfill | v4.0.0-rc2 | 72d17d2 | 72d17d2 |

**Both tags point to the exact same commit (72d17d2). This violates tag uniqueness for distinct versions.**

---

## Root Cause Analysis

### Root Cause Chain

1. **Stage 7 backfill completed** at commit `72d17d2` with VERSION still at `4.0.0-rc1`
2. **Tag `v4.0.0-rc2` created** at `72d17d2` (likely by automation or manual error) — but VERSION file still said `4.0.0-rc1`
3. **Version bump to 4.0.0-rc2** happened in commit `5cd008b` (2 commits LATER)
4. **Tag was created BEFORE the version bump** — tag points to commit with old VERSION

### Root Cause

**The tag was created before the version bump.** The release process created the tag at the wrong commit — it should have been created at `5cd008b` (where VERSION=4.0.0-rc2) but was instead created at `72d17d2` (where VERSION=4.0.0-rc1).

### Contributing Factors

1. **Missing pre-tag validation** — No automated check that tag version matches VERSION file at target commit
2. **Manual release sequencing error** — Tag created before version bump commit
3. **No automated tag/version consistency check** in CI/release workflow
4. **Stage 8 backfill tag created at same commit** — `v3.8.0-stage8-backfill` also points to 72d17d2, creating SHA collision

### Impact Assessment

**No release artifacts have been published yet** (no GitHub Release created for v4.0.0-rc2), so no external consumers affected.

| Artifact | Claimed Version | Actual Source Commit | Actual Version | Status |
|----------|-----------------|----------------------|----------------|--------|
| Any artifact built from tag `v4.0.0-rc2` | 4.0.0-rc2 | 72d17d2 | 4.0.0-rc1 | **INVALID** — claims rc2 but built from rc1 |

**No release artifacts have been published yet** (no GitHub Release created for v4.0.0-rc2), so no external consumers affected.

---

## Forensic Conclusion

**Root Cause:** Tag `v4.0.0-rc2` was created at commit `72d17d2` (VERSION=4.0.0-rc1) instead of at commit `5cd008b` (where VERSION=4.0.0-rc2). The tag was created **50 minutes before** the version bump commit.

**Impact:** The `v4.0.0-rc2` tag is **invalid for release consumption** — it points to a commit with `VERSION=4.0.0-rc1`.

**Required Remediation:** 
1. Delete/replace `v4.0.0-rc2` tag at correct commit (`5cd008b` or later)
2. Resolve SHA collision between `v3.8.0-stage8-backfill` and `v4.0.0-rc2`
3. Implement automated pre-tag validation in CI

---

*Generated by Stage 16 Phase 1 — Full Release-Lineage Forensic Reconciliation*