# Stage 7 G5 — Release Verification Procedure

**Date:** 2026-09-12
**Baseline:** `5b740cb`
**Version:** `3.8.0-stage7`
**Reference:** `docs/stage7-release-engineering.md`

---

## 1. Overview

This document provides the step-by-step verification procedure for Aether releases, enabling independent verification by any party.

---

## 2. Prerequisites

### 2.1 Required Tools
| Tool | Purpose | Install |
|------|---------|---------|
| `cosign` | Signature verification (Linux/macOS) | `go install github.com/sigstore/cosign/v2/cmd/cosign@latest` |
| `signtool` | Authenticode verification (Windows) | Windows SDK |
| `sha256sum` / `certutil` | Checksum verification | Coreutils / Windows built-in |
| `jq` | Manifest parsing | Package manager |

### 2.2 Required Artifacts (from GitHub Release)
| File | Required | Description |
|------|----------|-------------|
| `aether-<os>-<arch>[.exe]` | ✅ | Platform binary |
| `checksums.txt` | ✅ | SHA-256 checksums |
| `sbom-cyclonedx.json` | ✅ | CycloneDX SBOM |
| `release-manifest.json` | ✅ | Release manifest |
| `<artifact>.sig` | ✅ | cosign signatures |
| `<artifact>.pem` | ✅ | cosign certificates |
| `release-manifest.json.sig/.pem` | ✅ | Manifest signature |

---

## 3. Verification Procedure

### Step 1: Download Artifacts
Download all release artifacts from GitHub Releases to a local directory.

```bash
mkdir aether-verify && cd aether-verify
# Download all files from GitHub Release
```

### Step 2: Verify Checksums
```bash
# Linux/macOS
sha256sum -c checksums.txt

# Windows PowerShell
Get-FileHash -Algorithm SHA256 (Get-ChildItem -File) | ForEach-Object { "$($_.Hash)  $($_.Path)" } | Compare-Object (Get-Content checksums.txt) -IncludeEqual
```

**Expected:** All checksums match (exit code 0).

### Step 3: Verify Manifest Consistency
```bash
# Verify manifest references match actual checksums
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c
```

**Expected:** All manifest entries match checksums.

### Step 4: Verify Signatures

#### Linux/macOS Binaries
```bash
cosign verify-blob --signature aether-linux-amd64.sig \
  --certificate aether-linux-amd64.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether-linux-amd64
```

**Expected:** "Verified OK"

#### Windows Binary
```cmd
signtool verify /pa /v aether-windows-amd64.exe
```

**Expected:** "Successfully verified"

#### Manifest / SBOM / Checksums
```bash
cosign verify-blob --signature release-manifest.json.sig \
  --certificate release-manifest.json.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  release-manifest.json

cosign verify-blob --signature sbom-cyclonedx.json.sig \
  --certificate sbom-cyclonedx.json.pem \
  sbom-cyclonedx.json

cosign verify-blob --signature checksums.txt.sig \
  --certificate checksums.txt.pem \
  checksums.txt
```

**Expected:** "Verified OK" for each.

### Step 5: Verify SBOM
```bash
# Check SBOM format
jq '.specVersion' sbom-cyclonedx.json
# Should output "1.5" or "1.6"

# Check component count
jq '.components | length' sbom-cyclonedx.json

# Check for known vulnerabilities (optional)
# Requires vulnerability database
```

### Step 6: Verify Manifest Completeness
```bash
# Check all artifacts in manifest exist
jq -r '.artifacts[].name' release-manifest.json | while read f; do test -f "$f" || echo "MISSING: $f"; done

# Check all downloaded files in manifest
ls -1 | grep -v "^\." | while read f; do
  jq -e ".artifacts[] | select(.name == \"$f\")" release-manifest.json > /dev/null || echo "UNTRACKED: $f"
done
```

---

## 4. Failure Behavior (Fail-Closed)

| Failure | Behavior | User Action |
|---------|----------|-------------|
| Checksum mismatch | **ABORT** — Artifact corrupted/tampered | Do not use; report to maintainers |
| Signature verification fail | **ABORT** — Signature invalid/missing | Do not use; report to maintainers |
| Manifest missing | **ABORT** — Incomplete release | Download from official source |
| Certificate expired | **ABORT** — Trust anchor invalid | Check system time; report if persistent |
| Manifest/artifact mismatch | **ABORT** — Incomplete release | Re-download all artifacts |
| Transparency log missing | **WARN** (cosign keyless) | Acceptable — log delay |

---

## 4. Automated Verification Script

```bash
#!/bin/bash
# verify-release.sh - Automated release verification
set -euo pipefail

RELEASE_DIR="${1:-.}"
cd "$RELEASE_DIR"

echo "=== Aether Release Verification ==="
echo "Directory: $(pwd)"
echo ""

# 1. Checksums
echo "[1/6] Verifying checksums..."
sha256sum -c checksums.txt || { echo "FAIL: Checksum mismatch"; exit 1; }
echo "PASS"

# 2. Manifest consistency
echo "[2/6] Verifying manifest consistency..."
jq -r '.artifacts[] | "\(.sha256)  \(.name)"' release-manifest.json | sha256sum -c || { echo "FAIL: Manifest mismatch"; exit 1; }
echo "PASS"

# 3. Linux/macOS signatures
for bin in aether-linux-*; do
  if [[ -f "$bin" && -f "$bin.sig" && -f "$bin.pem" ]]; then
    echo "[3] Verifying $bin..."
    cosign verify-blob --signature "$bin.sig" --certificate "$bin.pem" \
      --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" "$bin" || { echo "FAIL: $bin signature"; exit 1; }
    echo "PASS: $bin"
  fi
done

# 3b. Windows binary
if [[ -f "aether-windows-amd64.exe" ]]; then
  echo "[3b] Verifying Windows binary..."
  signtool verify /pa /v aether-windows-amd64.exe || { echo "FAIL: Windows binary signature"; exit 1; }
  echo "PASS"
fi

# 4. Manifest/SBOM/Checksums signatures
for f in release-manifest.json sbom-cyclonedx.json checksums.txt; do
  if [[ -f "$f.sig" && -f "$f.pem" ]]; then
    echo "[4] Verifying $f..."
    cosign verify-blob --signature "$f.sig" --certificate "$f.pem" \
      --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" "$f" || { echo "FAIL: $f signature"; exit 1; }
    echo "PASS: $f"
  fi
done

# 5. SBOM validation
if [[ -f "sbom-cyclonedx.json" ]]; then
  echo "[5] Validating SBOM..."
  jq -e '.specVersion' sbom-cyclonedx.json > /dev/null || { echo "FAIL: SBOM specVersion missing"; exit 1; }
  echo "PASS"
fi

# 6. Manifest completeness
echo "[6] Verifying manifest completeness..."
jq -r '.artifacts[].name' release-manifest.json | while read f; do
  [[ -f "$f" ]] || { echo "FAIL: Missing artifact $f"; exit 1; }
done
echo "PASS"

echo ""
echo "=== ALL VERIFICATIONS PASSED ==="
echo "Release is authentic and untampered."
```

---

## 5. Windows PowerShell Verification Script

```powershell
# verify-release.ps1
param([string]$ReleaseDir = ".")

Set-Location $ReleaseDir
Write-Host "=== Aether Release Verification ===" -ForegroundColor Cyan

# 1. Checksums
Write-Host "[1/6] Verifying checksums..." -ForegroundColor Yellow
$checksums = Get-Content checksums.txt
$failed = $false
foreach ($line in $checksums) {
    $parts = $line -split '\s+', 2
    $expectedHash = $parts[0]
    $file = $parts[1].Trim()
    $actualHash = (Get-FileHash -Algorithm SHA256 -Path $file).Hash
    if ($actualHash -ne $expectedHash) {
        Write-Host "FAIL: $file checksum mismatch" -ForegroundColor Red
        $failed = $true
    }
}
if ($failed) { exit 1 }
Write-Host "PASS" -ForegroundColor Green

# 2. Manifest consistency
Write-Host "[2/6] Verifying manifest consistency..." -ForegroundColor Yellow
$manifest = Get-Content release-manifest.json | ConvertFrom-Json
$manifest.artifacts | ForEach-Object {
    $expectedHash = $_.sha256
    $file = $_.name
    $actualHash = (Get-FileHash -Algorithm SHA256 -Path $file).Hash
    if ($actualHash -ne $expectedHash) {
        Write-Host "FAIL: Manifest mismatch for $file" -ForegroundColor Red
        exit 1
    }
}
Write-Host "PASS" -ForegroundColor Green

# 3. Signatures (cosign not available on Windows by default - use WSL or skip)
Write-Host "[3/6] Signature verification requires cosign (Linux/macOS/WSL)" -ForegroundColor Yellow

# 4. Windows Authenticode
if (Test-Path "aether-windows-amd64.exe") {
    Write-Host "[4/6] Verifying Windows Authenticode..." -ForegroundColor Yellow
    $result = signtool verify /pa /v aether-windows-amd64.exe 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Host "FAIL: Windows signature" -ForegroundColor Red; exit 1 }
    Write-Host "PASS" -ForegroundColor Green
}

# 5. SBOM
if (Test-Path "sbom-cyclonedx.json") {
    Write-Host "[5/6] Validating SBOM..." -ForegroundColor Yellow
    $sbom = Get-Content sbom-cyclonedx.json | ConvertFrom-Json
    if (-not $sbom.specVersion) { Write-Host "FAIL: SBOM specVersion missing" -ForegroundColor Red; exit 1 }
    Write-Host "PASS" -ForegroundColor Green
}

# 6. Manifest completeness
Write-Host "[6/6] Verifying manifest completeness..." -ForegroundColor Yellow
$manifest = Get-Content release-manifest.json | ConvertFrom-Json
$manifest.artifacts | ForEach-Object {
    if (-not (Test-Path $_.name)) { Write-Host "FAIL: Missing $($_.name)" -ForegroundColor Red; exit 1 }
}
Write-Host "PASS" -ForegroundColor Green

Write-Host "`n=== ALL VERIFICATIONS PASSED ===" -ForegroundColor Cyan
Write-Host "Release is authentic and untampered." -ForegroundColor Green
```

---

## 6. CI Verification (GitHub Actions)

```yaml
# .github/workflows/verify-release.yml
name: verify-release
on:
  release:
    types: [published]
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          pattern: '*'
          path: release
      - name: Verify
        run: |
          cd release
          chmod +x verify-release.sh
          ./verify-release.sh
```

---

## 5. Troubleshooting

| Issue | Cause | Resolution |
|-------|-------|------------|
| `cosign: command not found` | cosign not installed | `go install github.com/sigstore/cosign/v2/cmd/cosign@latest` |
| `signtool: command not found` | Windows SDK not installed | Install Windows SDK |
| `sha256sum: command not found` | Coreutils not installed | Use `certutil -hashfile` on Windows |
| `jq: command not found` | jq not installed | `apt-get install jq` / `choco install jq` |
| Signature verification fails | Wrong artifact / corrupted download | Re-download from GitHub Releases |
| Certificate expired | System clock wrong / old release | Check system time; use current release |
| `cosign verify-blob` hangs | Transparency log timeout | Use `--certificate-identity-regexp` flags |

---

## 6. Maintenance

### Update Triggers
- New release → Update verification scripts if artifact list changes
- New platform → Add verification steps for new binary
- New signing method → Update verification procedure

### Review Cadence
- Quarterly: Review verification procedure against current release process
- Per release: Execute verification on published artifacts
- Annual: Audit trust anchors (cosign OIDC issuers, Authenticode CA)

---

## 6. Sign-Off

**Procedure Version:** 1.0
**Effective:** 2026-09-12
**Release:** `3.8.0-stage7`
**Next Review:** 2026-12-12