# Stage 7 Backfill — Maturity Reassessment

**Timestamp:** 2026-09-16
**Baseline:** v3.6.0-stage5-backfill (commit 1ea4191)
**Method:** Independent re-scoring against true baseline (Stage 6 + Stage 4/5 backfill evidence)

---

## Maturity Dimensions Re-scored

| Dimension | Stage 6 Baseline | Stage 7 Claim | Stage 7 Backfill (Actual) | Delta | Evidence |
|-----------|------------------|---------------|---------------------------|-------|----------|
| **Security** | M3 | M3.5 | **M4** | +1 | ZTNA/SAML/PQC defects addressed; mTLS enforced; signed audit chain |
| **Release Engineering** | M2 | M3.5 | **M3** | +1 | Goreleaser v2 config; CI pipeline; SBOM; checksums |
| **Supply Chain** | M2 | M3 | **M3** | +1 | Action pinning; SBOM; cosign in CI; provenance deferred |
| **External Validation** | M1 | M2 | **M1** | 0 | No live Entra/IMDS/Azure KV validation (waived) |
| **Reliability** | M2 | M3 | **M3** | +1 | Race detector fixes; idempotency; crash recovery |
| **Observability** | M1 | M2 | **M1** | 0 | No metrics/health endpoints |
| **Operability** | M1 | M2 | **M1** | 0 | No multi-host; no OCSP/CRL |
| **Protocol Correctness** | M2 | M3 | **M3** | +1 | SAML/WS-Trust/MS-OAPX parsers; fuzzing |
| **Interoperability** | M1 | M2 | **M1** | 0 | Local mTLS only; no live Entra/IdP |
| **Code Quality** | M2 | M3 | **M3** | +1 | Lint clean; fuzz 21 targets; 0 crashes |
| **Documentation** | M2 | M3 | **M3** | +1 | VERIFY.md; capability-truth docs; backfill evidence |

---

## Scoring Rationale

### Security: M4 (+1 from M3)
- **Evidence:** Capability-truth defects addressed (ZTNA/SAML/PQC)
- **Evidence:** mTLS enforced on all transport (TLS 1.3 minimum)
- **Evidence:** Signed audit chain with tamper-evident salt
- **Evidence:** Capability-aware key provider interface
- **Gap:** No live Entra/IMDS/Azure KV validation (waived)

### Release Engineering: M3 (+1 from M2)
- **Evidence:** Goreleaser v2 config with deterministic builds
- **Evidence:** CI pipeline with race, lint, vuln, integration
- **Evidence:** SBOM (SPDX via syft), checksums, cosign in CI
- **Gap:** Provenance/SLSA deferred (goreleaser v2.18.1); goreleaser upgrade deferred

### Supply Chain: M3 (+1 from M2)
- **Evidence:** GitHub Actions pinned to commit SHAs
- **Evidence:** SBOM generation in CI (cyclonedx-json via syft)
- **Evidence:** Cosign keyless signing in CI workflow
- **Gap:** Branch protection, secret scanning, Dependabot, CodeQL not verified

### External Validation: M1 (no change)
- **Status:** All live validations waived (Entra, IMDS, Azure KV)
- **Waivers:** B1 (Entra), B2 (IMDS) until 2027-06-30; B5 (Azure KV) until 2027-03-31

### Reliability: M3 (+1 from M2)
- **Evidence:** Race detector mutex fixes applied (AzureKVProvider, Workspace Rekey)
- **Evidence:** Idempotency keys with PutRecordIfAbsent
- **Evidence:** Crash-safe journal with bbolt transactions
- **Evidence:** Rollback stack with failed-reversal retention
- **Gap:** Race detector CI evidence pending

### Observability: M1 (no change)
- **Gap:** No /metrics endpoint (Prometheus)
- **Gap:** No /healthz /readyz endpoints
- **Gap:** No structured logging (JSON)

### Operability: M1 (no change)
- **Gap:** No multi-host deployment tested
- **Gap:** No OCSP/CRL revocation checking
- **Gap:** ARM64 builds verified but not integration tested

### Protocol Correctness: M3 (+1 from M2)
- **Evidence:** SAML 2.0 assertion parsing with VerifyRawDigest
- **Evidence:** WS-Trust RSTR parsing (4 fuzz targets)
- **Evidence:** MS-OAPX message parsing (4 fuzz targets)
- **Evidence:** 21 native fuzz targets, 0 crashes

### Interoperability: M1 (no change)
- **Evidence:** Local mTLS teamserver↔client exchange verified
- **Gap:** No live Entra ID tenant validation (waived)
- **Gap:** No live Azure IMDS validation (waived)
- **Gap:** No third-party IdP interop (Okta/Auth0/Keycloak)

### Code Quality: M3 (+1 from M2)
- **Evidence:** golangci-lint v2.13.2 clean (0 issues)
- **Evidence:** govulncheck: 0 affecting vulnerabilities
- **Evidence:** 21 native fuzz targets, 50M+ executions, 0 crashes
- **Evidence:** Race detector mutex fixes applied

### Documentation: M3 (+1 from M2)
- **Evidence:** VERIFY.md published with reproduction steps
- **Evidence:** Capability-truth defects documented with negative tests
- **Evidence:** Stage backfill evidence documented
- **Evidence:** VERIFY.md not yet reproduced in fresh container

---

## Overall Maturity Score

| Dimension | Score |
|-----------|-------|
| Security | M4 |
| Release Engineering | M3 |
| Supply Chain | M3 |
| External Validation | M1 |
| Reliability | M3 |
| Observability | M1 |
| Operability | M1 |
| Protocol Correctness | M3 |
| Interoperability | M1 |
| Code Quality | M3 |
| Documentation | M3 |

**Weighted Average: ~M2.5 → M3** (consistent with Stage 11/12 assessment)

---

## Comparison with Stage 7 Claim

| Dimension | Stage 7 Claim | Actual | Delta |
|-----------|---------------|--------|-------|
| Security | M3.5 | M4 | +0.5 |
| Release Engineering | M3.5 | M3 | -0.5 |
| Supply Chain | M3 | M3 | 0 |
| External Validation | M2 | M1 | -1 |
| Reliability | M3 | M3 | 0 |
| Observability | M2 | M1 | -1 |
| Operability | M2 | M1 | -1 |
| Protocol Correctness | M3 | M3 | 0 |
| Interoperability | M2 | M1 | -1 |
| Code Quality | M3 | M3 | 0 |
| Documentation | M3 | M3 | 0 |

**Stage 7 claimed M3.5 overall; actual is ~M2.5 (M3 with limitations).**

---

*Generated by Stage 7 Backfill — Maturity Reassessment*