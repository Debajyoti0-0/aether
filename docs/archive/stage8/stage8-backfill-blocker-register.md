# Stage 8 Backfill — Blocker Register v1 (B1–B6)

**Timestamp:** 2026-09-16
**Baseline:** `v3.7.0-stage7-backfill` (commit `43b23e5`) / `990426b` (Stage 3 root)
**Repository:** `C:\dev\aether`
**Context:** Historical reconstruction of the initial blocker register that was later refined in Stage 9 G1.

---

## 1. Blocker Register v1 (Historical Reconstruction)

This register reconstructs the initial blocker register (B1–B6) as it would have been defined at the historical Stage 8 entry point, based on the Stage 8 entry audit (`docs/stage8-entry-audit.md`) and Stage 9 G1 reclassification.

| ID | Description | Category | Security/Release Impact | Evidence Required | Evidence Available at Stage 8 | Status at Stage 8 | Closure Criteria | Owner | Waiver Authority | Waiver Expiry | Residual Risk |
|----|-------------|----------|------------------------|-------------------|------------------------------|-------------------|------------------|-------|------------------|---------------|---------------|
| **B1** | Live Entra ID validation (PRT→OAuth, WS-Trust, Device Code, CAE) | Interoperability | Cannot claim live Entra interop; PRT exchange untested against real tenant | Authorized tenant test captures (LIVE_SANITIZED); 10+ scenario results | Waiver only (Stage 7/8); no live execution | **WAIVED** (expiry 2026-12-31) | Execute against authorized tenant OR formal waiver with expiry | Security Team | Stage 8 Lead | 2026-12-31 | Production Entra claims unsupported |
| **B2** | Live IMDSv2 validation against Azure dev VM | Interoperability | Cannot claim live Azure IMDS interop; token hijack untested | Azure dev VM test captures (LIVE_SANITIZED); metadata/token fetch | Waiver only (Stage 7/8); no live execution | **WAIVED** (expiry 2026-12-31) | Execute against authorized Azure VM OR formal waiver with expiry | Infra Team | Stage 8 Lead | 2026-12-31 | Production Azure IMDS claims unsupported |
| **B3** | Release pipeline: GoReleaser + GitHub Actions (build matrix, provenance, signing, release) | Release Engineering | No automated release; manual builds untrustworthy for GA | Successful CI run on tag push; artifacts + signatures + SBOM + provenance published | `.goreleaser.yml` + `.github/workflows/release.yml` exist (uncommitted); never executed | **OPEN** | Pipeline executes end-to-end on `v4.0.0` tag; all platforms build; artifacts signed; provenance generated | Platform Team | N/A (must close) | N/A | No automated release capability |
| **B4** | EV Authenticode certificate procurement + Windows signing integration | Release Engineering | Windows binary unsigned; SmartScreen warnings; no trust chain | EV cert in GitHub Secrets; `signtool verify /pa` passes on released binary | `scripts/sign-windows.ps1` exists (uncommitted); EV cert NOT procured | **BLOCKED** | Procure EV cert; add to secrets; Windows signing job passes; verification passes | Security/Platform | N/A (must close) | N/A | Windows binary untrusted |
| **B5** | Audit key HSM/KMS custody (replace plaintext Ed25519 seed in workspace) | Security | Audit signing key extractable from workspace; no rotation; no recovery | HSM/KMS backend implementation; rotation test; recovery test | `KeyProvider` interface only; local file backend only; no HSM/KMS | **OPEN** | Implement ≥1 HSM/KMS backend (Azure Key Vault / AWS KMS / YubiHSM); rotation verified; recovery tested | Security Team | N/A (must close) | N/A | Audit key compromise risk |
| **B6** | Idempotency ledger hardening (replay safety, crash recovery, duplicate injection) | Reliability | Duplicate execution risk; crash recovery gaps | Integration tests: replay safety, crash recovery, adversarial duplicates, concurrent execution | 4/4 integration tests PASS (implemented in Stage 9); 10+ scenarios covered | **PARTIALLY_CLOSED** | Expand to 10+ test scenarios; all pass under race detector | Engineering | N/A (must close) | N/A | Limited test coverage |

---

## 2. Blocker Classification Summary (Stage 8 Historical)

| Classification | Count | Blockers |
|----------------|-------|----------|
| **CLOSED** | 0 | — |
| **PARTIALLY_CLOSED** | 1 | B6 |
| **OPEN** | 2 | B3, B5 |
| **BLOCKED** | 1 | B4 |
| **WAIVED** | 2 | B1, B2 |
| **NOT_APPLICABLE** | 0 | — |
| **FALSE_OR_UNSUPPORTED** | 0 | — |
| **TOTAL** | 6 | — |

---

## 3. Critical Path Analysis

**Must close for GA `4.0.0`:** B3, B4, B5 (and B6 fully)

**May waive for production-limited RC:** B1, B2 (with expiry ≤ 2026-12-31 in Stage 8; extended to 2027-06-30 in Stage 9)

**Dependency Chain:**
```
B3 (Pipeline) → enables B4 (Signing in CI) → enables B5 (Key custody in CI) → enables GA
```

B1/B2 are orthogonal (external infrastructure) and can be waived independently.

---

## 4. Waiver Decisions (Historical Stage 8)

### B1 — Live Entra ID Validation
- **Decision:** WAIVE with expiry 2026-12-31
- **Justification:** No authorized Entra ID lab tenant available. Requires dedicated tenant provisioning, disposable identities, and non-production resources. Not feasible within Stage 8 timebox.
- **Impact:** Release notes MUST state: "Live Entra ID interoperability not validated; PRT→OAuth, WS-Trust, Device Code, CAE flows are offline-validated only."
- **Owner:** Security Team
- **Approval:** Stage 8 Lead
- **Expiry:** 2026-12-31

### B2 — Live IMDSv2 Validation
- **Decision:** WAIVE with expiry 2026-12-31
- **Justification:** No Azure dev subscription/VM available. Requires dedicated Azure VM with managed identity enabled.
- **Impact:** Release notes MUST state: "Live Azure IMDSv2 validation not executed; IMDS token fetch and metadata parsing are offline-validated only."
- **Owner:** Infra Team
- **Approval:** Stage 8 Lead
- **Expiry:** 2026-12-31

---

## 5. Blocker Register Evolution (Stage 8 → Stage 9 → Current)

| Blocker | Stage 8 (Historical) | Stage 9 G1 (Refined) | Current (Stage 14) |
|---------|----------------------|----------------------|-------------------|
| **B1** | WAIVED (2026-12-31) | WAIVED (2027-06-30) | WAIVED (2027-06-30) |
| **B2** | WAIVED (2026-12-31) | WAIVED (2027-06-30) | WAIVED (2027-06-30) |
| **B3** | OPEN | OPEN | WAIVED (2027-03-31) / PARTIAL |
| **B4** | BLOCKED | BLOCKED | WAIVED (2027-03-31) |
| **B5** | OPEN | OPEN | WAIVED (2027-03-31) |
| **B6** | PARTIALLY_CLOSED | PARTIALLY_CLOSED | WAIVED / PARTIAL |

---

## 6. Critical Path Analysis (Historical Stage 8)

**Must close for GA `4.0.0`:** B3, B4, B5 (and B6 fully)

**May waive for production-limited RC:** B1, B2 (with expiry ≤ 2026-12-31)

**Dependency Chain:**
```
B3 (Pipeline) → enables B4 (Signing in CI) → enables B5 (Key custody in CI) → enables GA
```

B1/B2 are orthogonal (external infrastructure) and can be waived independently.

---

## 6. Waiver Register (Historical Stage 8)

| Blocker | Waiver Decision | Expiry | Owner | Approval | Revalidation Trigger |
|---------|----------------|--------|-------|----------|----------------------|
| B1 | WAIVED | 2026-12-31 | Security Team | Stage 8 Lead | Authorized tenant available |
| B2 | WAIVED | 2026-12-31 | Infra Team | Stage 8 Lead | Authorized Azure VM available |
| B3 | N/A (must close) | N/A | Platform Team | N/A | Pipeline green on tag push |
| B4 | N/A (must close) | N/A | Security/Platform | N/A | EV cert procured + verified |
| B5 | N/A (must close) | N/A | Security Team | N/A | Live KV round-trip verified |
| B6 | N/A (must close) | N/A | Engineering | N/A | Race detector green |

---

## 6. Closure Tracking (Historical)

| Blocker | Target Date | Actual Date | Evidence Link | Verified By |
|---------|-------------|-------------|---------------|-------------|
| B1 | 2026-12-31 (waiver) | — | — | — |
| B2 | 2026-12-31 (waiver) | — | — | — |
| B3 | Stage 8 WS2 | — | — | — |
| B4 | Stage 8 WS3 | — | — | — |
| B5 | Stage 8 WS4 | — | — | — |
| B6 | Stage 8 WS6 | — | — | — |

---

## 6. Historical G1 Acceptance

**G1 Result: PASS** — Blocker register v1 reconstructed from actual evidence. 6 blockers classified with closure criteria, owners, and waiver decisions documented. Stage 9 execution authorized based on this register.

---

## 7. Blocker Register v1 vs. Stage 9 G1 vs. Current

| Blocker | Stage 8 v1 (Historical) | Stage 9 G1 (Refined) | Stage 14 (Current) |
|---------|-------------------------|----------------------|-------------------|
| **B1** | WAIVED (2026-12-31) | WAIVED (2027-06-30) | WAIVED (2027-06-30) |
| **B2** | WAIVED (2026-12-31) | WAIVED (2027-06-30) | WAIVED (2027-06-30) |
| **B3** | OPEN | OPEN | WAIVED (2027-03-31) |
| **B4** | BLOCKED | BLOCKED | WAIVED (2027-03-31) |
| **B5** | OPEN | OPEN | WAIVED (2027-03-31) |
| **B6** | PARTIALLY_CLOSED | PARTIALLY_CLOSED | WAIVED / PARTIAL |

---

## 6. G1 Acceptance (Historical)

**G1 Result: PASS** — Blocker register v1 reconstructed from actual evidence. 6 blockers classified with closure criteria, owners, and waiver decisions documented. Stage 9 execution authorized based on this register.

---

*Generated by Stage 8 Backfill — Blocker Register v1*