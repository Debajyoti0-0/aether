# Stage 11 Phase 10 — Documentation & Operations Updates

**Timestamp:** 2026-09-16
**Scope:** Production-facing documentation updates with maturity distinctions

## Documentation Update Matrix

| Document | Status | Maturity Distinctions Required |
|----------|--------|-------------------------------|
| Azure Key Vault Provider | ❌ Not exist | Implemented / Unit-tested / Integration-tested / Externally validated / Production-authorized |
| Configuration Reference | ⚠️ Partial | Same |
| Authentication Requirements | ⚠️ Partial | Same |
| Key Custody Limitations | ❌ Not exist | Same |
| External Validation Guide | ❌ Not exist | Same |
| Entra Validation Profile | ❌ Not exist | Same |
| IMDS Validation Profile | ❌ Not exist | Same |
| Security & Privacy Guidance | ⚠️ Partial | Same |
| Release Process | ⚠️ Partial | Same |
| Artifact Verification | ❌ Not exist | Same |
| SBOM Verification | ❌ Not exist | Same |
| Signing Status | ❌ Not exist | Same |
| Provider Support Matrix | ❌ Not exist | Same |
| 4.1.0 Operational Limitations | ❌ Not exist | Same |

## Required Maturity Labels

Every document MUST distinguish:

| Label | Meaning |
|-------|---------|
| **Implemented** | Code exists, compiles |
| **Unit-tested** | Unit tests pass (local + CI) |
| **Integration-tested** | Integration tests pass (local mocks + CI) |
| **Externally validated** | Tested against real external system (Azure, Entra, IMDS) |
| **Production-authorized** | Approved for production use by security/release |
| **Planned** | Designed, not implemented |
| **Unsupported** | Explicitly not implemented, no plans |

## Template: Azure Key Vault Provider Documentation

```markdown
# Azure Key Vault Key Provider

**Status: Implemented • Unit-tested ❌ • Integration-tested ❌ • Externally validated ❌ • Production-authorized ❌**

## Overview
The `azure_kv` provider implements key custody operations using Azure Key Vault.
It manages EC-P256 key lifecycle (create, rotate, list versions) but does NOT
provide Ed25519 keys for audit signing.

## Capabilities

| Operation | Supported | Notes |
|-----------|-----------|-------|
| Key creation (EC-P256) | ✅ | Creates if missing |
| Key versioning | ✅ | Lists all versions |
| Key rotation | ✅ | Native KV rotation |
| Current version tracking | ✅ | Via `Enabled` attribute |
| Ed25519 signing key | ❌ | **Not supported** — KV doesn't export private keys |
| Ed25519 verification key | ❌ | **Not supported** — KV doesn't support Ed25519 |

## Configuration

```yaml
# Via environment variables
AZURE_KEY_VAULT_URL: "https://myvault.vault.azure.net"
AZURE_KEY_VAULT_KEY_NAME: "aether-audit-key"

# Or programmatically
store.NewAzureKVProvider(ctx, store.AzureKVConfig{
    VaultURL: "https://myvault.vault.azure.net",
    KeyName:  "aether-audit-key",
    Credential: nil, // Uses DefaultAzureCredential
})
```

## Authentication

Uses `DefaultAzureCredential` chain (in order):
1. Environment variables (`AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_SECRET`)
2. Workload Identity (Kubernetes)
3. Managed Identity (Azure VM, App Service, AKS)
4. Azure CLI (`az login`)
5. Azure PowerShell
6. Azure Developer CLI

## Limitations

1. **Not an audit signing provider** — Cannot satisfy `KeyProvider.GetSigningKey`/`GetVerificationKey`
2. **EC-P256 only** — Azure KV doesn't support Ed25519
3. **No private key export** — By design (HSM-grade)
4. **Requires network connectivity** — To Azure KV endpoint
5. **Rate limits apply** — Azure KV throttling (429 handling not implemented)
6. **No delete/purge** — Provider doesn't support key deletion

## Usage Pattern

```go
// For audit signing: use LocalKeyProvider or MockKMSProvider
signingProvider, _ := store.NewLocalKeyProvider(path, passphrase)

// For key custody: use AzureKVProvider
custodyProvider, _ := store.NewAzureKVProviderFromEnv(ctx)

// They serve DIFFERENT purposes
```

## Testing Status

| Test Type | Status | Command |
|-----------|--------|---------|
| Unit | ❌ Not implemented | `go test ./internal/store/...` |
| Integration (mock) | ❌ Not implemented | `go test -tags=integration ./internal/store/...` |
| External (live Azure) | ❌ Not run | Requires `AZURE_KEY_VAULT_URL` |

## Security Considerations

- Private keys never leave Azure Key Vault
- Uses Azure SDK default retry/timeout (configurable)
- No credentials logged or persisted
- Supports managed identity (no secrets in config)
- Fail-closed on authentication failure

## Future Work

- [ ] Add mutex for thread safety
- [ ] Implement retry policy for 429/timeout
- [ ] Add configurable timeouts
- [ ] Integration tests with mock transport
- [ ] External validation against live Azure KV
- [ ] Document rotation behavior under concurrency
```

## Template: External Validation Guide

```markdown
# External Validation Guide

**Status: Planned • Unit-tested ❌ • Integration-tested ❌ • Externally validated ❌ • Production-authorized ❌**

## Overview
Aether supports external validation against authorized cloud environments.
This guide describes the validation architecture, profiles, and safety boundaries.

## Validation Profiles

### Entra ID Validation (`entra-validation-v1`)
- **Authorization**: Required (dedicated test tenant)
- **Operations**: Token acquisition, refresh, device code, CAE, conditional access
- **Output**: Sanitized token metadata (no raw tokens)
- **Waiver**: Expires 2027-06-30

### IMDS Validation (`imds-validation-v1`)
- **Authorization**: Required (dedicated test VM)
- **Operations**: Endpoint reachability, instance metadata, identity availability
- **Token acquisition**: Opt-in only, explicit approval required
- **Output**: Sanitized metadata (no tokens, redacted IDs)
- **Waiver**: Expires 2027-06-30

### Azure Key Vault Validation (`keyvault-validation-v1`)
- **Authorization**: Required (dedicated test vault)
- **Operations**: Key create, rotate, list versions
- **Output**: Key metadata (no key material)
- **Waiver**: None (required for B5 closure)

## Running Validations

```bash
# Dry-run (default, safe)
aether validate entra --dry-run
aether validate imds --dry-run
aether validate keyvault --dry-run

# Live validation (requires authorization)
aether validate entra --profile entra-validation-v1 --output evidence.json
aether validate imds --profile imds-validation-v1 --output evidence.json
aether validate keyvault --profile keyvault-validation-v1 --output evidence.json

# IMDS token acquisition (requires explicit approval)
aether validate imds --acquire-token --resource https://vault.azure.net \
  --approval-id "IMDS-TOKEN-2026-09-16-APPROVED"
```

## Evidence Format

All validations produce sanitized JSON evidence:
```json
{
  "validation_profile_id": "entra-validation-v1",
  "run_id": "entra-20260916-001",
  "provider": "entra",
  "protocol": "OAuth2/OIDC",
  "operation_category": "token_metadata",
  "sanitized_result": { ... },
  "timestamp": "2026-09-16T10:00:00Z",
  "verification_state": "PASS",
  "risk_decision": "LOW",
  "redacted_fields": ["access_token", "refresh_token", "client_secret"],
  "limitations": ["single_tenant", "test_identities_only"]
}
```

## Safety Boundaries

| Boundary | Enforcement |
|----------|-------------|
| No credential persistence | Adapters never store tokens/secrets |
| No raw token output | Evidence redacts all sensitive fields |
| No uncontrolled enumeration | Profile defines exact endpoints |
| No hidden network activity | All requests explicit in profile |
| Clear dry-run behavior | `DryRun()` validates config only |
| Fail-closed on missing auth | Returns `SKIPPED` with limitation |

## Waiver Tracking

| Validation | Waiver Expiry | Status |
|------------|---------------|--------|
| Entra live | 2027-06-30 | Active |
| IMDS live | 2027-06-30 | Active |
| Key Vault live | None (required) | N/A |

**If waiver expires without validation**: Release blocked until validated.
```

## Template: Provider Support Matrix

```markdown
# Key Provider Support Matrix

**Status: Implemented • Unit-tested ✅ • Integration-tested ✅ • Externally validated ⚠️ • Production-authorized ⚠️**

| Provider | Type | Ed25519 Signing | Key Custody | Rotation | External Validation | Production Ready |
|----------|------|-----------------|-------------|----------|---------------------|------------------|
| `local` | File (encrypted) | ✅ | ✅ | ✅ | ✅ (local) | ✅ |
| `mock_kms` | In-memory | ✅ | ✅ | ✅ | ✅ (mock) | ❌ (test only) |
| `azure_kv` | Azure Key Vault | ❌ | ✅ | ✅ | ❌ | ❌ |
| `aws_kms` | AWS KMS | ❌ | ❌ | ❌ | ❌ | ❌ (not implemented) |
| `yubihsm` | YubiHSM | ❌ | ❌ | ❌ | ❌ | ❌ (not implemented) |

## Legend
- ✅ = Fully supported and validated
- ⚠️ = Partial / in progress
- ❌ = Not supported / not implemented / not validated

## Notes
1. **Ed25519 Signing** required for audit log signing. Only `local` and `mock_kms` provide this.
2. **Key Custody** = key lifecycle (create, rotate, list versions). `azure_kv` excels here.
3. **Production Ready** = authorized for production workloads by security review.
4. **External Validation** = tested against real external system (not mocks).
```

## Template: 4.1.0 Operational Limitations

```markdown
# 4.1.0 Operational Limitations

**Release: 4.1.0 (Operations Track)**
**Date: 2026-09-16**

## Known Limitations (Explicit)

### External Validation
| Component | Limitation | Impact | Mitigation |
|-----------|------------|--------|------------|
| Entra ID | Live validation not executed (waiver 2027-06-30) | Cannot guarantee production Entra compatibility | Dry-run validation; waiver documented |
| IMDS | Live validation not executed (waiver 2027-06-30) | Cannot guarantee Azure VM metadata behavior | Dry-run validation; waiver documented |
| Key Vault | Provider implemented but not externally validated | B5 not closed | Integration test planned |

### Signing & Trust
| Component | Limitation | Impact | Mitigation |
|-----------|------------|--------|------------|
| EV Authenticode | Certificate not procured | Windows binary unsigned | Cosign keyless for Linux/macOS; SmartScreen warning on Windows |
| Provenance | SLSA provenance not generated | Supply chain verification limited | SBOM + checksums + cosign signatures |
| Action pinning | GitHub Actions not pinned to SHAs | Supply chain risk | Pinning planned for 4.1.1 |

### Runtime Support
| Platform | Status | Notes |
|----------|--------|-------|
| linux/amd64 | ✅ Supported | Fully tested |
| linux/arm64 | ⚠️ Build-verified | Built, not integration-tested |
| darwin/amd64 | ✅ Supported | Fully tested |
| darwin/arm64 | ⚠️ Build-verified | Built, not integration-tested |
| windows/amd64 | ✅ Supported | Fully tested (unsigned) |
| windows/arm64 | ❌ Not built | Ignored in goreleaser |

### Key Custody
| Provider | Limitation |
|----------|------------|
| `azure_kv` | No Ed25519 support; thread-safety not verified; no 429 handling |
| `local` | Single-machine; no HSM isolation |
| `mock_kms` | Test only; no persistence |

### Observability
- No metrics endpoint (`/metrics`)
- No health endpoint (`/healthz`, `/readyz`)
- Structured logging not implemented

### Revocation
- File-based revocation only
- No OCSP/CRL support
- No real-time revocation checking

## Planned for 4.1.1 / 4.2.0
- [ ] Action pinning to SHAs
- [ ] EV certificate procurement
- [ ] ARM64 integration testing
- [ ] Metrics/health endpoints
- [ ] OCSP/CRL revocation
- [ ] Azure KV thread-safety fixes
- [ ] External validation execution (if authorized)
```

## Documentation Generation Checklist

- [ ] Create `docs/azure-kv-provider.md` from template
- [ ] Create `docs/external-validation-guide.md` from template
- [ ] Create `docs/provider-support-matrix.md` from template
- [ ] Create `docs/4.1.0-operational-limitations.md` from template
- [ ] Update `docs/configuration-reference.md` with Azure KV config
- [ ] Update `docs/security-privacy.md` with validation boundaries
- [ ] Update `docs/release-process.md` with 4.1.0 gates
- [ ] Update `docs/artifact-verification.md` with cosign/Authenticode steps
- [ ] Update `docs/sbom-verification.md` with SPDX/CycloneDX details
- [ ] Update `README.md` with 4.1.0 status and limitations

---

*Generated by Stage 11 Phase 10 Documentation Plan*