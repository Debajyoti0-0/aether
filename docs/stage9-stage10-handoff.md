# Stage 9 → Stage 10 Handoff Document

**Date:** 2026-09-13
**From:** Stage 9 (GA Evidence Closure & Production Authorization)
**To:** Stage 10 (Production GA Achievement)
**Repository:** `C:\dev\aether`
**Baseline Commit:** `ffda1dc`
**Target Release:** `4.0.0-rc1` → `4.0.0` GA

---

## 1. Handoff Summary

Stage 9 has completed forensic reconciliation of Stage 8 claims, implemented release infrastructure, and produced a **production-limited release candidate `4.0.0-rc1`** with formally documented waivers and limitations.

**Stage 9 Verdict:** `4.0.0-rc1` authorized. `4.0.0` GA **BLOCKED** pending:
1. EV Authenticode certificate procurement
2. HSM/KMS audit key custody implementation
3. CI pipeline execution with full trust chain verification

---

## 2. Current State (Authoritative)

| Item | Value |
|------|-------|
| Repository | `C:\dev\aether` (off OneDrive) |
| Commit | `ffda1dc` |
| Version | `4.0.0-rc1` (in VERSION file) |
| Branch | `master` |
| Tags | None (tag `v4.0.0-rc1` to be created) |
| Working Tree | Clean (all Stage 9 artifacts committed) |

---

## 3. What Works (Verified)

| Capability | Evidence |
|------------|----------|
| Multi-platform builds (4 targets) | GoReleaser snapshot PASS |
| SHA-256 checksums | `dist/checksums.txt` verified |
| GitHub Actions release workflow | `.github/workflows/release.yml` committed |
| Cosign keyless signing design | `.goreleaser.yml` + CI workflow |
| Authenticode signing script | `scripts/sign-windows.ps1` committed |
| Verification + 9 tamper tests | `scripts/verify-release.sh/.ps1` committed |
| 19 native fuzz targets | 10.8M+ execs, 0 crashes |
| Request idempotency (spine) | 4/4 integration tests PASS |
| Audit key rotation (local provider) | Unit tests PASS |
| Mock KMS provider | Unit tests PASS |
| Revocation model | Key rotation + operator cert + idempotency |

---

## 4. What Requires Stage 10 Execution

| Blocker | Required Action | Stage 10 Workstream |
|---------|-----------------|---------------------|
| **B4** EV Authenticode cert | Purchase EV cert; add to GitHub Secrets; verify Windows signing in CI | WS1 |
| **B3** CI pipeline execution | Push `v4.0.0-rc1` tag; verify all jobs pass (validate→goreleaser→windows-sign→verify→publish) | WS3 |
| **B5** HSM/KMS custody | Implement Azure Key Vault or AWS KMS provider; test rotation/recovery | WS2 |
| **B6** Idempotency hardening | Expand to 10+ test scenarios; all pass under race detector | WS6 (partial) |

---

## 5. Waivers Carried Forward

| Blocker | Waiver ID | Expiry | Impact |
|---------|-----------|--------|--------|
| B1 Live Entra ID | B1-WAIVER-2026-09-13 | 2027-06-30 | Entra ID claims = OFFLINE_VALIDATED_ONLY |
| B2 Live IMDS | B2-WAIVER-2026-09-13 | 2027-06-30 | Azure IMDS claims = OFFLINE_VALIDATED_ONLY |

**Stage 10 MUST re-evaluate before expiry.**

---

## 6. Release Artifacts Expected from Stage 10

Upon successful `v4.0.0-rc1` CI execution:

| Artifact | Verification |
|----------|--------------|
| `aether_4.0.0-rc1_{linux,darwin,windows}_{amd64,arm64}.{tar.gz,zip}` | cosign verify-blob + checksums |
| `checksums.txt` | SHA-256 verified |
| `sbom-cyclonedx.json` | syft generated, cosign signed |
| `release-manifest.json` | Complete inventory, cosign signed |
| `provenance.json` | SLSA provenance, cosign signed |
| `*.sig` / `*.pem` | Cosign keyless signatures |
| `aether_4.0.0-rc1_windows_amd64.exe` | Authenticode signed + timestamped |

---

## 7. Stage 10 Entry Criteria

Stage 10 may begin when:
- [ ] `v4.0.0-rc1` tag created and pushed
- [ ] CI pipeline executes end-to-end (all 5 jobs PASS)
- [ ] EV certificate procured and Windows signing verified
- [ ] At least one HSM/KMS provider implemented and tested in CI
- [ ] All 9 tamper tests PASS in CI verify job
- [ ] Independent verification of CI artifacts completed

---

## 8. Stage 10 Scope Lock

**Authorized Workstreams (WS1-WS10):**

1. **WS1** — EV Authenticode certificate procurement
2. **WS2** — HSM/KMS provider implementation (Azure KV / AWS KMS)
3. **WS3** — CI pipeline execution on `v4.0.0-rc1` tag
4. **WS4** — CI configuration hardening (action SHA pinning, branch protection)
5. **WS5** — Independent verification of CI release artifacts
6. **WS6** — Idempotency test expansion (10+ scenarios)
7. **WS7** — Performance/scale testing (load, stress, soak)
8. **WS8** — Security audit & penetration testing
9. **WS9** — Compliance matrix (SOC2/ISO 27001)
10. **WS10** — Operational runbooks (incident response, DR, monitoring)

**Explicitly Prohibited:**
- New protocol families or cloud providers
- New execution backends or relay mechanisms
- UI systems or plugin architectures
- Broad architectural rewrites
- Feature expansion beyond blocker closure

---

## 9. Version & Release Policy

| Channel | Version | Criteria |
|---------|---------|----------|
| **Production-Limited RC** | `4.0.0-rc1` | Current — blockers B3/B4/B5 open |
| **Production-Limited RC** | `4.0.0-rc2` | If B3/B4/B5 partially resolved |
| **GA** | `4.0.0` | ALL production blockers resolved |

**No GA tag until all production blockers CLOSED.**

---

## 10. Key Documents for Stage 10

| Document | Path | Purpose |
|----------|------|---------|
| Stage 8 Reconciliation | `docs/stage9-stage8-reconciliation.md` | Baseline truth |
| Blocker Register | `docs/stage9-blocker-register.md` | Current blocker status |
| Pipeline Execution | `docs/stage9-release-pipeline-execution.md` | CI design + local verification |
| Artifact Trust | `docs/stage9-artifact-trust-report.md` | Verification procedures |
| Authenticode Closure | `docs/stage9-authenticode-closure.md` | Signing status |
| Audit Key Custody | `docs/stage9-audit-key-custody.md` | KeyProvider + providers |
| Revocation Model | `docs/stage9-revocation-model.md` | Operational authorization |
| Entra Decision | `docs/stage9-entra-evidence-decision.md` | Waiver B1 |
| IMDS Decision | `docs/stage9-imds-evidence-decision.md` | Waiver B2 |
| Independent Verification | `docs/stage9-independent-verification.md` | Reproduction steps |
| Adversarial Audit | `docs/stage9-adversarial-ga-audit.md` | Trust chain gaps |
| GA Decision | `docs/stage9-ga-decision.md` | Release verdict |
| Final Report | `docs/stage9-final-report.md` | This summary |

---

## 11. Contact & Ownership

| Role | Owner |
|------|-------|
| Stage 10 Lead | [To be assigned] |
| Security (EV cert, HSM/KMS) | Security Team |
| Platform (CI, GoReleaser) | Platform Team |
| Engineering (Idempotency, Testing) | Engineering |
| Compliance (SOC2, ISO) | Compliance |
| Operations (Runbooks, Monitoring) | Operations |

---

## 12. Sign-Off

**Stage 9 Complete:** ✅ All gates executed, artifacts committed, release decision documented.

**Stage 10 Authorized:** ✅ Scope locked, entry criteria defined, workstreams assigned.

**Next Action:** Create and push tag `v4.0.0-rc1` to trigger CI pipeline.

---

**Handoff Prepared By:** Stage 9 Automated Execution
**Date:** 2026-09-13
**Final Commit:** `ffda1dc`
**Status:** **HANDOFF COMPLETE — STAGE 10 AUTHORIZED**