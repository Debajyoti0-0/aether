# Stage 15 — Post-RC2 Candidate Validation

**Timestamp:** 2016-09-16
**Scope:** Validate remediation changes without mutating frozen RC2 tag

---

## Validation Branch

**Branch:** `stage15/waiver-closure`
**Base:** `fc062e0` (Stage 14 HEAD)
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

**Total executions:** ~50M | **Crashes:** 0 | **Panics:** 0

---

## Goreleaser Snapshot Verification

### Artifacts Generated
| Artifact | Platform | Format | SHA-256 (snapshot) |
|----------|----------|--------|-------------------|
| aether_linux_amd64.tar.gz | linux/amd64 | tar.gz | ca74b2a145953efa443ab953c92ee3d957bad72b329f22a60ec819c41f8d640f |
| aether_linux_arm64.tar.gz | linux/arm64 | tar.gz | 4c9e302238bb0e2bce9fca1270799a5e01269fd47b60950c4176e82f747098d5 |
| aether_darwin_amd64.tar.gz | darwin/amd64 | tar.gz | 719665e1fd367abce0ed8bcf28a68c8520f823c1fc6ebc045b5db172fe7a2540 |
| aether_windows_amd64.zip | windows/amd64 | zip | 8e0f203f99accae27c0b66597870e287033c798cd6c54aa062d5e777b4691aa4 |

### SBOMs Generated
- 4 SBOMs (SPDX 2.3 via syft)
- ~39 components each

### Checksums
- SHA-256 for all 8 artifacts (4 archives + 4 SBOMs)

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

## Goreleaser Configuration Status

| Feature | Status | Notes |
|---------|--------|-------|
| goreleaser check | ✅ PASS | v2.18.1 |
| Snapshot build | ✅ PASS | 4 platforms |
| SBOM (cyclonedx) | ✅ | via syft |
| Checksums (SHA-256) | ✅ | |
| Provenance (SLSA) | ❌ | Requires goreleaser ≥ v2.19 |
| Cosign in goreleaser | ❌ | Requires goreleaser ≥ v2.19 |
| Cosign in CI | ✅ | release.yml verify job |
| Authenticode | ❌ | WAIVED (B4) |

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

## Gate S15-G08 Status

| Sub-gate | Status |
|----------|--------|
| S15-G08.1 | Unit/integration/vet/lint/govulncheck/fuzz | ✅ PASS |
| S15-G08.2 | Goreleaser snapshot | ✅ PASS |
| S15-G08.3 | Local validate steps | ✅ PASS |
| S15-G08.4 | Race detector | ⏳ CI pending |
| S15-G08.5 | SBOM/checksums/artifacts | ✅ Generated |

**S15-G08 Status: PASS (local) — CI race evidence pending**

---

*Generated by Stage 15 Phase 8 — Post-RC2 Candidate Validation*