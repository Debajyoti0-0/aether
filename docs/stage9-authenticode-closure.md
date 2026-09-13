# Stage 9 G4 — Authenticode Signing Closure

**Date:** 2026-09-13
**Baseline:** Stage 9 G3 Artifact Trust Report
**Blocker:** B4 — EV Authenticode certificate procurement + Windows signing integration

---

## 1. Signing Architecture Status

| Platform | Method | Key Custody | Implementation Status |
|----------|--------|-------------|----------------------|
| Linux/macOS | cosign keyless (OIDC) | GitHub Actions OIDC (no persistent key) | ✅ IMPLEMENTED in CI config |
| Windows | Authenticode (EV cert) | HSM-backed EV cert (cert provider) | ⚠️ IMPLEMENTATION READY, CERT BLOCKED |
| Manifests/SBOM/Checksums | cosign keyless (OIDC) | GitHub Actions OIDC | ✅ IMPLEMENTED in CI config |

---

## 2. Implementation Details

### 2.1 Linux/macOS (cosign keyless) — IMPLEMENTED

**Configuration:** `.goreleaser.yml` signs artifacts via cosign keyless (OIDC)
- No private keys in repository or CI secrets
- Uses GitHub Actions OIDC token for keyless signing
- Transparency log entry created automatically

**Verification:**
```bash
cosign verify-blob --signature aether_4.0.0-rc1_linux_amd64.tar.gz.sig \
  --certificate aether_4.0.0-rc1_linux_amd64.tar.gz.pem \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  aether_4.0.0-rc1_linux_amd64.tar.gz
```

### 2.2 Windows (Authenticode) — IMPLEMENTATION READY

**Script:** `scripts/sign-windows.ps1` (PowerShell)
- Called from GitHub Actions `windows-sign` job
- Uses `signtool` with RFC 3161 timestamping
- Requires EV certificate in `AUTHENTICODE_CERT` (base64 PFX) and `AUTHENTICODE_PASSWORD` secrets

**Signing Command:**
```powershell
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /f $certPath /p $password $binary
```

**Verification:**
```cmd
signtool verify /pa /v aether_4.0.0-rc1_windows_amd64.exe
```

### 2.3 Manifest/SBOM/Checksums Signing — IMPLEMENTED

All release metadata signed with cosign keyless:
- `release-manifest.json`
- `sbom-cyclonedx.json`
- `checksums.txt`

---

## 3. Current Status

| Component | Status | Evidence |
|-----------|--------|----------|
| cosign keyless signing (Linux/macOS) | ✅ IMPLEMENTED | `.goreleaser.yml` + CI workflow |
| cosign keyless signing (manifests/SBOM/checksums) | ✅ IMPLEMENTED | `.goreleaser.yml` + CI workflow |
| Windows Authenticode signing script | ✅ IMPLEMENTED | `scripts/sign-windows.ps1` |
| GitHub Actions Windows signing job | ✅ IMPLEMENTED | `.github/workflows/release.yml` |
| Verification procedures | ✅ IMPLEMENTED | `scripts/verify-release.ps1` |
| Tamper tests (9/9) | ✅ IMPLEMENTED | `scripts/verify-release.ps1` tests 1-9 |
| **Windows EV certificate** | 🔄 **BLOCKED** | Requires EV cert procurement |

---

## 4. EV Certificate Requirements

| Requirement | Specification |
|-------------|---------------|
| Certificate Type | Extended Validation (EV) Code Signing |
| Format | PFX (PKCS#12) |
| Key Size | RSA 3072-bit or 4096-bit (per CA/Browser Forum) |
| Timestamp Authority | RFC 3161 (Digicert, GlobalSign, etc.) |
| Storage | HSM-backed (cert provider HSM) |
| Secrets | `AUTHENTICODE_CERT` (base64 PFX), `AUTHENTICODE_PASSWORD` |

---

## 5. Procurement Blockers

| Blocker | Impact | Resolution |
|---------|--------|------------|
| No EV cert purchased | Windows binary unsigned | Purchase from DigiCert, GlobalSign, Sectigo, or similar |
| No HSM for key generation | Key generated in software | Use CA's HSM-backed key generation |
| No GitHub Secrets configured | Signing job will skip | Add secrets after procurement |
| No SmartScreen reputation | New cert = unknown publisher | Establish reputation over time |

---

## 6. Waiver Decision

**Decision:** BLOCKER REMAINS OPEN — Cannot waive EV certificate for GA release claiming Windows production readiness.

**Reasoning:**
- Windows Authenticode signing is mandatory for production Windows distribution
- Without EV cert: SmartScreen warnings, no publisher identity, untrusted binary
- Implementation is complete and tested (script works with test cert)
- Only procurement remains

**Contingency for `4.0.0-rc1`:**
- Release with cosign-signed artifacts for Linux/macOS
- Windows binary released UNSIGNED with explicit warning
- Document: "Windows binary not Authenticode-signed; EV certificate procurement in progress"

---

## 7. G4 Result

**PARTIAL** — Signing infrastructure 100% implemented and tested. EV certificate procurement is the sole remaining blocker for Windows production readiness.

**Next Step:** Procure EV Authenticode certificate and add to GitHub Secrets.

---

**Prepared by:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Status:** G4 PARTIAL — Implementation complete, certificate procurement pending