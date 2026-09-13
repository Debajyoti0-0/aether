# Stage 9 G3 — Artifact Trust and Provenance Verification

**Date:** 2026-09-13
**Baseline:** Stage 9 G2 Release Pipeline Execution
**Repository:** `C:\dev\aether` @ `9f413d7`

---

## 1. Artifact Set for Release Candidate `v4.0.0-rc1`

| Artifact | Type | Description |
|----------|------|-------------|
| `aether_4.0.0-rc1_linux_amd64.tar.gz` | Binary archive | Linux AMD64 release |
| `aether_4.0.0-rc1_linux_arm64.tar.gz` | Binary archive | Linux ARM64 release |
| `aether_4.0.0-rc1_darwin_amd64.tar.gz` | Binary archive | macOS AMD64 release |
| `aether_4.0.0-rc1_windows_amd64.zip` | Binary archive | Windows AMD64 release |
| `checksums.txt` | Manifest | SHA-256 checksums for all artifacts |
| `sbom-cyclonedx.json` | SBOM | CycloneDX Software Bill of Materials |
| `release-manifest.json` | Manifest | Complete artifact inventory with metadata |
| `provenance.json` | Provenance | SLSA provenance attestation |
| `*.sig` / `*.pem` | Signatures | Cosign signatures + certificates |

---

## 2. Trust Verification Requirements

### 2.1 Checksum Verification
```bash
sha256sum -c checksums.txt
# All artifacts must match their recorded hashes
```

### 2.2 SBOM Verification
```bash
# Verify SBOM structure
jq -e '.specVersion' sbom-cyclonedx.json
jq '.components | length' sbom-cyclonedx.json

# Verify SBOM corresponds to binary
syft aether_linux_amd64 -o cyclonedx-json | jq -e '.specVersion'
```

### 2.3 Provenance Verification
```bash
# Verify SLSA provenance
jq -e '.buildType' provenance.json
jq -e '.builder.id' provenance.json
jq -e '.subject[] | select(.name | test("aether_"))' provenance.json
```

### 2.4 Signature Verification (Linux/macOS)
```bash
# For each binary archive
cosign verify-blob --signature aether_4.0.0-rc1_linux_amd64.tar.gz.sig \
  --certificate aether_4.0.0-rc1_linux_amd64.tar.gz.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether_4.0.0-rc1_linux_amd64.tar.gz

# For manifests/SBOM/checksums
cosign verify-blob --signature release-manifest.json.sig \
  --certificate release-manifest.json.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  release-manifest.json
```

### 2.5 Windows Authenticode Verification
```cmd
signtool verify /pa /v aether_4.0.0-rc1_windows_amd64.exe
```

---

## 3. Tamper Tests (Fail-Closed Verification)

All tests MUST fail (reject tampered artifacts):

| # | Tamper Test | Method | Expected Result |
|---|-------------|--------|-----------------|
| 1 | Modified binary | `echo "x" >> binary` | `sha256sum -c` FAIL; `cosign verify-blob` FAIL |
| 2 | Modified manifest | Change version in manifest | `cosign verify-blob` FAIL |
| 3 | Modified SBOM | Change component version | `cosign verify-blob` FAIL |
| 4 | Modified checksums | Change hash in checksums.txt | `sha256sum -c` FAIL; manifest mismatch |
| 5 | Remove signature | Delete `.sig` file | `cosign verify-blob` FAIL |
| 6 | Replace signature | Use wrong `.sig` file | `cosign verify-blob` FAIL (cert mismatch) |
| 7 | Wrong public key | Use different cert | `cosign verify-blob` FAIL |
| 8 | Different release artifact | Verify v1.0 with v2.0 cert | FAIL (hash mismatch) |
| 9 | Mismatched manifest/artifact | Swap binary, keep manifest | FAIL (checksum mismatch) |

---

## 4. Reproducibility Classification

| Level | Definition | Status |
|-------|------------|--------|
| `REPRODUCIBLE_CONFIGURATION` | Build config ensures reproducible builds (`-trimpath`, fixed ldflags, deterministic modules) | ✅ CONFIGURED |
| `REPEATABLE_BUILD_OBSERVED` | Same commit produces same artifacts on same machine | ✅ LOCAL SNAPSHOT |
| `INDEPENDENTLY_REPRODUCED` | Different machine/environment produces identical artifacts | ⚠️ NOT YET TESTED |
| `BIT_FOR_BIT_REPRODUCIBLE` | Bit-for-bit identical across all environments | ⚠️ NOT YET TESTED |

**Current Classification:** `REPRODUCIBLE_CONFIGURATION` + `REPEATABLE_BUILD_OBSERVED`

---

## 5. Independent Verification Procedure

A clean verifier must be able to:

1. Clone the repository
2. Check out the release tag (`v4.0.0-rc1`)
3. Verify source/version/tag integrity:
   ```bash
   git rev-parse HEAD
   cat VERSION
   git describe --tags --exact-match
   ```
4. Build or retrieve the artifact
5. Verify checksum: `sha256sum -c checksums.txt`
6. Verify SBOM: `jq -e '.specVersion' sbom-cyclonedx.json`
7. Verify provenance: `jq -e '.buildType' provenance.json`
8. Verify signature: `cosign verify-blob ...` (Linux/macOS) or `signtool verify` (Windows)
9. Run test suite: `go test -count=1 ./...`
10. Reproduce declared release checks
11. Detect tampering (tests 1-9 above)
12. Understand all limitations

---

## 6. Current Status (Local Snapshot)

| Check | Status | Evidence |
|-------|--------|----------|
| Checksums generated | ✅ | `dist/checksums.txt` |
| Checksums verified | ✅ | `sha256sum -c` PASS |
| SBOM generated | ❌ | Requires syft in CI |
| SBOM verified | ❌ | Not generated |
| Provenance generated | ❌ | Requires CI attestations |
| Provenance verified | ❌ | Not generated |
| Signatures generated | ❌ | Requires cosign keyless in CI |
| Signatures verified | ❌ | Not generated |
| Tamper tests executed | ❌ | Requires CI verify job |
| Independent verification | ❌ | Not yet tested |

---

## 7. CI Integration Plan

The GitHub Actions workflow (`.github/workflows/release.yml`) implements:

1. **validate** job: build, test, vet, vulncheck, fuzz smoke tests
2. **goreleaser** job: build matrix, cosign keyless signing, syft SBOM, SLSA provenance
3. **windows-sign** job: Authenticode signing on Windows runner (EV cert required)
4. **verify** job: All checksum, signature, SBOM, provenance, tamper tests
5. **publish** job: GitHub Release with all artifacts

---

## 8. G3 Result

**PARTIAL** — Pipeline configuration complete and verified locally for build/checksums. SBOM, provenance, signatures, and tamper tests require CI execution with OIDC credentials.

**Next Step:** Push `v4.0.0-rc1` tag to trigger CI pipeline and verify full artifact trust chain.

---

**Prepared by:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Status:** G3 PARTIAL — CI execution required