# Stage 9 G7 — Entra ID Evidence Decision

**Date:** 2026-09-13
**Baseline:** Stage 9 G6 Revocation Model
**Blockers:** B1 — Live Entra ID lab tenant validation

---

## 1. Decision Summary

**Decision:** FORMAL WAIVER with expiry 2027-06-30

Live Entra ID validation against an authorized tenant is **waived** for the `4.0.0` GA release. All Entra ID interoperability capabilities are classified as **OFFLINE_VALIDATED_ONLY**.

---

## 2. Capability Classification

| Capability | Live Validated | Offline Validated | Classification |
|------------|----------------|-------------------|----------------|
| PRT → OAuth (MS-OAPX) | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| WS-Trust / SAML relay | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| Device Code Flow | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| CAE Claims Challenge | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| Token Protection bypass | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| Conditional Access eval | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |

---

## 3. Justification

| Factor | Assessment |
|--------|------------|
| Authorized tenant available | No — requires dedicated Entra ID lab tenant provisioning |
| Disposable identities available | No — requires tenant admin to create test users/SPs |
| Non-production resources | No — no isolated test environment |
| Timebox for Stage 9 | 1 day — insufficient for tenant provisioning + test execution |
| Risk of production claim | HIGH — without live validation, cannot claim production Entra interop |

---

## 4. Offline Validation Evidence

All Entra ID capabilities have been validated offline:

| Capability | Validation Method | Evidence |
|------------|-------------------|----------|
| PRT parsing | Unit tests with fixture PRTs | `internal/engine/token/*_test.go` |
| MS-OAPX exchange | Protocol logic tests | `internal/protocol/msoapx/*_test.go` |
| WS-Trust RSTR parsing | Fuzz + unit tests | `internal/protocol/wstrust/*_test.go` |
| SAML assertion parsing | Fuzz + unit tests | `internal/protocol/saml/*_test.go` |
| Device Code flow | Flow logic tests | `internal/engine/token/*_test.go` |
| CAE challenge handling | Logic tests | `internal/protocol/oauth2/*_test.go` |
| Token Protection | Channel binding injection tests | `internal/protocol/msoapx/*_test.go` |

**Fuzzing Coverage:** 19 native Go fuzz targets including PRT, MS-OAPX, WS-Trust, SAML parsing — 10.8M+ executions, 0 crashes.

---

## 5. Release Notes Requirement

The `4.0.0` release notes MUST include:

> **Entra ID Interoperability Notice:** Live validation against Microsoft Entra ID has not been executed. All Entra ID capabilities (PRT→OAuth, WS-Trust, Device Code, CAE, Token Protection, Conditional Access) are validated offline using fixture data and protocol logic tests only. Production use against live Entra ID tenants is untested and unsupported in this release.

---

## 6. Waiver Terms

| Term | Value |
|------|-------|
| Waiver ID | B1-WAIVER-2026-09-13 |
| Expiry | 2027-06-30 |
| Owner | Security Team |
| Approval | Stage 9 Lead |
| Re-evaluation | Required before expiry or next major release |

---

## 7. Re-evaluation Criteria

This waiver must be re-evaluated when ANY of the following occurs:

- [ ] Authorized Entra ID lab tenant provisioned
- [ ] Disposable test identities created
- [ ] Live validation test suite executed (10+ scenarios)
- [ ] Evidence captured as LIVE_SANITIZED
- [ ] Next major release cycle begins

---

## 8. G7 Result

**WAIVED** — Live Entra ID validation formally waived with expiry 2027-06-30. All capabilities classified as OFFLINE_VALIDATED_ONLY. Release notes updated accordingly.

---

**Prepared by:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Status:** G7 WAIVED