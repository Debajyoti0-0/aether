# Stage 11 Phase 9 — Testing Requirements

**Timestamp:** 2026-09-16
**Scope:** Unit, integration, adversarial, race, and external validation tests

## Test Coverage Matrix

| Layer | Target | Current | Required for Phase A |
|-------|--------|---------|---------------------|
| Unit | All packages | ✅ 34 packages pass | ✅ Maintain |
| Integration | `test/integration` | ✅ Passes | ✅ Maintain |
| Race | All packages | ❌ CI fails | ✅ Must fix |
| Fuzz | 6 native targets | ✅ Passes | ✅ Maintain |
| Adversarial | New | ❌ Not exist | ✅ Must add |
| External (Azure) | Azure KV | ❌ Not run | ✅ If authorized |
| External (Entra) | Entra ID | ❌ Not run | ⏳ Phase B |
| External (IMDS) | Azure VM | ❌ Not run | ⏳ Phase B |
| CLI | Commands | ⚠️ Partial | ✅ Expand |
| Artifact | Build outputs | ✅ Verified | ✅ Maintain |

---

## 9.1 Unit Tests (All Modified Logic)

### New Tests Required

| Component | Test Cases |
|-----------|------------|
| `AzureKVProvider` | Init (valid/invalid config), auth chain, key create/exists, version parsing, rotation, list versions, close, error handling (403, 429, timeout, network) |
| `ProviderFactory` | Create each provider type, invalid type, missing config fields |
| `Workspace` | Rekey (keyless→passphrase, passphrase→passphrase), concurrent SaveRecord/LoadRecord, migration path |
| `Teamserver` | Concurrent publish/subscribe/unsubscribe (race test), connection limits, in-flight cap, shutdown |
| `Vault` | Concurrent transactions, schema migration, rollback failed retention |
| `RollbackStack` | Push/pop/peek/list under concurrent access |
| `AuditLog` | Concurrent append, hash chain integrity, meta set-if-absent |
| `KeyProvider` interface | Contract tests for all implementations |

### Test Patterns

```go
// Table-driven tests for provider factory
func TestProviderFactory_CreateProvider(t *testing.T) {
    tests := []struct {
        name        string
        cfg         ProviderConfig
        wantErr     bool
        errContains string
    }{
        {"local_valid", ProviderConfig{Type: "local", Path: "/tmp/test", Passphrase: "pass"}, false, ""},
        {"local_missing_path", ProviderConfig{Type: "local", Passphrase: "pass"}, true, "path"},
        {"mock_kms", ProviderConfig{Type: "mock_kms"}, false, ""},
        {"azure_kv_no_env", ProviderConfig{Type: "azure_kv"}, true, "env vars required"},
        {"unknown", ProviderConfig{Type: "unknown"}, true, "unknown provider"},
    }
    // ...
}

// Concurrency stress test
func TestWorkspace_ConcurrentRecordAccess(t *testing.T) {
    w := createTestWorkspace(t)
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(2)
        go func(i int) { defer wg.Done(); w.SaveRecord("b", fmt.Sprintf("k%d", i), "v") }(i)
        go func(i int) { defer wg.Done(); w.LoadRecord("b", fmt.Sprintf("k%d", i), &out) }(i)
    }
    wg.Wait()
}
```

---

## 9.2 Integration Tests

### Local Integration (No External Dependencies)

| Test | Scope | Dependencies |
|------|-------|--------------|
| `TestVaultACID` | bbolt transactions, crash safety | None (temp dir) |
| `TestWorkspaceMigration` | Legacy → vault migration | None (temp dir) |
| `TestTeamserverTLS` | mTLS auth, cert validation | Local CA (test PKI) |
| `TestWorkspaceRekey` | Keyless ↔ passphrase | None |
| `TestRollbackCrashRecovery` | Simulated crash during push/pop | None |
| `TestAuditChainIntegrity` | Hash chain, tamper detection | None |
| `TestIdempotencyPutIfAbsent` | Concurrent deduplication | None |

### Mock-Based Azure Integration

```go
// Mock Azure SDK transport for unit testing
type mockAzureTransport struct {
    responses map[string]*http.Response
    requests  []*http.Request
}

func (m *mockAzureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    m.requests = append(m.requests, req)
    if resp, ok := m.responses[req.URL.Path]; ok {
        return resp, nil
    }
    return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestAzureKVProvider_Mocked(t *testing.T) {
    // Mock: 404 on GetKey -> CreateKey succeeds -> ListKeyVersions returns versions
    // Test: ensureKey creates on 404, loadVersions parses KIDs, RotateKey calls API
}
```

### External Integration (Opt-In, Requires Authorization)

| Test | Environment | Skip Condition |
|------|-------------|----------------|
| `TestAzureKVProvider_Live` | Real Azure KV | `AZURE_KEY_VAULT_URL` not set |
| `TestEntraTokenAcquisition` | Real Entra tenant | `ENTRA_TENANT_ID` not set |
| `TestIMDSMetadata` | Real Azure VM | `IMDS_VALIDATION` not set |

**Skip Pattern:**
```go
func TestAzureKVProvider_Live(t *testing.T) {
    vaultURL := os.Getenv("AZURE_KEY_VAULT_URL")
    if vaultURL == "" {
        t.Skip("AZURE_KEY_VAULT_URL not set; skipping live Azure KV test")
    }
    // ... test with real vault
}
```

---

## 9.3 Adversarial Tests

### Security Invariant Tests

| Invariant | Test | Expected |
|-----------|------|----------|
| No secret in logs | Run all tests with log capture, grep for secrets | Zero matches |
| No secret in errors | Trigger all error paths, inspect error messages | No keys/tokens/secrets |
| No secret in evidence | Run validation adapters, inspect output | Redacted fields only |
| Fail-closed on auth failure | Invalid certs, revoked tokens, wrong passphrase | All return error, no partial success |
| No credential persistence | Run validation, check memory/heap | No tokens/secrets retained |
| No uncontrolled enumeration | Call validation without profile | Returns error/skip |
| Tamper detection | Modify artifacts, verify fail-closed | All 9 tamper vectors fail |

### Adversarial Test Suite

```go
// TestAdversarial_SecretLeakageInErrors
func TestAdversarial_SecretLeakageInErrors(t *testing.T) {
    // Trigger every error path in:
    // - AzureKVProvider (auth fail, 403, 429, timeout)
    // - Workspace (wrong passphrase, corrupt salt, tampered vault)
    // - Teamserver (invalid cert, revoked cert, malformed request)
    // - Vault (locked, corrupt, schema mismatch)
    // Assert: no error message contains "key", "secret", "token", "password", "passphrase"
}

// TestAdversarial_TamperResistance
func TestAdversarial_TamperResistance(t *testing.T) {
    // For each artifact type:
    // 1. Flip single bit in binary
    // 2. Modify SBOM component version
    // 3. Change checksum in checksums.txt
    // 4. Replace signature file
    // 5. Modify manifest sha256
    // Assert: verification fails for all
}

// TestAdversarial_UntrustedWorkflowContext
func TestAdversarial_UntrustedWorkflowContext(t *testing.T) {
    // Simulate compromised CI:
    // 1. Modified go.mod (supply chain)
    // 2. Injected env vars
    // 3. Tampered goreleaser config
    // Assert: build fails or produces verifiably different artifacts
}

// TestAdversarial_RaceConditions
func TestAdversarial_RaceConditions(t *testing.T) {
    // Run with -race (when available):
    // 1. Concurrent workspace operations
    // 2. Concurrent teamserver publish/subscribe
    // 3. Concurrent vault transactions
    // 4. Concurrent audit log appends
    // Assert: no data races detected
}
```

---

## 9.4 CLI Tests

### Commands to Test

| Command | Test Cases |
|---------|------------|
| `aether serve` | Cert init, start/stop, config validation, TLS errors |
| `aether workspace` | Create, open, list, delete, rekey, migration |
| `aether cert` | Issue, revoke, verify, chain validation |
| `aether validate` | Entra (dry-run), IMDS (dry-run), Key Vault |
| `aether export` | Evidence, SBOM, manifest, verification |
| `aether --version` | Version output, commit, build info |

### CLI Test Patterns

```go
func TestCLI_WorkspaceRekey(t *testing.T) {
    // 1. Create keyless workspace
    // 2. Rekey with passphrase
    // 3. Open with passphrase
    // 4. Rekey with new passphrase
    // 5. Open with new passphrase
    // 6. Verify vault accessible throughout
}

func TestCLI_ValidateEntraDryRun(t *testing.T) {
    // Without auth env vars
    out := runCLI(t, "validate", "entra", "--dry-run")
    assert.Contains(out, "dry-run")
    assert.NotContains(out, "access_token")
}
```

---

## 9.5 Race Testing

### Strategy

Since local race detector unavailable:
1. **CI-based isolation workflow** (see Phase 2)
2. **Code review** for known patterns
3. **Fix known issues** (AzureKVProvider mutex, Workspace pass mutex)
4. **Add `-race` to CI matrix** for all packages

### CI Race Isolation Workflow

```yaml
# .github/workflows/race-isolate.yml
name: Race Detector Isolation

on:
  workflow_dispatch:
  push:
    branches: [main]

jobs:
  race-api:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27.x' }
      - run: go test -race -count=1 ./internal/api/...

  race-workspace:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27.x' }
      - run: go test -race -count=1 ./internal/workspace/...

  race-store:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27.x' }
      - run: go test -race -count=1 ./internal/store/...

  race-engine:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27.x' }
      - run: go test -race -count=1 ./internal/engine/...

  race-protocol:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27.x' }
      - run: go test -race -count=1 ./internal/protocol/...

  race-all:
    needs: [race-api, race-workspace, race-store, race-engine, race-protocol]
    runs-on: ubuntu-latest
    steps:
      - run: echo "All race isolation jobs completed"
```

---

## 9.6 External Validation Tests

### Azure KV Integration Test (When Authorized)

```go
//go:build integration && azure_kv_live

package integration

func TestAzureKVProvider_LiveIntegration(t *testing.T) {
    vaultURL := os.Getenv("AZURE_KEY_VAULT_URL")
    keyName := os.Getenv("AZURE_KEY_VAULT_KEY_NAME")
    if vaultURL == "" || keyName == "" {
        t.Skip("Azure KV env vars not set")
    }

    ctx := context.Background()
    provider, err := store.NewAzureKVProviderFromEnv(ctx)
    require.NoError(t, err)
    defer provider.Close()

    // 1. GetKeyVersion
    version, err := provider.GetKeyVersion(ctx)
    require.NoError(t, err)
    assert.NotEmpty(t, version)

    // 2. ListKeyVersions
    versions, err := provider.ListKeyVersions(ctx)
    require.NoError(t, err)
    assert.NotEmpty(t, versions)

    // 3. RotateKey
    newVersion, err := provider.RotateKey(ctx)
    require.NoError(t, err)
    assert.NotEqual(t, version, newVersion)

    // 4. Verify rotation
    versions, err = provider.ListKeyVersions(ctx)
    require.NoError(t, err)
    found := false
    for _, v := range versions {
        if v.Version == newVersion && v.Active {
            found = true
            break
        }
    }
    assert.True(t, found, "new version should be active")

    // 5. Verify old version inactive
    for _, v := range versions {
        if v.Version == version {
            assert.False(t, v.Active, "old version should be inactive")
        }
    }
}
```

### Entra/IMDS Validation Tests (When Authorized)

```go
//go:build integration && entra_live

func TestEntraValidation_Live(t *testing.T) {
    tenantID := os.Getenv("ENTRA_TENANT_ID")
    if tenantID == "" {
        t.Skip("ENTRA_TENANT_ID not set")
    }
    // Run validation profile, collect sanitized evidence
    // Assert: evidence.Verification == "PASS"
    // Assert: no redacted fields in evidence
}
```

---

## 9.7 Quality Gates (Must Pass)

```bash
# Unit tests
go test -count=1 ./...

# Integration tests
go test -tags=integration -count=1 ./test/integration/...

# Vet
go vet ./...

# Lint
golangci-lint run --timeout 5m

# Govulncheck
govulncheck ./...

# Fuzz (smoke)
go test -fuzz=FuzzParseAssertion -fuzztime=10s ./internal/protocol/saml/...
go test -fuzz=FuzzParseRSTR -fuzztime=10s ./internal/protocol/wstrust/...
go test -fuzz=FuzzDecodeKey -fuzztime=10s ./internal/protocol/msoapx/...
go test -fuzz=FuzzDecodePayload -fuzztime=10s ./internal/api/...
go test -fuzz=FuzzParsePRT -fuzztime=10s ./internal/engine/token/...
go test -fuzz=FuzzParseIMDSIdentityToken -fuzztime=10s ./internal/engine/exec/...
go test -fuzz=FuzzLoadRecord -fuzztime=10s ./internal/workspace/...

# Race (CI only)
go test -race -count=1 ./...

# Build
go build ./...

# Goreleaser snapshot
goreleaser release --snapshot --clean

# Artifact verification
sha256sum -c dist/checksums.txt
```

---

## 9.8 Test Evidence Collection

All test runs must produce:
- **Exit code** (0 = pass)
- **Duration**
- **Package-level results**
- **Race detector output** (if applicable)
- **Coverage** (optional but recommended)

```bash
# Example: JUnit output for CI
go test -count=1 -v ./... 2>&1 | tee test-results.txt
go test -tags=integration -count=1 -v ./test/integration/... 2>&1 | tee integration-results.txt
```

---

*Generated by Stage 11 Phase 9 Testing Requirements*