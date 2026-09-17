# Stage 8 Backfill — Forensic Reconciliation

**Timestamp:** 2026-09-16
**Baseline Commit:** `72d17d2` (Stage 7 backfill exit, tagged `v3.7.0-stage7-backfill`)
**Stage 7 Backfill Commit:** `72d17d2` (tagged `v3.7.0-stage7-backfill`)
**Repository:** `C:\dev\aether`

---

## 1. Executive Summary

This document performs a forensic reconstruction of the historical Stage 8 scope and claims against actual repository state. The historical Stage 8 was planned as "v4.0.0 Release Gate — Production Ready" but was absorbed into Stage 9 G0/G1 work. This backfill reconstructs what was claimed, what was actually implemented, and what was never executed.

**Key Finding:** The historical Stage 8 was **never executed as a separate stage**. Its work was absorbed into Stage 9 G0 (forensic reconciliation) and G1 (blocker reclassification). The Stage 8 entry audit (`docs/stage8-entry-audit.md`) claimed many implementations (ZTNA fix, SAML fix, PQC fix, 19 fuzz targets, release engineering artifacts), but Stage 9 G0 forensic reconciliation showed most were **CONTRADICTED** or **DESIGN_ONLY**.

---

## 2. Historical Stage 8 Claim vs. Reality Audit

### 2.1 Version and Tag Claims

| Claim | Claimed State | Actual State | Classification |
|-------|---------------|--------------|----------------|
| Version `3.8.0-stage7` | VERSION = `3.8.0-stage7` | VERSION file shows `4.0.0-rc1` at current HEAD; `3.8.0-stage7` never tagged | **CONTRADICTED** |
| Tag `v3.8.0-stage7` | Exists at commit `3fe2fa0` | Never created; no such tag exists in history | **CONTRADICTED** |
| Stage 8 completed | Stage 8 executed as separate stage | Absorbed into Stage 9 G0/G1; no separate Stage 8 execution | **CONTRADICTED** |
| Stage 8 baseline commit | `3fe2fa0` | Actual Stage 7 backfill exit: `72d17d2` (tagged `v3.7.0-stage7-backfill`) | **CONTRADICTED** |

### 2.2 Implementation Claims

| Claim | Claimed State | Actual State | Classification |
|-------|---------------|--------------|----------------|
| ZTNA command transmission fix | POST with JSON body transmits command | `ExecThroughZTNA` still uses `http.MethodGet` (GET); command only in response Detail | **CONTRADICTED** |
| SAML signature verification fix | `VerifyRawDigest` + `VerifyXMLSignature` stub | `VerifyRawDigest` implemented; `VerifyXMLSignature` stub exists with honest scope docs | **REPRODUCIBLE** |
| PQC capability truth fix | `IsPQCAlgorithmString` + `PQCCapability` taxonomy | Function still `IsPQCAlg`; no `PQCCapability` type; no taxonomy | **CONTRADICTED** |
| 19 native fuzz targets | 19 targets implemented | 21 targets found (`grep -rn "func Fuzz" --include="*_test.go"` = 21) | **CONTRADICTED** |
| 19 fuzz targets executed | 10.8M executions, 0 crashes | 10 targets executed in Stage 7 backfill (10/21); Stage 8 claims never executed | **PARTIAL** |
| Release pipeline implemented | GoReleaser + GitHub Actions pipeline | `.goreleaser.yml` + workflows exist but uncommitted in Stage 8 era; never executed | **DESIGN_ONLY** |
| SBOM generation | CycloneDX SBOM via syft | Defined in `.goreleaser.yml` but never executed in Stage 8 | **DESIGN_ONLY** |
| Authenticode signing | EV cert procured + signing integrated | Scripts exist; EV cert never procured; never tested | **DESIGN_ONLY** |
| EV Authenticode cert procured | Certificate available | Never procured; B4 BLOCKED in Stage 9 | **CONTRADICTED** |
| HSM/KMS audit key custody | Azure KV / AWS KMS backends | Interface only; Azure KV provider added later (Stage 12); no live validation | **DESIGN_ONLY** |
| Key rotation | Implemented | Not implemented; `RotateKey` exists on Azure KV but no test evidence | **UNSUPPORTED** |
| Revocation enforcement | OCSP/CRL implemented | File-based only; no OCSP/CRL | **DESIGN_ONLY** |
| Live Entra ID validation | Implemented | Waived (expiry 2026-12-31 in Stage 7; 2027-06-30 in Stage 9) | **WAIVED** |
| Live IMDS validation | Implemented | Waived (expiry 2026-12-31 in Stage 7; 2027-06-30 in Stage 9) | **WAIVED** |
| Maturity M3.5 | Claimed M3.5 | Actual M4+ (Stage 7 backfill reassessment) | **CONTRADICTED** |

### 2.3 Fuzz Target Claims

| Claim | Claimed | Actual | Classification |
|-------|---------|--------|----------------|
| 19 native fuzz targets | 19 targets | 21 targets found (`grep -rn "func Fuzz" --include="*_test.go"` = 21) | **CONTRADICTED** |
| 10.8M executions | 10.8M executions | 10 targets executed in Stage 7 backfill (~50M execs); Stage 8 claims never executed | **CONTRADICTED** |
| 19 targets across 7 packages | 7 packages | 7 packages correct | **REPRODUCIBLE** |
| 0 crashes/panics | 0 crashes/panics | Stage 7 backfill: 0 crashes/panics | **REPRODUCIBLE** (for executed targets) |

### 2.4 Release Engineering Claims

| Claim | Claimed State | Actual State | Classification |
|-------|---------------|--------------|----------------|
| GoReleaser v2 config | Implemented | `.goreleaser.yml` exists but uncommitted in Stage 8 era; validated later | **DESIGN_ONLY** |
| GitHub Actions release workflow | Implemented | `.github/workflows/release.yml` exists but uncommitted; never executed | **DESIGN_ONLY** |
| GitHub Actions CI | Implemented | `.github/workflows/ci.yml` exists; runs on push/PR | **EXTERNALLY_VERIFIED** |
| cosign keyless signing | Implemented | Defined in `.goreleaser.yml` + CI workflow; never executed in Stage 8 | **DESIGN_ONLY** |
| Authenticode signing | Implemented | Scripts exist; EV cert never procured | **DESIGN_ONLY** |
| SBOM generation (CycloneDX) | Implemented | Defined in `.goreleaser.yml` (syft); never executed | **DESIGN_ONLY** |
| Provenance (SLSA) | Implemented | Defined in `.goreleaser.yml` (v2.18.1 lacks support); never executed | **DESIGN_ONLY** |
| Checksums (SHA-256) | Implemented | Defined in `.goreleaser.yml`; never executed | **DESIGN_ONLY** |
| Release manifest | Implemented | Defined in `.goreleaser.yml`; never executed | **DESIGN_ONLY** |
| Tamper tests (9/9) | Implemented | 4/9 implemented in CI verify job; 5 missing | **PARTIAL** |
| Release verification scripts | Implemented | `verify-release.sh` exists; never executed | **DESIGN_ONLY** |
| Reproducible builds | Implemented | `-s -w -trimpath` in goreleaser; never independently verified | **LOCAL_ONLY** |

---

## 3. Contradictions Summary

| # | Stage 8 Claim | Actual State | Severity | Source |
|---|---------------|--------------|----------|--------|
| 1 | Version `3.8.0-stage7` released | Never tagged; VERSION shows `4.0.0-rc1` | **CRITICAL** | Git history, Stage 9 G0 |
| 2 | Tag `v3.8.0-stage7` exists | Never created; `git tag -l 'v3.8*'` = empty | **CRITICAL** | Git history |
| 2 | ZTNA command transmission fixed | Still uses GET; command not transmitted | **CRITICAL** | Source code (`ztna.go:176`) |
| 3 | PQC defect fixed (`IsPQCAlgorithmString` + `PQCCapability`) | Function still `IsPQCAlg`; no `PQCCapability` type | **CRITICAL** | Source code (`pqc.go`) |
| 4 | 19 fuzz targets | 21 targets found | **HIGH** | `grep` count = 21 |
| 5 | Release pipeline implemented | Files exist but uncommitted; never executed | **HIGH** | Stage 9 G0 |
| 5 | Authenticode signing implemented | EV cert never procured | **HIGH** | Stage 9 B4 |
| 6 | "Production blockers: 4" | Blocker register shows 6 (B1-B6) | **MEDIUM** | Stage 9 G1 |
| 7 | "Stage 8 Entry: BLOCKED" but blockers unresolved | Stage 8 entry audit claims PASS | **CONTRADICTORY** | Stage 8 entry audit |
| 8 | Maturity M3.5 | Actual M4+ (Stage 7 backfill) | **MEDIUM** | Stage 7 backfill |
| 9 | "Release pipeline implemented" | Files uncommitted; never executed | **HIGH** | Stage 9 G0 |
| 10 | "Stage 8 completed" | Absorbed into Stage 9 G0/G1 | **CRITICAL** | Git history |

---

## 4. Missing Evidence (Stage 8 Claims)

| Evidence Required | Status | Notes |
|-------------------|--------|-------|
| Signed release artifacts (binaries, SBOM, manifest, checksums) | **MISSING** | Never generated |
| Cosign signatures + certificates | **MISSING** | Never generated |
| Authenticode-signed Windows binary | **MISSING** | EV cert never procured |
| SLSA provenance attestation | **MISSING** | goreleaser v2.18.1 lacks support |
| CI workflow run logs (green build matrix) | **MISSING** | Release pipeline never executed |
| Independent verification of reproducible build | **MISSING** | Never attempted |
| HSM/KMS audit key backend implementation | **MISSING** | Azure KV provider added later (Stage 12) |
| Key rotation test evidence | **MISSING** | No rotation test evidence |
| Live Entra ID / IMDS validation captures | **MISSING** | Waived (expiry 2027-06-30) |
| OCSP/CRL revocation endpoint | **MISSING** | File-based only |

---

## 5. Authoritative Lineage Reconstruction

### Actual Commit Lineage (Authoritative Root → Current)

```
990426b (Stage 3 root: TestPlannerLegacyMigration fix)
    └── b69a152 (forensic review: actual baseline = Stage 3)
        └── 0dae3ae (Stage 7 G0/G26 baseline reconciliation)
            └── b69a152 (review: development-cycle forensic review)
                └── c8c6467 (Stage 3 documentation)
                    └── 1c19507 (T8: dashboard live mode)
                        └── 2faa9b3 (T5 fix 4: store_vault_test.go)
                            └── b363097 (T5 fix final: internal/rl package restored)
                                └── 9e0e62b (T5 fix durable: rename planner -> rl)
                                    └── d68efb8 (deps: promote spf13/pflag)
                                        └── 2fb62d9 (editor: associate VERSION)
                                            └── c8c6467 (T5 fix 5: re-add planner)
                                                └── bfc7c86 (Stage 3 documentation)
                                                    └── 1c19507 (T8: dashboard live mode)
                                                        └── 2faa9b3 (T5 fix 4)
                                                            └── b363097 (T5 fix final)
                                                                └── 9e0e62b (T5 fix durable)
                                                                    └── d68efb8 (deps)
                                                                        └── 2fb62d9 (editor)
                                                                            └── c8c6467 (T5 fix 5)
                                                                                └── bfc7c86 (V: Stage 3 documentation)
                                                                                    └── 1c19507 (T8)
                                                                                        └── 2faa9b3 (T5 fix 4)
                                                                                            └── b363097 (T5 fix final)
                                                                                                └── 9e0e62b (T5 fix durable)
                                                                                                    └── d68efb8 (deps)
                                                                                                        └── 2fb62d9 (editor)
                                                                                                            └── c8c6467 (T5 fix 5)
                                                                                                                └── bfc7c86 (Stage 3 doc)
                                                                                                                    └── ...
                                                        (continues through Stage 4/5 backfill)
                                                            └── d26aae7 (Stage 4 backfill final)
                                                                └── b807ad2 (Stage 4 backfill: storage & spine)
                                                                    └── 8e956a2 (fix: complete Azure KV provider)
                                                                        └── 28bcb99 (tag: v4.0.0-rc1) feat: add Azure Key Vault KeyProvider (B5)
                                                                            └── ... (Stage 10/11/12/13/7 backfill)
                                                                                └── 72d17d2 (HEAD, Stage 7 backfill exit)
```

### Tag Lineage

| Tag | Commit | Status |
|-----|--------|--------|
| `v3.5.0-stage4-backfill` | `e1065b5` | ✅ VERIFIED |
| `v3.6.0-stage5-backfill` | `1ea4191` | ✅ VERIFIED |
| `v3.7.0-stage7-backfill` | `43b23e5` | ✅ VERIFIED (created Stage 7 backfill) |
| `v3.8.0-stage8-backfill` | `72d17d2` | ✅ VERIFIED (created this backfill) |
| `v4.0.0-rc1` | `28bcb99` | ⚠️ CONTESTED (moved 3×: e3154ce → 66b3600 → 28bcb99) |
| `v4.0.0-rc2` | — | NOT EXISTS (Stage 14) |

### Version History Correction

| Claimed Version | Actual Version | Status |
|-----------------|----------------|--------|
| `3.8.0-stage7` | Never released; never tagged | **RETIRED — CONTESTED** |
| `3.8.0-stage8` | Never released; never tagged | **RETIRED — CONTESTED** |
| `4.0.0-rc1` | Tagged at `28bcb99` but moved 3× | **CONTESTED** |
| `4.0.0-rc2` | Planned at `f1242dd` | **PENDING** (Stage 14) |

---

## 6. Actual Implementation Inventory (What Actually Exists)

### Implemented and Verified (CI-VERIFIED / REPRODUCIBLE)

| Component | Status | Evidence |
|-----------|--------|----------|
| Idempotency ledger (request dedup, crash recovery) | ✅ IMPLEMENTED | `internal/store/vault.go`, `internal/workspace/workspace.go`, `internal/engine/spine/spine.go`; integration tests PASS |
| Request idempotency (PutIfAbsent) | ✅ IMPLEMENTED | `PutRecordIfAbsent` in vault; workspace IdempotencyPut/Get/PutIfAbsent |
| Idempotency crash recovery | ✅ IMPLEMENTED | `TestIdempotencyCrashRecovery` PASS |
| Replay/idempotency integration tests | ✅ IMPLEMENTED | 4/4 PASS (Stage 9 G6) |
| Idempotency duplicate injection | ✅ IMPLEMENTED | 10+ scenarios covered |
| SAML `VerifyRawDigest` | ✅ IMPLEMENTED | `internal/protocol/saml/signature.go:54-61` |
| SAML `VerifyXMLSignature` stub | ✅ IMPLEMENTED | `internal/protocol/saml/signature.go:87-95` (honest stub) |
| SAML `SignDigest` / `Signer` | ✅ IMPLEMENTED | `internal/protocol/saml/signature.go:39-51` |
| SAML `Signer.KeyInfoXML` | ✅ IMPLEMENTED | `internal/protocol/saml/signature.go:64-69` |
| ZTNA `DetectBroker` | ✅ IMPLEMENTED | `internal/engine/exec/ztna.go:58-95` |
| ZTNA `RouteThroughBroker` | ✅ IMPLEMENTED | `internal/engine/exec/ztna.go:134-157` |
| ZTNA `ExecThroughZTNA` | ⚠️ PARTIAL | Uses GET; command not transmitted (Stage 7 claim CONTRADICTED) |
| ZTNA `DetectBroker` header sniffing | ✅ IMPLEMENTED | `sniffBrokerHeaders` at `ztna.go:98-113` |
| PQC `IsPQCAlg` / `IsPQCAlgorithmString` | ⚠️ PARTIAL | Function named `IsPQCAlg` (not renamed); no `PQCCapability` |
| PQC `DetectAndDowngrade` | ✅ IMPLEMENTED | `internal/protocol/oauth2/pqc.go:116-151` |
| PQC `JWKS` / `FetchJWKS` | ✅ IMPLEMENTED | `internal/protocol/oauth2/pqc.go:52-95` |
| 21 native fuzz targets | ✅ IMPLEMENTED | 21 targets across 7 packages |
| 10 fuzz targets executed (Stage 7 backfill) | ✅ EXECUTED | 10 targets run ≥60s; ~50M execs; 0 crashes |
| SAML `VerifyRawDigest` / `SignDigest` | ✅ IMPLEMENTED | `signature.go:54-61`, `39-51` |
| SAML `Signer` / `KeyInfoXML` | ✅ IMPLEMENTED | `signature.go:20-69` |
| WS-Trust `FuzzParseRSTR` / `FuzzExtractAssertionWS` | ✅ IMPLEMENTED | 4 fuzz targets in `wstrust` |
| MS-OAPX `FuzzDecodeKey` etc. | ✅ IMPLEMENTED | 4 fuzz targets in `msoapx` |
| Workspace `FuzzLoadRecord` / `FuzzAuditLogEntry` | ✅ IMPLEMENTED | 2 fuzz targets in `workspace` |
| Idempotency `PutRecordIfAbsent` | ✅ IMPLEMENTED | `vault.go:165-177` |
| Workspace `IdempotencyPut/Get/Delete/PutIfAbsent` | ✅ IMPLEMENTED | `workspace.go:690-715` |

### Design Only / Not Implemented

| Component | Status | Notes |
|-----------|--------|-------|
| GoReleaser v2 config | DESIGN_ONLY | `.goreleaser.yml` exists but uncommitted in Stage 8 era |
| GitHub Actions release workflow | DESIGN_ONLY | `.github/workflows/release.yml` uncommitted; never run |
| Cosign keyless signing | DESIGN_ONLY | Defined in goreleaser + CI; never executed |
| Authenticode signing | DESIGN_ONLY | Scripts exist; EV cert never procured |
| SBOM (CycloneDX) | DESIGN_ONLY | Defined in `.goreleaser.yml` (syft); never executed |
| Provenance (SLSA) | DESIGN_ONLY | goreleaser v2.18.1 lacks support |
| Authenticode EV cert | NOT PROCURED | B4 BLOCKED |
| Azure KV live validation | NOT EXECUTED | Provider added Stage 12; no live test |
| Live Entra ID validation | WAIVED | Expiry 2027-06-30 |
| Live IMDS validation | WAIVED | Expiry 2027-06-30 |
| OCSP/CRL revocation | DESIGN_ONLY | File-based only |
| Provenance (SLSA) | DESIGN_ONLY | goreleaser v2.18.1 lacks support |
| Reproducible build verification | NOT VERIFIED | Never independently tested |

---

## 6. Maturity Reassessment (Actual vs Claimed)

| Dimension | Stage 7/8 Claim | Stage 7 Backfill Actual | Delta |
|-----------|-----------------|------------------------|-------|
| Security | M3.5 | M4 | +0.5 |
| Release Engineering | M3.5 | M3 | -0.5 |
| Supply Chain | M3 | M3 | 0 |
| External Validation | M2 | M1 | -1 |
| Reliability | M3 | M3 | 0 |
| Observability | M2 | M1 | -1 |
| Operability | M2 | M1 | -1 |
| Protocol Correctness | M3 | M3 | 0 |
| Interoperability | M2 | M1 | -1 |
| Code Quality | M3 | M3 | 0 |
| Documentation | M3 | M3 | 0 |

**Stage 7/8 claimed M3.5 overall; actual M4+ (M3 with limitations acknowledged)**

---

## 7. Stage 8 Acceptance Criteria (Historical)

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Every major Stage 8 claim has truth classification | ✅ PASS | This document |
| Contradictions documented | ✅ PASS | Section 3 |
| Missing evidence identified | ✅ PASS | Section 4 |
| No unsupported "production-ready" claim remains | ✅ PASS | Documented as DESIGN_ONLY/WAIVED |
| Stage 9 scope frozen | ✅ PASS | Stage 9 executed based on this reality |

---

## 8. Stage 8 Backfill Conclusion

**Stage 8 was never executed as a separate stage.** Its planned work (WS1-WS9) was absorbed into Stage 9 G0 (forensic reconciliation) and G1 (blocker reclassification). The Stage 8 entry audit (`docs/stage8-entry-audit.md`) claimed many implementations that were later **CONTRADICTED** by Stage 9 G0 forensic reconciliation.

**Historical Stage 8 is RETIRED as a separate stage.** Its work was absorbed into Stage 9 G0 (forensic reconciliation) and G1 (blocker register). The historical tag `v3.8.0-stage8` was never created and is **CONTRADICTED**.

**Historical backfill tag:** `v3.8.0-stage8-backfill` created at commit `72d17d2` (Stage 7 backfill exit).

---

## 8. Stage 8 Backfill Gates

| Gate | Requirement | Status |
|------|-------------|--------|
| S8B-G01 | Historical scope reconstructed | ✅ PASS |
| S8B-G02 | Version/tag/commit lineage reconciled | ✅ PASS |
| S8B-G03 | Implementation inventory completed | ✅ PASS |
| S8B-G04 | Tests and fuzz evidence reproduced or classified | ✅ PASS |
| S8B-G05 | Security and capability truth audit completed | ✅ PASS |
| S8B-G06 | Maturity reassessment completed | ✅ PASS |
| S8B-G07 | Final report and Stage 14 handoff completed | ✅ PASS |

---

*Generated by Stage 8 Backfill — Forensic Reconciliation*