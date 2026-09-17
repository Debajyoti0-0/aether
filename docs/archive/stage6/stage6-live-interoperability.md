# Aether — Stage 6 Live Interoperability Validation Design

**Baseline:** Stage 3 (3.4.0-stage3), HEAD `2a82000`
**Phase:** 4 of 9
**Scope:** Teamserver↔Client mTLS, WS-Trust/SAML, Device Code, CAE, IMDS — Design only; no live execution without Phase 2 authorization

---

## 1. Interoperability Targets (Release Scope)

### Tier 0 — Must Validate (Blocking for Staged RC)

| Target | Protocol | Auth Mode | Transport | Version Tested | Status |
|--------|----------|-----------|-----------|----------------|--------|
| **Local Teamserver** | Protocol v2 (mTLS) | Client cert (CA-issued) | TLS 1.3 + uTLS (Chrome) | `3.4.0-stage3` | **READY** (self-hosted) |
| **Entra ID (lab tenant)** | MS-OAPX (PRT→OAuth) | PRT cookie + session key | TLS 1.3 + uTLS (Chrome/Edge) | `3.4.0-stage3` | **PENDING** (needs tenant) |
| **Entra ID (lab tenant)** | WS-Trust 1.3 / SAML 1.1 Bearer | Username/password | TLS 1.3 + uTLS (Chrome) | `3.4.0-stage3` | **PENDING** (needs tenant) |
| **Entra ID (lab tenant)** | Device Code Flow | User code + polling | TLS 1.3 + uTLS (Chrome) | `3.4.0-stage3` | **PENDING** (needs tenant) |

### Tier 1 — Should Validate (Non-Blocking for RC)

| Target | Protocol | Auth Mode | Transport | Status |
|--------|----------|-----------|-----------|--------|
| **Entra ID (lab tenant)** | CAE Claims Challenge | Refresh token + claims | TLS 1.3 + uTLS | **PENDING** |
| **Azure VM (dev sub)** | IMDSv2 Identity Token | Managed identity | HTTP (169.254.169.254) | **PENDING** (needs VM) |

### Tier 2 — Opt-In / Future

| Target | Protocol | Notes |
|--------|----------|-------|
| AD FS (on-prem) | WS-Trust / SAML | Requires AD FS lab |
| PingFederate | WS-Trust / SAML | Requires Ping lab |
| Okta | SAML / OIDC | Requires Okta lab |
| AWS IMDS | Instance Identity | Requires AWS lab |
| GCP Metadata | Instance Identity | Requires GCP lab |

**Rule:** Tier 2 targets do not block Staged RC. They are explicitly opt-in.

---

## 2. Teamserver ↔ Client mTLS Validation

### 2.1 Test Matrix

| Scenario | Expected Result | Offline/Live | Evidence |
|----------|----------------|--------------|----------|
| Valid operator cert (CA-issued) | mTLS handshake success; commands execute | Live (local) | `LIVE_SANITIZED` |
| Expired operator cert | Handshake fail (cert expired) | Live (local) | `LIVE_SANITIZED` |
| Not-yet-valid operator cert | Handshake fail (not yet valid) | Live (local) | `LIVE_SANITIZED` |
| Wrong CA (self-signed) | Handshake fail (unknown CA) | Live (local) | `LIVE_SANITIZED` |
| Revoked operator cert | Handshake fail (revoked list) | Live (local) | `LIVE_SANITIZED` |
| SAN mismatch (wrong URI) | Handshake fail / operator name mismatch | Live (local) | `LIVE_SANITIZED` |
| EKU mismatch (no client auth) | Handshake fail | Live (local) | `LIVE_SANITIZED` |
| Confusable operator name | `FromClientCert` rejects homograph | Offline | Unit test |
| Invalid IA5String in SAN | `FromClientCert` rejects | Offline | Unit test |
| Key mismatch (cert/key pair) | Handshake fail | Live (local) | `LIVE_SANITIZED` |
| Handshake interruption | Connection closed; no partial state | Live (local) | `LIVE_SANITIZED` |
| Reconnect with cursor | Event replay from last seq | Live (local) | `LIVE_SANITIZED` |
| Concurrent connections (32) | All accepted; per-conn multiplexing | Live (local) | `LIVE_SANITIZED` |
| In-flight cap (8) respected | 9th concurrent → backpressure error | Live (local) | `LIVE_SANITIZED` |

### 2.2 Negative Interoperability

| Negative Case | Protocol Layer | Expected Classification |
|---------------|----------------|------------------------|
| TLS 1.2 client | TLS | Handshake fail (MinVersion=TLS13) |
| No client cert | TLS | Handshake fail (RequireAndVerifyClientCert) |
| Malformed frame (oversized) | Protocol v2 | `MsgError` with "too large" |
| Missing RequestID on command | Protocol v2 | `MsgError` with "request id" |
| Unknown message type | Protocol v2 | `MsgError` with "unknown message type" |
| Ping/Pong failure | Protocol v2 | Connection timeout |
| Slow subscriber | Event stream | Frames dropped (not blocked) |

---

## 3. WS-Trust / SAML / OAuth Relay Validation

### 3.1 Test Matrix (Entra ID Lab Tenant)

| Scenario | Flow | Expected Result | Evidence |
|----------|------|----------------|----------|
| Valid username/password | WS-Trust RST → SAML → OAuth | Tokens returned | `LIVE_SANITIZED` |
| Wrong password | WS-Trust RST | SOAP Fault / classifiable error | `LIVE_SANITIZED` |
| Expired password | WS-Trust RST | SOAP Fault / classifiable error | `LIVE_SANITIZED` |
| Federated domain (MEX) | UserRealm → MEX → UsernameMixed | Downgrade opportunity detected | `LIVE_SANITIZED` |
| MFA challenge (SMS/Authenticator) | WS-Trust RST | MFA challenge in response | `LIVE_SANITIZED` |
| SAML assertion replay | SAML Bearer → OAuth | Replay accepted/rejected | `LIVE_SANITIZED` |
| Expired SAML assertion | SAML Bearer → OAuth | Token exchange fail | `LIVE_SANITIZED` |
| Wrong audience in SAML | SAML Bearer → OAuth | Token exchange fail | `LIVE_SANITIZED` |
| Stripped SAML signature | `aether relay saml-strip` | Signature removed; SP acceptance test | `CONTROLLED_SYNTHETIC` |

### 3.2 Device Code Flow

| Scenario | Expected Result | Evidence |
|----------|----------------|----------|
| Valid flow (user approves) | Tokens returned after poll | `LIVE_SANITIZED` |
| User denies | Authorization_pending → access_denied | `LIVE_SANITIZED` |
| Poll timeout | Timeout error | `LIVE_SANITIZED` |
| Expired device_code | Invalid_grant error | `LIVE_SANITIZED` |

### 3.3 CAE (Continuous Access Evaluation)

| Scenario | Expected Result | Evidence |
|----------|----------------|----------|
| Valid refresh + CAE challenge | New access token with claims | `LIVE_SANITIZED` |
| Unsupported claim (e.g., xms_cc) | Classifiable error | `LIVE_SANITIZED` |
| Expired refresh token | Invalid_grant | `LIVE_SANITIZED` |

---

## 4. IMDSv2 Validation (Azure VM)

### 4.1 Test Matrix

| Scenario | Expected Result | Evidence |
|----------|----------------|----------|
| IMDSv2 token fetch (TTL=300) | Token returned; valid for 5 min | `LIVE_SANITIZED` |
| Identity token (system-assigned MI) | OAuth token for resource | `LIVE_SANITIZED` |
| Identity token (user-assigned MI, client_id) | OAuth token for resource | `LIVE_SANITIZED` |
| Identity token (user-assigned MI, object_id) | OAuth token for resource | `LIVE_SANITIZED` |
| IMDSv1 fallback (no token header) | Works if v1 enabled; fails if v2 required | `LIVE_SANITIZED` |
| Token TTL expiry | Subsequent requests fail with 401 | `LIVE_SANITIZED` |
| Instance metadata | Parseable JSON with compute/subscription | `LIVE_SANITIZED` |
| Network isolation (no IMDS) | Connection timeout / refused | `LIVE_SANITIZED` |

---

## 5. uTLS Fingerprint Validation

### 5.1 Browser Presets Tested

| Preset | JA3/JA4 Target | Test Method |
|--------|----------------|-------------|
| Chrome | HelloChrome_Auto | TLS handshake captured; JA3 verified |
| Edge | HelloEdge_Auto | TLS handshake captured; JA3 verified |
| Firefox | HelloFirefox_Auto | TLS handshake captured; JA3 verified |
| JA4 Pool Rotation | Rotating profiles | Per-connection fingerprint varies |

### 5.2 Validation Method

1. Capture `ClientHello` with Wireshark/tcpdump on target endpoint
2. Compute JA3/JA4 fingerprint
3. Compare to expected browser fingerprint (reference: `refraction-networking/utls` test vectors)
4. Record: `LIVE_SANITIZED` with fingerprint hash (no raw packets)

---

## 6. Negative Interoperability (Cross-Cutting)

| Category | Test Cases | Purpose |
|----------|------------|---------|
| **Invalid credentials** | Wrong password, expired password, revoked cert | Verify error classification |
| **Expired credentials** | Expired PRT, expired SAML, expired refresh token, expired IMDS token | Verify expiry handling |
| **Wrong audience/scope** | Wrong resource, wrong tenant, wrong applies-to | Verify scope enforcement |
| **Invalid certificate chain** | Self-signed, wrong CA, expired, not-yet-valid, SAN mismatch | Verify mTLS fail-closed |
| **Revoked/rejected cert** | Operator cert in revocation list | Verify revocation enforcement |
| **Protocol version mismatch** | TLS 1.2, Protocol v1 frames | Verify version negotiation |
| **Malformed request** | Truncated frames, oversized, invalid JSON, invalid XML | Verify parser safety |
| **Timeout** | Connection timeout, read timeout, poll timeout | Verify timeout handling |
| **Connection interruption** | TCP RST, TLS close_notify, server crash | Verify cleanup/reconnect |
| **Replay/duplicate** | Same RequestID twice, same PRT twice, same SAML twice | Verify idempotency/deduplication |
| **Server-side rejection** | 400/401/403/500 from endpoint | Verify error propagation |

---

## 7. Evidence Requirements (Per Gate B4)

### 7.1 PASS Criteria for Tier 0

| Target | Minimum Evidence |
|--------|------------------|
| Local Teamserver | All 2.1 scenarios `LIVE_SANITIZED` or `CONTROLLED_SYNTHETIC` |
| Entra ID PRT | PRT-001 through PRT-012 (Phase 3) `LIVE_SANITIZED` or `BLOCKED` |
| Entra ID WS-Trust | 3.1 positive + 2 negative cases `LIVE_SANITIZED` or `BLOCKED` |

### 7.2 Gate B4 PASS Definition

**PASS only if all Tier 0 targets have either:**
- Verified live evidence (`LIVE_SANITIZED`), OR
- Clearly documented limitation preventing claim (`BLOCKED` with reason)

**Tier 1/2:** Documented as `PENDING` or `DEFERRED` — do not block RC.

---

## 8. Current Implementation Coverage

| Protocol | Implementation | Unit Tests | Integration Tests | Live Tests |
|----------|----------------|------------|-------------------|------------|
| Teamserver v2 (mTLS) | `internal/api/server.go`, `client.go`, `protocol.go` | ✓ Frame, validation, multiplex | ✓ `TestMultiplexedCommands` | **PENDING** |
| WS-Trust | `internal/protocol/wstrust/*.go` | ✓ Request, response, MEX, downgrade | ✗ | **PENDING** |
| SAML | `internal/protocol/saml/*.go` | ✓ Assertion, signature, strip | ✗ | **PENDING** |
| MS-OAPX (PRT) | `internal/protocol/msoapx/prt.go` | ✓ Proof, exchange, validation | ✗ | **PENDING** |
| Device Code | `internal/protocol/oauth2/*.go`, `engine/relay/devicecode.go` | ✓ Flow | ✗ | **PENDING** |
| CAE | `internal/protocol/oauth2/*.go`, `cli/relay_cae.go` | ✓ Challenge handling | ✗ | **PENDING** |
| IMDSv2 | `internal/engine/exec/imds.go` | ✓ Token, metadata | ✗ | **PENDING** |
| uTLS | `internal/transport/tls.go`, `stealth.go`, `ja4_pool.go` | ✓ Preset, rotation, retry | ✗ | **PENDING** |

---

## 9. Known Limitations (Documented for RC)

1. **WS-Trust/SAML**: Only tested against mock servers; no live Entra ID / AD FS validation
2. **PRT→OAuth**: Offline validation complete; live Entra validation pending authorized tenant
3. **Device Code**: Flow implemented; live polling untested
4. **CAE**: Challenge handling implemented; live claims challenges untested
5. **IMDSv2**: Azure-only; no AWS/GCP metadata service support
6. **uTLS**: Chrome/Edge/Firefox presets; no Safari; JA4 rotation untested against fingerprinting services
7. **Teamserver**: Local only; no cross-network / load balancer testing

---

## 10. Phase 4 Gate (B4) — Status

| Target | Status | Blocker |
|--------|--------|---------|
| Local Teamserver | **READY** | None (self-hosted) |
| Entra ID PRT | **BLOCKED** | Requires lab tenant authorization |
| Entra ID WS-Trust | **BLOCKED** | Requires lab tenant authorization |
| Entra ID Device Code | **BLOCKED** | Requires lab tenant authorization |
| Entra ID CAE | **BLOCKED** | Requires lab tenant authorization |
| Azure IMDS | **BLOCKED** | Requires dev subscription VM |

**Gate B4 Status:** **CONDITIONAL PASS** — Tier 0 local teamserver ready; Entra ID targets blocked pending authorization. Documented limitations accepted for Staged RC.

---

## Sign-Off

**Design Completed:** 2026-09-12
**Baseline Commit:** `2a82000`
**Next Phase:** Phase 5 — Artifact Signing & Trust Chain Design