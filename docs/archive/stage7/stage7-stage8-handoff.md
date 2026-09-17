# Stage 7 — Stage 8 Handoff Document

**Date:** 2026-09-12
**From:** Stage 7 (`3.8.0-stage7`, commit `f40e06f`)
**To:** Stage 8 (`v4.0.0` Release Gate — Production Ready)
**Repository:** `C:\dev\aether`

---

## 1. Handoff Summary

Stage 7 has completed all 8 gates (G26–G40) and produced a **Signed Release Candidate** (`3.8.0-stage7`) ready for internal lab / authorized partner validation. Stage 8 must resolve 4 production blockers to achieve `v4.0.0` Production Ready status.

**Current State:** `3.8.0-stage7` — Staged RC (Internal Lab / Authorized Partner)
**Target State:** `v4.0.0` — Production Ready

---

## 2. Production Blockers (Must Resolve for Stage 8 Entry)

| Blocker | Category | Description | Owner | Target |
|---------|----------|-------------|-------|--------|
| **B1** | Interoperability | Live Entra ID lab tenant + IMDSv2 Azure VM | Security Team | Stage 8 entry |
| **B2** | Release Engineering | Goreleaser + GitHub Actions release pipeline | Platform Team | Stage 8 entry |
| **B3** | Release Engineering | EV Authenticode certificate procurement | Security/Platform | Stage 8 entry |
| **B4** | Security | Audit key HSM/KMS custody integration | Security Team | Stage 8 entry |

**Stage 8 cannot grant stable-release status if any blocker remains unresolved.**

---

## 3. Current Capability State (What Works)

| Capability | Status | Evidence |
|------------|--------|----------|
| Local Teamserver mTLS | ✅ VALIDATED | 10/10 scenarios PASS (LIVE_SANITIZED) |
| PRT → OAuth (MS-OAPX) | ✅ OFFLINE VALIDATED | Parser, proof, exchange logic; live BLOCKED |
| WS-Trust / SAML / Device Code | ✅ OFFLINE VALIDATED | Parser, signing, exchange; live BLOCKED |
| Device Code Flow | ✅ OFFLINE VALIDATED | Flow logic implemented; live BLOCKED |
| CAE Claims Challenge | ✅ OFFLINE VALIDATED | Logic implemented; live BLOCKED |
| IMDSv2 Identity Token | ✅ OFFLINE VALIDATED | Parser, token fetch; live BLOCKED |
| Native Fuzzing | ✅ IMPLEMENTED | 19 targets, 10.8M execs, 0 crashes |
| Request Idempotency | ✅ IMPLEMENTED | RequestID → ActionID ledger (pending/completed/failed) |
| Audit Chain | ✅ VALIDATED | Ed25519 chain, tamper-evident |
| Crash Recovery | ✅ VALIDATED | bbolt COW + ungraceful handoff test |
| mTLS CA Hierarchy | ✅ VALIDATED | serve cert init/issue/revoke, RequireAndVerifyClientCert |
| Request Idempotency Ledger | ✅ IMPLEMENTED | RequestID → ActionID (pending/completed/failed) |
| Revocation Model | ✅ DOCUMENTED | File-based; production requirements defined |
| Audit Trust-Root | ✅ DOCUMENTED | Ed25519 seed in workspace; limitations documented |
| Version Truth | ✅ CONSISTENT | `3.8.0-stage7` everywhere |
| Reproducible Builds | ✅ VERIFIED | `-s -w`, fixed ldflags, deterministic modules |
| SBOM/Checksums/Manifest | ✅ DESIGNED | CycloneDX, SHA-256, JSON schema |
| Signing Design | ✅ DESIGNED | cosign keyless + Authenticode; 9 tamper tests |
| Release Verification | ✅ DOCUMENTED | `verify-release.sh`, `verify-release.ps1`, `VERIFY.md` |
| CI Pipeline Design | ✅ DESIGNED | GitHub Actions workflow (build → sign → release) |
| Native Fuzzing | ✅ 19 TARGETS | 10.8M execs, 1,722 interesting, 0 crashes |

---

## 3. Known Limitations (Must Be in Release Notes)

| Limitation | Impact | Staged RC Mitigation |
|------------|--------|---------------------|
| Live Entra ID validation blocked | Cannot claim live PRT/WS-Trust/Device Code/CAE interop | Claims restricted to "offline validated" |
| Live IMDSv2 validation blocked | Cannot claim live Azure IMDS interop | Claims restricted to "offline validated" |
| No EV Authenticode cert | Windows binary not authenticode-signed | Manual build + checksums acceptable for RC |
| No automated release pipeline | Manual build + sign required | Documented; acceptable for RC |
| Audit key plaintext in workspace | Key extractable if workspace stolen | Documented; operator controls file |
| File-based revocation only | No auto-propagation; single-host | Documented; acceptable for lab |
| No OCSP/CRL for operators | No standard revocation check | Documented; file-based only |
| No request idempotency (pre-fix) | ✅ FIXED | Idempotency ledger implemented |
| No multi-host deployment | Single-host only | Documented; lab scope |
| No macOS/ARM64 runtime tests | Build-verified only | Documented as BUILD-VERIFIED only |
| OneDrive repo relocated | ✅ DONE | Now at `C:\dev\aether` |

---

## 4. Residual Risks (Staged RC)

| Risk | Classification | Mitigation |
|------|----------------|------------|
| Audit key plaintext in workspace | MITIGATED (doc) | Operator controls workspace file |
| No OCSP/CRL for operator revocation | DEFERRED | File-based revocation only |
| No per-event signatures | DEFERRED | Audit chain is trust anchor |
| No plugin supply-chain signing | DEFERRED | Plugins not in release surface |
| OneDrive repo location | ✅ RESOLVED | Relocated to `C:\dev\aether` |
| No live external interoperability | WAIVED | Documented as BLOCKED with waivers |

---

## 5. Stage 8 Scope (`v4.0.0` Release Gate)

### 4.1 Must-Have (Blocking)
1. **Execute Live Interoperability Validation** (G4 completion)
   - Provision authorized Entra ID lab tenant
   - Execute PRT→OAuth, WS-Trust, Device Code, CAE scenarios
   - Provision Azure dev VM for IMDSv2 validation
   - Update `docs/stage7-interoperability-results.md` with live evidence

2. **Implement CI/CD Release Pipeline** (G5 implementation)
   - Configure goreleaser for multi-platform builds
   - GitHub Actions workflow: build → sign → release
   - SLSA provenance generation
   - Artifact upload with signatures/checksums/SBOM

3. **Procure & Integrate EV Authenticode Certificate**
   - Purchase EV code signing certificate
   - Integrate with Windows signing job (signtool)
   - Verify SmartScreen reputation

4. **Implement Audit Key External Custody**
   - Design HSM/KMS integration (AWS KMS, Azure Key Vault, or HSM)
   - Implement key derivation from external root
   - Add key rotation capability

### 4.2 Should-Have (Non-Blocking for Stable Release)
- macOS/ARM64 runtime validation in CI matrix
- Dependabot/Renovate for dependency updates
- License compliance check (go-licenses)
- SLSA Level 3 provenance
- Public transparency log for cosign signatures

### 4.3 Nice-to-Have (Post-v4.0.0)
- Multi-host deployment (Raft/etcd)
- OCSP/CRL revocation endpoint
- Hardware key support (PKCS#11)
- Plugin supply-chain signing
- Metrics/observability stack

---

## 6. Stage 8 Entry Criteria (G8 Re-evaluation)

| Criterion | Required | Status |
|-----------|----------|--------|
| All 4 production blockers resolved | ✅ MANDATORY | ❌ NOT MET |
| Live interoperability evidence (Entra + IMDS) | ✅ MANDATORY | ❌ BLOCKED |
| CI/CD pipeline green on all platforms | ✅ MANDATORY | ❌ NOT IMPLEMENTED |
| EV Authenticode certificate integrated | ✅ MANDATORY | ❌ NOT PROCURED |
| Audit key in HSM/KMS | ✅ MANDATORY | ❌ NOT IMPLEMENTED |
| Version `4.0.0` tagged | ✅ MANDATORY | ❌ PENDING |
| Full tamper test suite in CI | ✅ MANDATORY | ❌ NOT IMPLEMENTED |
| SLSA provenance generated | ✅ MANDATORY | ❌ NOT IMPLEMENTED |

**Stage 8 Entry: BLOCKED** — All 4 production blockers must be resolved first.

---

## 7. Stage 8 Exit Criteria (v4.0.0 Release Gate)

| Criterion | Target |
|-----------|--------|
| All 4 production blockers resolved | ✅ |
| Live interoperability evidence for Entra ID + IMDS | ✅ |
| CI/CD pipeline green on Windows/Linux/macOS × amd64/arm64 | ✅ |
| EV Authenticode certificate integrated and verified | ✅ |
| Audit key in HSM/KMS with rotation | ✅ |
| Full tamper test suite in CI (all 9 PASS) | ✅ |
| SLSA provenance generated and published | ✅ |
| SBOM/checksums/manifest signed and verified | ✅ |
| Version `4.0.0` tagged and released | ✅ |
| Release notes with limitations resolved | ✅ |
| Verification procedure tested by independent party | ✅ |

---

## 7. Sign-Off

**Handoff Prepared By:** Stage 7 Automated Execution
**Date:** 2026-09-12
**Source Commit:** `f40e06f` (`3.8.0-stage7`)
**Target Stage:** Stage 8 (`v4.0.0` Release Gate)
**Handoff Status:** **READY FOR STAGE 8 EXECUTION** (pending blocker resolution)

**Stage 7 Lead:** Automated Execution
**Stage 8 Owner:** [To be assigned]
**Next Review:** Upon blocker resolution