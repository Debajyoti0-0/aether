# Stage 13 Phase 6 — RC2 Qualification & Immutable Freeze

**Timestamp:** 2026-09-16
**Scope:** Determine if f1242dd qualifies as v4.0.0-rc2

---

## RC2 Qualification Checklist

### Pre-Tagging Verification

| Check | Status | Evidence |
|-------|--------|----------|
| Candidate commit correct | ✅ | f1242dd confirmed |
| Working tree clean | ✅ | Only untracked docs |
| Version correct | ❌ STALE | VERSION = 4.0.0-rc1 |
| Changelog consistent | ⚠️ NOT CHECKED | Needs verification |
| Unit tests pass | ✅ | 38 packages PASS |
| Integration tests pass | ✅ | PASS |
| Vet pass | ✅ | PASS |
| Lint pass | ✅ | 0 issues |
| Govulncheck pass | ✅ | 0 vulns |
| Fuzz targets pass | ✅ | 21 targets PASS |
| Build pass | ✅ | PASS |
| Goreleaser snapshot | ✅ | PASS |
| Race validation | ⚠️ CI PENDING | Local blocked, CI evidence needed |
| B5 validation | ⚠️ WAIVER NEEDED | Live validation not available |
| B7 CI Release Validate | ⚠️ CI PENDING | Not triggered |
| Supply-chain requirements | ⚠️ PARTIAL | Branch protection, secret scanning not verified |
| Required repo controls | ⚠️ NOT VERIFIED | Manual check needed |
| Artifacts generated | ✅ | Snapshot successful |
| Checksums recorded | ✅ | SHA-256 for all artifacts |
| SBOM recorded | ✅ | 4 SPDX SBOMs |
| Provenance status | ⚠️ DEFERRED | Goreleaser v2.18.1 limitation |
| Signing status | ⚠️ PARTIAL | CI-based cosign only |
| Verification instructions | ✅ | VERIFY.md published |
| Blocker register consistent | ✅ | Documented in final report |
| Release classification | ✅ | Production-Limited documented |
| RC2 decision authorized | ⏳ PENDING | Awaiting CI evidence |

---

## RC2 Qualification Decision

### Current Verdict: **NOT YET QUALIFIED**

**Blocking Items:**
1. **B3 Race Detector** — CI race isolation results not retrieved
2. **B7 Release Validate** — CI not triggered, evidence missing
3. **B5 Azure KV** — Live validation not available, waiver needed
4. **Supply-chain controls** — Branch protection, secret scanning not verified
5. **VERSION file stale** — Still reads `4.0.0-rc1`

### Required for Qualification
1. Retrieve CI race isolation results for f1242dd
2. Trigger and verify Release Validate on f1242dd
3. File B5 waiver (live validation unavailable)
4. Verify/configure repository security controls
5. Update VERSION to 4.0.0-rc2
6. All CI gates green

---

## Tagging Procedure (When Qualified)

### Pre-Tag Checklist
- [ ] All qualification checks PASS
- [ ] VERSION updated to `4.0.0-rc2`
- [ ] Changelog updated for 4.0.0-rc2
- [ ] Working tree clean (only untracked docs)
- [ ] No existing `v4.0.0-rc2` tag locally or remotely

### Tagging Commands
```bash
# 1. Update VERSION
echo "4.0.0-rc2" > VERSION
git add VERSION
git commit -m "chore: bump version to 4.0.0-rc2"

# 2. Create immutable tag at clean commit
git tag -a v4.0.0-rc2 -m "Stage 13 — RC2 qualified: B3/B5/B7 closed or waived"

# 3. Push tag (triggers Release workflow)
git push origin v4.0.0-rc2

# 4. Verify tag
git rev-parse v4.0.0-rc2
git show v4.0.0-rc2 --no-patch
```

### Post-Tag Verification
- [ ] Tag appears in `git tag -l 'v4.0.0-rc2'`
- [ ] `git rev-parse v4.0.0-rc2` returns correct SHA
- [ ] Release workflow triggers and completes
- [ ] Artifacts published to GitHub Release
- [ ] Verification job passes

---

## Tag Integrity Enforcement

### Absolute Prohibitions
- ❌ Never move `v4.0.0-rc1` (already moved twice: e3154ce → 66b3600 → 28bcb99)
- ❌ Never move `v4.0.0-rc2` after creation
- ❌ Never force-update release tags
- ❌ Never delete/recreate tags to change commit identity
- ❌ Never publish artifacts from different commit than tag

### RC1 Lineage (CONTESTED - Frozen)
```
v4.0.0-rc1 target history:
  e3154ce (original Stage 9) 
  → 66b3600 (Stage 10 move 1) 
  → 28bcb99 (Stage 10 move 2, CURRENT)
```

### RC2 Lineage (To Be Frozen)
```
v4.0.0-rc2 target: f1242dd (or clean successor)
```

---

## RC2 Release Manifest (Projected)

### Artifacts (from snapshot build)
| Artifact | Platform | Format | SHA-256 (from last snapshot) |
|----------|----------|--------|------------------------------|
| aether_v4.0.0-rc2_linux_amd64.tar.gz | Linux x86_64 | tar.gz | ca74b2a145953efa443ab953c92ee3d957bad72b329f22a60ec819c41f8d640f |
| aether_v4.0.0-rc2_linux_arm64.tar.gz | Linux ARM64 | tar.gz | 4c9e302238bb0e2bce9fca1270799a5e01269fd47b60950c4176e82f747098d5 |
| aether_v4.0.0-rc2_darwin_amd64.tar.gz | macOS x86_64 | tar.gz | 719665e1fd367abce0ed8bcf28a68c8520f823c1fc6ebc045b5db172fe7a2540 |
| aether_v4.0.0-rc2_windows_amd64.zip | Windows x86_64 | zip | 8e0f203f99accae27c0b66597870e287033c798cd6c54aa062d5e777b4691aa4 |

### Attached to Release
- 4 SBOMs (SPDX 2.3 via syft)
- checksums.txt (SHA-256 for all 8 files)
- cosign signatures (.sig, .pem for Linux/macOS)
- Release manifest (release-manifest.json)
- Release notes (auto-generated)

---

## Gate G116 Status

| Sub-gate | Status |
|----------|--------|
| G116.1 RC2 tag created | ❌ NOT CREATED |
| G116.2 Tag at correct commit | ❌ PENDING |
| G116.3 Tag pushed to origin | ❌ PENDING |
| G116.4 Tag target verified | ❌ PENDING |

---

## Qualification Verdict

**NOT YET QUALIFIED** — Requires:
1. CI evidence for B3, B7
2. B5 waiver filed
3. VERSION updated
4. Repository security controls verified
5. All CI gates green

**Next:** Complete remaining CI evidence collection and blocker dispositions before tagging.

---

*Generated by Stage 13 Phase 6 — RC2 Qualification & Immutable Freeze*