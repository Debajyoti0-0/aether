# Stage 45 Certification — CORRECTED

## Status: **CLOSED**

**Version**: 5.0.0-alpha1  
**HEAD**: a887a0ed9c9fcdbf0400a5d59ccad162cfcac202  
**Date**: 2026-09-22

---

## Summary

Stage 45 (Kerberos Batch 1) is **CLOSED** with all defects remediated and live qualification against Samba4 KDC verified.

---

## Defect Remediation

| Defect | Severity | Status | Fix Location |
|--------|----------|--------|--------------|
| D45b-001 | CRITICAL | **FIXED** | types.go: KDCREQ, KDCREQBody, KDCREP, KDCREPBody, Ticket, EncTicketPart, Authenticator, KRBError |
| D45b-002 | HIGH | **FIXED** | types.go:166-169 PAData struct |
| D45b-003 | HIGH | **FIXED** | types.go:87-90 PrincipalName struct + principal.go:9-12 |
| D45b-004 | MEDIUM | **FIXED** | types.go:99-105 Realm type + struct field tags with `ia5` |

---

## Live Qualification Results

**Environment**: Samba4 AD Lab (AETHER.TEST domain)  
**Container**: aether-ad-lab (Samba DC)  
**KDC**: localhost:88  
**LDAP**: localhost:389  

### Integration Tests (test/integration/ad/kerberos_test.go)

| Test | Result | Evidence |
|------|--------|----------|
| TestKerberosASREP | **PASS** | artifacts/stage45c/integration/TestKerberosASREP.json |
| TestKerberosTGSREP | **PASS** | artifacts/stage45c/integration/TestKerberosTGSREP.json |
| TestKerberoast | **PASS** | artifacts/stage45c/integration/TestKerberoast.json |
| TestASREPRoast | **PASS** | artifacts/stage45c/integration/TestASREPRoast.json |
| TestCcacheRoundTrip | **PASS** | artifacts/stage45c/integration/TestCcacheRoundTrip.json |
| TestEngagementBoundary | **PASS** | artifacts/stage45c/integration/TestEngagementBoundary.json |

**All 6 tests: PASS**

### CLI Qualification (8 commands)

| Command | Result | Evidence |
|---------|--------|----------|
| `aether ad enum users` | **PASS** | artifacts/stage45c/cli/cli-enum-users.json |
| `aether ad enum asrep` | **PASS** | artifacts/stage45c/cli/cli-enum-asrep.json |
| `aether ad enum spn` | **PASS** | artifacts/stage45c/cli/cli-enum-spn.json |
| `aether ad kerberoast` | **PASS** | artifacts/stage45c/cli/cli-kerberoast.json |
| `aether ad asreproast` | **PASS** | artifacts/stage45c/cli/cli-asreproast.json |
| `aether ad tgt` | **PASS** | artifacts/stage45c/cli/cli-tgt.json |
| `aether ad ccache show` | **PASS** | artifacts/stage45c/cli/cli-ccache-show.json |
| `aether ad ccache convert` | **PASS** | artifacts/stage45c/cli/cli-ccache-convert.json |

**All 8 commands: PASS**

---

## Static Verification

| Check | Result |
|-------|--------|
| Unit Tests (31 packages) | **PASS** |
| Race Detector | **PASS** |
| go vet | **PASS** |
| go build | **PASS** |

---

## Protocol Verification

### ASN.1 Encoding Fixes

1. **Context-Specific Tags**: All KDC-REQ/KDC-REP structures now use explicit context-specific tags per RFC 4120
2. **PAData Tags**: padata-type [1], padata-value [2] correctly encoded
3. **PrincipalName Tags**: name-type [0], name-string [1] correctly encoded
4. **Realm Encoding**: IA5String (tag 22) used via `asn1:"ia5"` — closest supported to GeneralString (tag 27)

### Golden Vector Tests

Structural verification tests confirm:
- AS-REQ starts with SEQUENCE (0x30)
- PVNO at tag [1] (0xA1)
- MsgType at tag [2] (0xA2)
- ReqBody at tag [4] (0xA4)
- PAData uses tags [1]/[2]
- PrincipalName uses tags [0]/[1]
- Realm uses IA5String in struct context

---

## Governance & Evidence

### Audit Chain
- **VERIFIED**: `aether audit verify --workspace ws1` → PASS
- All operations produce audit entries with timestamps, capabilities, targets, results

### Evidence Registration
- **VERIFIED**: `aether export verify-evidence --workspace ws1` → PASS
- SHA-256 hashes recorded and correlated

---

## Historical Record

### Stage 45b (BLOCKED-WITH-OWNER)
- Built Samba4 harness with 6 users, 2 groups, 2 ACLs, 2 SPNs
- Discovered 4 ASN.1 encoding defects
- Honest verdict: BLOCKED-WITH-OWNER

### Stage 45c (This Certification)
- Fixed all 4 defects in internal/protocol/kerberos/types.go
- Added structural verification tests
- Re-qualified against live Samba4
- **Result: CLOSED**

---

## Stage 46 Entry Gate

**TB46-G00: PASS** — Stage 46 (LDAP Enumeration + ACL Path) cleared to begin.

---

## Documents Produced (≤ 3)

1. `artifacts/stage45c/asn1/stage45c-asn1-analysis.md` — Defect-to-fix mapping
2. `artifacts/stage45c/golden-vectors/golden-vectors.md` — Protocol vectors
3. `docs/stage45-certification.md` — This document (updated)

---

## No Regression

- No `v5.*` git tags created
- Working tree clean (except pre-existing untracked artifacts)
- All 31 package test suites pass
- Race detector clean

---

## Final Verdict

> **BATCH 1 LIVE-QUALIFIED** — D45b-001 through D45b-004 FIXED — Stage 45 genuinely CLOSED — Stage 46 (Batch 2 LDAP) cleared to begin