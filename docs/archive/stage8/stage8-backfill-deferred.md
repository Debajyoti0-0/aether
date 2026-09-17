# Stage 8 Backfill — Deferred Register

**Timestamp:** 2026-09-16
**Baseline:** `v3.7.0-stage7-backfill` (commit `72d17d2`)
**Scope:** Deferred work items from historical Stage 8 scope

---

## 1. Deferred Items from Historical Stage 8 Scope

| ID | Item | Original Stage 8 WS | Current Status | Reason for Deferral | Target Stage | Owner |
|----|------|---------------------|----------------|---------------------|--------------|-------|
| D1 | Live Entra ID validation (B1) | WS1 | WAIVED (2027-06-30) | No authorized tenant | Stage 14+ | Security Team |
| D2 | Live IMDSv2 validation (B2) | WS1 | WAIVED (2027-06-30) | No authorized Azure VM | Stage 14+ | Infra Team |
| D3 | EV Authenticode certificate (B4) | WS3 | WAIVED (2027-03-31) | Cert not procured | Stage 14+ | Security/Platform |
| D4 | Live Azure KV validation (B5) | WS4 | WAIVED (2027-03-31) | No authorized Azure environment | Stage 14+ | Security Team |
| D3 | Provenance/SLSA generation | WS5 | DEFERRED | goreleaser v2.18.1 limitation | Stage 14+ | Platform Team |
| D4 | Cosign in goreleaser | WS5 | DEFERRED | goreleaser v2.18.1 limitation | Stage 14+ | Platform Team |
| D5 | Branch protection | WS5 | NOT VERIFIED | Requires GitHub UI access | Stage 14+ | Platform Team |
| D6 | Secret scanning | D5 | NOT VERIFIED | Requires GitHub UI access | Stage 14+ | Security Team |
| D6 | Push protection | D5 | NOT VERIFIED | Requires GitHub UI access | Stage 14+ | Security Team |
| D7 | Dependabot alerts | D5 | NOT VERIFIED | Requires GitHub UI access | Stage 14+ | Security Team |
| D8 | Code scanning (CodeQL) | D5 | NOT VERIFIED | Requires GitHub UI access | Stage 14+ | Security Team |
| D8 | Tag protection/rulesets | D5 | NOT VERIFIED | Requires GitHub UI access | Stage 14+ | Platform Team |
| D9 | VERIFY.md reproduction | D5 | NOT TESTED | Not yet tested in fresh container | Stage 14+ | Release Eng |
| D9 | ARM64 integration testing | WS8 | NOT TESTED | linux_arm64 builds but not integration tested | Stage 14+ | Platform Team |
| D10 | OCSP/CRL revocation | WS5 | NOT IMPLEMENTED | File-based only; OCSP/CRL not implemented | Stage 14+ | Security Team |
| D10 | Observability (metrics/health) | WS8 | NOT IMPLEMENTED | No /metrics, /healthz endpoints | Stage 14+ (4.1.0) | Engineering |
| D10 | Multi-host deployment docs | WS8 | NOT DONE | Phase B scope | Stage 14+ (4.1.0) | Engineering |
| D11 | Third-party IdP interop | D10 | NOT DONE | Okta/Auth0/Keycloak not tested | Stage 14+ (4.1.0) | Engineering |
| D11 | ARM64 runtime promotion | WS8 | BUILD-VERIFIED | linux_arm64/darwin_arm64 build but not integration tested | Stage 14+ (4.1.0) | Platform Team |

---

## 2. Deferred by Category

### External Validation (Require External Resources)

| ID | Item | Blocking Factor | Waiver Expiry | Revalidation Trigger |
|----|------|-----------------|---------------|----------------------|
| D1 | Live Entra ID validation | No authorized tenant | 2027-06-30 | Authorized tenant available |
| D2 | Live IMDSv2 validation | No authorized Azure VM | 2027-06-30 | Authorized Azure VM available |
| D3 | EV Authenticode cert | No cert procured | 2027-03-31 | Cert procured + verified |
| D4 | Live Azure KV validation | No authorized Azure env | 2027-03-31 | Authorized Azure env available |

### Supply Chain / Repository Security (Require GitHub Admin)

| ID | Item | Blocking Factor | Target Resolution |
|----|------|-----------------|-------------------|
| D5 | Branch protection | Requires GitHub admin | Stage 14 |
| D6 | Secret scanning | Requires repo settings | 2027-03-31 |
| D6 | Push protection | Requires repo settings | 2027-03-31 |
| D7 | Dependabot alerts | Requires repo settings | 2027-03-31 |
| D7 | Code scanning (CodeQL) | Requires repo settings | 2027-03-31 |
| D8 | Tag protection/rulesets | Requires repo settings | 2027-03-31 |

### Provenance / Signing (Require Tooling Upgrade)

| ID | Item | Blocking Factor | Target Resolution |
|----|------|-----------------|-------------------|
| D3 | Provenance/SLSA | goreleaser v2.18.1 limitation | Stage 14 (upgrade to ≥ v2.19) |
| D3 | Cosign in goreleaser | goreleaser v2.18.1 limitation | Stage 14 |
| D3 | SLSA provenance | goreleaser v2.18.1 limitation | Stage 14 |

### Testing / Validation (Require External Resources)

| ID | Item | Blocking Factor | Target Resolution |
|----|------|-----------------|-------------------|
| D8 | VERIFY.md reproduction | Requires fresh container | Stage 14 |
| D9 | ARM64 integration testing | Requires ARM64 runner | Stage 14+ (4.1.0) |
| D9 | Live Azure KV validation | Requires authorized Azure env | Stage 14+ (4.1.0) |
| D10 | Live Entra/IMDS validation | Requires authorized env | 2027-06-30 |
| D10 | OCSP/CRL revocation | Not implemented | Stage 14+ (4.1.0) |

### Phase B / 4.1.0 Features (Post-GA)

| ID | Item | Category | Target Stage |
|----|------|----------|--------------|
| D10 | Observability (metrics/health) | Phase B / 4.1.0 | Post-GA |
| D10 | Multi-host deployment docs | Phase B / 4.1.0 | Post-GA |
| D11 | Third-party IdP interop | Phase B / 4.1.0 | Post-GA |
| D11 | ARM64 runtime promotion | Phase B / 4.1.0 | Post-GA |
| D11 | OCSP/CRL revocation | Phase B / 4.1.0 | Post-GA |
| D11 | Live Entra/IMDS validation | Phase B / 4.1.0 | Pre-2027-06-30 |

---

## 2. Deferred by Resolution Type

### External Dependencies (Cannot Resolve Internally)

| ID | Item | External Dependency | Earliest Resolution |
|----|------|---------------------|---------------------|
| D1 | Live Entra ID | Authorized tenant | External (Microsoft) |
| D2 | Live IMDS | Authorized Azure VM | External (Azure) |
| D3 | EV Cert | Certificate Authority | External (CA) |
| D4 | Live Azure KV | Azure subscription | External (Microsoft) |

### Tooling/Infrastructure (Internal but Requires Action)

| ID | Item | Required Action | Target Stage |
|----|------|-----------------|--------------|
| D3 | Provenance | Upgrade goreleaser ≥ v2.19 | Stage 14 |
| D3 | Cosign in goreleaser | Upgrade goreleaser ≥ v2.19 | Stage 14 |
| D5-D8 | Repo security controls | GitHub UI configuration | Stage 14 |

### Code Implementation (Internal, No External Deps)

| ID | Item | Status | Target Stage |
|----|------|--------|--------------|
| D10 | OCSP/CRL | NOT IMPLEMENTED | Stage 14+ (4.1.0) |
| D10 | Observability | NOT IMPLEMENTED | Phase B / 4.1.0 |
| D10 | Multi-host docs | NOT DONE | Phase B / 4.1.0 |
| D10 | ARM64 promotion | BUILD-VERIFIED | Phase B / 4.1.0 |
| D10 | Third-party IdP | NOT DONE | Phase B / 4.1.0 |

---

## 3. Deferred Register Summary

| Category | Count | Items |
|----------|-------|-------|
| External Validation (waived) | 4 | D1, D2, D3, D4 |
| Supply Chain / Repo Security | 5 | D5-D8 |
| Provenance/Signing (tooling) | 3 | D3 |
| Testing/Validation | 4 | D8, D9, D10 |
| Phase B / 4.1.0 Features | 5 | D10, D11 |

**Total Deferred Items:** 21

---

## 3. Revalidation Triggers

| Deferred Item | Revalidation Trigger | Owner |
|---------------|---------------------|-------|
| B1 Live Entra | Authorized tenant available | Security Team |
| B2 Live IMDS | Authorized Azure VM available | Infra Team |
| B4 EV Cert | Certificate procured | Security/Platform |
| B5 Live Azure KV | Authorized Azure env available | Security Team |
| Provenance | goreleaser ≥ v2.19 available | Platform Team |
| Repo security | GitHub admin access granted | Platform/Security |
| VERIFY.md reproduction | Fresh container available | Release Eng |
| ARM64 integration | ARM64 runner available | Platform Team |
| Live Entra/IMDS | Authorized env + timebox | Security/Infra |

---

## 3. Deferred Register Gates

| Gate | Requirement | Status |
|------|-------------|--------|
| S8B-G17 | Deferred register complete | ✅ COMPLETE |
| S8B-G18 | No new design documents | ✅ PASS |
| S8B-G19 | Stage 9 handoff | ✅ COMPLETE |

---

## 3. Deferred Register Summary

| Category | Deferred Count | Waived | Blocked by Tooling | Blocked by External |
|----------|----------------|--------|-------------------|---------------------|
| External Validation | 4 | 4 (waived) | 0 | 4 |
| Supply Chain / Repo Security | 5 | 0 (waivers pending) | 5 | 0 |
| Provenance / Signing | 3 | 0 (deferred) | 3 | 0 |
| Testing / Validation | 4 | 0 (partial) | 0 | 4 |
| Phase B / 4.1.0 | 5 | 0 (deferred) | 0 | 0 |

**Total Deferred:** 21 items

**Critical Path:** Supply chain controls (6 items) → GA authorization

---

*Generated by Stage 8 Backfill — Deferred Register*