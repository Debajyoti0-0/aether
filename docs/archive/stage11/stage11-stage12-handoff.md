# Stage 11 → Stage 12 Handoff

**Timestamp:** 2026-09-16
**From:** Stage 11 (GA Blocker Closure + Operations Design)
**To:** Stage 12 (Operations Track or GA Closure)

---

## Current State Summary

| Metric | Value |
|--------|-------|
| **Version** | 4.0.0-rc2 (Production-Limited) |
| **Commit** | d26aae7dccce484d395b40b268941d90ca5c8503 |
| **Tag** | v4.0.0-rc1 at 28bcb99 (CONTESTED — moved from e3154ce) |
| **GA Status** | NOT GRANTED |
| **Phase A (Blockers)** | INCOMPLETE — B3, B4, B5, B7 open |
| **Phase B (Operations)** | DESIGN COMPLETE — Not implemented |

---

## Blocker Status for Stage 12

### Must Close Before Any GA (Phase A)

| Blocker | Required for GA | Stage 12 Action |
|---------|-----------------|-----------------|
| **B3 Race Detector** | YES | CI isolation workflow → identify failing package → fix races → CI green |
| **B4 EV Authenticode** | YES | Procure cert OR file waiver (expiry ≤ 2027-03-31) |
| **B5 Azure KV Validation** | YES | Add mutex, unit/integration tests, live Azure KV round-trip |
| **B7 Release Validate** | YES | Debug CI failure → fix → green |

### Supply Chain Hardening (Required for GA)

| Item | Stage 12 Action |
|------|-----------------|
| Action pinning to SHAs | Replace all `@vX` with commit SHAs in ci.yml, release.yml |
| Branch protection | Enable via `gh repo edit` or GitHub UI |
| Secret scanning | Enable in repository settings |
| Dependabot alerts | Enable in repository settings |
| Provenance (SLSA) | Add `attestations` to `.goreleaser.yml` |
| Cosign in goreleaser | Add `signs` section to `.goreleaser.yml` |
| VERIFY.md | Create stranger-reproducible verification guide |

### Tag Integrity

| Action | Required |
|--------|----------|
| Freeze `v4.0.0-rc2` at clean commit | Create at `d26aae7` or successor |
| Never re-move `v4.0.0-rc1` | Document as CONTESTED in release notes |

---

## Phase B: 4.1.0 Operations Track (If Phase A Closes)

### Implemented (Design Only)

| Component | Document | Status |
|-----------|----------|--------|
| Observability architecture | `phase6-410-operations-design.md` | Designed |
| External validation framework | `phase6-410-operations-design.md` | Designed |
| Entra validation plan | `phase7-entra-validation-plan.md` | Planned |
| IMDS validation plan | `phase8-imds-validation-plan.md` | Planned |
| Testing requirements | `phase9-testing-requirements.md` | Defined |
| Documentation templates | `phase10-documentation.md` | Templates ready |

### To Implement in Stage 12

| Priority | Item | Dependencies |
|----------|------|--------------|
| P0 | Observability (metrics/health) | None |
| P0 | OCSP/CRL revocation | None |
| P1 | ARM64 integration testing | Access to ARM64 runners |
| P1 | Third-party IdP interop | Okta/Auth0/Keycloak test tenant |
| P2 | Multi-host deployment docs | None |
| P2 | Live Entra validation | Authorized tenant + approval |
| P2 | Live IMDS validation | Authorized VM + approval |
| P3 | Enhanced CLI UX | None |
| P3 | Structured logging | None |

---

## Evidence Index for Stage 12

| Document | Gate | Purpose |
|----------|------|---------|
| `stage11-phase0-baseline.md` | G71 | Repository baseline lock |
| `stage11-phase1-azure-kv-audit.md` | G79 | Azure KV implementation audit |
| `stage11-phase1-goreleaser-sbom-audit.md` | G2, G83 | Build/artifact audit |
| `stage11-phase2-race-detector.md` | G75 | Race detector analysis |
| `stage11-phase3-release-validate.md` | G80 | Release workflow failure analysis |
| `stage11-phase4-azure-kv-validation.md` | G79 | Azure KV validation plan |
| `stage11-phase5-artifact-trust.md` | G2, G6, G7, G8 | Supply chain audit |
| `stage11-phase6-410-operations-design.md` | G88 | Operations architecture |
| `stage11-phase7-entra-validation-plan.md` | G89 | Entra validation plan |
| `stage11-phase8-imds-validation-plan.md` | G89 | IMDS validation plan |
| `stage11-phase9-testing-requirements.md` | All | Test specifications |
| `stage11-phase10-documentation.md` | All | Documentation templates |
| `stage11-final-report.md` | G87 | Final verdict |
| `stage11-stage12-handoff.md` | G90 | This document |

---

## Commands for Stage 12 Entry

```bash
# 1. Verify baseline
git rev-parse HEAD
# Should be d26aae7 or clean successor

# 2. Freeze rc2 tag (if not done)
git tag -a v4.0.0-rc2 -m "Stage 11 — GA blocker closure candidate"

# 3. Verify all gates pass locally
go test ./...
go vet ./...
golangci-lint run --timeout 5m
govulncheck ./...
go test -tags=integration ./test/integration/...
go build ./...
goreleaser release --snapshot --clean

# 4. Check blocker register
cat docs/stage11-final-report.md | grep -A 20 "BLOCKER REGISTER"

# 5. Verify tag integrity
git rev-parse v4.0.0-rc1
# Should be 28bcb99 (CONTESTED)
git rev-parse v4.0.0-rc2
# Should be d26aae7 (FROZEN)
```

---

## Decision Tree for Stage 12

```
Stage 12 Start
      │
      ▼
┌─────────────────┐
│ Can B3 be fixed │──NO──▶ Document limitation, ship rc2, defer GA
│ in 1 sprint?    │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ Can B4 cert be  │──NO──▶ File waiver (≤2027-03-31), ship rc2
│ procured?       │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ Can B5 Azure KV │──NO──▶ Document limitation, ship rc2, defer GA
│ be validated?   │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ Can B7 Validate │──NO──▶ Debug CI, fix, ship rc2
│ be fixed?       │
└────────┬────────┘
         │YES
         ▼
┌─────────────────┐
│ All supply chain│──NO──▶ Harden, then proceed
│ hardening done? │
└────────┬────────┘
         │YES
         ▼
    🎉 GRANT GA
    Tag v4.0.0
    Begin 4.1.0 ops
```

---

## Risk Register for Stage 12

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Race detector fixes extensive | Medium | High | Isolate packages, fix incrementally |
| EV cert procurement delayed | High | Medium | File waiver early |
| Azure KV auth unavailable | Medium | High | Mock tests first, live when authorized |
| Flaky test in Validate job | Medium | Medium | CI debug run first |
| ARM64 runners unavailable | Low | Low | Document as build-verified only |
| Entra/IMDS authorization denied | High | Medium | Waiver renewal process documented |

---

## Sign-Off

**Stage 11 Complete:** ✅ Phase A analysis + Phase B design
**GA Blockers Closed:** ❌ 0/4 (B3, B4, B5, B7 open)
**Phase B Implemented:** ❌ 0/6 (design only)
**Tag Integrity:** ⚠️ Violation documented
**Supply Chain Hardened:** ❌ Not done
**Independent Verification:** ❌ Not done

**Recommendation:** Stage 12 should focus **exclusively** on Phase A blocker closure. Do not begin Phase B implementation until all 4 blockers are CLOSED or WAIVED with documented expiry.

---

*Generated by Stage 11 — 2026-09-16*