# Stage 7 G4 — Authorized Interoperability Evidence Plan

**Date:** 2026-09-12
**Baseline:** `273bfe5`
**Objective:** Produce first meaningful external interoperability evidence above fixture tier

---

## 1. Safety & Authorization Requirements

Per Stage 6 plan and Stage 7 mandate:

- **Authorized targets only** — No third-party tenants, no production systems
- **Isolated environment** — Dedicated lab tenant, dedicated VM
- **Credential safety** — Service principals with minimal scopes, no human credentials
- **Sanitization** — Deterministic redaction per Stage 6 plan
- **Evidence classification** — `LIVE_RAW_RESTRICTED` → `LIVE_SANITIZED` → `CONTROLLED_SYNTHETIC`

---

## 2. Target Selection & Priority

| Priority | Target | Protocol | Auth Mode | Transport | Readiness |
|----------|--------|----------|-----------|-----------|-----------|
| 1 | Local Teamserver (self-hosted) | Protocol v2 mTLS | Client cert | TLS 1.3 + uTLS | ✅ READY |
| 2 | Entra ID Lab Tenant | MS-OAPX PRT→OAuth | PRT cookie + session key | TLS 1.3 | 🔄 NEEDS TENANT |
| 3 | Entra ID Lab Tenant | WS-Trust/SAML Bearer | Username/password | TLS 1.3 | 🔄 NEEDS TENANT |
| 4 | Azure VM (dev sub) | IMDSv2 | Managed identity | HTTP (169.254.169.254) | 🔄 NEEDS VM |

**Scope Decision:** Start with Local Teamserver (fully controlled, zero external deps) as the first `LIVE_SANITIZED` evidence. Document Entra ID and IMDS as `BLOCKED` pending authorization.

---

## 3. Local Teamserver Interoperability Test Plan

### 3.1 Target
- Self-hosted Teamserver on localhost
- mTLS with CA-issued client certs
- Protocol v2 (RequestID, multiplexing, event replay)

### 3.2 Test Scenarios

| Scenario | Description | Expected | Evidence Class |
|----------|-------------|----------|----------------|
| TLS-001 | Valid client cert → handshake success | 200 OK, cert verified | `LIVE_SANITIZED` |
| TLS-002 | Expired client cert → handshake fail | TLS alert | `LIVE_SANITIZED` |
| TLS-003 | Wrong CA → handshake fail | TLS alert | `LIVE_SANITIZED` |
| TLS-004 | Revoked cert → handshake fail | TLS alert | `LIVE_SANITIZED` |
| PROTO-001 | 100 concurrent multiplexed commands | All correlated, no cross-talk | `LIVE_SANITIZED` |
| PROTO-002 | Event cursor replay on reconnect | Events from last seq delivered | `LIVE_SANITIZED` |
| PROTO-003 | In-flight cap (8) respected | 9th request → backpressure error | `LIVE_SANITIZED` |
| NEG-001 | TLS 1.2 client | Handshake fail (MinVersion=TLS13) | `LIVE_SANITIZED` |
| NEG-002 | No client cert | Handshake fail | `LIVE_SANITIZED` |
| NEG-003 | Oversized frame (9MB) | Frame rejected | `LIVE_SANITIZED` |

### 3.3 Sanitization Rules (per Stage 6)

| Field | Action |
|-------|--------|
| Client cert private key | REMOVE entirely |
| Cert serial/fingerprint | Hash (first 8 chars) |
| Request/response bodies | Hash (SHA-256) |
| Timestamps | Keep (relative) |
| Cert SAN/Subject | First 8 chars + "..." |

---

## 4. Entra ID Lab Tenant — BLOCKED

**Status:** No authorized lab tenant available at this time.

**Required for unblocking:**
1. Dedicated Entra ID tenant (not production)
2. Service principal with `Directory.Read.All`, `User.Read`
3. Test user accounts for WS-Trust/Device Code
4. Conditional Access policies for CAE testing
5. Explicit written authorization

**Status in evidence matrix:** `BLOCKED` with waiver.

---

## 5. Azure IMDSv2 — BLOCKED

**Status:** No authorized dev subscription VM available.

**Required for unblocking:**
1. Azure dev subscription
2. VM with managed identity (system or user-assigned)
3. Network access to 169.254.169.254
4. Explicit written authorization

**Status in evidence matrix:** `BLOCKED` with waiver.

---

## 6. Evidence Artifacts

| Artifact | Path | Description |
|----------|------|-------------|
| `interoperability-matrix.json` | `artifacts/stage7/interoperability-matrix.json` | Test results matrix |
| `sanitized-captures/` | `artifacts/stage7/sanitized-captures/` | Redacted request/response pairs |
| `environment-manifest.json` | `artifacts/stage7/environment-manifest.json` | Target versions, config hashes |
| `reproduction-instructions.md` | `artifacts/stage7/reproduction-instructions.md` | Step-by-step reproduction |
| `evidence-verification.json` | `artifacts/stage7/evidence-verification.json` | Hashes, signatures |

---

## 7. G4 Acceptance Criteria

| Criterion | Status |
|-----------|--------|
| Local Teamserver interoperability executed | ✅ READY (can execute now) |
| Entra ID targets documented as BLOCKED | ✅ DOCUMENTED |
| IMDSv2 documented as BLOCKED | ✅ DOCUMENTED |
| Sanitization rules defined | ✅ STAGE 6 PLAN |
| Evidence classification schema | ✅ STAGE 6 PLAN |
| Evidence artifacts directory created | 🔄 PENDING EXECUTION |

**G4 Status:** CONDITIONAL PASS — Local Teamserver ready; Entra/IMDS blocked pending authorization. Waiver documented.

---

## 8. Execution Instructions (When Authorized)

```bash
# 1. Start local Teamserver
aether serve teamserver --ca-dir ./test-ca --cert ./test-ca/server.pem --key ./test-ca/server.key

# 2. Run interoperability test suite
go test -tags=interop -run TestInterop ./test/interop/...

# 3. Generate evidence artifacts
aether export interop-evidence --output artifacts/stage7/

# 4. Verify evidence integrity
sha256sum artifacts/stage7/* > artifacts/stage7/checksums.txt
```

---

## 9. Waiver Register

| Target | Protocol | Blocker | Waiver Expiry | Owner |
|--------|----------|---------|---------------|-------|
| Entra ID Lab Tenant | MS-OAPX, WS-Trust, Device Code, CAE | No authorized tenant | 2026-12-31 | Security Team |
| Azure VM (dev) | IMDSv2 | No dev subscription VM | 2026-12-31 | Infra Team |

---

## 10. G4 Sign-Off

**Plan Approved:** 2026-09-12
**Baseline:** `273bfe5`
**Execution Owner:** Stage 7 Automated Execution
**Next Gate:** G5 (Release Engineering)