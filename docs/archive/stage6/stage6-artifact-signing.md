# Aether — Stage 6 Artifact Signing & Trust Chain Design

**Baseline:** Stage 3 (3.4.0-stage3), HEAD `12a2175`
**Phase:** 5 of 9
**Goal:** Define verifiable, reproducible, operationally safe signing process for release artifacts

---

## 1. Current Release Artifact Audit

### 1.1 Artifacts Generated Today

| Artifact | Source | Deterministic? | Contains Timestamps? | Env-Specific? | Covered by Checksums? |
|----------|--------|----------------|---------------------|---------------|----------------------|
| `bin/aether.exe` (Windows) | `go build` | Yes (with fixed ldflags) | No (strip `-s -w`) | GOOS/GOARCH | No (committed binary) |
| `bin/aether-*` (cross-platform) | `scripts/build.sh` | Yes | No | GOOS/GOARCH | No |
| `VERSION` | Repo root | Yes | No | No | N/A (source of truth) |
| `CHANGELOG.md` | Manual | No | Yes (release dates) | No | No |
| `go.mod` / `go.sum` | `go mod tidy` | Yes | No | No | No |

### 1.2 Missing Artifacts (Stage 6+)

| Artifact | Purpose | Generation Method |
|----------|---------|-------------------|
| `release-manifest.json` | Canonical list of release files + hashes | Custom generator |
| `checksums.txt` | SHA-256 of all release files | `sha256sum` / custom |
| `sbom-cyclonedx.json` | Software Bill of Materials | `syft` or `cyclonedx-gomod` |
| `protocol-conformance.json` | Protocol test results | Test runner output |
| `interoperability-matrix.json` | Live interop results | Live-lab runner output |
| `evidence-verification.json` | Evidence chain verification | `aether export verify-evidence` |
| `crash-matrix.json` | Crash recovery test results | Integration test runner |

---

## 2. Signing Architecture Design

### 2.1 Algorithm & Key Selection

| Option | Algorithm | Key Type | Pros | Cons | Decision |
|--------|-----------|----------|------|------|----------|
| **cosign (keyless)** | ECDSA-P256 / RSA-2048 | Ephemeral (OIDC) | No key custody; Sigstore transparency log; GitHub Actions native | Requires OIDC identity; transparency log public | **PRIMARY** for Linux/macOS binaries |
| **cosign (key-based)** | ECDSA-P256 / RSA-2048 | Hardware-backed (YubiHSM, Cloud KMS) | Full control; offline signing possible | Key custody operational burden | **BACKUP** / Windows Authenticode |
| **Windows Authenticode** | SHA-256 + RSA-2048 | Code signing cert (EV) | Windows SmartScreen trust; kernel driver loading | EV cert cost ($300-600/yr); separate process | **REQUIRED** for Windows `bin/aether.exe` |
| **minisign** | Ed25519 | Static keypair | Simple; fast; small sigs | No transparency log; custom verification | **ALTERNATIVE** for checksums/manifests |

**Selected Architecture:**
- **Linux/macOS binaries:** `cosign sign-blob --yes` (keyless, GitHub OIDC) → `.sig` files
- **Windows binary:** Authenticode sign via `signtool` (EV cert) → embedded signature
- **Manifest/checksums/SBOM:** `cosign sign-blob` (keyless) → `.sig` files
- **Verification:** `cosign verify-blob` + `signtool verify /pa` (Windows)

### 2.2 Key Custody

| Key | Custody | Rotation | Revocation |
|-----|---------|----------|------------|
| **cosign keyless** | GitHub Actions OIDC (no persistent key) | Automatic per-sign | N/A (transparency log) |
| **cosign key-based (backup)** | YubiHSM 2 / Cloud KMS (AWS KMS, GCP KMS) | Annual | KMS key disable + transparency log |
| **Authenticode EV cert** | HSM-backed (cert provider) | Annual (cert renewal) | CRL/OCSP via cert provider |

**No private keys in repository.** Ever.

### 2.3 Signature Format

| Artifact | Signature Format | Verification Command |
|----------|------------------|---------------------|
| `aether-linux-amd64` | `cosign` bundle (`.sig` + `.pem` + transparency log entry) | `cosign verify-blob --signature aether-linux-amd64.sig --certificate aether-linux-amd64.pem aether-linux-amd64` |
| `aether-windows-amd64.exe` | Embedded Authenticode (PE signature) | `signtool verify /pa /v aether-windows-amd64.exe` |
| `release-manifest.json` | `cosign` bundle | `cosign verify-blob ...` |
| `checksums.txt` | `cosign` bundle | `cosign verify-blob ...` |
| `sbom-cyclonedx.json` | `cosign` bundle | `cosign verify-blob ...` |

---

## 3. Release Manifest Specification

### 3.1 Schema (`release-manifest.json`)

```json
{
  "version": "3.5.0-rc1",
  "commit": "abc123def456",
  "timestamp": "2026-09-12T14:30:00Z",
  "builder": "github-actions[bot]",
  "go_version": "go1.27.1",
  "artifacts": [
    {
      "name": "aether-linux-amd64",
      "sha256": "a1b2c3d4...",
      "size": 12345678,
      "signature": "aether-linux-amd64.sig",
      "certificate": "aether-linux-amd64.pem",
      "algorithm": "cosign-ecdsa-p256"
    },
    {
      "name": "aether-windows-amd64.exe",
      "sha256": "e5f6g7h8...",
      "size": 13456789,
      "signature": "embedded-authenticode",
      "algorithm": "authenticode-sha256-rsa2048"
    },
    {
      "name": "checksums.txt",
      "sha256": "i9j0k1l2...",
      "size": 1024,
      "signature": "checksums.txt.sig",
      "certificate": "checksums.txt.pem",
      "algorithm": "cosign-ecdsa-p256"
    }
  ],
  "sbom": {
    "name": "sbom-cyclonedx.json",
    "sha256": "m3n4o5p6...",
    "signature": "sbom-cyclonedx.json.sig",
    "certificate": "sbom-cyclonedx.json.pem"
  }
}
```

### 3.2 Manifest Properties

- **Immutable:** Manifest generated once per release tag; never modified
- **Complete:** References every file in release archive
- **Self-describing:** Includes verification commands for each artifact
- **Signed:** Manifest itself signed (cosign bundle)

---

## 4. Checksum Policy

### 4.1 Algorithm
- **SHA-256** for all artifacts (FIPS 180-4 compliant)
- **Format:** `sha256 <hash>  <filename>` (standard `sha256sum` output)

### 4.2 Coverage
Every file in release archive:
- All platform binaries
- `release-manifest.json`
- `sbom-cyclonedx.json`
- `protocol-conformance.json` (if generated)
- `interoperability-matrix.json` (if generated)
- `evidence-verification.json` (if generated)
- `crash-matrix.json` (if generated)
- `LICENSE`, `README.md`, `CHANGELOG.md` (source files)

### 4.3 Verification
```bash
# Verify all checksums
sha256sum -c checksums.txt

# Verify manifest matches checksums
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c
```

---

## 5. SBOM Generation

### 5.1 Tool: `cyclonedx-gomod` (preferred) or `syft`

```bash
# Module-level SBOM (build-time deps)
cyclonedx-gomod app -output sbom-cyclonedx.json

# Binary-level SBOM (includes runtime deps via syft)
syft bin/aether-linux-amd64 -o cyclonedx-json=sbom-cyclonedx.json
```

### 5.2 SBOM Contents (CycloneDX 1.5+)

- **Components:** All Go modules (direct + indirect) with versions, hashes, licenses
- **Services:** External APIs (login.microsoftonline.com, management.azure.com, 169.254.169.254)
- **Vulnerabilities:** Cross-referenced via `govulncheck` output
- **Metadata:** Tool version, timestamp, author, component (aether)

---

## 6. Verification Procedure (Independent Verifier)

### 6.1 Prerequisites
- `cosign` installed (`go install github.com/sigstore/cosign/v2/cmd/cosign@latest`)
- `signtool` (Windows SDK) for Authenticode
- `sha256sum` / `certutil -hashfile` for checksums
- `jq` for manifest parsing

### 6.2 Verification Steps

```bash
# 1. Download release artifacts + signatures + manifest + checksums
# 2. Verify checksums
sha256sum -c checksums.txt

# 3. Verify manifest matches checksums
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c

# 4. Verify cosign signatures (Linux/macOS/manifest/SBOM/checksums)
cosign verify-blob --signature aether-linux-amd64.sig \
  --certificate aether-linux-amd64.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether-linux-amd64

# 5. Verify Authenticode (Windows)
signtool verify /pa /v aether-windows-amd64.exe

# 6. Verify manifest signature
cosign verify-blob --signature release-manifest.json.sig \
  --certificate release-manifest.json.pem \
  release-manifest.json
```

### 6.3 Failure Behavior (Fail-Closed)

| Failure | Behavior |
|---------|----------|
| Checksum mismatch | **ABORT** — Artifact corrupted/tampered |
| Signature verification fail | **ABORT** — Signature invalid/missing |
| Manifest missing | **ABORT** — Incomplete release |
| Certificate expired | **ABORT** — Trust anchor invalid |
| Transparency log missing | **WARN** (cosign keyless) — Log delay acceptable |
| Authenticode cert revoked | **ABORT** — Cert compromised/expired |

---

## 7. CI/CD Integration (GitHub Actions)

### 7.1 Release Workflow (`.github/workflows/release.yml`)

```yaml
name: release
on:
  push:
    tags: ['v*']
permissions:
  contents: write
  id-token: write  # Required for cosign keyless
jobs:
  build:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27.x' }
      - name: Build
        run: |
          VERSION=$(cat VERSION)
          COMMIT=$(git rev-parse --short HEAD)
          go build -ldflags="-s -w -X github.com/Debajyoti0-0/aether/internal/version.Version=${VERSION} -X github.com/Debajyoti0-0/aether/internal/version.Commit=${COMMIT}" -o bin/aether${EXT} ./cmd/aether
      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with: { name: aether-${{ matrix.os }}, path: bin/ }

  sign:
    needs: build
    runs-on: ubuntu-latest
    permissions: { id-token: write, contents: read }
    steps:
      - uses: actions/download-artifact@v4
      - name: Install cosign
        uses: sigstore/cosign-installer@v3
      - name: Sign Linux/macOS binaries + manifest + checksums + SBOM
        run: |
          cosign sign-blob --yes aether-linux-amd64
          cosign sign-blob --yes aether-darwin-amd64
          cosign sign-blob --yes release-manifest.json
          cosign sign-blob --yes checksums.txt
          cosign sign-blob --yes sbom-cyclonedx.json
      - name: Sign Windows (separate job on windows-latest)
        # Uses signtool with EV cert from secret

  release:
    needs: sign
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            bin/*
            *.sig
            *.pem
            release-manifest.json
            checksums.txt
            sbom-cyclonedx.json
```

### 7.2 Permissions Minimization
- `id-token: write` only for `sign` job
- `contents: write` only for `release` job
- No secrets in workflow (cosign keyless uses OIDC; Authenticode cert in GitHub Secrets)

---

## 8. Tamper Tests (Gate B5 Requirements)

| Test Case | Method | Expected Result |
|-----------|--------|-----------------|
| Modify binary | `echo "x" >> aether-linux-amd64` | `sha256sum -c` FAIL; `cosign verify-blob` FAIL |
| Modify manifest | Change version in `release-manifest.json` | `cosign verify-blob` FAIL |
| Modify SBOM | Change component version | `cosign verify-blob` FAIL |
| Modify checksums | Change hash in `checksums.txt` | `sha256sum -c` FAIL; manifest mismatch |
| Remove signature | Delete `.sig` file | `cosign verify-blob` FAIL (signature missing) |
| Replace signature | Use wrong `.sig` file | `cosign verify-blob` FAIL (cert mismatch) |
| Wrong public key | Use different cert | `cosign verify-blob` FAIL (cert verification fail) |
| Different release artifact | Verify v1.0 artifact with v2.0 cert | FAIL (hash mismatch) |
| Mismatched manifest/artifact | Swap binary but keep manifest | FAIL (checksum mismatch) |

**All tamper tests MUST FAIL as designed.**

---

## 9. Trust Root Distribution

| Channel | Method | Audience |
|---------|--------|----------|
| **GitHub Releases** | Cosign bundles (`.sig` + `.pem`) attached to release | All users |
| **Sigstore Transparency Log** | Public log (rekor) | Anyone (audit trail) |
| **Authenticode** | Microsoft Trusted Root Program | Windows users (SmartScreen) |
| **Documentation** | `VERIFY.md` in repo + release notes | Operators |

**Verification material:** Public keys/certs distributed via release artifacts (`.pem` files) and transparency log. No separate key distribution needed for cosign keyless.

---

## 10. Key Rotation & Recovery

| Scenario | Procedure |
|----------|-----------|
| **cosign keyless** | Automatic — new OIDC token per sign; no persistent key |
| **cosign key-based (backup)** | 1. Generate new keypair in HSM/KMS 2. Update GitHub Secrets 3. Sign new release 4. Old keys retained for verification of past releases |
| **Authenticode EV cert** | 1. Renew cert with provider (annual) 2. Update GitHub Secrets 3. Sign new release 4. Old cert remains valid for past releases (timestamped) |
| **Compromise** | 1. Revoke cert (CRL/OCSP) 2. Rotate keys 3. Re-sign current release 4. Publish security advisory |

---

## 11. Stage 6 Deliverables (Design Only — Implementation Stage 7)

| Deliverable | Status | Location |
|-------------|--------|----------|
| Signing architecture | **DESIGNED** | This document |
| Manifest schema | **DESIGNED** | §3.1 |
| Checksum policy | **DESIGNED** | §4 |
| SBOM generation | **DESIGNED** | §5 |
| Verification procedure | **DESIGNED** | §6 |
| CI/CD workflow | **DESIGNED** | §7 |
| Tamper test cases | **DESIGNED** | §8 |
| Trust root distribution | **DESIGNED** | §9 |
| Key rotation/recovery | **DESIGNED** | §10 |

**Gate B5 PASS Criteria (Design Phase):**
- [x] Signing procedure defined with key custody
- [x] Signature verification independently reproducible
- [x] Tamper detection test cases defined
- [x] Private key material excluded from repo/artifacts
- [x] Release operator documentation complete
- [x] Failure behavior is fail-closed

**Implementation deferred to Stage 7** (goreleaser pipeline, EV cert procurement, GitHub Actions release workflow).

---

## Sign-Off

**Design Completed:** 2026-09-12
**Baseline Commit:** `12a2175`
**Next Phase:** Phase 6 — CI/Cross-Platform/Supply-Chain Review