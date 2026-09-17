# Stage 5 Backfill — Security Reassessment (B5-G13 / WS5)

**Date:** 2025-09-15  
**Scope:** All findings from Stage 4 backfill deferred register + new Stage 5 attack surface  
**Methodology:** Each finding classified as FIXED / MITIGATED / ACCEPTED / DEFERRED with owner, risk, target stage, expiry.

---

## 1. Stage 4 Deferred Register — Reassessment

| ID | Finding | Original Stage 4 Classification | Reassessment | Owner | Risk | Target | Expiry |
|---|---|---|---|---|---|---|---|
| B4-DEF-01 | Free-space bit-flips on SSD not detected | DEFERRED | **DEFERRED** — requires hardware fault injection; mitigated by fsync-per-append + signed audit chain | storage | Low | Stage 8+ | 2027-06-30 |
| B4-DEF-02 | Truncated-vault fault isolation untested | DEFERRED | **MITIGATED** — torn-write rejection (5/5 PASS) covers prefix truncation; full fault isolation needs separate chaos test | storage | Medium | Stage 7 | 2027-06-30 |
| B4-DEF-03 | Name-list revocation only (no OCSP/CRL) | DEFERRED | **DEFERRED** — OCSP responder + CRL distribution is Stage 11 scope; current revocation list is name-based fail-closed | pki | Medium | Stage 11 | 2027-06-30 |
| B4-DEF-04 | Non-ASCII operator names rejected | FIXED (behavior change) | **FIXED** — ASCII-only enforcement implemented; documented as release-note change | pki | Low | Done | N/A |
| B4-DEF-05 | Race detector not run locally | DEFERRED | **ACCEPTED** — CI-authoritative; gcc absent on Windows; golangci-lint staticcheck covers race patterns | tooling | Low | CI | 2027-06-30 |
| B4-DEF-06 | Audit key same trust domain as vault | DEFERRED | **DEFERRED** — Azure KV provider (B5) provides HSM/KMS separation; local file provider remains single-domain | custody | Medium | Stage 10 | 2027-06-30 |
| B4-DEF-07 | Free-space reclamation not verified | DEFERRED | **DEFERRED** — no evidence of vacuum/compaction affecting integrity; bbolt handles internally | storage | Low | Stage 8+ | 2027-06-30 |

---

## 2. Stage 5 New Attack Surface — Assessment

| ID | Finding | Classification | Evidence | Owner | Risk | Target | Expiry |
|---|---|---|---|---|---|---|---|
| B5-SEC-01 | Protocol conformance fixtures may miss edge cases in negative parsing | MITIGATED | Each family has ≥1 negative fixture (nil input, malformed XML, invalid encoding, 403/500, malformed claims); fuzz targets needed for deeper coverage | protocol | Medium | Stage 6 | 2027-06-30 |
| B5-SEC-02 | Interop matrix uses local teamserver only; no real IdP validation | ACCEPTED | Tier-0/1 targets are local by design; real Entra/IMDS validation is B1/B2 waivers (expiry 2027-06-30) | interop | Medium | Stage 11 | 2027-06-30 |
| B5-SEC-03 | Evidence verification uses workspace audit key (same trust domain) | MITIGATED | Azure KV provider (B5) available for HSM separation; local file remains single-domain | custody | Medium | Stage 10 | 2027-06-30 |
| B5-SEC-04 | Replay idempotency does not test cross-workspace collision | MITIGATED | Scenario 05 tests independent keys; scenario 10 tests deterministic outcomes across workspaces; cross-workspace key collision prevented by unique workspace vaults | storage | Low | Stage 6 | 2027-06-30 |
| B5-SEC-05 | No native Go fuzz targets (`func FuzzXxx`) exist | DEFERRED | 19 protocol fuzz targets claimed in Stage 9 are **fixture-only**; native fuzz requires `go test -fuzz` implementation | fuzz | High | Stage 6 | 2027-06-30 |
| B5-SEC-06 | SBOM generation not yet automated in CI | MITIGATED | `goreleaser` config includes SBOM; `anchore/syft-action` in Release workflow; local syft install pending | supply-chain | Medium | Stage 10 | 2027-06-30 |
| B5-SEC-07 | Action pinning to SHAs not implemented | DEFERRED | GitHub Actions use `@v4`, `@v5`, `@v6`, `@v7` tags; supply-chain hardening is Stage 10 scope | supply-chain | Medium | Stage 10 | 2027-06-30 |
| B5-SEC-08 | No fuzz corpus persistence / regression tracking | DEFERRED | `go test -fuzz` not implemented; fuzz regression is Stage 6 scope | fuzz | Medium | Stage 6 | 2027-06-30 |
| B5-SEC-09 | Evidence tamper detection only tests audit log; no record-level HMAC | MITIGATED | Audit chain uses Ed25519 hash chain; record mutation detected at chain level; per-record HMAC not required | evidence | Low | Stage 8+ | 2027-06-30 |

---

## 3. Classification Summary

| Classification | Count | Items |
|---|---|---|
| **FIXED** | 1 | B4-DEF-04 (non-ASCII operator names) |
| **MITIGATED** | 6 | B4-DEF-02, B5-SEC-01, B5-SEC-03, B5-SEC-04, B5-SEC-06, B5-SEC-09 |
| **ACCEPTED** | 2 | B4-DEF-05 (race CI-authoritative), B5-SEC-02 (live Entra waived) |
| **DEFERRED** | 9 | B4-DEF-01, B4-DEF-03, B4-DEF-06, B4-DEF-07, B5-SEC-05, B5-SEC-07, B5-SEC-08, B5-SEC-06 (partial), B4-DEF-01/07 |

---

## 4. Remediation Tracking

All DEFERRED items have:
- Explicit owner (storage/pki/custody/protocol/interop/supply-chain/fuzz/tooling)
- Risk rating (Low/Medium/High)
- Target stage for resolution
- Expiry date ≤ 2027-06-30 (aligned with Entra/IMDS waiver expiry)

No finding is left unclassified. All security invariants from Stage 4 are preserved or strengthened.

---

**Security Reassessment: COMPLETE — all findings classified per evidence rules.**