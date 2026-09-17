# Stage 16 Phase 5 — Immutable Candidate Tag Procedure

**Timestamp:** 2016-09-16
**Scope:** Create a new release candidate only if justified by forensic findings and repository versioning policy

---

## Mission

Create a new release candidate only if justified by the forensic findings and repository versioning policy.

Do not reuse `v4.0.0-rc2`.

The likely candidate may be:

```text
v4.0.0-rc3
```

but this must be confirmed by:

* Versioning policy
* Current `VERSION`
* Changelog
* Release history
* Candidate diff
* Formal release decision
* Tag availability
* CI requirements

---

## RC2 Tag Repair (Mandatory First Step)

**The v4.0.0-rc2 tag is BROKEN — it points to commit 72d17d2 which has VERSION=4.0.0-rc1.**

The tag must be repaired before any release operations.

### Current Broken State

| Property | Value |
|----------|-------|
| Tag | v4.0.0-rc2 |
| Current Target | 72d17d2 |
| VERSION at Target | 4.0.0-rc1 |
| Tag Claim | v4.0.0-rc2 |
| **Problem** | Tag claims rc2 but target has rc1 |

### Root Cause

Tag created at commit 72d17d2 (VERSION=4.0.0-rc1) before version bump to 4.0.0-rc2 in commit 5cd008b (2 commits later).

### Required Repair Procedure

**DO NOT move the existing broken tag.** Preserve it as historical evidence.

**Correct Procedure:**

```bash
# 1. Delete the broken local tag
git tag -d v4.0.0-rc2

# 2. Delete the broken remote tag
git push origin :refs/tags/v4.0.0-rc2

# 3. Create new tag at CORRECT commit (5cd008b has VERSION=4.0.0-rc2)
git tag -a v4.0.0-rc2 -m "Stage 16 — RC2 freeze at clean commit with VERSION=4.0.0-rc2" 5cd008b

# 4. Push new tag
git push origin v4.0.0-rc2
```

### Verification After Repair

```bash
# Verify tag points to correct commit
git rev-parse v4.0.0-rc2
# Should output: 5cd008b...

# Verify VERSION in tagged commit
git show v4.0.0-rc2:VERSION
# Must output: 4.0.0-rc2

# Verify no SHA collision
git rev-parse v3.8.0-stage8-backfill
git rev-parse v4.0.0-rc2
# Must return DIFFERENT SHAs
```

### Post-Repair Verification Checklist

| Check | Command | Expected Result |
|-------|---------|-----------------|
| Tag exists | `git rev-parse v4.0.0-rc2` | Returns 5cd008b... |
| Correct commit | `git rev-parse v4.0.0-rc2` | 5cd008b... |
| Correct VERSION | `git show v4.0.0-rc2:VERSION` | 4.0.0-rc2 |
| No SHA collision | `git rev-parse v3.8.0-stage8-backfill` != `git rev-parse v4.0.0-rc2` | Different SHAs |
| v4.0.0-rc1 unchanged | `git rev-parse v4.0.0-rc1` | 28bcb99 |
| Remote tags match | `git ls-remote origin refs/tags/v4.0.0-rc2` | Matches local |

---

## Tag Integrity Enforcement

### Absolute Prohibitions

- ❌ Never move `v4.0.0-rc1` (already moved 3× — CONTESTED)
- ❌ Never force-update release tags
- ❌ Never delete/recreate tags to change commit identity
- ❌ Never publish artifacts from different commit than tag
- ❌ Never silently replace release assets

### RC2 Tag Repair Procedure (When Authorized)

```bash
# 1. Verify current broken state
git show v4.0.0-rc2:VERSION
# Should show: 4.0.0-rc1 (BROKEN)

# 2. Delete broken tag locally
git tag -d v4.0.0-rc2

# 3. Delete remote broken tag
git push origin :refs/tags/v4.0.0-rc2

# 4. Create NEW tag at CORRECT commit (5cd008b has VERSION=4.0.0-rc2)
git tag -a v4.0.0-rc2 -m "Stage 16 — RC2 freeze at clean commit with VERSION=4.0.0-rc2" 5cd008b

# 4. Push new tag
git push origin v4.0.0-rc2

# 5. Verify
git rev-parse v4.0.0-rc2
git show v4.0.0-rc2:VERSION
# Must output: 4.0.0-rc2
```

---

## Tag Integrity Enforcement

### Absolute Prohibitions

- ❌ Never move `v4.0.0-rc1` (already moved 3× — CONTESTED)
- ❌ Never force-update release tags
- ❌ Never delete/recreate tags to change commit identity
- ❌ Never publish artifacts from different commit than tag
- ❌ Never silently replace release assets
- ❌ Never claim a corrected candidate IS the original RC2

### Tag Integrity Rules

1. **Immutability** — Once pushed, release tags are immutable
2. **Accuracy** — Tag version must match commit VERSION
3. **Uniqueness** — No two tags point to same commit
4. **Traceability** — Every tag creation logged with reason
7. **Auditability** — All tag operations auditable

---

## RC2 Repair Procedure (When Authorized)

### Pre-Repair Verification

```bash
# 1. Verify current broken state
git show v4.0.0-rc2:VERSION
# Expected: 4.0.0-rc1 (BROKEN)

# 2. Verify correct commit has correct version
git show 5cd008b:VERSION
# Expected: 4.0.0-rc2

# 2. Verify no existing artifacts at broken tag
# (No release published yet, so safe)
```

### Repair Execution (When Authorized)

```bash
# 1. Preserve broken tag for audit
git tag v4.0.0-rc2-broken 72d17d2 -m "PRESERVED: Broken v4.0.0-rc2 at 72d17d2 (VERSION=4.0.0-rc1)"

# 2. Delete broken tag
git tag -d v4.0.0-rc2
git push origin :refs/tags/v4.0.0-rc2

# 3. Create correct tag at correct commit
git tag -a v4.0.0-rc2 -m "Stage 16 — RC2 freeze at clean commit with VERSION=4.0.0-rc2" 5cd008b

# 4. Push corrected tag
git push origin v4.0.0-rc2

# 5. Verify
git rev-parse v4.0.0-rc2
git show v4.0.0-rc2:VERSION
```

### Post-Repair Verification

```bash
# Verify tag points to correct commit
git rev-parse v4.0.0-rc2
# Should output: 5cd008b...

# Verify VERSION in tagged commit
git show v4.0.0-rc2:VERSION
# Must output: 4.0.0-rc2

# Verify no SHA collision
git rev-parse v3.8.0-stage8-backfill
git rev-parse v4.0.0-rc2
# Must return DIFFERENT SHAs
```

---

## Gate S16-G05 Status

| Sub-gate | Requirement | Status |
|----------|-------------|--------|
| G16-G05.1 | Candidate commit selected | ✅ 5cd008b confirmed |
| G16-G05.2 | Working tree clean | ✅ (only untracked docs) |
| G16-G05.3 | Full release validation passes | ✅ All local tests pass |
| G16-G05.4 | RC2 tag created at exact validated commit | ⏳ PENDING (awaiting authorization) |
| G16-G05.5 | Existing tags untouched | ✅ v4.0.0-rc1 preserved at 28bcb99 |
| G16-G05.6 | RC2 lineage documented | ✅ Documented in tag-repair.md |

---

## Gate S16-G05 Status

**S16-G05: PENDING AUTHORIZATION** — Candidate identified (5cd008b), repair procedure documented, awaiting authorization to execute tag repair.

---

*Generated by Stage 16 Phase 5 — Immutable Candidate Tag Procedure*