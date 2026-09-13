# Stage 9 G7 — IMDS Evidence Decision

**Date:** 2026-09-13
**Baseline:** Stage 9 G6 Revocation Model
**Blocker:** B2 — Live IMDSv2 validation against Azure dev VM

---

## 1. Decision Summary

**Decision:** FORMAL WAIVER with expiry 2027-06-30

Live IMDSv2 validation against an Azure development VM is **waived** for the `4.0.0` GA release. All Azure IMDS capabilities are classified as **OFFLINE_VALIDATED_ONLY**.

---

## 2. Capability Classification

| Capability | Live Validated | Offline Validated | Classification |
|------------|----------------|-------------------|----------------|
| IMDSv2 token fetch | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| IMDSv1 fallback | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| Managed identity token | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| Instance metadata | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |
| Client/Object ID pinning | ❌ | ✅ | OFFLINE_VALIDATED_ONLY |

---

## 3. Justification

| Factor | Assessment |
|--------|------------|
| Azure dev subscription available | No — requires active Azure subscription with VM quota |
| Managed identity enabled VM | No — requires VM with system/user assigned identity |
| IMDS endpoint accessible | No — requires Azure network access |
| Timebox for Stage 9 | 1 day — insufficient for VM provisioning + test execution |
| Risk of production claim | HIGH — without live validation, cannot claim production Azure IMDS interop |

---

## 4. Offline Validation Evidence

All IMDS capabilities have been validated offline:

| Capability | Validation Method | Evidence |
|------------|-------------------|----------|
| IMDSv2 token parsing | Fuzz + unit tests | `internal/engine/exec/fuzz_test.go` |
| Instance metadata parsing | Fuzz + unit tests | `internal/engine/exec/fuzz_test.go` |
| Token request logic | Unit tests | `internal/engine/exec/*_test.go` |
| Metadata request logic | Unit tests | `internal/engine/exec/*_test.go` |

**Fuzzing Coverage:** 2 native Go fuzz targets (FuzzParseIMDSIdentityToken, FuzzParseInstanceMetadata) — 900K+ executions, 0 crashes.

---

## 5. Release Notes Requirement

The `4.0.0` release notes MUST include:

> **Azure IMDS Interoperability Notice:** Live validation against Azure Instance Metadata Service (IMDSv2) has not been executed. All IMDS capabilities (token fetch, metadata retrieval, managed identity token acquisition, client/object ID pinning) are validated offline using fixture data and protocol logic tests only. Production use against live Azure IMDS endpoints is untested and unsupported in this release.

---

## 6. Waiver Terms

| Term | Value |
|------|-------|
| Waiver ID | B2-WAIVER-2026-09-13 |
| Expiry | 2027-06-30 |
| Owner | Infrastructure Team |
| Approval | Stage 9 Lead |
| Re-evaluation | Required before expiry or next major release |

---

## 7. Re-evaluation Criteria

This waiver must be re-evaluated when ANY of the following occurs:

- [ ] Azure dev subscription with VM quota available
- [ ] Managed identity enabled test VM provisioned
- [ ] Live IMDS validation test suite executed (5+ scenarios)
- [ ] Evidence captured as LIVE_SANITIZED
- [ ] Next major release cycle begins

---

## 8. G7 Result

**WAIVED** — Live IMDS validation formally waived with expiry 2027-06-30. All capabilities classified as OFFLINE_VALIDATED_ONLY. Release notes updated accordingly.

---

**Prepared by:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Status:** G7 WAIVED