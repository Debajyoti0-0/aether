# Stage 15 — Limitation Register

**Timestamp:** 2016-09-16
**Scope:** Public-facing limitation register for 4.0.0-rc2 Production-Limited release

---

## Limitation Register

| ID | Limitation | Blocker | Expiry | Impact | Mitigation |
|----|------------|---------|--------|--------|------------|
| LIM-001 | Race detector not CI-validated | B3 | 2027-03-31 | Potential data races in production under load | Mutex fixes applied; Linux-only race verification available |
| LIM-002 | Windows binary unsigned | B4 | 2027-03-31 | SmartScreen warnings; no trust chain verification | Linux/macOS binaries cosign-signed; document Windows limitation |
| LIM-003 | Azure KV not live-validated | B5 | 2027-03-31 | Key custody not production-proven | Contract fixed; capability-aware interface; local/MockKMS for Ed25519 |
| LIM-004 | Release Validate CI unconfirmed | B7 | 2027-03-31 | Release pipeline not CI-validated | Local validation PASS; CI workflow ready |
| LIM-005 | No provenance/SLSA | Supply Chain | 2027-03-31 | Supply chain verification limited | SBOM + checksums + cosign in CI; goreleaser upgrade deferred |
| LIM-006 | No branch protection | Supply Chain | 2027-03-31 | Force-push risk to main | Documented as waived; manual enablement required |

---

## Limitation Details

### LIM-001: Race Detector Not CI-Validated (B3)
- **Status:** WAIVED (expiry 2027-03-31)
- **Root Cause:** No gcc/mingw locally; CI race isolation workflow created but not executed on candidate commit
- **Mitigation:** Mutex fixes applied to AzureKVProvider and Workspace Rekey; Linux-only race verification available via CI
- **Revalidation:** CI race isolation workflow must pass on candidate commit before GA

### LIM-002: Windows Binary Unsigned (B4)
- **Status:** WAIVED (expiry 2027-03-31)
- **Root Cause:** No EV Authenticode certificate procured
- **Impact:** Windows SmartScreen warnings; no Authenticode trust chain
- **Mitigation:** Linux/macOS binaries cosign-signed; explicit Windows limitation documented
- **Revalidation:** Procure EV certificate or renew waiver before 2027-03-31

### LIM-003: Azure KV Not Live-Validated (B5)
- **Status:** WAIVED (expiry 2027-03-31)
- **Root Cause:** No authorized Azure environment for live validation
- **Implementation Status:** Contract fixed (capability-aware interface, ErrUnsupportedOperation, mutex); key lifecycle ops implemented
- **Limitation:** No live Azure Key Vault round-trip validation
- **Mitigation:** Contract fixed with capability-aware interface; local/MockKMS providers available for Ed25519 operations
- **Revalidation:** Live Azure KV round-trip required before GA

### LIM-004: Release Validate CI Unconfirmed (B7)
- **Status:** WAIVED (expiry 2027-03-31)
- **Root Cause:** Release Validate workflow not triggered for candidate commit f1242dd/fc062e0
- **Local Status:** All 11 validation steps PASS locally
- **Limitation:** Release pipeline not CI-validated on candidate commit
- **Revalidation:** Trigger Release workflow on candidate; must pass before GA

### LIM-005: No Provenance/SLSA (Supply Chain)
- **Status:** WAIVED (expiry 2027-03-31)
- **Root Cause:** goreleaser v2.18.1 lacks `attestations` and `signs` support
- **Impact:** Supply chain verification limited
- **Mitigation:** SBOM + checksums + cosign in CI; goreleaser upgrade deferred
- **Revalidation:** Upgrade goreleaser to ≥ v2.19 for provenance/cosign support

### LIM-006: No Branch Protection (Supply Chain)
- **Status:** WAIVED (expiry 2027-03-31)
- **Root Cause:** Manual GitHub configuration required
- **Risk:** Force-push risk to main
- **Mitigation:** Documented as waived; manual enablement required
- **Revalidation:** Enable branch protection via GitHub UI before GA

---

## Waiver Expiry Summary

| Blocker | Waiver Expiry | Days Until Expiry (from 2026-09-16) |
|---------|---------------|-------------------------------------|
| B3 Race Detector | 2027-03-31 | ~195 days |
| B4 EV Authenticode | 2027-03-31 | ~195 days |
| B5 Azure KV | 2027-03-31 | ~195 days |
| B7 Release Validate | 2027-03-31 | ~195 days |
| B1 Live Entra | 2027-06-30 | ~285 days |
| B2 IMDS | 2027-06-30 | ~285 days |

---

## Revalidation Triggers

| Limitation | Revalidation Trigger |
|------------|---------------------|
| LIM-001 (B3) | CI race isolation workflow GREEN on candidate commit |
| LIM-002 (B4) | EV certificate procured and verified |
| LIM-003 (B5) | Live Azure KV round-trip executed and verified |
| LIM-004 (B7) | Release Validate CI GREEN on candidate commit |
| LIM-005 (Supply Chain) | Goreleaser ≥ v2.19 with provenance/cosign; repo controls enabled |
| LIM-006 (B6) | Race detector CI green + expanded test scenarios |

---

## Public Release Notes Language

```markdown
## Known Limitations (4.0.0-rc2 Production-Limited)

### LIM-001: Race Detector Not CI-Validated
The Go race detector has not been validated in CI for this release. Mutex fixes have been applied to key components (AzureKVProvider, Workspace Rekey), but CI race validation is pending.
**Impact:** Potential data races under high concurrency.
**Expiry:** 2027-03-31
**Workaround:** Linux-only race verification available; avoid high-concurrency production use.

### LIM-002: Windows Binary Unsigned
The Windows binary is not Authenticode signed with an EV certificate. Users will encounter Windows SmartScreen warnings.
**Impact:** Windows SmartScreen warnings; no Authenticode trust chain.
**Expiry:** 2027-03-31
**Workaround:** Linux/macOS binaries are cosign-signed and fully verified.

### LIM-003: Azure Key Vault Not Live-Validated
The Azure Key Vault provider has not been validated against a live Azure Key Vault instance.
**Impact:** Key custody operations using Azure KV are not production-proven.
**Expiry:** 2027-03-31
**Workaround:** Contract fixed with capability-aware interface; local/MockKMS providers available for Ed25519 operations.

### LIM-004: Release Validate CI Unconfirmed
The Release Validate CI workflow has not been executed against the release candidate.
**Impact:** Release pipeline not CI-validated on this candidate.
**Expiry:** 2027-03-31

### LIM-005: No Provenance/SLSA
SLSA provenance attestations are not generated due to goreleaser version limitation.
**Impact:** Supply chain verification limited to SBOM + checksums + cosign.
**Expiry:** 2027-03-31

### LIM-006: No Branch Protection
GitHub branch protection rules are not configured on the main branch.
**Impact:** Force-push risk to main branch.
**Expiry:** 2027-03-31
```

---

*Generated by Stage 15 — Limitation Register*