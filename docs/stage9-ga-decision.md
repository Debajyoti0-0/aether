# Stage 9 G10 — Final GA Decision

**Date:** 2026-09-13
**Baseline:** Stage 9 G9 Adversarial Audit
**Repository:** `C:\dev\aether` @ `ffda1dc`
**Target Version:** `4.0.0` or `4.0.0-rc1`

---

## 1. Blocker Resolution Summary

| Blocker | ID | Status | Resolution |
|---------|----|--------|------------|
| Live Entra ID validation | B1 | **WAIVED** | Expiry 2027-06-30; OFFLINE_VALIDATED_ONLY |
| Live IMDS validation | B2 | **WAIVED** | Expiry 2027-06-30; OFFLINE_VALIDATED_ONLY |
| Release pipeline (GoReleaser + CI) | B3 | **PARTIAL** | Local build PASS; CI execution required |
| EV Authenticode certificate | B4 | **BLOCKED** | Implementation ready; cert procurement pending |
| Audit key HSM/KMS custody | B5 | **PARTIAL** | Local encrypted provider ready; HSM/KMS backends needed for production |
| Idempotency hardening | B6 | **PARTIALLY_CLOSED** | 4/4 integration tests PASS; 10+ scenarios covered |

---

## 2. GA Readiness Assessment

### 2.1 Must-Have Criteria (from Stage 8 Handoff)

| Criterion | Status | Evidence |
|-----------|--------|----------|
| All 4 production blockers resolved | ❌ NO | B4 BLOCKED; B3/B5 PARTIAL |
| Live interoperability evidence | ❌ NO | B1/B2 WAIVED |
| CI/CD pipeline green on all platforms | ⚠️ PARTIAL | Local PASS; CI not executed |
| EV Authenticode certificate integrated | ❌ NO | B4 BLOCKED |
| Audit key in HSM/KMS with rotation | ❌ NO | B5 PARTIAL |
| Full tamper test suite in CI | ❌ NO | CI not executed |
| SLSA provenance generated | ❌ NO | CI not executed |
| SBOM/checksums/manifest signed | ❌ NO | CI not executed |
| Version `4.0.0` tagged | ❌ NO | Not tagged |
| Verification by independent party | ⚠️ PARTIAL | Local verification documented |

### 2.2 Release Decision Matrix

| Context | Decision | Rationale |
|---------|----------|-----------|
| **Expert lab / Authorized internal team** | **APPROVED_WITH_LIMITATIONS** | Core functionality verified; waivers documented; suitable for controlled testing |
| **Controlled enterprise** | **CONDITIONAL** | Requires HSM/KMS deployment and EV cert procurement |
| **Public release** | **BLOCKED** | EV cert not procured; CI pipeline not executed; HSM/KMS not available |
| **Production infrastructure** | **NOT_READY** | Requires HSM/KMS, EV cert, CI execution |

---

## 3. Recommended Release Channel

**Decision:** **`4.0.0-rc1` (Production-Limited Release Candidate)**

### Rationale:
1. **Core release infrastructure functional** — GoReleaser, signing scripts, verification procedures all implemented and tested locally
2. **Critical trust chain components designed** — cosign keyless, Authenticode, KeyProvider, revocation
3. **Waivers formally documented** — B1/B2 with expiry 2027-06-30
4. **Limitations explicitly stated** — OFFLINE_VALIDATED_ONLY for Entra/IMDS; no EV cert; local key provider
4. **Production blockers remain** — EV cert procurement, HSM/KMS implementation, CI execution

### Version: `4.0.0-rc1`
### Channel: Production-Limited RC
### Tag: `v4.0.0-rc1`

---

## 4. Path to `4.0.0` GA

| Step | Requirement | Owner | Target |
|------|-------------|-------|--------|
| 1 | Procure EV Authenticode certificate | Security/Platform | 2026-Q4 |
| 2 | Implement Azure Key Vault or AWS KMS provider | Security | 2026-Q4 |
| 3 | Execute CI pipeline on `v4.0.0-rc1` tag | Platform | 2026-Q4 |
| 4 | Verify all 9 tamper tests in CI | Platform | 2026-Q4 |
| 5 | Independent verification of CI artifacts | External | 2026-Q4 |
| 6 | Fix CI configuration gaps (action pinning) | Platform | 2026-Q4 |
| 7 | Tag `v4.0.0` after all above complete | Release Lead | 2027-Q1 |

---

## 5. Release Notes for `4.0.0-rc1`

### Supported Capabilities (Verified)
- ✅ Teamserver mTLS with CA hierarchy
- ✅ Spine-routed action execution with audit
- ✅ Request idempotency (replay safety, crash recovery)
- ✅ Native fuzzing (19 targets, 0 crashes)
- ✅ Audit chain (Ed25519, tamper-evident)
- ✅ Reproducible builds (`-trimpath`, deterministic)
- ✅ Local encrypted audit key provider with rotation

### Known Limitations (Explicit)
- ⚠️ **Entra ID interoperability**: OFFLINE_VALIDATED_ONLY — Live PRT→OAuth, WS-Trust, Device Code, CAE, Token Protection, Conditional Access not validated against live tenant
- ⚠️ **Azure IMDS interoperability**: OFFLINE_VALIDATED_ONLY — Live IMDSv2 token fetch and metadata not validated against Azure VM
- ⚠️ **Windows Authenticode signing**: NOT SIGNED — EV certificate procurement pending; Windows binary released unsigned
- ⚠️ **Audit key custody**: Local encrypted file provider only — HSM/KMS backends (Azure Key Vault, AWS KMS, YubiHSM) not implemented; production deployment requires HSM/KMS
- ⚠️ **Revocation**: File-based only — No OCSP/CRL endpoints; no cross-instance propagation
- ⚠️ **CI/CD pipeline**: Not executed in CI — Local GoReleaser build verified; GitHub Actions workflow configured but not run
- ⚠️ **Multi-host deployment**: Single-host only — No Raft/etcd clustering
- ⚠️ **macOS/ARM64 runtime**: Build-verified only — No runtime test execution

---

## 6. G10 Result

**FINAL DECISION:** `4.0.0-rc1` Production-Limited Release Candidate

**NOT `4.0.0` GA** — Production blockers B4 (EV cert), B3 (CI execution), B5 (HSM/KMS) remain unresolved.

**Next Release:** `4.0.0` GA after all production blockers resolved (target 2027-Q1).

---

**Prepared by:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Status:** G10 COMPLETE — Release decision documented