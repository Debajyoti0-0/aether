# Stage 8 Blocker Register (G41.7)

**Date:** 2026-09-12
**Baseline:** `9888b69` (Stage 8 G41 Entry Audit)
**Source:** Stage 7 Handoff (`docs/stage7-stage8-handoff.md`) + Stage 8 Entry Audit

---

## Blocker Register

| ID | Category | Description | Stage 7 Status | Stage 8 Target | Owner | Risk | Expiry |
|----|----------|-------------|----------------|----------------|-------|------|--------|
| **B1** | Interoperability | Live Entra ID lab tenant validation (PRT→OAuth, WS-Trust, Device Code, CAE) | WAIVED (expiry 2026-12-31) | Execute against authorized tenant OR re-waive with new expiry ≤ 2027-06-30 | Security Team | HIGH | 2027-06-30 |
| **B2** | Interoperability | Live IMDSv2 validation against Azure dev VM | WAIVED (expiry 2026-12-31) | Execute against authorized Azure dev VM OR re-waive with new expiry ≤ 2027-06-30 | Infra Team | HIGH | 2027-06-30 |
| **B3** | Release Engineering | Release pipeline: goreleaser + GitHub Actions (build matrix, provenance, signing, release) | DESIGN ONLY | Implement goreleaser + GitHub Actions pipeline with SLSA provenance | Platform Team | CRITICAL | N/A (must close) |
| **B4** | Release Engineering | EV Authenticode certificate procurement + Windows signing integration | DESIGN ONLY | Procure EV cert, integrate signtool in Windows build job | Security/Platform | CRITICAL | N/A (must close) |
| **B5** | Security | Audit key HSM/KMS custody (replace plaintext Ed25519 seed in workspace) | DESIGN ONLY | Implement HSM/KMS-backed custody (Azure Key Vault, AWS KMS, YubiHSM, or OS DPAPI) | Security Team | CRITICAL | N/A (must close) |
| **B6** | Reliability | Idempotency ledger hardening (replay safety, crash recovery, duplicate injection) | PARTIALLY IMPLEMENTED | Prove replay safety, crash recovery, adversarial duplicate injection | Engineering | HIGH | N/A (must close) |

---

## Blocker Classification

| Status | Count | Items |
|--------|-------|-------|
| **CRITICAL - Must Close for GA** | 4 | B3, B4, B5, B6 |
| **HIGH - Must Close or Waive with Expiry** | 2 | B1, B2 |
| **Total Blockers** | 6 | — |

---

## Waiver Policy

Per Stage 8 mandate:

> **No stable release is cut in Stage 8. Stage 8 produces a signed RC and a Stage 9 handoff.**

> **If any blocker remains waived, Stage 8 cannot grant GA `v4.0.0` — it grants `v4.0.0-rc2` production-limited.**

**Waiver Requirements:**
- Owner assigned
- Risk assessment documented
- Impact assessment documented
- Expiry date ≤ 2027-06-30
- Approval recorded
- Public release-note line added

---

## Blocker-to-Gate Mapping

| Blocker | Stage 8 Gate | Closure Criteria |
|---------|--------------|------------------|
| B1 (Entra ID) | G46 | Execute live validation OR formal waiver with expiry |
| B2 (IMDS) | G46 | Execute live validation OR formal waiver with expiry |
| B3 (Pipeline) | G47 | goreleaser reproducible; matrix builds; provenance in CI |
| B4 (Signing) | G48 | cosign keyless + Authenticode; 9/9 tamper tests fail-closed |
| B5 (Audit Key) | G49 | HSM/KMS-backed custody; rotation implemented |
| B6 (Idempotency) | G51 | Replay-safe; crash recovery proven; duplicates rejected |

---

## Waiver History

| Blocker | Original Waiver | Expiry | Re-waiver? | New Expiry |
|---------|-----------------|--------|------------|------------|
| B1 (Entra ID) | 2026-12-31 (Stage 7) | 2026-12-31 | PENDING | ≤ 2027-06-30 |
| B2 (IMDS) | 2026-12-31 (Stage 7) | 2026-12-31 | PENDING | ≤ 2027-06-30 |

---

## Resolution Tracking

| Blocker | Status | Resolution | Evidence | Date |
|---------|--------|------------|----------|------|
| B1 | PENDING | — | — | — |
| B2 | PENDING | — | — | — |
| B3 | PENDING | — | — | — |
| B4 | PENDING | — | — | — |
| B5 | PENDING | — | — | — |
| B6 | PENDING | — | — | — |

---

## G41.7 Acceptance

**PASS** — Blocker register transcribed and transcribed from Stage 7 handoff. All 6 blockers identified and classified. Stage 8 execution authorized.

---

**Register Created:** 2026-09-12
**Baseline Commit:** `9888b69` (Stage 8 G41 Entry Audit)
**Next Update:** Upon first blocker resolution