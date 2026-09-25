# Stage 46 Baseline — LDAP Enumeration + ACL Path

**Generated:** 2026-09-22
**Version:** 5.0.0-alpha1
**HEAD:** a887a0ed9c9fcdbf0400a5d59ccad162cfcac202
**Branch:** master

---

## Repository State

- **Modified files (tracked):** 4
  - VERSION
  - cmd/aether/main.go
  - internal/cli/governance.go
  - internal/cli/root.go
- **Untracked files/dirs:** Pre-existing artifacts, docs, internal/, scripts/, test/

---

## Stage 46 Implementation Inventory

### Current Implementation: **NONE**

LDAP enumeration and ACL path analysis are **not implemented**. This is a greenfield implementation for Stage 46.

### Existing Infrastructure (from Stages 45/45b/45c)

| Component | Location | Status |
|-----------|----------|--------|
| Kerberos Protocol | internal/protocol/kerberos/ | ✅ Complete |
| Kerberos Engine | internal/engine/ad/kerberos/ | ✅ Complete |
| Kerberos CLI | internal/cli/ad/*.go | ✅ Complete (enum, roast, tgt, ccache) |
| Samba4 Harness | scripts/ad-lab/ | ✅ Built, not live-qualified |
| Integration Tests | test/integration/ad/kerberos_test.go | ✅ 6 tests |
| Governance/Audit/Evidence | internal/engine/spine/, internal/workspace/ | ✅ Operational |

### Required Implementation for Stage 46

| Area | Required | Status |
|------|----------|--------|
| LDAP Protocol (RFC 4511) | internal/protocol/ldap/ | ❌ Not started |
| LDAP Bind/Connection | | ❌ |
| RootDSE Discovery | | ❌ |
| User/Group/Computer/OU Enumeration | | ❌ |
| SPN Enumeration (LDAP) | | ❌ |
| Security Descriptor Parsing | internal/protocol/ldap/sd.go | ❌ |
| ACE Parsing | | ❌ |
| ACL Graph Construction | internal/engine/ad/acl/ | ❌ |
| ACL Path Analysis | | ❌ |
| LDAP Engine | internal/engine/ad/ldap/ | ❌ |
| CLI Commands | internal/cli/ad/ldap.go | ❌ |

---

## Stage 45c Verification (Prerequisite)

| Check | Result |
|-------|--------|
| Unit Tests (31 packages) | PASS |
| Race Detector | PASS |
| go vet | PASS |
| go build | PASS |
| Kerberos Structural Tests | PASS |
| D45b-001 through D45b-004 | FIXED |

---

## Baseline Test Results

```text
Unit tests (31 packages):     PASS
Race detector:                PASS
go vet:                       PASS
go build:                     PASS
Integration tests:            NOT RUN (Samba4 down)
LDAP tests:                   N/A (not implemented)
```

---

## Next Phase

**Phase 1: LDAP Protocol Implementation**
- RFC 4511 LDAP v3 bind, search, controls
- RootDSE discovery
- Paged results control (1.2.840.113556.1.4.319)
- Attribute selection and filtering

**Phase 2: AD Object Enumeration**
- Users, Groups, Computers, OUs
- SPN enumeration via LDAP
- SID/GUID resolution

**Phase 3: Security Descriptor & ACL**
- SDDL parsing
- DACL/ACE parsing
- Access mask decoding
- Inheritance handling

**Phase 4: ACL Graph & Path Analysis**
- Principal → Object relationships
- Effective permissions
- Path discovery (WriteDACL, WriteOwner, GenericAll, etc.)

**Phase 5: Engine & CLI Integration**
- LDAP engine with governance
- Mutation types for each capability
- CLI commands under `aether ad enum` and `aether ad acl`

**Phase 6: Live Qualification**
- Samba4 integration tests
- CLI qualification against live AD
- Governance/Audit/Evidence verification