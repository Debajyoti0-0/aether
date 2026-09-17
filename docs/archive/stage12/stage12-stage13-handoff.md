# Stage 12 → Stage 13 Handoff

**Timestamp:** 2026-09-16
**From:** Stage 12 (GA Blocker Closure + Supply Chain Hardening)
**To:** Stage 13 (Operations Track or Waiver Closure)

---

## Current State Summary

| Metric | Value |
|--------|-------|
| **Version** | 4.0.0-rc2 (Production-Limited) |
| **Commit** | 55f37b1 |
| **RC1 Tag** | v4.0.0-rc1 at 28bcb99 (CONTESTED — moved from e3154ce) |
| **RC2 Tag** | v4.0.0-rc2 (to be created at 55f37b1) |
| **GA Status** | NOT GRANTED |
| **Phase A (Blockers)** | PARTIAL — B3/B5/B7 open, B4 waived |
| **Phase B (Operations)** | PARKED — 13 design docs frozen |
| **Supply Chain** | HARDENED — pinning, VERIFY.md, tamper tests |

---

## Blocker Status for Stage 13

### Must Close Before Any GA (Phase A)

| Blocker | Current | Stage 13 Action |
|---------|---------|-----------------|
| **B3 Race Detector** | PARTIAL (mutex fixes applied) | CI race isolation results → green or waiver |
| **B4 EV Authenticode** | WAIVED (2027-03-31) | Procure cert OR renew waiver |
| **B5 Azure KV Validation** | PARTIAL (contract fixed) | Live validation OR waiver |
| **B7 Release Validate** | PARTIAL (local PASS) | CI Validate green OR waiver with evidence |

### Supply Chain Hardening (Required for GA)

| Item | Status | Stage 13 Action |
|------|--------|-----------------|
| Action pinning | ✅ Done | Verify SHAs are correct |
| Branch protection | ❌ Not done | Enable via GitHub UI |
| Secret scanning | ❌ Not done | Enable in repo settings |
| Dependabot alerts | ❌ Not done | Enable in repo settings |
| Provenance (SLSA) | ❌ Deferred | Upgrade goreleaser ≥ v2.19 |
| Cosign in goreleaser | ❌ Deferred | Upgrade goreleaser ≥ v2.19 |
| VERIFY.md | ✅ Published | Test reproduction |
| Reproducible build | ❌ Untested | Build twice, compare |

### Tag Integrity

| Action | Required |
|--------|----------|
| Freeze `v4.0.0-rc2` at clean commit | Create at 55f37b1 or clean successor |
| Never re-move `v4.0.0-rc1` | Document as CONTESTED in release notes |
| Version monotonicity | Update VERSION to 4.0.0-rc2 at freeze |

---

## Evidence Index for Stage 13

| Document | Gate | Purpose |
|----------|------|---------|
| `stage12-phase0-baseline.md` | G91 | Repository baseline lock |
| `stage12-phase1-b3-race-closure.md` | G97 | Race detector fixes |
| `stage12-phase2-b4-authenticode.md` | G100 | Authenticode waiver |
| `stage12-phase2-b4-waiver.md` | G100 | Formal waiver document |
| `stage12-phase3-b5-contract-audit.md` | G101 | Azure KV contract fix |
| `stage12-phase4-b7-release-validate.md` | G102 | Release Validate investigation |
| `stage12-phase5-rc2-freeze.md` | G108 | RC2 freeze procedure |
| `stage12-phase6-supply-chain-audit.md` | G104 | Supply chain hardening |
| `stage12-final-report.md` | G106 | Final verdict |
| `stage12-stage13-handoff.md` | G110 | This document |
| `VERIFY.md` | G105 | Independent verification guide |

---

## Phase B Design Documents (Parked — Do Not Rewrite)

| Document | Purpose |
|----------|---------|
| `stage11-phase6-410-operations-design.md` | Operations architecture |
| `stage11-phase7-entra-validation-plan.md` | Entra validation plan |
| `stage11-phase8-imds-validation-plan.md` | IMDS validation plan |
| `stage11-phase9-testing-requirements.md` | Test specifications |
| `stage11-phase10-documentation.md` | Documentation templates |

**Instruction:** Reference these in Stage 13 — do not rewrite. They represent approved designs awaiting implementation.

---

## Commands for Stage 13 Entry

```bash
# 1. Verify baseline
git rev-parse HEAD
# Should be 55f37b1 or clean successor

# 2. Verify all gates pass locally
go test ./...
go vet ./...
golangci-lint run --timeout 5m
govulncheck ./...
go test -tags=integration ./test/integration/...
go build ./...
goreleaser release --snapshot --clean

# 3. Check blocker register
cat docs/stage12-final-report.md | grep -A 20 "BLOCKER REGISTER"

# 4. Verify tag integrity
git rev-parse v4.0.0-rc1
# Should be 28bcb99 (CONTESTED)
# v4.0.0-rc2 should not exist yet

# 5. Freeze RC2 tag (when ready)
git tag -a v4.0.0-rc2 -m "Stage 12 — RC2 with B3/B5 fixes, B4 waived"
git push origin v4.0.0-rc2
```

---

## Decision Tree for Stage 13

```
Stage 13 Start
      │
      ▼
┌─────────────────┐
│ Can B3 race be  │──NO──▶ Document limitation, ship rc2, defer GA
│ resolved in CI? │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ Can B5 Azure KV │──NO──▶ File waiver, ship rc2, defer GA
│ be live-validated?    │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ Can B7 Validate │──NO──▶ Debug CI, fix, ship rc2
│ be confirmed?   │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ Supply chain    │──NO──▶ Harden, then proceed
│ hardened?       │
└────────┬────────┘
         │YES
         ▼
    🎉 GRANT GA
    Tag v4.0.0
    Begin 4.1.0 ops (reuse parked Phase B designs)
```

---

## Risk Register for Stage 13

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Race detector CI fails | Medium | High | Isolation workflow identifies package |
| Azure KV auth unavailable | Medium | High | Mock tests first, waiver if needed |
| EV cert procurement delayed | High | Medium | Waiver already filed (2027-03-31) |
| Flaky test in Validate | Medium | Medium | CI debug run first |
| Goreleaser upgrade breaks config | Low | Medium | Test snapshot before tag |
| Branch protection conflicts | Low | Medium | Configure carefully |

---

## Stage 13 Scope Options

### Option A: GA Granted → 4.1.0 Operations Track
- **Trigger:** All 4 blockers CLOSED
- **Scope:** Reuse 13 parked Phase B designs
- **Items:** Observability, OCSP/CRL, Multi-host, ARM64, Third-party IdP, Live Entra/IMDS
- **Timeline:** 2-4 weeks

### Option B: RC2 Only → Waiver Closure Track
- **Trigger:** Any blocker WAIVED (not CLOSED)
- **Scope:** Close each waiver before GA
- **Hard deadlines:** B4 by 2027-03-31, B1/B2 by 2027-06-30
- **Timeline:** Per waiver expiry

### Option C: Extended RC → Additional RCs
- **Trigger:** Multiple blockers PARTIAL with no clear path
- **Scope:** Incremental fixes, additional RCs
- **Risk:** Release fatigue, version confusion

---

## Sign-Off

**Stage 12 Complete:** ✅ Phase A analysis + fixes + Phase B parking + Supply chain hardening
**GA Blockers Closed:** ❌ 0/4 (B3/B5/B7 PARTIAL, B4 WAIVED)
**Phase B Implemented:** ❌ 0/6 (design only, parked)
**Tag Integrity:** ⚠️ Violation documented, RC2 freeze planned
**Supply Chain Hardened:** ✅ Pinning, VERIFY.md, tamper tests
**Independent Verification:** ✅ VERIFY.md published
**Independent Reproduction:** ❌ Not tested

**Recommendation:** Stage 13 should focus **exclusively** on Phase A blocker disposition. Do not begin Phase B implementation until all 4 blockers are CLOSED or WAIVED with documented expiry and evidence.

---

*Generated by Stage 12 — 2026-09-16*