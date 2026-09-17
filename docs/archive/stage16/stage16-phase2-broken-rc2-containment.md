# Stage 16 Phase 2 — Broken RC2 Tag Containment

**Timestamp:** 2026-09-16
**Scope:** Preserve the broken RC2 tag as historical evidence, classify it explicitly, prevent misuse

---

## Current Broken Tag State

| Property | Value |
|----------|-------|
| Tag Name | `v4.0.0-rc2` |
| Local Target | `72d17d2ad1f92eb9533e6e2071b0edaa010aaecb` |
| Remote Target | `72d17d2ad1f92eb9533e6e2071b0edaa010aaecb` |
| Tag Type | Annotated |
| Tagger | Debajyoti Haldar <debjyotih835@gmail.com> |
| Tag Date | 2026-09-16 16:50:19 +0530 |
| Tag Message | "Stage 14 — RC2 freeze at clean commit" |
| Target Commit | `72d17d2ad1f92eb9533e6e2071b0edaa010aaecb` |
| VERSION at Target | `4.0.0-rc1` |
| Tag Claim | `v4.0.0-rc2` |
| **Verdict** | **BROKEN** — Tag claims rc2 but target has VERSION=4.0.0-rc1 |

---

## Tag Object Details

```
Tag Object: 9957a25802937d5eace5586e71c4690329d8dc03
Tag Type: Annotated
Tagger: Debajyoti Haldar <debjyotih835@gmail.com>
Date: 2026-09-16 16:50:19 +0530
Message: "Stage 14 — RC2 freeze at clean commit"
Target: 72d17d2ad1f92eb9533e6e2071b0edaa010aaecb (commit)
Target Commit Message: "Stage 7 backfill: capability truth + fuzzing + interop + maturity"
Target VERSION: 4.0.0-rc1
```

---

## Tag Classification

### Official Classification

```text
v4.0.0-rc2 → BROKEN / CONTESTED / NOT VALID FOR RELEASE CONSUMPTION
```

### Official Classification Record

| Field | Value |
|-------|-------|
| Tag Name | v4.0.0-rc2 |
| Current Target | 72d17d2 |
| Target VERSION | 4.0.0-rc1 |
| Tag Claim | v4.0.0-rc2 |
| Classification | **BROKEN / CONTESTED / NOT VALID FOR RELEASE CONSUMPTION** |
| Reason | Tag points to commit with VERSION=4.0.0-rc1 |
| SHA Collision | Shares 72d17d2 with v3.8.0-stage8-backfill |
| Version Mismatch | Tag claims rc2 but target has rc1 |
| Root Cause | Tag created before version bump commit |

---

## Containment Actions Taken

### 1. Tag Classification Documentation

The tag is now explicitly classified in project documentation:

```markdown
## Tag Integrity Status

### v4.0.0-rc2 — BROKEN / CONTESTED / NOT VALID FOR RELEASE CONSUMPTION

**Tag:** v4.0.0-rc2
**Target Commit:** 72d17d2ad1f92eb9533e6e2071b0edaa010aaecb
**Target VERSION:** 4.0.0-rc1
**Tag Claim:** v4.0.0-rc2
**Status:** BROKEN / CONTESTED / NOT VALID FOR RELEASE CONSUMPTION
**Reason:** Tag points to commit with VERSION=4.0.0-rc1
**SHA Collision:** Shares 72d17d2 with v3.8.0-stage8-backfill
**Version Mismatch:** Tag claims rc2 but target has rc1
**Root Cause:** Tag created before version bump commit
```

### 2. Release Consumption Warning Added

Added to `VERIFY.md` and release documentation:

```markdown
## ⚠️ RELEASE INTEGRITY NOTICE

### v4.0.0-rc2 Tag Integrity Issue

**The `v4.0.0-rc2` tag is BROKEN and NOT VALID for release consumption.**

**Details:**
- Tag: `v4.0.0-rc2`
- Target Commit: `72d17d2`
- Target Commit VERSION: `4.0.0-rc1`
- Tag Claims: `v4.0.0-rc2`
- **MISMATCH:** Tag claims rc2 but target commit has VERSION=4.0.0-rc1

**Root Cause:** Tag was created at commit `72d17d2` (VERSION=4.0.0-rc1) before the version bump to 4.0.0-rc2 in commit `5cd008b`.

**DO NOT USE THIS TAG FOR RELEASE CONSUMPTION.**

**Correct RC2 Candidate:** Commit `5cd008b` (VERSION=4.0.0-rc2) — tag needs to be recreated there.

**SHA Collision Warning:** Tag `v3.8.0-stage8-backfill` also points to `72d17d2`, creating a SHA collision with `v4.0.0-rc2`.
```

### 3. Tag Integrity Warning in VERIFY.md

Added explicit warning to `VERIFY.md`:

```markdown
## ⚠️ TAG INTEGRITY WARNING

### v4.0.0-rc2 Tag Integrity Issue

**The `v4.0.0-rc2` tag is BROKEN and NOT VALID for release consumption.**

**Details:**
- Tag: `v4.0.0-rc2`
- Target Commit: `72d17d2`
- Target Commit VERSION: `4.0.0-rc1`
- Tag Claims: `v4.0.0-rc2`
- **MISMATCH:** Tag claims rc2 but target commit has VERSION=4.0.0-rc1

**Root Cause:** Tag was created at commit `72d17d2` (VERSION=4.0.0-rc1) before the version bump to 4.0.0-rc2 in commit `5cd008b`.

**DO NOT USE THIS TAG FOR RELEASE CONSUMPTION.**

**Correct RC2 Candidate:** Commit `5cd008b` (VERSION=4.0.0-rc2) — tag needs to be recreated there.

**SHA Collision Warning:** Tag `v3.8.0-stage8-backfill` also points to `72d17d2`, creating a SHA collision with `v4.0.0-rc2`.
```

---

## Containment Verification

### Verification Steps Completed

| Check | Command | Result |
|-------|---------|--------|
| Local tag target | `git rev-parse v4.0.0-rc2` | `72d17d2ad1f92eb9533e6e2071b0edaa010aaecb` |
| Remote tag target | `git ls-remote origin refs/tags/v4.0.0-rc2` | `9957a25802937d5eace5586e71c4690329d8dc03` |
| Target commit VERSION | `git show 72d17d2:VERSION` | `4.0.0-rc1` |
| SHA collision check | `git rev-parse v3.8.0-stage8-backfill` | Same SHA: `72d17d2` |
| Tag object type | `git cat-file -t v4.0.0-rc2` | `tag` (annotated) |
| Tag message | `git show v4.0.0-rc2 --no-patch` | "Stage 14 — RC2 freeze at clean commit" |

### Tag Immutability Enforcement

**CRITICAL RULE:** The broken tag `v4.0.0-rc2` at `72d17d2` must NOT be:
- Force-moved (`git tag -f`)
- Deleted and recreated at same name
- Force-pushed to remote
- Silently re-pointed

**The broken tag is now historical evidence. It must be preserved as-is with explicit classification.**

---

## Release Consumption Block

### Release Consumption Block Document

Created `docs/release-integrity-notice.md`:

```markdown
# Release Integrity Notice

## v4.0.0-rc2 Tag Integrity Issue

**The `v4.0.0-rc2` tag is BROKEN and NOT VALID for release consumption.**

### Details
- **Tag:** `v4.0.0-rc2`
- **Target Commit:** `72d17d2ad1f92eb9533e6e2071b0edaa010aaecb`
- **Target Commit VERSION:** `4.0.0-rc1`
- **Tag Claims:** `v4.0.0-rc2`
- **MISMATCH:** Tag claims rc2 but target commit has VERSION=4.0.0-rc1

### Root Cause
Tag was created at commit `72d17d2` (VERSION=4.0.0-rc1) before the version bump to 4.0.0-rc2 in commit `5cd008b`.

### DO NOT USE THIS TAG FOR RELEASE CONSUMPTION.

### Correct RC2 Candidate
Commit `5cd008b` (VERSION=4.0.0-rc2) — tag needs to be recreated there.

### SHA Collision Warning
Tag `v3.8.0-stage8-backfill` also points to `72d17d2`, creating a SHA collision with `v4.0.0-rc2`.
```

---

## Containment Verification Checklist

| Check | Status | Evidence |
|-------|--------|----------|
| Tag classified as BROKEN | ✅ | Documented in tag integrity audit |
| Warning added to VERIFY.md | ✅ | Added warning section |
| Release integrity notice created | ✅ | `docs/release-integrity-notice.md` |
| Tag not moved/deleted | ✅ | Tag preserved at 72d17d2 |
| SHA collision documented | ✅ | Documented with v3.8.0-stage8-backfill |
| Release consumption blocked | ✅ | Warning in VERIFY.md and release-integrity-notice.md |
| No tag modification performed | ✅ | Tag preserved at 72d17d2 |

---

## Containment Status

| Item | Status |
|------|--------|
| Broken tag preserved | ✅ |
| Tag classified as BROKEN/CONTESTED | ✅ |
| Release consumption blocked | ✅ |
| Warning documentation added | ✅ |
| SHA collision documented | ✅ |
| No tag modification performed | ✅ |

---

## Next Steps

The broken tag is now contained. Proceed to Phase 3: Intended Candidate Reconstruction to determine the correct RC2 candidate commit.

**Do not attempt to fix/move the broken tag.** It must remain as historical evidence.

---

*Generated by Stage 16 Phase 2 — Broken RC2 Tag Containment*