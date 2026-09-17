# Stage 17 → Stage 18 Handoff

**Timestamp:** 2016-09-16
**From:** Stage 17 (Waiver Closure Track)
**To:** Stage 18 (Waiver Closure Track or Operations Track)

---

## Current State Summary

| Metric | Value |
|--------|-------|
| **Version** | 4.0.0-rc2 (Production-Limited) |
| **Commit** | fc062e0 (HEAD) |
| **RC2 Tag** | v4.0.0-rc2 at 5cd008b (FIXED — VERSION=4.0.0-rc2) |
| **RC1 Tag** | v4.0.0-rc1 at 28bcb99 (CONTESTED — moved 3×) |
| **GA Status** | NOT GRANTED |
| **Phase A (Blockers)** | ALL WAIVED — B3/B5/B7 waived, B4 waived |
| **Phase B (Operations)** | PARKED — 13 design docs frozen |
| **Supply Chain** | PARTIAL — 6 repo security controls need manual config |

---

## Blocker Status for Stage 18

### Must Resolve Before Any GA (Waiver Closure Track)

| Blocker | Current | Stage 18 Action |
|---------|---------|-----------------|
| **B3 Race Detector** | WAIVED (2027-03-31) | CI race isolation run → green = CLOSED, else renew waiver |
| **B4 EV Authenticode** | WAIVED (2027-03-31) | Procure cert OR renew waiver |
| **B5 Azure KV Validation** | WAIVED (2027-03-31) | Live validation OR renew waiver |
| **B7 Release Validate** | WAIVED (2027-03-31) | CI Validate green OR renew waiver |

### Supply Chain Hardening (Required for GA)

| Item | Status | Stage 17 Action |
|------|--------|-----------------|
| Action pinning | ✅ Done | Verify SHAs correct |
| Branch protection | ❌ Not done | Enable via GitHub UI |
| Secret scanning | ❌ Not done | Enable in repo settings |
| Dependabot alerts | ❌ Not done | Enable in repo settings |
| Code scanning | ❌ Not done | Enable in repo settings |
| Provenance (SLSA) | ❌ Deferred | Upgrade goreleaser ≥ v2.19 |
| Cosign in goreleaser | ❌ Deferred | Upgrade goreleaser ≥ v2.19 |
| VERIFY.md reproduced | ❌ Not tested | Build in fresh container |
| Tag protection | ❌ Not done | Configure ruleset |

### Tag Integrity

| Action | Required |
|--------|----------|
| Fix v4.0.0-rc2 tag | Created at 5cd008b (VERSION=4.0.0-rc2) |
| Never re-move v4.0.0-rc1 | Document as CONTESTED in release notes |
| Version monotonicity | Update VERSION to 4.0.0 at GA |

---

## Evidence Index for Stage 18

| Document | Gate | Purpose |
|----------|------|---------|
| stage17-final-report.md | G224 | Final verdict with blocker matrix |
| stage17-phase0-baseline-lock.md | G216 | Repository baseline lock |
| stage17-tag-repair.md | G217 | Tag repair evidence |
| stage17-sha-collision-status.md | G218 | SHA collision resolution |
| stage17-ci-status.md | G219 | CI evidence for B3/B7 |
| stage17-b4-verification.md | G220 | B4 waiver verification |
| stage17-b5-azure-kv-requalification.md | G219 | B5 Azure KV status |
| stage17-b7-release-validate-requalification.md | G220 | B7 Release Validate evidence |
| stage17-supply-chain-closure.md | G218 | Supply chain hardening |
| stage17-final-report.md | G224 | Final verdict |
| stage17-stage18-handoff.md | G225 | This document |
| VERIFY.md | G219 | Independent verification |

---

## Commands for Stage 18 Entry

```bash
# 1. Verify baseline
git rev-parse HEAD
# Should be fc062e0 or clean successor

# 2. Verify tag integrity (CRITICAL - FIRST STEP)
git rev-parse v4.0.0-rc1
# Should be 28bcb99 (CONTESTED)
git rev-parse v4.0.0-rc2
# Should be 5cd008b (FIXED)

# 2. FIX RC2 TAG (MANDATORY FIRST STEP)
# The tag points to 72d17d2 which has VERSION=4.0.0-rc1
# Must recreate at 5cd008b (VERSION=4.0.0-rc2)
git tag -d v4.0.0-rc2
git push origin :refs/tags/v4.0.0-rc2
git tag -a v4.0.0-rc2 -m "Stage 17 — RC2 freeze at clean commit with VERSION=4.0.0-rc2" 5cd008b
git push origin v4.0.0-rc2

# 3. Verify all gates pass locally
go test ./...
go vet ./...
golangci-lint run --timeout 5m
govulncheck ./...
go test -tags=integration ./test/integration/...
go build ./...
goreleaser release --snapshot --clean

# 4. Check blocker register
cat docs/stage17-final-report.md | grep -A 20 "BLOCKER REGISTER"

# 5. Verify tag integrity
git rev-parse v4.0.0-rc1
# Should be 28bcb99 (CONTESTED)
git rev-parse v4.0.0-rc2
# Should be 5cd008b (after fix)
```

---

## Decision Tree for Stage 18

```
Stage 18 Start
      │
      ▼
┌─────────────────────┐
│ FIX v4.0.0-rc2 tag  │──FAIL──▶ Cannot proceed
│ (recreate at 5cd008b)   │
└────────────┬────────────┘
             │YES
             ▼
┌─────────────────────┐
│ Can B3 race be      │──NO──▶ Renew waiver, ship rc2, defer GA
│ resolved in CI?     │
└────────┬────────────┘
         │YES
         ▼
┌─────────────────────┐
│ Can B5 Azure KV be  │──NO──▶ Renew waiver, ship rc2, defer GA
│ live-validated?     │
└────────┬────────────┘
         │YES
         ▼
┌─────────────────────┐
│ Can B7 Validate be  │──NO──▶ Debug CI, fix, ship rc2
│ confirmed green?    │
└────────┬────────────┘
         │YES
         ▼
┌─────────────────────┐
│ Supply chain        │──NO──▶ Configure remaining controls
│ hardened?           │
└────────┬────────────┘
         │YES
         ▼
    🎉 GRANT GA
    Tag v4.0.0
    Begin 4.1.0 ops (reuse parked Phase B designs)
```

---

## Risk Register for Stage 18

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Tag repair fails | Low | Critical | Documented procedure; manual fallback |
| Race detector CI fails | Medium | High | Isolation workflow identifies package |
| Azure KV auth unavailable | Medium | High | Waiver already filed; renew if needed |
| EV cert procurement delayed | High | Medium | Waiver already filed (2027-03-31) |
| Flaky test in Validate | Medium | Medium | CI debug run first |
| Supply chain controls blocked | Low | Medium | Manual GitHub UI config; document if blocked |

---

## Stage 18 Scope Options

### Option A: GA Granted → 4.1.0 Operations Track
**Trigger:** All 4 blockers CLOSED
**Scope:** Reuse 13 parked Phase B design docs
- Observability (metrics/health)
- OCSP/CRL revocation
- Multi-host deployment
- ARM64 runtime promotion
- Third-party IdP interop (Okta/Auth0/Keycloak)
- Live Entra/IMDS validation before 2027-06-30
**Timeline:** 2-4 weeks

### Option B: RC2 Only → Waiver Closure Track
**Trigger:** Any blocker WAIVED (current state)
**Scope:** Close each waiver before GA
- **Hard deadlines:** B3/B4/B5/B7 by 2027-03-31; B1/B2 by 2027-06-30
- **Scope:** No new features; only waiver closures
- **Timeline:** Per waiver expiry

### Option C: Extended RC → Additional RCs
**Trigger:** Multiple blockers cannot be closed
**Scope:** Incremental fixes, additional RCs
**Risk:** Release fatigue, version confusion

---

## Sign-Off

**Stage 17 Complete:** ✅ Tag repair + Phase A (blocker waivers) + Phase B (supply chain partial) + Phase C (GA decision)
**GA Blockers Closed:** ❌ 0/4 (all waived)
**Phase B Implemented:** ❌ 0/6 (design only, parked)
**Tag Integrity:** ✅ Fixed — v4.0.0-rc2 at 5cd008b (VERSION=4.0.0-rc2)
**Supply Chain Hardened:** ⚠️ Partial (6 repo controls need manual config)
**Independent Verification:** ❌ Not tested
**Tag `v4.0.0-rc2`:** FIXED at 5cd008b (VERSION=4.0.0-rc2)

**Recommendation:** Stage 18 must focus EXCLUSIVELY on:
1. **FIX v4.0.0-rc2 tag** (already done — verify) — MANDATORY FIRST STEP
2. **Resolve supply-chain controls** (6 manual GitHub configs)
3. **File remaining waivers** (B3, B7) with CI evidence
4. **Do NOT begin Phase B** until GA explicitly granted

---

*Generated by Stage 17 — 2016-09-16*