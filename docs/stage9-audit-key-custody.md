# Stage 9 G5 — Audit-Key Custody and Rotation Closure

**Date:** 2026-09-13
**Baseline:** Stage 9 G4 Authenticode Closure
**Blocker:** B5 — Audit key HSM/KMS custody (replace plaintext Ed25519 seed in workspace)

---

## 1. KeyProvider Architecture

| Provider | Type | Status | Use Case |
|----------|------|--------|----------|
| `LocalKeyProvider` | Encrypted local file | ✅ IMPLEMENTED | Development, testing, air-gapped |
| `MockKMSProvider` | In-memory mock | ✅ IMPLEMENTED | Unit/integration testing |
| `AzureKeyVaultProvider` | Azure Key Vault | 🔄 NOT IMPLEMENTED | Azure production |
| `AWSKMSProvider` | AWS KMS | 🔄 NOT IMPLEMENTED | AWS production |
| `YubiHSMProvider` | YubiHSM PKCS#11 | 🔄 NOT IMPLEMENTED | Hardware-backed production |

**Interface:** `KeyProvider` (in `internal/store/keyprovider.go`)
- `GetSigningKey(ctx)` — returns Ed25519 private key for signing
- `GetVerificationKey(ctx)` — returns Ed25519 public key for verification
- `GetKeyVersion(ctx)` — returns current key version
- `RotateKey(ctx)` — generates new key, archives old, returns new version
- `ListKeyVersions(ctx)` — returns all key versions with metadata
- `Close()` — releases resources

**Factory:** `ProviderFactory.CreateProvider(cfg ProviderConfig)` — creates provider from config

---

## 2. LocalKeyProvider Implementation

### 2.1 Security Model

| Aspect | Implementation |
|--------|----------------|
| Encryption | XChaCha20-Poly1305 (AEAD) |
| Key Derivation | Argon2id (3 iterations, 64MB memory, 4 threads) |
| Salt | 16-byte random per file |
| Nonce | 24-byte random per encryption |
| File Format | salt(16) || nonce(24) || ciphertext |
| Key Storage | Private key derived deterministically from passphrase + version |
| Rotation | New random key, old key archived (metadata only) |

### 2.2 Key Derivation

```go
// Deterministic derivation from passphrase + version
salt := SHA256("aether-key-derivation:" + version)
key := Argon2id(passphrase, salt, t=3, m=64MB, p=4, keylen=32)
privateKey := Ed25519.NewKeyFromSeed(key[:32])
```

**Properties:**
- Same passphrase + version → same private key (deterministic)
- No private key stored in encrypted file (only metadata)
- Passphrase required for every operation
- Wrong passphrase → decryption failure (fail-closed)

### 2.3 Rotation

```go
func (p *LocalKeyProvider) RotateKey(ctx context.Context) (string, error) {
	// 1. Generate new random Ed25519 key
	// 2. Archive current version (Active=false)
	// 3. Set new version as current (Active=true)
	// 4. Save encrypted state
	// Returns new version string (e.g., "v2")
}
```

---

## 3. MockKMSProvider Implementation

For testing without external dependencies:
- In-memory key storage
- Same interface as production providers
- Used in integration tests

---

## 4. Security Analysis

### 4.1 Threats Mitigated

| Threat | Mitigation |
|--------|------------|
| Workspace file theft | Keys encrypted with passphrase; Argon2id + XChaCha20-Poly1305 |
| Passphrase brute-force | Argon2id memory-hard KDF (64MB, 3 iterations) |
| Ciphertext tampering | AEAD (XChaCha20-Poly1305) provides integrity |
| Key reuse across versions | Deterministic derivation per version; rotation creates new random key |
| Concurrent access corruption | Mutex-protected operations |

### 4.2 Residual Risks (Local Provider)

| Risk | Severity | Mitigation |
|------|----------|------------|
| Passphrase in memory | MEDIUM | Clear on Close(); OS memory protection |
| Keylogger captures passphrase | HIGH | Out of scope; use hardware provider for production |
| Malware reads decrypted key | HIGH | Out of scope; use HSM/KMS for production |
| Backup file contains keys | MEDIUM | Encrypted at rest; passphrase required |

---

## 5. Production Deployment Requirements

For production GA release, **at least one** HSM/KMS backend MUST be implemented:

| Backend | Integration Points | Status |
|---------|-------------------|--------|
| Azure Key Vault | Managed HSM, Key Vault SDK, Managed Identity | 🔄 NOT IMPLEMENTED |
| AWS KMS | KMS SDK, IAM roles, cross-account | 🔄 NOT IMPLEMENTED |
| YubiHSM | PKCS#11, yubihsm-go, physical device | 🔄 NOT IMPLEMENTED |
| HashiCorp Vault | Transit engine, AppRole auth | 🔄 NOT IMPLEMENTED |

### Minimum Production Requirements

1. **Key generation in HSM** — Private key never leaves HSM boundary
2. **Signing in HSM** — `GetSigningKey` returns HSM-backed signer, not raw key
3. **Rotation** — HSM-managed rotation with versioning
4. **Audit trail** — HSM logs all sign/rotate operations
5. **Disaster recovery** — HSM backup/restore procedures documented
6. **High availability** — HSM cluster or multi-region KMS

---

## 6. Current Status

| Component | Status | Evidence |
|-----------|--------|----------|
| KeyProvider interface | ✅ IMPLEMENTED | `internal/store/keyprovider.go` |
| LocalKeyProvider | ✅ IMPLEMENTED | `internal/store/keyprovider_impl.go` |
| MockKMSProvider | ✅ IMPLEMENTED | `internal/store/keyprovider_impl.go` |
| ProviderFactory | ✅ IMPLEMENTED | `internal/store/keyprovider_impl.go` |
| Unit tests | ✅ PASS | `internal/store/keyprovider_test.go` |
| Integration tests | ✅ PASS | Key rotation, concurrent access, sign/verify |
| Azure Key Vault provider | ❌ NOT IMPLEMENTED | Requires Azure SDK |
| AWS KMS provider | ❌ NOT IMPLEMENTED | Requires AWS SDK |
| YubiHSM provider | ❌ NOT IMPLEMENTED | Requires PKCS#11 |

---

## 7. Waiver Decision

**Decision:** BLOCKER PARTIALLY CLOSED — Local provider implemented and tested; HSM/KMS backends required for production GA.

**Reasoning:**
- Local encrypted provider provides strong security for development/test/air-gapped
- Production GA requires HSM/KMS for true key isolation (keys never in memory)
- Implementation ready for HSM/KMS integration (interface defined, factory pattern)
- Can release `4.0.0-rc1` with local provider, document HSM/KMS as production requirement

**Production Requirement for `4.0.0` GA:**
- Implement ≥1 HSM/KMS backend (Azure KMS, AWS KMS, or YubiHSM)
- Document deployment procedures
- Test rotation and recovery in CI

---

## 8. G5 Result

**PARTIAL** — Local encrypted provider and mock KMS fully implemented with tests. HSM/KMS production backends remain for GA.

**Next Step:** Implement Azure Key Vault or AWS KMS provider for production deployment.

---

**Prepared by:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Status:** G5 PARTIAL — Development provider complete, production HSM/KMS pending