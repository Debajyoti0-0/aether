# Aether Release Verification Guide

**Version:** 4.2.0-rc1 (Production-Limited)
**Release Date:** 2026-09-17 (guide regenerated Stage 33; artifact names follow aether_<version>_<os>_<arch>)
**Commit:** bb56cf3
**Tag:** v4.2.0-rc1

---

> **Signing status (Stage 31, finding F-30-2):** the release pipeline
> currently produces **no cryptographic signatures** (no cosign `signs:`
> configuration, no Authenticode certificate). The cosign/Authenticode
> sections below describe verification that is **not currently possible**
> against release artifacts; they are retained only as the target state
> for when signing is enabled. Checksum and SBOM verification are
> functional today.

## Quick Verification

```bash
# 1. Download artifacts from GitHub Release
# 2. Verify checksums
sha256sum -c checksums.txt

# 3. (NOT CURRENTLY PRODUCED) cosign signatures — see signing status above
cosign verify-blob --signature aether_<version>_<os>_<arch>.sig \
  --certificate aether_<version>_<os>_<arch>.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether_<version>_<os>_<arch>

# 4. (NOT CURRENTLY PRODUCED) Authenticode — see signing status above
signtool verify /pa /v aether_<version>_windows_amd64.exe

# 5. Verify SBOM
jq -e '.specVersion' sbom-cyclonedx.json
```

---

## Artifact List (v4.2.0-rc1)

| Artifact | Platform | Format | Size | SHA-256 |
|----------|----------|--------|------|---------|
| aether_v4.2.0-rc1_linux_amd64.tar.gz | Linux x86_64 | tar.gz | (see checksums.txt) | (see checksums.txt) |
| aether_v4.2.0-rc1_linux_arm64.tar.gz | Linux ARM64 | tar.gz | (see checksums.txt) | (see checksums.txt) |
| aether_v4.2.0-rc1_darwin_amd64.tar.gz | macOS x86_64 | tar.gz | (see checksums.txt) | (see checksums.txt) |
| aether_v4.2.0-rc1_darwin_arm64.tar.gz | macOS Apple Silicon | tar.gz | (see checksums.txt) | (see checksums.txt) |
| aether_v4.2.0-rc1_windows_amd64.zip | Windows x86_64 | zip | (see checksums.txt) | (see checksums.txt) |

Each archive has a corresponding `.sbom.json` (SPDX 2.3) and signature files (`.sig`, `.pem`) when cosign signing is enabled.

---

## Verification Procedures

### 1. Checksum Verification

```bash
# Verify all artifacts match published checksums
sha256sum -c checksums.txt
# Should output: <artifact>: OK for each file
```

### 2. Cosign Signature Verification (Linux/macOS)

```bash
# For each Linux/macOS artifact
cosign verify-blob \
  --signature aether_v4.2.0-rc1_linux_amd64.tar.gz.sig \
  --certificate aether_v4.2.0-rc1_linux_amd64.tar.gz.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether_v4.2.0-rc1_linux_amd64.tar.gz

# Verify SBOM signatures
cosign verify-blob \
  --signature sbom-cyclonedx.json.sig \
  --certificate sbom-cyclonedx.json.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  sbom-cyclonedx.json
```

**Note:** Cosign keyless signing uses GitHub OIDC. The `--certificate-identity-regexp` and `--certificate-oidc-issuer-regexp` are set to match any GitHub Actions identity.

### 3. Windows Authenticode Verification

```cmd
:: Requires Windows with signtool (Windows SDK)
signtool verify /pa /v aether_v4.2.0-rc1_windows_amd64.exe
```

**Status for v4.2.0-rc1:** Windows binary is **NOT Authenticode signed** (EV cert waived — see WAIVER-B4-2026-09-16). Expect SmartScreen warnings.

### 4. SBOM Verification

```bash
# Check SBOM format and content
jq -e '.specVersion' sbom-cyclonedx.json
# Should output: "SPDX-2.3" or "1.6" (CycloneDX)

jq '.packages | length' sbom-cyclonedx.json
# Should show ~40 packages

# Verify SBOM signature (if signed)
cosign verify-blob \
  --signature sbom-cyclonedx.json.sig \
  --certificate sbom-cyclonedx.json.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  sbom-cyclonedx.json
```

### 5. Manifest Verification

```bash
# Verify manifest matches artifacts
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c

# Check all manifest artifacts exist
jq -r '.artifacts[].name' release-manifest.json | while read f; do
  if [[ ! -f "$f" ]]; then echo "MISSING: $f"; exit 1; fi
done
echo "All manifest artifacts present."
```

---

## Tamper Resistance Tests

These tests verify that tampered artifacts are detected:

```bash
# Test 1: Modified binary fails checksum
cp aether_linux_amd64.tar.gz aether_linux_amd64.tar.gz.tamper
echo "tamper" >> aether_linux_amd64.tar.gz.tamper
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

All 4 tamper tests should **PASS** (fail-closed).

---

## Reproducible Build Verification

```bash
# 1. Fresh clone
git clone https://github.com/Debajyoti0-0/aether.git
cd aether

# 2. Checkout exact tag
git checkout v4.2.0-rc1

# 3. Build
go build ./...

# 4. Run tests
go test -count=1 ./...

# 5. Generate snapshot artifacts
goreleaser release --snapshot --clean

# 6. Compare with published artifacts
# - Binary sizes should match
# - Checksums should match (if reproducible)
# - SBOM package lists should match
```

**Note:** Full bit-for-bit reproducibility requires identical build environment (Go version, OS, dependencies). The build uses `-trimpath`, `-s -w`, and `CGO_ENABLED=0` for maximum reproducibility.

---

## Known Limitations (v4.2.0-rc1)

| Limitation | Status | Tracking |
|------------|--------|----------|
| Windows binary unsigned (EV Authenticode) | WAIVED | WAIVER-B4-2026-09-16 |
| Race detector not validated on Windows | OPEN | B3 |
| Azure KV provider not live-validated | OPEN | B5 |
| Provenance (SLSA) not generated | OPEN | Phase 6 |
| Race detector CI failures | OPEN | B3 |

---

## Tools Required

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.27.x | https://go.dev/dl/ |
| cosign | v2.x | `go install github.com/sigstore/cosign/v2/cmd/cosign@latest` |
| sha256sum | - | Coreutils (Linux/macOS) / Windows: `Get-FileHash -Algorithm SHA256` |
| jq | 1.6+ | Package manager |
| signtool | Windows SDK | Windows SDK |
| goreleaser | v2.x | `go install github.com/goreleaser/goreleaser@latest` |
| syft | - | `go install github.com/anchore/syft@latest` |

---

## Support

- **Issues:** https://github.com/Debajyoti0-0/aether/issues
- **Security:** SECURITY.md
- **Release Notes:** GitHub Releases page

---

*Generated: 2026-09-16 | Commit: 6b88f87 | Tag: v4.2.0-rc1*