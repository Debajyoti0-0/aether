# Stage 11 Phase 6 — 4.1.0 Operations & External Validation Design

**Timestamp:** 2026-09-16
**Scope:** Define 4.1.0 scope, external validation architecture, operational boundaries

## 4.1.0 Scope Definition

### Required for 4.1.0 (Blockers from Stage 10/11 Audit)

| Item | Source | Justification |
|------|--------|---------------|
| Observability (metrics/health) | Stage 10 handoff | Production operations requirement |
| OCSP/CRL revocation checking | Stage 10 handoff | Certificate lifecycle completeness |
| Multi-host deployment docs | Stage 10 handoff | Teamserver↔client topology |
| ARM64 runtime promotion | Stage 10 handoff | linux/arm64, darwin/arm64 build-verified |
| Third-party IdP interop | Stage 10 handoff | Okta/Auth0/Keycloak validation |
| Live Entra/IMDS validation | Stage 10 handoff | Before 2027-06-30 waiver expiry |

### Optional (Non-Blocking Improvements)

| Item | Justification |
|------|---------------|
| Enhanced CLI UX | Operator productivity |
| Structured logging (JSON) | Observability integration |
| Configuration validation CLI | Fail-fast operator experience |
| Workspace backup/restore | Operational resilience |

### Deferred (Require Broader Architecture/Authorization)

| Item | Blocked By |
|------|------------|
| Full SLSA Level 3 provenance | Build infrastructure investment |
| Hardware HSM integration (YubiHSM, AWS CloudHSM) | Hardware access / cost |
| Kubernetes operator | Platform scope expansion |
| GraphQL API for teamserver | API design effort |

### Unsupported (Not Implemented / Not Safe)

| Item | Reason |
|------|--------|
| Automatic credential rotation | Security risk, requires policy engine |
| Cross-tenant Entra sync | Multi-tenant complexity |
| Real-time threat intel feed | External dependency, licensing |

## External Validation Architecture

### Conceptual Model

```
┌─────────────────────────────────────────────────────────────────┐
│                        Local Aether                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Profile    │──│  Validator   │──│  Evidence Collector  │  │
│  │  Selection   │  │  Orchestrator│  │  (sanitized output)  │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 Authorized Test Environment                     │
│  ┌─────────────┐  ┌─────────────┐  ┌────────────────────────┐  │
│  │  Entra ID   │  │   IMDS      │  │   Key Vault / KMS      │  │
│  │  (Tenant)   │  │  (VM)       │  │   (Azure/AWS/GCP)      │  │
│  └─────────────┘  └─────────────┘  └────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Evidence & Decision                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Risk Score   │  │  Limitation  │  │  Release Gate Input  │  │
│  │  (0-10)      │  │  Register    │  │  (PASS/CONDITIONAL)  │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Validation Profile Schema

```yaml
# validation-profile.yaml
profile:
  id: "entra-validation-v1"
  name: "Entra ID Configuration Validation"
  version: "1.0"
  authorization:
    required: true
    tenant_scope: "single"
    identity_types: ["service_principal", "managed_identity", "user"]
  boundaries:
    max_requests: 100
    timeout: 30s
    dry_run_default: true
    allowed_endpoints:
      - "https://login.microsoftonline.com/{tenant}/v2.0/.well-known/openid-configuration"
      - "https://graph.microsoft.com/v1.0/me"
      - "https://management.azure.com/.../providers/Microsoft.KeyVault/vaults"
  operations:
    - name: "token_acquisition"
      scopes: ["https://graph.microsoft.com/.default"]
      validate: ["issuer", "audience", "expiry", "scopes"]
    - name: "token_refresh"
      validate: ["new_token_issued", "old_token_revoked"]
    - name: "device_code_flow"
      validate: ["user_code_issued", "polling_works", "token_obtained"]
    - name: "cae_evaluation"
      validate: ["claims_challenge_handled", "conditional_access_respected"]
  output:
    evidence_format: "json"
    redaction_rules:
      - "remove: access_token"
      - "remove: refresh_token"
      - "remove: client_secret"
      - "mask: tenant_id"
      - "mask: object_id"
    risk_factors:
      - "token_expiry < 5min"
      - "cae_claims_challenge_failed"
      - "conditional_access_denied"
```

### Bounded Validation Adapter Interface

```go
// ValidationAdapter defines the contract for external validation.
type ValidationAdapter interface {
    // Name returns the adapter identifier (e.g., "entra", "imds", "keyvault").
    Name() string

    // Validate executes the validation profile against the target environment.
    // ctx must support cancellation and timeout.
    // Returns sanitized Evidence, never raw credentials.
    Validate(ctx context.Context, profile *ValidationProfile) (*Evidence, error)

    // DryRun performs a non-mutating connectivity/configuration check.
    DryRun(ctx context.Context) (*DryRunResult, error)

    // SupportedProfiles returns the validation profiles this adapter supports.
    SupportedProfiles() []ValidationProfile
}

// Evidence is the sanitized output of a validation run.
type Evidence struct {
    ProfileID      string                 `json:"profile_id"`
    RunID          string                 `json:"run_id"`
    Timestamp      time.Time              `json:"timestamp"`
    Adapter        string                 `json:"adapter"`
    Protocol       string                 `json:"protocol"`
    Operations     []OperationEvidence    `json:"operations"`
    Verification   VerificationState      `json:"verification"` // PASS, FAIL, PARTIAL
    RiskScore      int                    `json:"risk_score"`   // 0-10
    Limitations    []string               `json:"limitations"`
    RedactedFields []string               `json:"redacted_fields"`
}

type OperationEvidence struct {
    Operation  string         `json:"operation"`
    Result     string         `json:"result"` // SUCCESS, FAIL, SKIPPED
    LatencyMS  int64          `json:"latency_ms"`
    Sanitized  map[string]any `json:"sanitized_output"`
    Error      *ErrorInfo     `json:"error,omitempty"`
}
```

### Security Requirements

| Requirement | Implementation |
|-------------|----------------|
| No credential persistence | Adapter never stores tokens/secrets; uses ephemeral clients |
| No raw token output | Evidence redacts `access_token`, `refresh_token`, `client_secret` |
| No uncontrolled enumeration | Profile defines exact endpoints/operations; no discovery |
| No hidden network activity | All requests explicit in profile; dry-run shows what would run |
| Clear dry-run behavior | `DryRun()` validates config without side effects |
| Clear unsupported behavior | Returns `SKIPPED` with reason; never panics |
| Reproducible evidence | Deterministic output for same environment state |
| Cleanup procedures | Adapter implements `Close()` for connection cleanup |

### Fail-Closed Design

```go
// ValidationResult encodes the decision.
type ValidationResult struct {
    Authorized  bool     `json:"authorized"`  // Can proceed with operation
    Evidence    Evidence `json:"evidence"`
    Limitations []string `json:"limitations"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty"` // For time-bound validation
}

// Policy: Any validation error → Authorized=false
//         Missing authorization context → Authorized=false
//         Redaction failure → Authorized=false
//         Timeout/cancellation → Authorized=false
```

## Entra/IMDS Validation Integration

### Entra Validation Adapter
- **Scope**: Token acquisition, refresh, device code, CAE, conditional access
- **Authorization**: Requires explicit tenant + app registration + permissions
- **Output**: Sanitized token metadata (no raw tokens)
- **Waiver**: If no authorized tenant → `ValidationResult{Authorized: false, Limitations: ["no_authorized_tenant"]}`

### IMDS Validation Adapter
- **Scope**: Endpoint reachability, identity availability, token acquisition
- **Authorization**: Requires dedicated test VM with managed identity
- **Output**: Sanitized identity metadata (no tokens unless explicitly requested)
- **Safety**: Dry-run by default; token acquisition opt-in with explicit approval
- **Waiver**: If no authorized VM → `ValidationResult{Authorized: false, Limitations: ["no_authorized_vm"]}`

## 4.1.0 Release Gate Integration

### Pre-Release Validation (Automated)
```yaml
# .github/workflows/validate-4.1.0.yml
jobs:
  external-validation:
    runs-on: ubuntu-latest
    if: github.event_name == 'workflow_dispatch'  # Manual trigger only
    steps:
      - uses: actions/checkout@v4
      - name: Run Entra validation (if authorized)
        if: env.ENTRA_TENANT_ID != ''
        run: aether validate entra --profile entra-validation-v1 --output evidence.json
      - name: Run IMDS validation (if authorized)
        if: env.IMDS_VM_IP != ''
        run: aether validate imds --profile imds-validation-v1 --output evidence.json
      - name: Upload evidence
        uses: actions/upload-artifact@v4
        with:
          name: external-validation-evidence
          path: evidence.json
```

### Release Decision Matrix

| Validation | 4.0.0 GA | 4.1.0 Operations |
|------------|----------|------------------|
| Entra (live) | Required for GA | Required for full ops |
| IMDS (live) | Required for GA | Required for full ops |
| Key Vault (live) | Required for B5 | Required for B5 closure |
| Third-party IdP | Optional | Required for 4.1.0 |
| ARM64 runtime | Optional | Build-verified minimum |

---

*Generated by Stage 11 Phase 6 4.1.0 Operations Design*