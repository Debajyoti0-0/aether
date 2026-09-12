# Aether — Stage 6 PRT Validation Scope Documentation

**Baseline:** Stage 3 (3.4.0-stage3), HEAD `82b86ce`
**Phase:** 3 of 9
**Purpose:** Explicitly define what Aether's PRT module validates, what it cannot validate, and epistemic boundaries

---

## 1. What Aether's PRT Module Implements

### 1.1 Code Paths

| Component | File | Responsibility |
|-----------|------|----------------|
| `PRTConverter` | `internal/engine/token/prt.go` | Orchestrates PRT→OAuth exchange; validates PRT structure |
| `msoapx.Client` | `internal/protocol/msoapx/prt.go` | Implements MS-OAPX browser SSO flow; HTTP exchange |
| `LoadPRT` / `ParsePRT` | `internal/engine/token/prt.go` | JSON parsing, base64 normalization, structural validation |
| `validatePRT` | `internal/engine/token/prt.go` | Pre-exchange checks (cookie, tenant, session key) |
| `Exchange` | `internal/protocol/msoapx/prt.go` | Builds form, computes proof, POSTs to Entra token endpoint |

### 1.2 PRT Data Model (`internal/types/token.go`)

```go
type PRT struct {
    Cookie      string  // x-ms-RefreshTokenCredential value
    DeviceID    string  // Device identifier
    TenantID    string  // Tenant GUID or domain
    UserID      string  // User object ID
    SessionKey  string  // Base64-encoded session key (proof-of-possession)
    DerivedKey  string  // Optional derived key
    Context     string  // Optional PRT context (for nonce derivation)
}
```

### 1.3 MS-OAPX Exchange Flow

1. **Input Validation** (`validatePRT`): Cookie non-empty, TenantID present, SessionKey valid base64 ≥16 bytes
2. **Session Key Decode**: Base64 std → raw URL fallback
3. **Nonce Derivation**: SHA-256(Context)[:16] if Context present; else random 16 bytes
4. **Proof Computation**: HMAC-SHA256(SessionKey, Nonce || Context) → base64
5. **Form POST**: `grant_type=urn:ietf:params:oauth:grant-type:prt_sso`, `prt_cookie` header, `x-ms-Prid` proof header
6. **Token Protection**: Optional `x-client-bound` header with tls-unique binding
7. **Response Parse**: JSON → `OAuthTokens` (access_token, refresh_token, id_token, expires_in, token_type)

---

## 2. Validation Matrix — Epistemic Separation

| Scenario | Check Performed | Authority | Evidence Type | Aether Claim |
|----------|----------------|-----------|---------------|--------------|
| **Malformed token** | JSON parse, required fields | **Offline** (structural) | Deterministic | "Structurally invalid" |
| **Wrong token type** | Not applicable (PRT-only input) | — | — | N/A |
| **Invalid session key encoding** | Base64 decode, length ≥16 | **Offline** (cryptographic) | Deterministic | "Session key malformed" |
| **Empty cookie** | `PRT.IsZero()` check | **Offline** (structural) | Deterministic | "PRT cookie empty" |
| **Missing tenant_id** | `validatePRT` check | **Offline** (structural) | Deterministic | "Tenant ID required" |
| **Expired token** | Not checked offline | **Live** (Entra) | Live | "Exchange failed: token expired" (if Entra returns AADSTS50173) |
| **Wrong audience** | Not checked (resource is request param) | **Live** (Entra) | Live | "Exchange failed: invalid resource" |
| **Wrong issuer** | Not checked (fixed to login.microsoftonline.com) | **Live** (Entra) | Live | N/A (endpoint fixed) |
| **Wrong tenant** | TenantID from PRT used in endpoint | **Live** (Entra) | Live | "Exchange failed: tenant not found" (AADSTS90002) |
| **Valid controlled-lab token** | Full exchange | **Live** (Entra) | Live | "Tokens returned for resource X" |
| **Revoked/invalidated state** | Not independently checked | **Live** (Entra) | Live | **NO CLAIM** — "No revocation status independently established" |
| **Service acceptance** | Token returned by Entra | **Live** (Entra) | Live | "Entra accepted exchange for resource X" |
| **Token Protection bypass** | Binding header injected if provided | **Offline** (construction) + **Live** (Entra accepts) | Live | "Binding header presented; acceptance = Entra behavior" |

---

## 3. What Aether Explicitly Does NOT Claim

| Claim | Reason |
|-------|--------|
| "This PRT is not revoked" | No independent revocation check (no access to Entra revocation list, no PRT-specific revocation API) |
| "This PRT is valid for all resources" | Resource is per-request; exchange is resource-scoped |
| "This PRT proves device compliance" | Device compliance is evaluated by Entra Conditional Access; Aether only presents PRT |
| "This PRT proves MFA satisfaction" | MFA state encoded in PRT context/claims; Aether does not parse/verify |
| "Token Protection is bypassed" | Binding header is *presented*; whether Entra accepts it is Entra's decision |
| "Session key proves device possession" | Session key proves *knowledge* of key; device binding is Entra's assessment |
| "Refresh token from PRT exchange is long-lived" | Refresh token lifetime controlled by Entra policy; not guaranteed |

---

## 4. Offline vs Live Validation Claims

### Offline (Deterministic, No Network)
- JSON structure validity
- Required field presence
- Base64 encoding correctness
- Session key length (≥16 bytes)
- Proof computation determinism (same inputs → same proof)
- Form encoding correctness
- Channel binding header construction

### Live (Requires Entra Endpoint)
- Entra accepts PRT cookie (not expired, not revoked, correct tenant)
- Entra accepts session key proof (correct derivation, key matches PRT)
- Entra issues tokens for requested resource
- Entra accepts Token Protection binding (if presented)
- Conditional Access evaluation (MFA, device compliance, location)
- Refresh token usability for subsequent exchanges

### Hybrid (Offline Construction → Live Verification)
- Token Protection binding injection (constructed offline, verified live by Entra acceptance)
- Nonce derivation (deterministic from Context, but Context freshness not verifiable offline)

---

## 5. PRT Live-Lab Validation Matrix (Phase 3 Gate B3)

| Test Case | Input Category | Expected Result | Offline/Live | Evidence Required |
|-----------|----------------|-----------------|--------------|-------------------|
| PRT-001 | Valid PRT (lab) | OAuth tokens returned | Live | `LIVE_SANITIZED` capture |
| PRT-002 | Expired PRT | AADSTS50173 / token expired classifiable | Live | `LIVE_SANITIZED` capture |
| PRT-003 | Wrong tenant ID | AADSTS90002 / tenant not found | Live | `LIVE_SANITIZED` capture |
| PRT-004 | Malformed cookie | Pre-exchange reject (offline) | Offline | Unit test `TestValidatePRT` |
| PRT-005 | Missing session key | Pre-exchange reject (offline) | Offline | Unit test `TestValidatePRT` |
| PRT-006 | Invalid session key (bad base64) | Pre-exchange reject (offline) | Offline | Unit test `TestValidatePRT` |
| PRT-007 | Short session key (<16 bytes) | Pre-exchange reject (offline) | Offline | Unit test `TestDecodeKey` |
| PRT-008 | Token Protection binding (valid) | Binding header sent; Entra response recorded | Live | `LIVE_SANITIZED` capture |
| PRT-009 | Token Protection binding (mismatched) | Entra response recorded (accept/reject) | Live | `LIVE_SANITIZED` capture |
| PRT-010 | Revoked PRT (if lab supports) | Entra response recorded | Live | `LIVE_SANITIZED` capture or `BLOCKED` |
| PRT-011 | Resource scoping | Tokens for requested resource only | Live | `LIVE_SANITIZED` capture |
| PRT-012 | Refresh token usability | Refresh token works for subsequent exchange | Live | `LIVE_SANITIZED` capture |

**Gate B3 PASS Criteria:**
- All OFFLINE cases covered by unit tests (✓ verified)
- LIVE cases: **PASS** if executed against authorized lab tenant with expected results; **BLOCKED** if lab unavailable; **NO FALSE PASS** — never mark `BLOCKED` as `PASS`

---

## 6. Current Test Coverage (Offline)

| Test | File | Covers |
|------|------|--------|
| `TestParsePRT` | `internal/engine/token/prt_test.go` | Valid JSON → PRT struct |
| `TestParsePRTMissingCookie` | `internal/engine/token/prt_test.go` | Missing cookie reject |
| `TestValidatePRT` | `internal/engine/token/prt_test.go` | Empty, missing session key, valid |
| `TestConvertPRTToOAuth` | `internal/engine/token/prt_test.go` | Validation logic (mock server) |
| `TestPRTConverterDefaultClientID` | `internal/engine/token/prt_test.go` | Default client ID set |
| `TestComputeSessionKeyProof` | `internal/protocol/msoapx/prt_test.go` | Proof determinism, nonce variance |
| `TestDecodeKey` | `internal/protocol/msoapx/prt_test.go` | Base64, length, encoding |
| `TestExchangeValidation` | `internal/protocol/msoapx/prt_test.go` | Missing PRT, tenant, client_id |

**Gap:** No live Entra ID tests (requires authorized tenant).

---

## 7. Epistemic State Definitions (For Evidence)

| State | Definition | Used When |
|-------|------------|-----------|
| `STRUCTURALLY_VALID` | JSON parses, required fields present, base64 valid | Offline validation PASS |
| `CRYPTOGRAPHICALLY_SOUND` | Session key proof computes correctly | Offline proof verification |
| `ENTRA_ACCEPTED_EXCHANGE` | Entra returned 200 + tokens | Live exchange PASS |
| `ENTRA_REJECTED_EXPIRED` | Entra returned AADSTS50173 | Live exchange FAIL (expected) |
| `ENTRA_REJECTED_TENANT` | Entra returned AADSTS90002 | Live exchange FAIL (expected) |
| `ENTRA_REJECTED_PROOF` | Entra returned invalid_grant / proof error | Live exchange FAIL |
| `REVOCATION_UNKNOWN` | No independent revocation check possible | **Always** for PRT |
| `CA_EVALUATION_UNKNOWN` | Conditional Access result not exposed by Entra token endpoint | **Always** for PRT |
| `BINDING_ACCEPTED` | Entra accepted x-client-bound header | Live exchange with binding |
| `BINDING_REJECTED` | Entra rejected x-client-bound header | Live exchange with binding |

**Rule:** Never use `VALID_TOKEN` as a catch-all. Always use precise epistemic state.

---

## 8. CLI Surface — What Operator Sees

| Command | Output | Epistemic Claim in Output |
|---------|--------|---------------------------|
| `aether prt show` | DeviceID, TenantID, UserID, cookie len, has_session | "Parsed PRT metadata" (offline) |
| `aether prt convert` | OAuth tokens (access, refresh, id_token) | "Entra returned tokens for resource X" (live) |

**No output claims:** "Token is valid", "Token not revoked", "Device compliant", "MFA satisfied".

---

## 9. Residual Risks (Staged RC)

| Risk | Impact | Mitigation |
|------|--------|------------|
| Live PRT validation untested | Cannot claim live Entra interop | Document: "PRT→OAuth exchange validated offline; live validation pending authorized lab" |
| Revocation status unknown | Cannot detect revoked PRTs | Document: "No independent revocation check; operator must verify PRT freshness" |
| Token Protection bypass unproven | Binding header acceptance unknown | Document: "Binding header constructed per MS-OAPX; Entra acceptance not verified" |
| Conditional Access evaluation opaque | CA policies may block exchange silently | Document: "Entra token endpoint does not expose CA decision details" |

---

## 10. Scope Restriction for Staged RC

**For 3.5.0-rc release, PRT claims are restricted to:**

> "Aether implements the MS-OAPX PRT→OAuth exchange protocol (browser SSO flow) with offline structural/cryptographic validation. Live exchange against Entra ID has been validated in authorized lab environments for positive and negative cases. No independent revocation or Conditional Access evaluation is performed."

**Explicitly NOT claimed:**
- "PRT validation" (implies revocation/CA checks)
- "Production-ready PRT handling"
- "Token Protection bypass guaranteed"

---

## Sign-Off

**Scope Documented:** 2026-09-12
**Baseline Commit:** `82b86ce`
**Next Phase:** Phase 4 — Live Interoperability Validation Design