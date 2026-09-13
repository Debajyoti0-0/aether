@echo off
REM scripts/sign-windows.ps1
REM Windows Authenticode signing script for goreleaser post-hook
REM Requires: AUTHENTICODE_CERT (base64-encoded PFX), AUTHENTICODE_PASSWORD env vars

param(
    [Parameter(Mandatory=$true)]
    [string]$BinaryPath
)

$ErrorActionPreference = "Stop"

if (-not $env:AUTHENTICODE_CERT) {
    Write-Host "AUTHENTICODE_CERT not set, skipping Windows signing" -ForegroundColor Yellow
    exit 0
}

if (-not $env:AUTHENTICODE_PASSWORD) {
    Write-Error "AUTHENTICODE_PASSWORD not set"
    exit 1
}

if (-not (Test-Path $BinaryPath)) {
    Write-Error "Binary not found: $BinaryPath"
    exit 1
}

# Write cert to temp file
$certPath = [IO.Path]::GetTempFileName()
[IO.File]::WriteAllBytes($certPath, [Convert]::FromBase64String($env:AUTHENTICODE_CERT))

try {
    Write-Host "Signing $BinaryPath with Authenticode..."
    
    # Sign with timestamp (RFC 3161)
    $signResult = & signtool sign `
        /fd SHA256 `
        /tr http://timestamp.digicert.com `
        /td SHA256 `
        /f $certPath `
        /p $env:AUTHENTICODE_PASSWORD `
        $BinaryPath
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "signtool sign failed: $signResult"
        exit 1
    }
    
    Write-Host "Signing successful"
    
    # Verify the signature
    $verifyResult = & signtool verify /pa /v $BinaryPath
    if ($LASTEXITCODE -ne 0) {
        Write-Error "signtool verify failed: $verifyResult"
        exit 1
    }
    
    Write-Host "Verification successful" -ForegroundColor Green
}
finally {
    # Clean up temp cert file
    if (Test-Path $certPath) {
        Remove-Item -Force $certPath
    }
}