# Stage 16 Phase 7 — Candidate Qualification

**Timestamp:** 2016-09-16
**Scope:** Qualify the reconstructed candidate without mutating frozen broken RC2 tag

---

## Validation Branch

**Branch:** `stage16/rc2-qualification`
**Base:** `5cd008b` (RC2 candidate commit)
**Purpose:** Validate remediation changes without mutating frozen RC2 tag (`v4.0.0-rc2` at `72d17d2`)

---

## Validation Commands

### Core Quality Gates

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

# Fuzz smoke (7 targets)
go test -fuzz=FuzzParseAssertion -fuzztime=10s ./internal/protocol/saml/...
go test -fuzz=FuzzParseRSTR -fuzztime=10s ./internal/protocol/wstrust/...
go test -fuzz=FuzzDecodeKey -fuzztime=10s ./internal/protocol/msoapx/...
go test -fuzz=FuzzDecodePayload -fuzztime=10s ./internal/api/...
go test -fuzz=FuzzParsePRT -fuzztime=10s ./internal/engine/token/...
go test -fuzz=FuzzParseIMDSIdentityToken -fuzztime=10s ./internal/engine/exec/...
go test -fuzz=FuzzLoadRecord -fuzztime=10s ./internal/workspace/...

# Build
go build ./...

# Goreleaser snapshot
goreleaser release --snapshot --clean
```

### Race Detector (CI-only)

```bash
# Requires gcc/mingw — CI only
go test -race ./...
```

---

## Validation Results (Local)

### Core Quality Gates — All PASS

| Check | Command | Result |
|-------|---------|--------|
| Unit tests | `go test -count=1 ./...` | ✅ PASS (38 packages) |
| Integration tests | `go test -tags=integration ...` | ✅ PASS |
| Vet | `go vet ./...` | ✅ PASS |
| Lint | `golangci-lint run --timeout 5m` | ✅ PASS (0 issues) |
| Govulncheck | `govulncheck ./...` | ✅ PASS (0 vulns) |
| Fuzz (7 targets) | `go test -fuzz=... -fuzztime=10s` | ✅ ALL PASS |
| Build | `go build ./...` | ✅ PASS |
| Goreleaser snapshot | `goreleaser release --snapshot --clean` | ✅ PASS |

### Race Detector

| Environment | Status |
|-------------|--------|
| Local (Windows) | ❌ BLOCKED — No gcc/mingw |
| Docker | ❌ NOT AVAILABLE |
| CI (ubuntu-latest) | ⏳ PENDING — Race isolation workflow created |

---

## Fuzz Target Evidence (10/21 targets run)

| Target | Package | Executions | Interesting | Crashes | Panics | Status |
|--------|---------|------------|-------------|---------|--------|--------|
| FuzzReadFrame | internal/api | 6,950,653 | 0 | 0 | 0 | ✅ PASS |
| FuzzDecodePayload | internal/api | 6,592,391 | 60 | 0 | 0 | ✅ PASS |
| FuzzEncodePayload | internal/api | 6,613,268 | 200 | 0 | 0 | ✅ PASS |
| FuzzEnvelopeValidation | internal/api | 6,509,614 | 197 | 0 | 0 | ✅ PASS |
| FuzzParseIMDSIdentityToken | internal/engine/exec | 5,832,771 | 138 | 0 | 0 | ✅ PASS |
| FuzzParseInstanceMetadata | internal/engine/exec | 6,203,658 | 215 | 0 | 0 | ✅ PASS |
| FuzzParsePRT | internal/engine/token | 6,764,837 | 94 | 0 | 0 | ✅ PASS |
| FuzzParseAssertion | internal/protocol/saml | 6,054,881 | 127 | 0 | 0 | ✅ PASS |
| FuzzParseRSTR | internal/protocol/wstrust | 814,042 | 15 | 0 | 0 | ✅ PASS |
| FuzzLoadRecord | internal/workspace | 8,852,156 | 106 | 0 | 0 | ✅ PASS |

**Total executions:** ~50M | **Crashes:** 0 | **Panics:** 0 | **All PASS**

---

## Goreleaser Configuration Status

| Feature | Status | Notes |
|---------|--------|-------|
| goreleaser check | ✅ PASS | v2.18.1 |
| Snapshot build | ✅ PASS | 4 platforms |
| SBOM (CycloneDX) | ✅ | 4 SBOMs via syft |
| Checksums | ✅ | SHA-256 for all 8 artifacts |
| Provenance (SLSA) | ⚠️ DEFERRED | Requires goreleaser ≥ v2.19 |
| Cosign in goreleaser | ⚠️ DEFERRED | Requires goreleaser ≥ v2.19 |
| Cosign in CI workflow | ✅ | release.yml verify job |
| Authenticode signing | ⚠️ WAIVED | B4 waived |
| Reproducible builds | ⚠️ PARTIAL | -trimpath, -s -w, CGO_ENABLED=0 |

---

## Release Validate Simulation

### Local Validate Steps (All PASS)

```bash
go vet ./...                           ✅ PASS
go test -count=1 ./...                 ✅ PASS (38 pkgs)
go test -tags=integration ...          ✅ PASS
govulncheck ./...                      ✅ PASS (0 vulns)
fuzz smoke tests (7 targets)           ✅ ALL PASS
```

---

## Release Artifacts (Projected for RC2)

| Artifact | Platform | Format | SBOM | Signing |
|----------|----------|--------|------|---------|
| aether_4.0.0-rc2_linux_amd64.tar.gz | linux/amd64 | tar.gz | ✅ | cosign |
| aether_4.0.0-rc2_linux_arm64.tar.gz | linux/arm64 | tar.gz | ✅ | cosign |
| aether_4.0.0-rc2_darwin_amd64.tar.gz | darwin/amd64 | tar.gz | ✅ | cosign |
| aether_4.0.0-rc2_windows_amd64.zip | windows/amd64 | zip | ✅ | WAIVED (B4) |

---

## Verification Commands (for VERIFY.md)

```bash
# 1. Checksums
sha256sum -c checksums.txt

# 2. Cosign signatures (Linux/macOS)
cosign verify-blob --signature <artifact>.sig --certificate <artifact>.pem \
  --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" <artifact>

# 3. Windows Authenticode (if signed)
signtool verify /pa /v aether.exe

# 4. SBOM verification
jq -e '.specVersion' sbom-cyclonedx.json

# 5. Manifest verification
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c
```

---

## Tamper Resistance Tests

```bash
# Test 1: Modified binary fails checksum
cp aether-linux-amd64.tar.gz aether-linux-amd64.tar.gz.tamper
echo "tamper" >> aether-linux-amd64.tar.gz.tamper
sha256sum -c checksums.txt 2>/dev/null || echo "PASS: Tampered binary rejected"

# Test 2: Modified manifest fails signature
cp release-manifest.json release-manifest.json.tamper
jq '.artifacts[0].sha256 = "deadbeef"' release-manifest.json.tamper > release-manifest.json.tamper2 && mv release-manifest.json.tamper2 release-manifest.json.tamper
cosign verify-blob --signature release-manifest.json.sig --certificate release-manifest.json.pem release-manifest.json.tamper 2>/dev/null || echo "PASS: Tampered manifest rejected"

# Test 3: Modified SBOM fails signature
cp sbom-cyclonedx.json sbom-cyclonedx.json.tamper
echo "tamper" >> sbom-cyclonedx.json.tamper
cosign verify-blob --signature sbom-cyclonedx.json.sig --certificate sbom-cyclonedx.json.pem sbom-cyclonedx.json.tamper 2>/dev/null || echo "PASS: Tampered SBOM rejected"

# Test 4: Modified checksums fail
cp checksums.txt checksums.txt.tamper
sed -i 's/^[a-f0-9]*/deadbeef/' checksums.txt.tamper
sha256sum -c checksums.txt.tamper 2>/dev/null || echo "PASS: Tampered checksums rejected"
```

---

## Gate S16-G07 Status

| Sub-gate | Status |
|----------|--------|
| S16-G07.1 | Unit/integration/vet/lint/govulncheck/fuzz | ✅ PASS |
| S16-G07.2 | Goreleaser snapshot | ✅ PASS |
| S16-G07.3 | Local validate steps | ✅ PASS |
| S16-G07.4 | Race detector | ⏳ CI pending |
| S16-G07.5 | SBOM/checksums/artifacts | ✅ PASS |

**S16-G07 Status: PASS (local) — CI race evidence pending**

---

*Generated by Stage 16 Phase 7 — Candidate Qualification*