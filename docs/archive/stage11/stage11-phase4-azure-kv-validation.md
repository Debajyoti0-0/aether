# Stage 11 Phase 4 — Azure Key Vault External Validation Plan

**Timestamp:** 2026-09-16
**Status:** PLANNED — Requires authorized Azure environment

## Current Implementation Status

From Phase 1 audit: **Azure KV provider implements key lifecycle operations but NOT Ed25519 audit signing.**

| Operation | Implemented | Notes |
|-----------|-------------|-------|
| Provider init (env/config) | ✅ | `NewAzureKVProvider`, `NewAzureKVProviderFromEnv` |
| Auth (DefaultAzureCredential) | ✅ | MI, SP, CLI, Workload Identity |
| Key existence/create (EC-P256) | ✅ | Creates if missing |
| Key versioning | ✅ | Lists all versions via pager |
| Current version tracking | ✅ | Via `Enabled` attribute |
| Key rotation | ✅ | Native Key Vault rotation |
| Version listing | ✅ | Metadata only (no public key) |
| `GetSigningKey` | ❌ | **Returns error** — KV doesn't export private keys |
| `GetVerificationKey` | ❌ | **Returns error** — KV doesn't support Ed25519 |
| Close/cleanup | ✅ | Nilifies client |

## Authentication Design Audit

### Supported Mechanisms (via `DefaultAzureCredential`)
1. **Environment Variables** — `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_SECRET`
2. **Managed Identity** — System-assigned or user-assigned (Azure VM, App Service, AKS)
3. **Azure CLI** — `az login` credentials (local dev)
4. **Workload Identity** — Kubernetes service account token exchange
5. **Azure PowerShell** — `Connect-AzAccount` context
6. **Developer CLI** — `azd auth login`

### Configuration
```go
type AzureKVConfig struct {
    VaultURL   string                    // Required: "https://myvault.vault.azure.net"
    KeyName    string                    // Required: "aether-audit-key"
    Credential azcore.TokenCredential    // Optional: nil = DefaultAzureCredential
}
```

### Environment Variables for `NewAzureKVProviderFromEnv`
- `AZURE_KEY_VAULT_URL` — Vault URI
- `AZURE_KEY_VAULT_KEY_NAME` — Key name

## Safe Test Scope Definition

### Required Authorization
- **Tenant**: Dedicated test tenant (not production)
- **Subscription**: Isolated test subscription
- **Resource Group**: Dedicated RG for test resources
- **Key Vault**: New or existing vault with purge protection **disabled**
- **Test Identity**: Service Principal or Managed Identity with minimal RBAC:
  - `Key Vault Secrets User` (for key operations)
  - `Key Vault Crypto User` (for sign/verify — though not used by this provider)

### Test Resource Naming
- Vault: `aether-test-kv-<unique-suffix>`
- Key: `aether-test-key-<timestamp>`
- Cleanup: Automatic deletion after test completion

### Permissions Required (Minimum)
```json
{
  "keyPermissions": ["get", "list", "create", "rotate", "update"]
}
```
**NOT required**: `delete`, `recover`, `backup`, `restore`, `sign`, `verify` (provider doesn't use these)

## Minimum Non-Destructive Validation Plan

### Phase 4A: Unit Test Mock Validation (No Azure Required)
```go
// Mock Azure SDK transport for unit testing
func TestAzureKVProvider_Mocked(t *testing.T) {
    // Test: config validation
    // Test: auth chain selection
    // Test: key creation logic (mock 404 -> create)
    // Test: version parsing from KID
    // Test: rotation flow (mock RotateKey response)
    // Test: error handling (403, 429, timeout)
}
```

### Phase 4B: Live Integration Test (Requires Azure Credentials)

**Prerequisites:**
```bash
export AZURE_KEY_VAULT_URL="https://aether-test-kv-xxx.vault.azure.net"
export AZURE_KEY_VAULT_KEY_NAME="aether-test-key-$(date +%s)"
# Auth via one of: az login, SP env vars, MI
```

**Test Sequence:**

| Step | Operation | Expected | Cleanup |
|------|-----------|----------|---------|
| 1 | `NewAzureKVProviderFromEnv` | Provider created, key exists/created | — |
| 2 | `GetKeyVersion` | Returns version string | — |
| 3 | `ListKeyVersions` | Returns []KeyVersionInfo with metadata | — |
| 4 | `RotateKey` | Returns new version, old marked inactive | — |
| 5 | `GetKeyVersion` (post-rotate) | Returns new version | — |
| 6 | `ListKeyVersions` (post-rotate) | Shows both versions, one active | — |
| 7 | `Close` | No error | — |
| 8 | Re-init provider | Loads existing key versions | — |
| 9 | Multiple rotations | Each creates new version | — |
| 10 | Delete test key | **MANUAL** - provider doesn't support delete | Manual |

### Phase 4C: Key Custody Verification

| Check | Method | Pass Criteria |
|-------|--------|---------------|
| Non-exportable keys | Key Vault policy / key attributes | Private key never returned |
| Key type | Key Vault key properties | EC-P256 (not Ed25519) |
| Version immutability | Rotate → list versions | Old versions retained |
| Access control | RBAC on test identity | Only authorized operations work |
| Audit logging | Key Vault diagnostic logs | Operations logged |
| No secret leakage | Code review + log inspection | No keys/tokens in output |

## External Validation Result Classification

| Outcome | Criteria |
|---------|----------|
| **PASS** | All Phase 4B steps succeed; key custody verified |
| **PASS WITH LIMITATIONS** | Core operations work; Ed25519 gap documented |
| **BLOCKED** | Cannot provision test vault / auth fails |
| **FAIL** | Any core operation fails unexpectedly |
| **NOT RUN** | No authorized Azure environment available |

## Current Status: NOT RUN

**Reason**: No authorized Azure test environment configured in this execution context.

**Required for Execution:**
1. Authorized Azure subscription/tenant
2. Service Principal or Managed Identity with Key Vault permissions
3. Test Key Vault provisioned (or permission to create)
4. Environment variables set
5. Explicit approval for live cloud operations

## Integration with KeyProvider Contract

**CRITICAL ARCHITECTURAL NOTE:**

The Azure KV provider **cannot satisfy** the `KeyProvider` interface for audit signing because:
1. Azure Key Vault does not support Ed25519 keys
2. Azure Key Vault does not export private keys (by design)

**Recommended Architecture:**
```
Audit Signing (Ed25519)     Key Custody (EC-P256/RSA)
┌─────────────────────┐     ┌─────────────────────┐
│ LocalKeyProvider    │     │ AzureKVProvider     │
│ MockKMSProvider     │     │ AWS KMS (future)    │
│ YubiHSM (future)    │     │                     │
└─────────────────────┘     └─────────────────────┘
       ↑                            ↑
   Signing keys               Key lifecycle only
   (exportable)               (non-exportable)
```

The Azure KV provider should be registered as a **key custody provider**, not an **audit signing provider**. The factory should distinguish these roles.

---

*Generated by Stage 11 Phase 4 Azure KV Validation Plan*