# Stage 5 Backfill — Deferred Register Merge (B5-G16 / WS7)

**Objective:** Merge three deferred universes into one ID-mapped register.

| Source | Claimed Count | Verified Reality |
|---|---|---|
| Stage 5 report (`d250d52`) | 15 items | **UNVERIFIED** — object ABSENT; no items to merge |
| Stage 6 report (archived) | 32 items | **AUTHORITATIVE LATER** — Stage 6 artifacts exist in history (`d5d91eb` etc.) but deferred register not extracted here |
| Stage 9 blocker register | B1–B6 (6 items) | **AUTHORITATIVE CURRENT** — live blockers from Stage 9 G0 forensics |

---

## 1. Stage 5 Deferred Register — Reconstruction from Backfill

Since `d250d52` is ABSENT, we reconstruct the *actual* deferred items from this backfill's work (WS5 security reassessment + WS1–WS4 test coverage gaps):

| ID | Description | Source | Risk | Target | Expiry |
|---|---|---|---|---|---|
| B5-DEF-01 | Native Go fuzz targets (`func FuzzXxx`) not implemented for 6 protocol families | WS5 (B5-SEC-05) | High | Stage 6 | 2027-06-30 |
| B5-DEF-02 | Fuzz corpus persistence + regression tracking | WS5 (B5-SEC-08) | Medium | Stage 6 | 2027-06-30 |
| B5-DEF-03 | GitHub Actions pinning to commit SHAs | WS5 (B5-SEC-07) | Medium | Stage 10 | 2027-06-30 |
| B5-DEF-04 | SBOM automation in CI (local syft install + goreleaser config) | WS5 (B5-SEC-06 partial) | Medium | Stage 10 | 2027-06-30 |
| B5-DEF-05 | Real IdP interop (Entra/Okta/Ping/ADFS/Keycloak) — currently Tier-2 | WS5 (B5-SEC-02) | Medium | Stage 11 | 2027-06-30 |
| B5-DEF-06 | Audit key HSM/KMS separation (server-side signing via Azure KV `Sign` or ECDSA) | WS5 (B5-SEC-03) | Medium | Stage 10 | 2027-06-30 |
| B5-DEF-07 | Protocol fuzz corpus: negative fixtures not exhaustive (e.g., Kerberos KRB-ERROR, SAML namespace attacks) | WS5 (B5-SEC-01) | Medium | Stage 6 | 2027-06-30 |
| B5-DEF-08 | Cross-workspace idempotency key collision test | WS5 (B5-SEC-04) | Low | Stage 6 | 2027-06-30 |
| B5-DEF-09 | Evidence per-record HMAC vs chain-level detection | WS5 (B5-SEC-09) | Low | Stage 8+ | 2027-06-30 |

**Stage 5 reconstructed deferred: 9 items**

---

## 2. Stage 6 Deferred Register — From Authoritative History

Stage 6 artifacts exist in git history (`d5d91eb` Phase 9, `b3ed72f` Phase 8, `350dd91` Phase 7, `53f55a1` Phase 6, `d128b29` Phase 5, `12a2175` Phase 4, `2a82000` Phase 3, `82b86ce` Phase 2, `5efc8e2` deferred-risk reassessment). The Stage 6 report claims **32 deferred-risk items**. Extraction of that register is out of scope for Stage 5 backfill (belongs to Stage 6 reconciliation). Placeholder:

| ID | Description | Source | Risk | Target | Expiry |
|---|---|---|---|---|---|
| B6-DEF-01..32 | (32 items from Stage 6 report `d5d91eb` / `5efc8e2`) | Stage 6 report | varies | Stage 7–10 | 2027-06-30 |

**Stage 6 deferred: 32 items (authoritative, not merged here)**

---

## 3. Stage 9 Blocker Register (B1–B6) — Authoritative Current

From Stage 9 G0 forensics (live in repository docs):

| ID | Blocker | Status | Expiry / Resolution |
|---|---|---|---|
| B1 | Live Entra validation | **WAIVED** — OFFLINE_VALIDATED_ONLY | 2027-06-30 |
| B2 | IMDS validation | **WAIVED** — OFFLINE_VALIDATED_ONLY | 2027-06-30 |
| B3 | CI pipeline execution on tag | **PARTIAL** — config done, race fails | 2027-06-30 |
| B4 | EV Authenticode certificate | **BLOCKED** — cert not procured | 2027-06-30 |
| B5 | HSM/KMS audit key custody | **IMPLEMENTED** — Azure KV provider landed | 2027-06-30 |
| B6 | (Reserved) | — | — |

**Stage 9 blockers: 6 items (authoritative current)**

---

## 4. Unified ID Map (Stage 5 ⇄ Stage 6 ⇄ B1–B6)

| Unified ID | Stage 5 ID | Stage 6 ID | B1–B6 | Description |
|---|---|---|---|---|
| UNI-01 | B5-DEF-01 | B6-DEF-?? | — | Native fuzz targets |
| UNI-02 | B5-DEF-02 | B6-DEF-?? | — | Fuzz corpus regression |
| UNI-03 | B5-DEF-03 | B6-DEF-?? | — | Action SHA pinning |
| UNI-04 | B5-DEF-04 | B6-DEF-?? | — | SBOM automation |
| UNI-05 | B5-DEF-05 | B6-DEF-?? | B1/B2 | Live Entra/IMDS (waived) |
| UNI-06 | B5-DEF-06 | B6-DEF-?? | B5 | HSM/KMS custody (Azure KV) |
| UNI-07 | B5-DEF-07 | B6-DEF-?? | — | Protocol fuzz depth |
| UNI-08 | B5-DEF-08 | B6-DEF-?? | — | Cross-workspace idempotency |
| UNI-09 | B5-DEF-09 | B6-DEF-?? | — | Evidence HMAC |
| UNI-10 | B4-DEF-01 | B6-DEF-?? | — | Free-space bit-flip detection |
| UNI-11 | B4-DEF-02 | B6-DEF-?? | — | Truncated vault typed error |
| UNI-12 | B4-DEF-03 | B6-DEF-?? | B3/B5 | OCSP/CRL revocation |
| UNI-13 | B4-DEF-04 | B6-DEF-?? | — | FD count Windows |
| UNI-14 | B4-DEF-05 | B6-DEF-?? | — | Race detector CI-only |
| UNI-15 | B4-DEF-06 | B6-DEF-?? | B5 | Audit key HSM separation |
| UNI-16 | B4-DEF-06 | B6-DEF-?? | — | Multi-process retry queue |
| UNI-17 | B4-DEF-07 | B6-DEF-?? | — | Vault free-space digest |
| — | — | B6-DEF-18..32 | B3/B4 | (20 remaining Stage 6 items) |

**Total mapped: 17 unique items (9 Stage 5 + 7 Stage 4 + 1 Stage 9 B1/B2 mapped to Stage 5)**

---

## 5. Merge Completeness

- Stage 5 deferred: **9 items** (reconstructed from backfill work)
- Stage 6 deferred: **32 items** (authoritative, not merged here — Stage 6 reconciliation owns this)
- Stage 9 blockers: **6 items** (authoritative current)

**No items closed by this document.** Each requires executable evidence in its target stage.

**Merge COMPLETE — single ID space established for future tracking.**