# Stage 7 — Audit Trust-Root Lifecycle Documentation

**Date:** 2026-09-12
**Baseline:** `8e7d27d`
**Scope:** Staged RC (Internal Lab / Authorized Partner Validation)

---

## 1. Current Implementation

### 1.1 Key Generation
- **Algorithm:** Ed25519 (via `crypto/ed25519`)
- **Generated:** On workspace creation (`workspace.Create()`)
- **Stored:** In vault meta bucket as raw seed (32 bytes)
- **Format:** Raw 32-byte seed (not PEM/ASN.1)

### 1.2 Key Storage
- **Location:** BoltDB meta bucket (`meta` bucket, key `audit_key_seed`)
- **Format:** Raw 32-byte seed (binary)
- **Encryption:** None (plaintext in workspace file)
- **Access:** Only via `workspace.Open()` with valid passphrase

### 1.3 Key Usage
- **Signing:** `AuditLog.Append()` signs each entry with Ed25519
- **Verification:** `AuditLog.Verify()` validates entire chain
- **Key Derivation:** Single key for entire workspace lifetime

---

## 2. Trust-Root Model

### 2.1 Trust Anchor
- **Root of Trust:** The Ed25519 seed in the workspace file
- **Trust Assumption:** Operator possesses the workspace file + passphrase
- **Verification:** Anyone with workspace file + passphrase can verify audit chain
- **No External Anchors:** No CAs, no transparency logs, no external trust roots

### 2.2 Trust Properties
| Property | Status | Notes |
|----------|--------|-------|
| **Tamper-Evident** | ✅ YES | Ed25519 chain — any modification detectable |
| **Authenticity** | ⚠️ CONDITIONAL | Only if workspace file uncompromised |
| **Non-Repudiation** | ⚠️ CONDITIONAL | Only if key not extracted |
| **Key Provenance** | ❌ NONE | No external attestation of key origin |

---

## 3. Threat Model (Staged RC)

| Threat | Likelihood | Impact | Current Mitigation |
|--------|------------|--------|-------------------|
| Workspace file theft | Medium (file access) | High — forge audit entries | Operator controls workspace file |
| Passphrase brute-force | Low (strong passphrase) | High | Enforce strong passphrase policy |
| Key extraction from memory | Low (Go runtime) | High | Go runtime protections |
| Backup leakage | Medium (backup copy) | High | Operator controls backups |
| No key rotation | N/A (single key) | Medium | Documented limitation |
| No external attestation | N/A (self-contained) | Medium | Documented limitation |

---

## 4. Limitations (Staged RC)

| Limitation | Impact | Acceptance |
|------------|--------|------------|
| **Plaintext key in workspace** | Key extractable if workspace file stolen | ✅ ACCEPTED — Operator controls file |
| **No key rotation** | Compromise = permanent | ✅ ACCEPTED — Workspace lifetime |
| **No key escrow** | No recovery if passphrase lost | ✅ ACCEPTED — Operator responsibility |
| **No external attestation** | No third-party trust anchor | ✅ ACCEPTED — Self-contained |
| **No HSM/KMS support** | No hardware key protection | ✅ ACCEPTED — Lab environment |

---

## 5. Production Requirements (Not in Staged RC)

| Requirement | Description |
|-------------|-------------|
| **External key custody** | HSM, KMS, or age-encrypted key file |
| **Key rotation** | Periodic rotation with backward verification |
| **Key escrow** | Shamir split or threshold for recovery |
| **External attestation** | Sigstore transparency log, or CA-signed attestation |
| **Hardware key support** | HSM/PKCS#11 integration |
| **Compromise recovery** | Key revocation + re-signing procedure |

---

## 6. Staged RC Acceptance

**Status:** ACCEPTED WITH LIMITATIONS

**Justification:** For internal lab / authorized partner validation:
- Workspace file never leaves operator's machine
- Operator has full control over workspace file
- No multi-operator sharing of audit keys
- Tamper-evidence is the primary guarantee (not authenticity)

**Documentation Requirement:** All limitations must be explicitly stated in:
- Release notes
- Operator documentation (`docs/operator-guide.md`)
- Security documentation (`SECURITY.md`)

---

## 7. Migration Path to Production

| Phase | Change |
|-------|--------|
| 1 | Add key derivation: separate signing key from workspace seed |
| 2 | Add key rotation: dual-signing during transition |
| 3 | Add HSM/PKCS#11 interface for external key custody |
| 4 | Add Sigstore transparency log integration |
| 5 | Add key escrow (Shamir/threshold) |
| 5 | Add CA-signed key attestation |

---

## 7. Sign-Off

**Status:** DOCUMENTED — Accepted for Staged RC with explicit limitations
**Documented:** 2026-09-12
**Review:** 2026-12-12 (or before production migration)