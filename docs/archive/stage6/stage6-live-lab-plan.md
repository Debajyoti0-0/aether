# Aether — Stage 6 Live-Lab Validation Architecture

**Baseline:** Stage 3 (3.4.0-stage3), HEAD `5efc8e2`
**Phase:** 2 of 9
**Classification:** DESIGN — No live execution authorized without explicit approval

---

## 1. Safety Boundary (Non-Negotiable)

Live testing MUST be limited to:

| Constraint | Enforcement |
|------------|-------------|
| **Owned infrastructure only** | Target list pre-approved; no third-party tenants |
| **Explicitly authorized lab tenants** | Tenant IDs documented; no production tenants |
| **Disposable test identities** | Test users created for validation; deleted after |
| **Controlled test accounts** | No real user credentials; service principals with minimal scope |
| **Non-production environments** | Dev/test tenants only; no prod data access |
| **Documented target scope** | Target list + protocols in this document |
| **Rate-limited operations** | Max 10 req/min per endpoint; circuit breaker at 5xx |
| **Auditable operations** | All requests logged with correlation IDs; no secrets in logs |

**No testing against:**
- Third-party tenants
- Public identities
- Systems without written authorization
- Production environments

---

## 2. Live-Lab Test Plan

### 2.1 Target Environment

| Environment | Type | Protocols | Authorization Status |
|-------------|------|-----------|---------------------|
| `lab-tenant-1.onmicrosoft.com` | Entra ID Dev Tenant | PRT→OAuth (MS-OAPX), WS-Trust, SAML, Device Code, CAE | **PENDING** — Requires tenant admin approval |
| `imds.azure.local` | Azure VM (dev subscription) | IMDSv2 Identity Token, Instance Metadata | **PENDING** — Requires VM provisioning |
| `teamserver.local:4433` | Local mTLS Teamserver | Protocol v2 (multiplex, events, replay) | **READY** — Self-hosted, full control |

### 2.2 Protocol & Feature Matrix

| Protocol | Feature Under Test | Required Credential State | Expected Behavior | Negative Cases |
|----------|-------------------|--------------------------|-------------------|----------------|
| **MS-OAPX (PRT)** | PRT→OAuth exchange | Valid PRT cookie + session key | Returns OAuth2 tokens (access, refresh, id_token) | Expired PRT, revoked PRT, wrong tenant, malformed cookie, missing session key, Token Protection binding mismatch |
| **WS-Trust** | Username/password → SAML → OAuth | Valid user@domain + password | Returns SAML assertion + OAuth tokens | Wrong password, expired password, federated domain (MEX), MFA challenge, SAML assertion replay |
| **SAML** | Assertion building/signing/stripping | Valid signing cert + key | Signed assertion verifies; stripped assertion accepted by vulnerable SP | Invalid signature, expired assertion, wrong audience, missing conditions |
| **Device Code** | Device authorization flow | Valid tenant + client_id | Returns device_code + user_code; polls to tokens | Expired device_code, denied authorization, poll timeout |
| **CAE** | Claims challenge handling | Valid refresh token + CAE challenge | Returns new access token satisfying claims | Unsupported claim, expired refresh token, revoked session |
| **IMDSv2** | Identity token + metadata | Azure VM with managed identity | Returns OAuth token for resource; metadata document | IMDSv1 fallback, token TTL expiry, network isolation |
| **Teamserver v2** | mTLS + multiplex + events | Valid operator cert (CA-issued) | Commands execute; events stream with cursor replay | Invalid cert, revoked cert, protocol version mismatch, frame too large, in-flight cap exceeded |

### 2.3 Data Collection Boundary

| Data Collected | Redaction Policy | Retention |
|----------------|------------------|-----------|
| Request/response headers (sanitized) | Remove `Authorization`, `Cookie`, `x-ms-RefreshTokenCredential`, `x-client-bound` | 30 days |
| Token metadata (type, expiry, scope) | No raw tokens; only type/lifetime/scope | 30 days |
| Certificate metadata (SAN, expiry, issuer) | No private keys; public cert OK | 30 days |
| Timing/latency (ms) | Full retention | 90 days |
| Error codes/classifications | Full retention | 90 days |
| Correlation IDs (RequestID, ActionID) | Full retention | 90 days |

**Never collect:**
- Raw bearer tokens (access, refresh, PRT cookie)
- Private keys
- User passwords
- Session keys (base64)
- Channel binding values (tls-unique)

### 2.4 Artifact Format

All live captures stored as JSON Lines (`.jsonl`) with schema:

```json
{
  "test_case_id": "prt-001",
  "protocol": "ms-oapx",
  "target": "lab-tenant-1.onmicrosoft.com",
  "timestamp": "2026-09-12T14:30:00Z",
  "tool_version": "3.4.0-stage3",
  "input_category": "valid_prt",
  "expected_result": "oauth_tokens",
  "actual_result": "oauth_tokens",
  "sanitized_request": { ... },
  "sanitized_response": { ... },
  "redaction_status": "sanitized",
  "evidence_hash": "sha256:...",
  "reproduction": { "command": "aether prt convert --prt-file=...", "env": "LAB" }
}
```

### 2.5 Cleanup Procedure

1. Revoke all test service principals / delete test users
2. Clear IMDS test VM (delete or deallocate)
3. Rotate teamserver CA if test certs issued from production CA
4. Purge raw logs; retain only sanitized `.jsonl` artifacts
5. Verify no secrets in git history (`git log --all --oneline --source --remotes -S "secret"`)

---

## 3. Evidence Classification

Every live artifact MUST be classified:

| Classification | Description | Release Artifact? |
|----------------|-------------|-------------------|
| `LIVE_RAW_RESTRICTED` | Raw capture with secrets; never committed; encrypted at rest | NO |
| `LIVE_SANITIZED` | Redacted per policy; structure preserved for verification | YES (in `artifacts/stage6/live-lab/`) |
| `CONTROLLED_SYNTHETIC` | Generated by test harness against mock server | YES (in `test/fixtures/`) |
| `DETERMINISTIC_FIXTURE` | Static test fixtures; no live component | YES (in `test/fixtures/`) |
| `DERIVED_REPORT` | Analysis/summary of live captures | YES (in `docs/`) |

**Rule:** Never label synthetic or fixture data as `LIVE_*`.

---

## 4. Redaction Rules (Deterministic)

| Field Pattern | Action | Example |
|---------------|--------|---------|
| `access_token` / `refresh_token` / `id_token` | Replace with `REDACTED_TOKEN_<type>_<hash>` | `REDACTED_ACCESS_a1b2c3` |
| `prt_cookie` / `x-ms-RefreshTokenCredential` | Replace with `REDACTED_PRT_COOKIE_<hash>` | `REDACTED_PRT_COOKIE_d4e5f6` |
| `session_key` (base64) | Replace with `REDACTED_SESSION_KEY_<hash>` | `REDACTED_SESSION_KEY_f7g8h9` |
| `client_bound` / `x-client-bound` / `tls-unique` | Replace with `REDACTED_BINDING_<hash>` | `REDACTED_BINDING_i0j1k2` |
| `password` / `client_secret` | Replace with `REDACTED_SECRET` | `REDACTED_SECRET` |
| `tenant_id` (GUID) | Keep first 8 chars + `...` | `12345678-...` |
| `user_id` / `object_id` / `device_id` | Keep first 8 chars + `...` | `abcd1234-...` |
| Certificate `private_key` | **REMOVE ENTIRE FIELD** | (field absent) |

**Hash:** First 6 chars of SHA-256 of original value (for correlation without exposure).

---

## 5. Expected Outcomes & Failure Semantics

### 5.1 Success Criteria (Per Protocol)

| Protocol | Minimum PASS Criteria |
|----------|----------------------|
| PRT→OAuth | Valid PRT → tokens returned; expired PRT → 400/401 classifiable; wrong tenant → 400/401 classifiable |
| WS-Trust | Valid creds → SAML + tokens; wrong password → classifiable error; federated domain → MEX discovery works |
| SAML | Signed assertion → verifies with public key; stripped → accepted by test SP (if applicable) |
| Device Code | Flow completes → tokens; timeout → classifiable |
| CAE | Challenge satisfied → new tokens; unsupported claim → classifiable error |
| IMDSv2 | Token fetched → identity token returned; metadata → parseable JSON |
| Teamserver | mTLS handshake → success; 100 multiplexed → all correlated; reconnect → cursor replay |

### 5.2 Failure Classification

| Class | Definition | Action |
|-------|------------|--------|
| `EXPECTED_NEGATIVE` | Negative case produced expected error class | PASS (test validates error handling) |
| `UNEXPECTED_ERROR` | Positive case failed with transport/5xx/timeout | INVESTIGATE (infrastructure vs code) |
| `PROTOCOL_DEVIATION` | Response doesn't match spec (unexpected field, wrong error code) | DOCUMENT (interop gap) |
| `UNAUTHORIZED` | 401/403 on expected-success case | CHECK CREDENTIALS (test setup issue) |
| `RATE_LIMITED` | 429/503 on test traffic | BACKOFF & RETRY (infrastructure limit) |
| `BLOCKED` | Target unavailable / auth not provisioned | BLOCKED (not PASS/FAIL) |

---

## 6. Reproducibility Requirements

| Requirement | Implementation |
|-------------|----------------|
| **Deterministic replay** | All live captures include `reproduction.command` + `reproduction.env` |
| **Version pinning** | `tool_version` in every artifact; binary hash recorded |
| **Environment snapshot** | Target endpoint, tenant ID (sanitized), protocol version recorded |
| **Seed control** | Where randomness used (nonces, IDs), seed recorded or deterministic derivation documented |
| **Artifact hashes** | SHA-256 of each `.jsonl` file in `artifacts/stage6/live-lab/checksums.txt` |

---

## 7. Authorization Workflow (Pre-Execution)

Before ANY live execution:

1. [ ] Target tenant provisioned and approved by security owner
2. [ ] Test identities created with minimal scopes (Directory.Read.All, User.Read)
3. [ ] Network egress allowlist configured (only target endpoints)
4. [ ] Redaction script reviewed and tested against sample capture
5. [ ] Evidence classification labels assigned to planned test cases
6. [ ] Rollback/cleanup procedure documented and tested
7. [ ] **Explicit written approval** from repository owner for each target

---

## 8. Current Status

| Item | Status |
|------|--------|
| Target tenant provisioned | **NOT STARTED** — Requires owner approval |
| Test identities created | **NOT STARTED** |
| Redaction script implemented | **NOT STARTED** — Design only |
| Evidence classification schema | **DEFINED** (this document) |
| Teamserver local test ready | **READY** (self-hosted) |
| IMDS test VM provisioned | **NOT STARTED** |
| Authorization obtained | **PENDING** |

---

## 9. Phase 2 Gate (B2) — PASS Criteria

**PASS only if:**
- [ ] Live-lab plan defines scope, authorization, safety, evidence handling, expected outcomes, failure semantics
- [ ] Redaction rules are deterministic and tested
- [ ] Evidence classification schema covers all planned captures
- [ ] Authorization workflow documented
- [ ] **No live execution occurs before explicit approval**

**This document satisfies B2.** Live execution is a separate Phase 3+ activity requiring explicit go/no-go.

---

## 10. Non-Goals for Stage 6

- Actual live captures (deferred to authorized execution window)
- Live PRT validation against real Entra ID (requires tenant approval)
- Production tenant testing (explicitly excluded)
- Automated CI integration for live tests (manual only for Stage 6)

---

## Sign-Off

**Architecture Designed:** 2026-09-12
**Baseline Commit:** `5efc8e2`
**Next Phase:** Phase 3 — PRT Validation Scope Documentation