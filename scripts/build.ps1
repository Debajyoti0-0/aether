#Requires -Version 5.1
<#
.SYNOPSIS
    Cross-platform build script for Aether on Windows.
.EXAMPLE
    .\scripts\build.ps1                    # build for the host platform
    .\scripts\build.ps1 -All               # build every release target
    .\scripts\build.ps1 -Test              # run the test suite
#>
param(
    [switch]$All,
    [switch]$Test,
    [switch]$Vet,
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# Single source of truth for the version (see internal/version/version.go).
$version = (Get-Content (Join-Path $root 'VERSION') -ErrorAction SilentlyContinue | Select-Object -First 1)
if (-not $version) { $version = 'dev' }
$commit = 'unknown'
try { $commit = (git rev-parse --short HEAD) 2>$null } catch { }
$ldflags = "-s -w -X github.com/Debajyoti0-0/aether/internal/version.Version=$version -X github.com/Debajyoti0-0/aether/internal/version.Commit=$commit"

$outDir = Join-Path $root 'bin'

if ($Clean) {
    if (Test-Path $outDir) { Remove-Item $outDir -Recurse -Force }
    Write-Host "Cleaned $outDir"
    if (-not ($All -or $Test)) { exit 0 }
}

if ($Vet) { go vet ./...; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE } }

if ($Test) {
    go test ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    if (-not $All) { exit 0 }
}

$targets = @(
    @{ os = 'windows'; arch = 'amd64'; ext = '.exe' },
    @{ os = 'windows'; arch = 'arm64'; ext = '.exe' },
    @{ os = 'linux';   arch = 'amd64'; ext = '' },
    @{ os = 'linux';   arch = 'arm64'; ext = '' },
    @{ os = 'darwin';  arch = 'amd64'; ext = '' },
    @{ os = 'darwin';  arch = 'arm64'; ext = '' },
    @{ os = 'freebsd'; arch = 'amd64'; ext = '' }
)

if (-not $All) {
    $hostOS = if ($IsWindows -or $env:OS -eq 'Windows_NT') { 'windows' }
              elseif ($IsMacOS) { 'darwin' } else { 'linux' }
    $targets = @($targets | Where-Object { $_.os -eq $hostOS -and $_.arch -eq [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower() })
    if ($targets.Count -eq 0) { $targets = @(@{ os = 'windows'; arch = 'amd64'; ext = '.exe' }) }
}

New-Item -ItemType Directory -Force -Path $outDir | Out-Null

foreach ($t in $targets) {
    $env:GOOS = $t.os
    $env:GOARCH = $t.arch
    $name = "aether-$($t.os)-$($t.arch)$($t.ext)"
    Write-Host "Building $name ..."
    go build -ldflags $ldflags -o (Join-Path $outDir $name) ./cmd/aether
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
Write-Host "`nDone. Binaries in $outDir" -ForegroundColor Green
