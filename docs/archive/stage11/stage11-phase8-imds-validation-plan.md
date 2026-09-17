# Stage 11 Phase 8 — IMDS Validation Plan

**Timestamp:** 2026-09-16
**Status:** PLANNED — Requires authorized test VM and explicit approval

## Strict Authorization Boundary

**IMDS validation is particularly sensitive** because the Instance Metadata Service can expose identity-related information including managed identity tokens.

### Non-Negotiable Rules

1. **NO validation against arbitrary hosts** — Only dedicated authorized test VM
2. **NO production VMs** — Must be isolated test VM
3. **NO token retrieval by default** — Token acquisition is opt-in with explicit approval
4. **NO token persistence** — Tokens never written to disk, logs, or evidence
5. **NO token output** — Tokens redacted from all evidence
6. **NO broad metadata dumping** — Only explicitly requested endpoints
7. **SHORT timeouts** — 5s max per request
8. **FAIL CLOSED** — Missing authorization → validation skipped with documented limitation

## Authorized Test Environment

### Test VM Specification

| Property | Value |
|----------|-------|
| Name | `aether-test-imds-vm` |
| Resource Group | `aether-test-rg` |
| Subscription | Dedicated test subscription |
| Region | Any (prefer same region as Key Vault) |
| OS | Ubuntu 22.04 LTS / Windows Server 2022 |
| Size | Standard_B1s (minimal cost) |
| Managed Identity | System-assigned + User-assigned |
| Network | Isolated VNet, no internet egress required |
| Tags | `purpose=aether-test`, `owner=[team]`, `expiry=2027-06-30` |

### Managed Identity Configuration

| Identity | Type | Role Assignments |
|----------|------|------------------|
| System-assigned | Built-in | `Key Vault Crypto User` on test vault |
| User-assigned | `aether-test-mi` | Same as above |

### Network Security

- **NSG**: Deny all inbound, allow outbound 443 to Azure management endpoints only
- **Private Endpoint**: For Key Vault (if testing KV integration)
- **IMDS Endpoint**: `http://169.254.169.254` (link-local, non-routable)

## Safe Validation Principles

### Endpoint Scope (Minimum Required)

| Endpoint | Purpose | Token Required |
|----------|---------|----------------|
| `/?api-version=2021-02-01` | IMDS availability | No |
| `/metadata/instance?api-version=2021-02-01` | Instance metadata | No |
| `/metadata/identity/oauth2/token?api-version=2018-02-01&resource=...` | Token acquisition | **OPT-IN ONLY** |

### Forbidden Endpoints (Unless Explicitly Approved)

| Endpoint | Reason |
|----------|--------|
| `/metadata/instance/compute` | Full compute metadata (excessive) |
| `/metadata/instance/network` | Network configuration (sensitive) |
| `/metadata/instance/storageProfile` | Storage details (unnecessary) |
| `/metadata/scheduledevents` | Host maintenance events (unrelated) |
| `/metadata/attested/document` | Attestation (requires special approval) |

### Token Handling (If Opt-In Approved)

```go
// Token acquisition - ONLY if explicitly approved
func acquireIMDSToken(ctx context.Context, resource string) (string, error) {
    // 1. Verify approval flag
    if !approvals.Has("imds_token_acquisition") {
        return "", errors.New("IMDS token acquisition not approved")
    }

    // 2. Request with minimal scope
    req, _ := http.NewRequestWithContext(ctx, "GET",
        "http://169.254.169.254/metadata/identity/oauth2/token",
        nil)
    req.Header.Set("Metadata", "true")
    q := req.URL.Query()
    q.Add("api-version", "2018-02-01")
    q.Add("resource", resource)
    req.URL.RawQuery = q.Encode()

    // 3. Short timeout
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    // 4. Parse and IMMEDIATELY redact
    var tokenResp struct {
        AccessToken string `json:"access_token"`
        ExpiresOn   string `json:"expires_on"`
        TokenType   string `json:"token_type"`
    }
    json.NewDecoder(resp.Body).Decode(&tokenResp)

    // 5. Return only metadata, NEVER the token
    return fmt.Sprintf("token_acquired: type=%s expires=%s",
        tokenResp.TokenType, tokenResp.ExpiresOn), nil
}
```

## Required IMDS Test States

| State | Description | Test Method | Expected Result |
|-------|-------------|-------------|-----------------|
| `NOT_CONFIGURED` | VM has no managed identity | Remove all identities | `400` or empty response |
| `UNAVAILABLE` | IMDS endpoint unreachable | Wrong IP / blocked NSG | Timeout / connection refused |
| `UNAUTHORIZED` | Identity lacks permissions | Request token for unauthorized resource | `403` / token with limited claims |
| `TIMEOUT` | IMDS slow/unresponsive | Network latency injection | Client timeout respected |
| `MALFORMED_RESPONSE` | Unexpected response format | Mock server / proxy | Parse error handled gracefully |
| `SUPPORTED` | IMDS responds with expected format | Valid VM + identity | JSON with expected fields |
| `VALIDATED` | Full validation passed | All above + token opt-in | PASS with sanitized evidence |

## Evidence Requirements

### Evidence MUST Contain

```json
{
  "validation_profile_id": "imds-validation-v1",
  "run_id": "imds-20260916-001",
  "provider": "imds",
  "protocol": "HTTP (link-local)",
  "operation_category": "metadata_reachability",
  "sanitized_result": {
    "endpoint_reachable": true,
    "api_version_supported": "2021-02-01",
    "instance_metadata": {
      "vm_id": "REDACTED",
      "location": "REDACTED",
      "vm_size": "Standard_B1s"
    },
    "identity_available": true,
    "token_acquisition": "NOT_ATTEMPTED"
  },
  "timestamp": "2026-09-16T10:00:00Z",
  "verification_state": "PASS",
  "risk_decision": "LOW",
  "redacted_fields": ["vm_id", "subscription_id", "tenant_id", "access_token"],
  "limitations": ["no_token_acquisition", "single_vm", "test_environment_only"]
}
```

### Evidence MUST NOT Contain

- Raw IMDS responses with full metadata
- Access tokens (even truncated)
- Subscription IDs (unredacted)
- Tenant IDs (unredacted)
- VM scale set details
- Network interface details
- Disk encryption details
- Any field not explicitly required for validation

## Dry-Run Mode (Default)

```bash
# Default behavior - no token acquisition
aether validate imds --dry-run

# Output: DryRunResult with reachability only
{
  "imds_reachable": true,
  "api_version": "2021-02-01",
  "identity_configured": true,
  "token_acquisition": "SKIPPED (dry-run)"
}
```

## Opt-In Token Acquisition (Requires Explicit Approval)

```bash
# Only with explicit approval flag
aether validate imds --acquire-token --resource https://vault.azure.net \
  --approval-id "IMDS-TOKEN-2026-09-16-APPROVED"
```

**Approval Process:**
1. Security team reviews request
2. Approval ID generated and logged
3. Flag passed to validation command
4. Token acquired, metadata extracted, token **immediately discarded**
5. Only token metadata (type, expiry) in evidence

## No Production Assumptions

| Assumption | Reality | Validation Approach |
|------------|---------|---------------------|
| Every Azure VM exposes same metadata | Varies by OS, image, config | Test only authorized VM |
| Every MI has same permissions | Role assignments vary | Test with known permissions |
| Every cloud supports same contract | Azure only (IMDS is Azure-specific) | Document as Azure-only |
| Reachable endpoint = token access | Network ≠ RBAC | Test both separately |
| Token response = authorization | Token ≠ permission for operation | Validate claims, not just token |
| Metadata response = security issue | Metadata is informational | Classify findings appropriately |

## Cleanup & Rotation

### Post-Validation Cleanup
- [ ] Delete test VM
- [ ] Delete user-assigned managed identity
- [ ] Remove role assignments
- [ ] Delete test Key Vault (if created)
- [ ] Rotate any client secrets used
- [ ] Verify no lingering resources

### Resource Tagging for Automation
```bash
az resource tag --tags purpose=aether-test expiry=2027-06-30 \
  --resource-group aether-test-rg \
  --name aether-test-imds-vm \
  --resource-type Microsoft.Compute/virtualMachines
```

## Integration with Release Gates

### Pre-Release Check (Automated)
```yaml
# .github/workflows/pre-release-check.yml
- name: Check IMDS waiver expiry
  run: |
    WAIVER_EXPIRY="2027-06-30"
    if [[ $(date -d "$WAIVER_EXPIRY" +%s) -lt $(date +%s) ]]; then
      echo "::error::IMDS validation waiver expired on $WAIVER_EXPIRY"
      exit 1
    fi
```

### Release Decision Input
- If IMDS validated: `VerificationState=PASS` → No limitation
- If IMDS not validated (waiver active): `Limitation="imds_not_validated"` → Conditional release
- If IMDS waiver expired: `BLOCKED` → No release until validated

---

*Generated by Stage 11 Phase 8 IMDS Validation Plan*