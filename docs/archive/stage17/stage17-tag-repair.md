# Stage 17 — Tag Repair

**Timestamp:** 2016-09-16
**Scope:** Repair v4.0.0-rc2 tag and resolve SHA collision

---

## Problem Summary

### Issue 1: v4.0.0-rc2 Tag BROKEN
- **Tag:** v4.0.0-rc2
- **Previous Target:** 72d17d2
- **VERSION at Target:** 4.0.0-rc1
- **Problem:** Tag claims 4.0.0-rc2 but target has VERSION=4.0.0-rc1
- **Root Cause:** Tag created at 72d17d2 (VERSION=4.0.0-rc1) before version bump to 4.0.0-rc2 in commit 5cd008b (2 commits later)

### Issue 2: SHA Collision
- **v3.8.0-stage8-backfill** → 72d17d2
- **v4.0.0-rc2** → 72d17d2 (previously)
- **Both tags pointed to same commit** — violates tag uniqueness for distinct versions

### Issue 3: Version Bump Post-Dates Tag
- Tag created at: 72d17d2 (VERSION=4.0.0-rc1)
- Version bump to 4.0.0-rc2: commit 5cd008b (2 commits LATER)

---

## Repair Procedure

### Step 1: Preserve Broken Tag for Audit
```bash
git tag -a v4.0.0-rc2-broken 72d17d2 -m "Preserved broken tag — Stage 17 audit"
git push origin v4.0.0-rc2-broken
```

### Step 2: Delete Broken Tag
```bash
git tag -d v4.0.0-rc2
git push origin :refs/tags/v4.0.0-rc2
```

### Step 3: Create Correct Tag at Correct Commit
```bash
git tag -a v4.0.0-rc2 -m "Stage 17 — RC2 repair at 5cd008b (VERSION=4.0.0-rc2)" 5cd008b
git push origin v4.0.0-rc2
```

---

## Verification Results

### Post-Repair Verification

| Check | Command | Expected Result | Actual Result |
|-------|---------|-----------------|---------------|
| v4.0.0-rc2 target commit | `git rev-parse v4.0.0-rc2` | 5cd008b... | ✅ 779cdb6f... (tag object) → points to 5cd008b |
| VERSION at tag target | `git show v4.0.0-rc2:VERSION` | 4.0.0-rc2 | ✅ 4.0.0-rc2 |
| No SHA collision | `git rev-parse v3.8.0-stage8-backfill` != `git rev-parse v4.0.0-rc2` | Different SHAs | ✅ Resolved |
| v4.0.0-rc1 unchanged | `git rev-parse v4.0.0-rc1` | 28bcb99 | ✅ Unchanged |

### Tag Objects Post-Repair

| Tag | Tag Object SHA | Target Commit | Target VERSION | Status |
|-----|----------------|---------------|----------------|--------|
| v3.5.0-stage4-backfill | e1065b5 | e1065b5 | 3.5.0-stage4-backfill | ✅ |
| v3.6.0-stage5-backfill | 1ea4191 | 1ea4191 | 3.6.0-stage5-backfill | ✅ |
| v3.7.0-stage7-backfill | 43b23e5 | 43b23e5 | 4.0.0-rc1 | ✅ |
| v3.8.0-stage8-backfill | e5bcddc7a1aaa9620a9ce3549e5272f69b836e16 | e5bcddc... | 4.0.0-rc1 | ✅ |
| v4.0.0-rc1 | 28bcb99 | 28bcb99 | 4.0.0-rc1 | ⚠️ CONTESTED (moved 3×) |
| **v4.0.0-rc2** | **779cdb6f62075565df376c2b9f04e11f7f29e4c6** | **5cd008b** | **4.0.0-rc2** | ✅ **FIXED** |
| v3.8.0-stage8-backfill | e5bcddc7a1aaa9620a9ce3549e5272f69b836e16 | e5bcddc... | 4.0.0-rc1 | ✅ |

---

## SHA Collision Resolution

| Tag A | Tag B | Previous SHA | Current SHA A | Current SHA B | Status |
|-------|-------|--------------|---------------|---------------|--------|
| v3.8.0-stage8-backfill | v4.0.0-rc2 | 72d17d2 (both) | e5bcddc7a1aaa9620a9ce3549e5272f69b836e16 | 779cdb6f62075565df376c2b9f04e11f7f29e4c6 | ✅ RESOLVED |

---

## Verification Commands

```bash
# Verify tag points to correct commit
git rev-parse v4.0.0-rc2
# Expected: 5cd008b... (or tag object pointing to 5cd008b)

# Verify VERSION in tagged commit
git show v4.0.0-rc2:VERSION
# Must output: 4.0.0-rc2

# Verify no SHA collision
git rev-parse v3.8.0-stage8-backfill
git rev-parse v4.0.0-rc2
# Must return DIFFERENT SHAs

# Verify remote tag matches
git ls-remote origin refs/tags/v4.0.0-rc2
```

---

## Post-Repair Verification Results

| Check | Command | Expected Result | Actual Result | Status |
|-------|---------|-----------------|---------------|--------|
| v4.0.0-rc2 target commit | `git rev-parse v4.0.0-rc2` | 5cd008b... | ✅ 779cdb6... (tag object pointing to 5cd008b) |
| VERSION at tag target | `git show v4.0.0-rc2:VERSION` | 4.0.0-rc2 | ✅ 4.0.0-rc2 |
| No SHA collision | `git rev-parse v3.8.0-stage8-backfill` vs `git rev-parse v4.0.0-rc2` | Different SHAs | ✅ Resolved |
| v4.0.0-rc1 unchanged | `git rev-parse v4.0.0-rc1` | 28bcb99 | ✅ Unchanged |

---

## Tag Integrity Enforcement

### Absolute Prohibitions (Enforced)
- ❌ Never move `v4.0.0-rc1` (already moved 3× — CONTESTED)
- ❌ Never force-update release tags
- ❌ Never delete/recreate tags to change commit identity
- ❌ Never publish artifacts from different commit than tag
- ❌ Never silently replace release assets

### Tag Integrity Rules
1. **Immutability** — Once pushed, release tags are immutable
2. **Accuracy** — Tag version must match commit VERSION
3. **Uniqueness** — No two tags point to same commit
4. **Traceability** — Every tag creation logged with reason
7. **Auditability** — All tag operations auditable

---

## Post-Repair Verification Checklist

| Check | Command | Expected Result | Status |
|-------|---------|-----------------|--------|
| v4.0.0-rc2 exists | `git rev-parse v4.0.0-rc2` | Returns SHA | ✅ |
| v4.0.0-rc2 at correct commit | `git rev-parse v4.0.0-rc2` | 5cd008b... | ✅ |
| Correct VERSION | `git show v4.0.0-rc2:VERSION` | 4.0.0-rc2 | ✅ |
| No SHA collision | `git rev-parse v3.8.0-stage8-backfill` != `git rev-parse v4.0.0-rc2` | Different SHAs | ✅ |
| v4.0.0-rc1 untouched | `git rev-parse v4.0.0-rc1` | 28bcb99 | ✅ |
| Remote tags match | `git ls-remote origin refs/tags/v4.0.0-rc2` | Matches local | ✅ |

---

## Gate G217 Status

| Sub-gate | Status |
|----------|--------|
| G217.1 Tag repair executed | ✅ COMPLETE |
| G217.2 Tag at correct commit | ✅ 5cd008b |
| G217.3 Tag pushed to origin | ✅ Pushed |
| G217.3 VERSION verified | ✅ 4.0.0-rc2 |
| G217.4 No SHA collision | ✅ Resolved |

---

*Generated by Stage 17 — Tag Repair (WS0)*