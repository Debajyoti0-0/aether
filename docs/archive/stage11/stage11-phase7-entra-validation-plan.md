# Stage 11 Phase 7 — Entra Validation Plan

**Timestamp:** 2026-09-16
**Status:** PLANNED — Requires authorized tenant and explicit approval

## Authorization Boundary

This validation plan requires **explicit written approval** before execution.

### Required Approvals
- [ ] Security team approval for tenant access
- [ ] Identity team approval for app registration
- [ ] Infrastructure approval for test resources
- [ ] Data governance approval for token handling

### Authorization Document
```
Authorization ID: ENTRA-VAL-2026-09-16
Approved By: [Name, Role]
Date: [Date]
Scope: Single test tenant, read-only operations
Expires: 2027-06-30 (aligned with Stage 10 waiver)
```

## Scope Definition

### Authorized Tenant
- **Tenant ID**: `[REDACTED - to be filled at execution]`
- **Display Name**: `Aether Test Tenant`
- **Type**: Dedicated test tenant (not production)
- **Purge Protection**: Disabled for test resources

### Authorized Test Identities

| Identity | Type | Permissions | Purpose |
|----------|------|-------------|---------|
| `aether-test-sp` | Service Principal | `User.Read`, `Application.Read.All` | Token acquisition, Graph API |
| `aether-test-mi` | Managed Identity (VM) | `Key Vault Crypto User` | IMDS + Key Vault integration |
| `aether-test-user` | Test User | `User.Read` | Device Code flow, interactive auth |

### Authorized Application Registrations

| App Registration | Client ID | Redirect URIs | Cert/Secret |
|------------------|-----------|---------------|-------------|
| `Aether Test Client` | `[REDACTED]` | `http://localhost:8080/callback` | Client secret (rotated post-test) |
| `Aether Test Daemon` | `[REDACTED]` | N/A (daemon) | Client secret (rotated post-test) |

### Required Permissions (Minimum)

**Microsoft Graph:**
- `User.Read` — Basic profile/token validation
- `Application.Read.All` — App registration metadata
- `Directory.Read.All` — Tenant configuration (read-only)

**Azure Resource Manager:**
- `Key Vault Crypto User` — Key operations (if testing Key Vault)
- `Managed Identity Operator` — MI assignment (if testing IMDS)

## Safe Validation Categories

### Preferred (Non-Destructive, Read-Only)

| Category | Operations | Risk |
|----------|------------|------|
| Configuration Correctness | OIDC discovery, JWKS retrieval, issuer validation | None |
| Token Metadata Handling | Parse claims, validate iss/aud/exp/nbf, check scopes | None |
| Issuer/Audience Validation | Verify `iss` matches tenant, `aud` matches client | None |
| Expiration Handling | Near-expiry tokens, expired tokens, clock skew | None |
| Permission Interpretation | Scope validation, roles claim, groups claim | None |
| Error Handling | Invalid token, revoked token, malformed token | None |
| Capability Normalization | Map Entra claims → Aether capabilities | None |
| Evidence Generation | Structured output for audit trail | None |
| Policy Decisions | Allow/deny based on claims, CAE evaluation | None |

### Avoided (Destructive or Data Access)

| Category | Reason |
|----------|--------|
| User enumeration | Unnecessary directory access |
| Group membership expansion | Excessive data access |
| Application modification | Write operation |
| Conditional Access policy changes | Configuration drift |
| Token revocation (broad) | Affects other sessions |
| Directory role assignment | Privilege escalation risk |

## Required Evidence Structure

```json
{
  "validation_profile_id": "entra-validation-v1",
  "run_id": "entra-20260916-001",
  "provider": "entra",
  "protocol": "OAuth2/OIDC",
  "operation_category": "token_metadata",
  "sanitized_result": {
    "issuer_validated": true,
    "audience_validated": true,
    "expiry_handling": "correct",
    "scope_mapping": "complete",
    "cae_support": "detected",
    "token_refresh": "works"
  },
  "timestamp": "2026-09-16T10:00:00Z",
  "verification_state": "PASS",
  "risk_decision": "LOW",
  "error_classification": "NONE",
  "redacted_fields": ["access_token", "refresh_token", "client_secret", "tenant_id", "object_id"],
  "limitations": ["single_tenant", "test_identities_only", "no_production_data"]
}
```

### Evidence MUST Contain
- Validation profile ID
- Run ID (unique per execution)
- Provider name
- Protocol tested
- Operation category
- Sanitized result (structured, no raw tokens)
- Timestamp (UTC)
- Verification state (PASS/FAIL/PARTIAL/SKIPPED)
- Risk/decision (LOW/MEDIUM/HIGH/CRITICAL)
- Error classification
- Redacted fields list
- Limitations

### Evidence MUST NOT Contain
- Raw access tokens
- Raw refresh tokens
- Client secrets
- Authorization headers
- Unredacted tenant IDs
- Unredacted object IDs
- User principal names (unless test user)
- Directory data (users, groups, apps beyond test scope)

## Expiry/Waiver Tracking

### Stage 10 Waiver Reference
- **Waiver**: Live Entra validation deferred
- **Expiry**: 2027-06-30
- **Source**: Stage 10 handoff / Stage 9 blocker register

### Verification Required
- [ ] Confirm 2027-06-30 date against project governance docs
- [ ] Check if waiver formally documented in blocker register
- [ ] Verify no earlier expiry in security policy
- [ ] Create operational reminder if date confirmed

### If Waiver Confirmed
- Add release gate: `if (now > 2027-06-30 && !entra_validated) block_release`
- Document in `VERIFY.md` and release checklist
- Create GitHub issue for tracking

## Execution Checklist (Pre-Run)

- [ ] Authorization document signed
- [ ] Test tenant provisioned and verified
- [ ] Test identities created with minimal permissions
- [ ] App registrations configured
- [ ] Secrets stored in secure vault (not repo)
- [ ] Validation profile YAML reviewed
- [ ] Redaction rules tested on sample output
- [ ] Dry-run executed successfully
- [ ] Cleanup procedure documented
- [ ] Rollback plan for test resources

## Execution Checklist (Post-Run)

- [ ] Evidence JSON generated and sanitized
- [ ] Raw tokens/secrets verified absent from evidence
- [ ] Test resources cleaned up (app registrations, identities)
- [ ] Secrets rotated (client secrets, certificates)
- [ ] Evidence stored in audit log
- [ ] Validation result recorded in blocker register
- [ ] Waiver status updated if validation passes

---

*Generated by Stage 11 Phase 7 Entra Validation Plan*