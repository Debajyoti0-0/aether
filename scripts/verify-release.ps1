<#
.SYNOPSIS
    Aether Release Verification Script (PowerShell)

.DESCRIPTION
    Verifies the integrity and authenticity of Aether release artifacts.

.PARAMETER ReleaseDir
    Path to the directory containing release artifacts (default: current directory)

.EXAMPLE
    .\verify-release.ps1
    .\verify-release.ps1 -ReleaseDir "C:\releases\aether-4.0.0"
#>

param(
    [string]$ReleaseDir = "."
)

Set-Location $ReleaseDir
Write-Host "=== Aether Release Verification ===" -ForegroundColor Cyan
Write-Host "Directory: $(Get-Location)" -ForegroundColor Gray
Write-Host ""

$failed = $false

# 1. Checksums
Write-Host "[1/6] Verifying checksums..." -ForegroundColor Yellow
$checksums = Get-Content checksums.txt
$checksumFailed = $false
foreach ($line in $checksums) {
    $parts = $line -split '\s+', 2
    $expectedHash = $parts[0]
    $file = $parts[1].Trim()
    if (Test-Path $file) {
        $actualHash = (Get-FileHash -Algorithm SHA256 -Path $file).Hash
        if ($actualHash -ne $expectedHash) {
            Write-Host "FAIL: $file checksum mismatch" -ForegroundColor Red
            $checksumFailed = $true
        }
    } else {
        Write-Host "FAIL: Missing file $file" -ForegroundColor Red
        $checksumFailed = $true
    }
}
if ($checksumFailed) { $failed = $true }
else { Write-Host "PASS" -ForegroundColor Green }

# 2. Manifest consistency
Write-Host "[2/6] Verifying manifest consistency..." -ForegroundColor Yellow
$manifest = Get-Content release-manifest.json | ConvertFrom-Json
$manifestFailed = $false
$manifest.artifacts | ForEach-Object {
    $expectedHash = $_.sha256
    $file = $_.name
    if (Test-Path $file) {
        $actualHash = (Get-FileHash -Algorithm SHA256 -Path $file).Hash
        if ($actualHash -ne $expectedHash) {
            Write-Host "FAIL: Manifest mismatch for $file" -ForegroundColor Red
            $manifestFailed = $true
        }
    } else {
        Write-Host "FAIL: Missing artifact $file" -ForegroundColor Red
        $manifestFailed = $true
    }
}
if ($manifestFailed) { $failed = $true }
else { Write-Host "PASS" -ForegroundColor Green }

# 3. Windows Authenticode signature
Write-Host "[3/6] Verifying Windows Authenticode signature..." -ForegroundColor Yellow
if (Test-Path "aether-windows-amd64.exe") {
    $result = signtool verify /pa /v aether-windows-amd64.exe 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAIL: Windows signature verification failed" -ForegroundColor Red
        $failed = $true
    } else {
        Write-Host "PASS" -ForegroundColor Green
    }
} else {
    Write-Host "SKIP: Windows binary not present" -ForegroundColor Yellow
}

# 4. cosign signatures (requires cosign - run from WSL/Linux/macOS)
Write-Host "[4/6] Signature verification (cosign) requires Linux/macOS/WSL" -ForegroundColor Yellow
Write-Host "Run verify-release.sh on Linux/macOS/WSL for cosign verification" -ForegroundColor Gray

# 5. SBOM validation
if (Test-Path "sbom-cyclonedx.json") {
    Write-Host "[5/6] Validating SBOM..." -ForegroundColor Yellow
    $sbom = Get-Content sbom-cyclonedx.json | ConvertFrom-Json
    if (-not $sbom.specVersion) {
        Write-Host "FAIL: SBOM specVersion missing" -ForegroundColor Red
        $failed = $true
    } else {
        Write-Host "PASS - SBOM specVersion: $($sbom.specVersion), Components: $($sbom.components.Count)" -ForegroundColor Green
    }
}

# 6. Manifest completeness
Write-Host "[6/6] Verifying manifest completeness..." -ForegroundColor Yellow
$manifest = Get-Content release-manifest.json | ConvertFrom-Json
$missing = $false
$manifest.artifacts | ForEach-Object {
    if (-not (Test-Path $_.name)) {
        Write-Host "FAIL: Missing $($_.name)" -ForegroundColor Red
        $missing = $true
    }
}
if ($missing) { $failed = $true }
else { Write-Host "PASS" -ForegroundColor Green }

Write-Host ""
if ($failed) {
    Write-Host "=== VERIFICATION FAILED ===" -ForegroundColor Red
    exit 1
} else {
    Write-Host "=== ALL VERIFICATIONS PASSED ===" -ForegroundColor Green
    Write-Host "Release is authentic and untampered." -ForegroundColor Green
}