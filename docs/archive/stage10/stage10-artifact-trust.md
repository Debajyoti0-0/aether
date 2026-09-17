# Stage 10 Artifact Trust + Provenance Closure (G2)

**Timestamp:** 2026-09-15
**Baseline Commit:** 58e6432 (v4.0.0-rc1 tag)

## Goreleaser Configuration Status

### Current .goreleaser.yml Features:
- ✅ Multi-platform builds (linux/amd64, linux/arm64, darwin/amd64, windows/amd64)
- ✅ Checksums (SHA-256)
- ✅ SBOM generation (cyclonedx-json via syft)
- ❌ Provenance generation (not supported in goreleaser v2.18.1 config)
- ❌ Cosign signing (handled in CI workflow, not goreleaser)

### CI Release Workflow Artifact Pipeline:
1. **Validate** - go vet, unit tests, integration tests, govulncheck, fuzz smoke
2. **GoReleaser** - builds, archives, checksums, SBOMs
3. **Windows Sign** - Authenticode (requires EV cert)
4. **Verify** - checksums, manifest, cosign signatures, SBOM, tamper tests
5. **Publish** - GitHub Release with all artifacts

## Artifact Trust Chain

### Expected Artifacts per Release:
| Artifact | Description | Verification |
|----------|-------------|--------------|
| `aether_{{version}}_{{os}}_{{arch}}.tar.gz` / `.zip` | Platform binaries | sha256 + cosign |
| `checksums.txt` | SHA-256 of all archives | Self-verifying |
| `checksums.txt.sig` / `.pem` | Cosign signature of checksums | Cosign verify |
| `release-manifest.json` | Complete artifact manifest | sha256 + cosign |
| `sbom-cyclonedx.json` | CycloneDX SBOM | sha256 + cosign |
| `provenance.json` | SLSA provenance (planned) | TBD |

### Current CI Verification Steps (from release.yml):
- ✅ `sha256sum -c checksums.txt` - Checksums verify
- ✅ Manifest consistency: `jq -r '.artifacts[] | "\(.sha256) \(.name)"' release-manifest.json | sha256sum -c`
- ✅ Cosign verify-blob for Linux/macOS binaries
- ✅ Cosign verify-blob for manifest/SBOM/checksums
- ✅ SBOM specVersion validation
- ✅ Manifest completeness check
- ✅ 9 Tamper tests (modified artifacts fail verification)

## Tamper Test Matrix (from CI):

| Test | Description | Expected Result |
|------|-------------|-----------------|
| 1 | Modified binary | FAIL checksum |
| 2 | Modified manifest | FAIL cosign signature |
| 3 | Modified SBOM | FAIL cosign signature |
| 4 | Modified checksums | FAIL checksum |
| 5 | Wrong artifact | FAIL manifest |
| 6 | Wrong version | FAIL manifest |
| 7 | Wrong architecture | FAIL manifest |
| 8 | Missing signature | FAIL verify |
| 9 | Invalid signature | FAIL verify |

## Provenance Generation

### Status: NOT IMPLEMENTED
- Goreleaser v2.18.1 provenance config not working (`field provenance not found in type config.Project`)
- CI workflow doesn't generate SLSA provenance
- Planned: Use `slsa-framework/slsa-github-generator` or goreleaser v2.20+ provenance

### Required for G2 Closure:
1. Enable provenance in goreleaser (upgrade to v2.20+)
2. Or add SLSA GitHub Generator to CI workflow
3. Verify provenance with `slsa-verifier`

## SBOM Generation

### Status: CONFIGURED, REQUIRES syft IN CI
- Goreleaser config: `sboms: - artifacts: archive, id: cyclonedx`
- CI installs syft via `anchore/syft-action@v1`
- Generates `aether_{{version}}_{{os}}_{{arch}}.sbom.json` per archive

### Local Test:
- syft not installed locally (go install timed out)
- CI has syft via action

## Cosign Signing

### Status: CI WORKFLOW ONLY
- Cosign installed via `sigstore/cosign-installer@v3`
- Keyless signing using GitHub OIDC (`id-token: write`)
- Signs: archives, checksums, manifest, SBOM
- Outputs: `.sig` and `.pem` files

### Verification Commands (from release footer):
```bash
# Linux/macOS
cosign verify-blob --signature aether_{{ .Version }}_{{ .Os }}_{{ .Arch }}.sig \
  --certificate aether_{{ .Version }}_{{ .Os }}_{{ .Arch }}.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether_{{ .Version }}_{{ .Os }}_{{ .Arch }}

# Windows (Authenticode)
signtool verify /pa /v aether_{{ .Version }}_windows_amd64.exe

# All artifacts
sha256sum -c checksums.txt
```

## G2 Gate Status

| Requirement | Status | Evidence |
|-------------|--------|----------|
| Multi-platform builds | ✅ | CI builds 4 platforms |
| Checksums (SHA-256) | ✅ | goreleaser + CI verify |
| SBOM (CycloneDX) | 🔄 | Config done, needs syft in CI |
| Manifest | ✅ | goreleaser generates |
| Cosign signatures | 🔄 | CI workflow configured |
| Provenance (SLSA) | ❌ | Not implemented |
| Tamper tests (9) | 🔄 | CI workflow configured |
| Windows Authenticode | ❌ | B4 blocker - no EV cert |

## Next Steps for G2 Closure

1. **Fix Release workflow Validate job** - Unit tests failing (need race detector fix or remove -race from Validate)
2. **Complete Release workflow run** - Get artifacts generated
3. **Verify all tamper tests pass** - In Verify job
4. **Implement provenance** - Upgrade goreleaser or add SLSA generator
5. **Procure EV cert** - For B4/Authenticode closure

## Run URLs

- Release workflow (latest - failed): https://github.com/Debajyoti0-0/aether/actions/runs/34957080665
- CI workflow (latest): https://github.com/Debajyoti0-0/aether/actions/runs/34955187150

---
*Stage 10 G2 - Artifact Trust + Provenance Closure*