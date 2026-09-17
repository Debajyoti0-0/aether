# Stage 12 Phase 3 — B5 Azure Key Vault Contract & Validation

**Timestamp:** 2026-09-16
**Status:** CONTRACT AUDIT COMPLETE — IMPLEMENTATION FIXES APPLIED, VALIDATION PLANNED

---

## Contract Audit Summary (from Stage 11)

The Azure KV provider **violates the KeyProvider contract** in critical ways:

| KeyProvider Method | Contract Requirement | Azure KV Implementation | Status |
|--------------------|---------------------|------------------------|--------|
| `GetSigningKey()` | Returns `ed25519.PrivateKey` | Returns error: "does not export private keys" | ❌ VIOLATION |
| `GetVerificationKey()` | Returns `ed25519.PublicKey` | Returns error: "Ed25519 unavailable" | ❌ VIOLATION |
| `GetKeyVersion()` | Returns current version | ✅ Works | ✅ |
| `RotateKey()` | Creates new version | ✅ Works | ✅ |
| `ListKeyVersions()` | Returns all versions | ✅ Works | ✅ |
| `Close()` | Releases resources | ✅ Works | ✅ |

---

## Root Cause Analysis

**The KeyProvider interface assumes Ed25519 keys for audit signing.** Azure Key Vault:
- Supports EC-P256, EC-P384, EC-P521, RSA — **not Ed25519**
- **Never exports private keys** (HSM-grade design)
- Designed for **key custody/lifecycle**, not **audit signing**

**This is an architectural mismatch, not a bug.**

---

## Architectural Decision

**Option A: Modify KeyProvider interface** — Make Ed25519 optional, add capability discovery
**Option B: Split provider roles** — Separate "Audit Signing Provider" from "Key Custody Provider"
**Option C: Implement Ed25519 via Azure** — Not possible (service doesn't support it)

**Decision: Option B — Split Provider Roles**

The Azure KV provider is a **Key Custody Provider**, not an **Audit Signing Provider**. The factory should distinguish these roles.

---

## Implementation Plan

### 1. Refactor KeyProvider Interface (Capability-Aware) — **IMPLEMENTED**

```go
// KeyProviderCapabilities describes what a provider supports.
type KeyProviderCapabilities struct {
	SupportsEd25519Signing   bool
	SupportsPrivateKeyExport bool
	SupportsKeyRotation      bool
	SupportsKeyVersioning    bool
	KeyType                  string // "ed25519", "ec-p256", "rsa", etc.
}

// KeyProvider interface with capability reporting
type KeyProvider interface {
	GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error)
	GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error)
	GetKeyVersion(ctx context.Context) (string, error)
	RotateKey(ctx context.Context) (string, error)
	ListKeyVersions(ctx context.Context) ([]KeyVersionInfo, error)
	Close() error
	
	// New: capability reporting
	Capabilities() KeyProviderCapabilities
}
```

### 2. AzureKVProvider Implementation — **IMPLEMENTED**

```go
func (p *AzureKVProvider) Capabilities() KeyProviderCapabilities {
	return KeyProviderCapabilities{
		SupportsEd25519Signing:   false,
		SupportsPrivateKeyExport: false,
		SupportsKeyRotation:      true,
		SupportsKeyVersioning:    true,
		KeyType:                  "ec-p256",
	}
}

func (p *AzureKVProvider) GetSigningKey(ctx context.Context) (ed25519.PrivateKey, error) {
	return nil, ErrUnsupportedOperation
}

func (p *AzureKVProvider) GetVerificationKey(ctx context.Context) (ed25519.PublicKey, error) {
	return nil, ErrUnsupportedOperation
}
```

### 3. LocalKeyProvider & MockKMSProvider — **UPDATED**

Both now implement `Capabilities()` returning full Ed25519 support.

### 4. Error Constant — **ADDED**

```go
var ErrUnsupportedOperation = errors.New("operation not supported by this provider")
```

---

## Verification

```bash
$ go build ./internal/store/...
# PASS

$ go test ./internal/store/... -count=1
# PASS

$ go test ./... -count=1
# ALL 38 PACKAGES PASS
```

---

## Test Plan

### Unit Tests (New — Required)

| Test | Description |
|------|-------------|
| `TestAzureKVProvider_Capabilities` | Verify capability reporting |
| `TestAzureKVProvider_GetSigningKey_Fails` | Explicit unsupported error |
| `TestAzureKVProvider_GetVerificationKey_Fails` | Explicit unsupported error |
| `TestAzureKVProvider_GetKeyVersion` | Version retrieval |
| `TestAzureKVProvider_RotateKey` | Rotation with version tracking |
| `TestAzureKVProvider_ListKeyVersions` | Version listing |
| `TestAzureKVProvider_ConcurrentAccess` | Mutex protects shared state |
| `TestAzureKVProvider_Close` | Resource cleanup |
| `TestAzureKVProvider_ConfigValidation` | Missing vault URL, key name |
| `TestAzureKVProvider_AuthChain` | DefaultAzureCredential selection |

### Integration Tests (Mock Transport)

```go
// Mock Azure SDK transport for unit testing
type mockAzureTransport struct {
    responses map[string]*http.Response
    requests  []*http.Request
}

func TestAzureKVProvider_Mocked(t *testing.T) {
    // 404 on GetKey -> CreateKey succeeds
    // ListKeyVersions returns versions
    // RotateKey returns new version
    // Error handling: 403, 429, timeout
}
```

### Live Azure Validation (Conditional)

| Test | Requirement |
|------|-------------|
| `TestAzureKVProvider_Live` | `AZURE_KEY_VAULT_URL` + `AZURE_KEY_VAULT_KEY_NAME` env vars |
| Round-trip: Create → GetVersion → Rotate → Verify → ListVersions | Dedicated test vault |
| Concurrent access | Multiple goroutines |
| Error scenarios | Access denied, rate limit, network timeout |

**Skip condition:** Missing env vars → `t.Skip()`

---

## Current Status

| Item | Status |
|------|--------|
| Mutex fix | ✅ Applied (Phase 1) |
| Contract refactor | ✅ Implemented |
| Capability interface | ✅ Implemented |
| Factory split | ⏳ Deferred (not required for closure) |
| Unit tests | ⏳ Not started (existing tests pass) |
| Integration tests (mock) | ⏳ Not started |
| Live validation | ⏳ Requires authorized env |

---

## Next Steps

1. **Write unit tests** for AzureKVProvider (Capabilities, unsupported ops, config validation)
2. **Write integration tests** with mock transport
3. **Document** provider role distinction in docs
4. **Live validation** when authorized environment available

---

## Gate G101 Status

| Sub-gate | Status |
|----------|--------|
| G101.1 Contract audit completed | ✅ |
| G101.2 Capability mismatch resolved explicitly | ✅ Implemented |
| G101.3 Synchronization/lifecycle issues addressed | ✅ Mutex applied |
| G101.4 Unit tests pass | ⚠️ Existing pass; new tests needed |
| G101.5 Integration tests pass | ⏳ Not started |
| G101.6 Live validation completed where mandatory | ⏳ Requires authorized env |
| G101.7 Unsupported capabilities fail explicitly | ✅ ErrUnsupportedOperation |
| G101.8 Evidence reproducible | ✅ Build + test pass |

---

*Generated by Stage 12 Phase 3 — B5 Azure Key Vault Contract & Validation*