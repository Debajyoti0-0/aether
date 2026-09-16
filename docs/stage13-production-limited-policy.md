# Stage 13 — Production-Limited Release Policy

**Timestamp:** 2026-09-16
**Release:** 4.0.0-rc2 (Production-Limited)
**Classification:** RESTRICTED USE — NOT GENERAL AVAILABILITY

---

## Purpose

This document defines the precise meaning, boundaries, and enforcement of the `PRODUCTION-LIMITED` release classification for Aether `4.0.0-rc2`. It exists to prevent misinterpretation of the release status and to establish explicit boundaries for authorized use.

---

## Classification Hierarchy

| Classification | Meaning | GA Equivalent? | Authorized Use |
|----------------|---------|----------------|----------------|
| Development | Not release-qualified | No | Internal development only |
| RC (Release Candidate) | Candidate under validation | No | Controlled testing only |
| **Production-Limited** | **Restricted use with documented limitations** | **No** | **Only under defined policy (this document)** |
| GA (General Availability) | Mandatory release requirements satisfied | Yes | Full release policy |

**CRITICAL:** `PRODUCTION-LIMITED` does **not** mean "almost GA." It means **restricted use with explicit, documented limitations.**

---

## Intended Users

| User Category | Authorized | Conditions |
|---------------|------------|------------|
| Aether Core Team | ✅ YES | Full access for validation |
| Authorized Internal Teams | ✅ YES | With documented risk acceptance |
| Expert Lab Environments | ✅ YES | Isolated, non-production |
| Security Researchers | ✅ YES | With vulnerability disclosure agreement |
| **General Public** | ❌ NO | Not authorized |
| **Production Infrastructure** | ❌ NO | Not authorized |
| **Controlled Enterprise** | ⚠️ CONDITIONAL | Requires explicit risk acceptance & mitigation plan |

---

## Intended Environments

| Environment | Authorized | Constraints |
|-------------|------------|-------------|
| Isolated lab networks | ✅ YES | No external connectivity required |
| Air-gapped test environments | ✅ YES | Full offline operation |
| CI/CD pipelines (test) | ✅ YES | Non-production pipelines only |
| Staging environments | ⚠️ CONDITIONAL | No real user data, no external dependencies |
| **Production networks** | ❌ NO | Not authorized |
| **Customer-facing systems** | ❌ NO | Not authorized |
| **Regulated environments** | ❌ NO | Not authorized |

---

## Supported Use Cases

| Use Case | Supported | Notes |
|----------|-----------|-------|
| Security research & validation | ✅ YES | Core capability |
| Protocol interoperability testing | ✅ YES | SAML, WS-Trust, OAuth2, MS-OAPX |
| Identity graph analysis | ✅ YES | Capability engine, pivot operations |
| Credential hygiene assessment | ✅ YES | PRT, token, session analysis |
| Attack path simulation | ✅ YES | Graph-based planning |
| Evidence collection & audit | ✅ YES | Signed audit chain |
| **Production identity operations** | ❌ NO | Requires GA |
| **Live Entra ID tenant operations** | ❌ NO | B1 waived, not validated |
| **Live Azure IMDS operations** | ❌ NO | B2 waived, not validated |
| **Production key custody** | ❌ NO | B5 not live-validated |

---

## Unsupported Use Cases

| Use Case | Reason |
|----------|--------|
| Production identity provider integration | B1/B2 waived, not validated |
| Production Azure Key Vault key custody | B5 not live-validated |
| High-availability production deployment | No HA testing, no observability |
| Regulated/compliance workloads | No compliance validation |
| Customer data processing | No data protection guarantees |
| Multi-tenant production SaaS | No tenant isolation validation |
| 24/7 production operations | No SLA, no observability, no on-call |

---

## Known Limitations (Explicit)

| Limitation | Blocker | Impact | Mitigation |
|------------|---------|--------|------------|
| **Race detector not CI-validated** | B3 | Potential data races in production | Restrict to single-threaded or lab use |
| **Windows binary unsigned** | B4 (waived) | SmartScreen warnings, no Authenticode trust | Linux/macOS preferred; Windows lab-only |
| **Azure KV not live-validated** | B5 | Key custody not production-proven | Use local/MockKMS for signing; KV for lab only |
| **Release Validate CI unconfirmed** | B7 | Release pipeline not CI-validated | Manual validation required |
| **No provenance/SLSA** | Supply-chain | Supply chain verification limited | Manual checksum/SIG verification |
| **No branch protection** | Supply-chain | Force-push risk to main | Manual gatekeeping |
| **No secret scanning** | Supply-chain | Credential leak risk | Manual hygiene |
| **No live Entra/IMDS** | B1/B2 | Cloud identity not validated | Lab-only cloud operations |

---

## Security Assumptions

1. **Trusted operator** — Only authorized, trained personnel operate the tool
2. **Isolated environment** — No untrusted network exposure
3. **No production data** — No real credentials, tokens, or customer data processed
4. **Manual verification** — All critical operations manually verified
5. **Incident response** — Operator capable of manual rollback/recovery
6. **Audit logging** — All operations logged and reviewed

---

## External Validation Requirements

| Validation | Status | Required For GA |
|------------|--------|-----------------|
| Live Entra ID (B1) | WAIVED (2027-06-30) | YES |
| Live Azure IMDS (B2) | WAIVED (2027-06-30) | YES |
| Live Azure Key Vault (B5) | WAIVED (2027-03-31) | YES |
| Race detector CI (B3) | PENDING | YES |
| Release Validate CI (B7) | PENDING | YES |
| EV Authenticode (B4) | WAIVED (2027-03-31) | YES |
| Provenance/SLSA | DEFERRED | YES |
| Branch protection | PENDING | YES |

---

## Operational Restrictions

### Must Do
- ✅ Run in isolated/test environments only
- ✅ Use Linux/macOS for production-like operations (Windows unsigned)
- ✅ Use local/MockKMS providers for audit signing
- ✅ Manually verify all artifacts via VERIFY.md
- ✅ Log all operations for audit
- ✅ Report anomalies immediately

### Must Not
- ❌ Process production credentials or tokens
- ❌ Connect to production Entra ID tenants
- ❌ Use Azure IMDS for production operations
- ❌ Use Azure Key Vault for production key custody
- ❌ Deploy in HA/production configurations
- ❌ Process regulated or customer data
- ❌ Expose to untrusted networks

---

## Support Expectations

| Aspect | Expectation |
|--------|-------------|
| **SLA** | None — no uptime guarantees |
| **Bug fixes** | Best effort, next RC/GA |
| **Security patches** | Best effort, next RC/GA |
| **Documentation** | As-is, may be incomplete |
| **Upgrades** | Manual, breaking changes possible |
| **Rollback** | Manual, operator responsibility |

---

## Conditions for GA Progression

The following MUST be satisfied before `v4.0.0` GA:

1. ✅ B3 Race Detector — CI green (race green)
2. ✅ B4 EV Authenticode — Cert procured & verified OR waiver expired
3. ✅ B5 Azure KV — Live round-trip validated
4. ✅ B7 Release Validate — CI green
5. ✅ Provenance/SLSA — Generated and verified
5. ✅ Branch protection — Enabled on main
6. ✅ Secret scanning — Enabled
7. ✅ Dependabot — Enabled
8. ✅ VERIFY.md — Reproduced in fresh container
9. ✅ VERSION = 4.0.0, tag v4.0.0 created and frozen

---

## Conditions Requiring Withdrawal/Reclassification

The release SHALL be withdrawn or reclassified if:

1. Critical security vulnerability discovered
2. Data corruption or loss observed
3. Race condition causes production data loss
4. Unauthorized production use detected
5. Critical dependency vulnerability unpatchable
5. GA blockers not resolved by waiver expiry dates

---

## Upgrade Expectations

| From Version | To Version | Expected Effort |
|--------------|------------|-----------------|
| 4.0.0-rc2 | 4.0.0 (GA) | Low (blocker fixes only) |
| 4.0.0-rc2 | 4.1.0 | Medium (Phase B features) |
| 3.x | 4.0.0-rc2 | High (breaking changes) |

**Breaking changes possible** between RC2 and GA. No upgrade guarantees.

---

## Artifact Verification Requirements

Before any use, operators MUST verify:

```bash
# 1. Checksums
sha256sum -c checksums.txt

# 2. Cosign signatures (Linux/macOS)
cosign verify-blob --signature <artifact>.sig --certificate <artifact>.pem \
  --certificate-identity-regexp ".*" --certificate-oidc-issuer-regexp ".*" <artifact>

# 3. SBOM verification
jq -e '.specVersion' sbom-cyclonedx.json

# 4. Windows (if used) — expect SmartScreen warning
signtool verify /pa /v aether.exe  # Will FAIL (unsigned)
```

---

## Limitation Register (Public)

| ID | Limitation | Blocker | Expiry | Status |
|----|------------|---------|--------|--------|
| LIM-001 | Race detector not CI-validated | B3 | 2027-03-31 | OPEN |
| LIM-002 | Windows binary unsigned | B4 | 2027-03-31 | WAIVED |
| LIM-003 | Azure KV not live-validated | B5 | 2027-03-31 | WAIVED |
| LIM-004 | Release Validate CI unconfirmed | B7 | 2027-03-31 | OPEN |
| LIM-005 | No provenance/SLSA | Supply-chain | 2027-03-31 | OPEN |
| LIM-006 | No branch protection | Supply-chain | 2027-03-31 | OPEN |

---

## Authorization

**This policy authorizes use of Aether 4.0.0-rc2 ONLY under the conditions defined herein.**

Any use outside these boundaries is **unauthorized** and **unsupported**.

**Authorized by:** Release Engineering Lead
**Date:** 2026-09-16
**Review date:** 2027-03-31 (or upon GA, whichever first)

---

*Generated by Stage 13 — Production-Limited Release Policy*