# Stage 11 Phase 1 — Azure Key Vault Provider Audit

**Timestamp:** 2026-09-16
**Scope:** `internal/store/azure_kv_provider.go`, `internal/store/keyprovider_impl.go` (ProviderFactory)

## Implementation Status

### What Is Implemented

| Operation | Status | Notes |
|-----------|--------|-------|
| Provider initialization | ✅ | `NewAzureKVProvider`, `NewAzureKVProviderFromEnv` |
| Configuration validation | ✅ | Validates VaultURL, KeyName |
| Authentication | ✅ | DefaultAzureCredential chain (managed identity, env, CLI, etc.) |
| Key existence check | ✅ | `ensureKey` creates EC-P256 if missing |
| Key creation | ✅ | EC-P256 with Sign/Verify ops |
| Key versioning | ✅ | `loadVersions` lists all versions via pager |
| Current version tracking | ✅ | Tracks active version via `Enabled` attribute |
| Key rotation | ✅ | `RotateKey` uses native Key Vault rotation |
| Version listing | ✅ | `ListKeyVersions` returns metadata |
| Resource cleanup | ✅ | `Close` nilifies client/credential |

### What Is NOT Implemented (Intentional Design Decisions)

| Operation | Status | Reason |
|-----------|--------|--------|
| `GetSigningKey` | ❌ Returns error | Azure Key Vault **does not export private keys** |
| `GetVerificationKey` | ❌ Returns error | Azure Key Vault supports EC/RSA only, **no Ed25519** |
| JWK parsing | ⚠️ Partial | Public key extraction from Key Vault not implemented |

### KeyProvider Interface Compliance

The `KeyProvider` interface requires:
```go
GetSigningKey(ctx) (ed25519.PrivateKey, error)      // ❌ FAILS
GetVerificationKey(ctx) (ed25519.PublicKey, error)  // ❌ FAILS
GetKeyVersion(ctx) (string, error)                   // ✅ Works
RotateKey(ctx) (string, error)                       // ✅ Works
ListKeyVersions(ctx) ([]KeyVersionInfo, error)       // ✅ Works
Close() error                                        // ✅ Works
```

**VERDICT: The Azure KV provider does NOT satisfy the full KeyProvider contract.**

It implements a **subset** focused on key lifecycle management (create, rotate, list versions) but cannot provide Ed25519 keys for audit signing/verification.

## Safety Analysis

### 1. Private Key Exposure
- **PASS**: Private keys never leave Azure Key Vault. `GetSigningKey` explicitly returns error.
- No key material in logs, memory, or return values.

### 2. Secret/Key Material in Logs
- **PASS**: No logging of keys, credentials, or token material.
- `DefaultAzureCredential` handles auth internally.

### 3. Versioning Correctness
- **PARTIAL**: `loadVersions` parses version from Key ID URL.
- Uses `Enabled` attribute to determine active version.
- Fallback to latest version if no `Enabled=true` found.
- **Risk**: Race condition between `RotateKey` and `loadVersions` — rotation updates `currentVersion` then calls `loadVersions`, but another caller could see stale state.

### 4. Rotation Behavior
- **PARTIAL**: `RotateKey` calls native Key Vault rotation, then refreshes versions.
- Returns new version ID.
- **Risk**: No atomic guarantee — if `loadVersions` fails after rotation, local state inconsistent.

### 5. Error Handling
- **PARTIAL**: Wraps Azure SDK errors with context.
- **Missing**: Specific handling for `403` (access denied), `429` (rate limit), network timeouts.
- No retry policy configured on client.

### 6. Authentication Configuration
- Uses `DefaultAzureCredential` (credential chain).
- Supports: Managed Identity, Environment Variables, Azure CLI, Workload Identity, etc.
- **PASS**: No hardcoded credentials.

### 7. Context Cancellation
- **PASS**: All methods accept `context.Context` and pass to Azure SDK.
- Operations respect cancellation.

### 8. Timeout Behavior
- **RISK**: No explicit timeout configuration on `azkeys.Client`.
- Relies on Azure SDK defaults (typically 30-100s).
- Should add configurable timeout.

### 9. Retry Behavior
- **RISK**: No explicit retry policy.
- Azure SDK has default retry for transient errors, but not configured.

### 10. Dependency Usage
- **PASS**: Uses official `azure-sdk-for-go` packages.
- Minimal dependencies: `azcore`, `azidentity`, `azkeys`.

### 11. Test Coverage
- **MISSING**: No unit tests for `azure_kv_provider.go` in repo.
- No mock-based tests.
- No integration tests (require real Azure KV).

### 12. Configuration Validation
- **PASS**: Validates `VaultURL` and `KeyName` non-empty.
- **MISSING**: No validation of URL format, key name format.

## Critical Finding: Ed25519 Contract Violation

The `KeyProvider` interface is designed around **Ed25519** keys for audit signing. Azure Key Vault:
- Does not support Ed25519 keys (supports EC-P256/P-384/P-521, RSA)
- Does not export private keys (by design, HSM-grade)
- Cannot satisfy `GetSigningKey` → `ed25519.PrivateKey`
- Cannot satisfy `GetVerificationKey` → `ed25519.PublicKey`

**This is not a bug — it's a fundamental architectural mismatch.**

The Azure KV provider is a **key custody provider**, not an **audit signing provider**.

## Remaining Unvalidated Externally

1. **Real Azure KV authentication** — requires valid Azure credentials
2. **Key creation/rotation against live vault** — requires provisioned vault
3. **Access denied / rate limit / timeout scenarios** — require live environment
4. **Version consistency under concurrent access** — requires load testing
5. **Credential chain behavior** — requires different auth modes (MI, SP, CLI)

## Classification

| Category | Status |
|----------|--------|
| Unit-tested | ❌ No |
| Integration-tested (mock) | ❌ No |
| Externally validated (live Azure) | ❌ No |
| Production-authorized | ❌ No |
| Implemented (code exists) | ✅ Yes |
| KeyProvider contract satisfied | ❌ No (Ed25519 mismatch) |

## Recommendation

**Do not claim B5 "HSM/KMS Custody" as CLOSED.**

The Azure KV provider is a **valid key custody implementation** for key lifecycle operations, but it **cannot replace** the Ed25519 audit signing provider. It should be:

1. **Documented as a separate provider type** — "key custody only, not audit signing"
2. **Paired with a local/HSM Ed25519 provider** for actual audit signing
3. **Integration-tested** against a live Azure KV before any production claim
4. **Not used** where `GetSigningKey`/`GetVerificationKey` are required

---

*Generated by Stage 11 Phase 1 Azure KV Provider Audit*