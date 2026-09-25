# Stage 46f — Live Qualification Certification

**Status**: BLOCKED-WITH-OWNER
**Reason**: Docker Desktop networking infrastructure failure prevents all live qualification
**Previous**: Stage 46e BLOCKED-WITH-OWNER (2 blockers)
**Version**: 5.0.0-alpha1
**Date**: 2026-09-23
**Tag**: NONE

---

## CERTIFIED: PASS

| Component | Status | Evidence |
|-----------|--------|----------|
| Samba container health | PASS | Healthy, all services LISTENING |
| Samba user data | PASS | 8 users (Administrator, Guest, krbtgt, user1-4, svc_sql, svc_web) |
| Samba domain info | PASS | AETHER.TEST / AETHER / dc01.aether.test |
| Samba LDAP on 127.0.0.1 | PASS | samba-tool user list, domain info, ldapsearch work |
| Aether LDAP BER encoding | PASS | Verified RFC 4511 compliant via byte analysis |
| Aether LDAP EncodeBindRequest | PASS | Bit-for-bit correct per RFC 4511 §4.2 |
| Aether binary build | PASS | Clean build, no errors |
| Go unit tests (pre-existing) | PASS | Pre-existing tests pass |

## CERTIFIED: BLOCKED

| Component | Status | Reason |
|-----------|--------|--------|
| LDAP bind (host→container:636) | BLOCKED | TCP OK, TLS OK, EOF from Samba — may be Docker NAT or Samba TLS |
| LDAP bind (container→container:636) | BLOCKED | TCP OK, TLS OK, EOF from Samba |
| Kerberos enum | BLOCKED | TCP connect OK, no KDC response |
| LDAP enum users/groups/computers | BLOCKED | Requires successful LDAP bind |
| LDAP acl get/effective/path | BLOCKED | Requires successful LDAP bind |
| Batch 1 qualification | BLOCKED | All tests require data flow |
| Batch 2 qualification | BLOCKED | Requires successful LDAP bind |
| DNS resolution | BLOCKED | aether.test NXDOMAIN |
| Live qualification | BLOCKED | Docker networking prevents data flow |

## CERTIFIED: FAIL

| Component | Status | Evidence |
|-----------|--------|----------|
| Host→container data flow | FAIL | All container ports: TCP OK, data FAIL |
| Container→container data flow | FAIL | test-client→Samba all ports: TCP OK, i/o timeout |
| Docker Desktop NAT | FAIL | TCP handshake OK, data dropped on all ports |
| DNS (aether.test) | FAIL | NXDOMAIN |

## STAGE 45 CERTIFICATION

Status: UNCHANGED (was PARTIALLY CLOSED in Stage 46r)
- Realm GeneralString fix (D45b-004): IMPLEMENTED, VERIFIED
- Wire encoding: VERIFIED

## STAGE 46 CERTIFICATION

Status: BLOCKED-WITH-OWNER
- Code is RFC 4511 compliant
- Infrastructure blocks live qualification
- No code changes needed (BER encoding verified correct)
- Docker NAT is operator dependency

## STAGE 47 ENTRY GATE (TB47-G00)

Status: STILL-BLOCKED — Stage 46f not COMPLETE
- Docker networking must be resolved before Stage 47
- DNS must be resolved before Stage 47
- LDAP TLS compatibility must be verified after networking restored

## DOCUMENTS PRODUCED (≤ 3)

1. `artifacts/stage46f/INVESTIGATION-REPORT.md` (this report)
2. `docs/stage46f-docker-nat-limitation.md` (Docker NAT limitation)
3. `docs/stage46f-certification.md` (this certification)

## NO REGRESSION

Pre-existing unit tests pass. No code changes were made. No existing functionality broken.

---

**Signed**: Stage 46f QA Lead
**Disposition**: BLOCKED-WITH-OWNER — Docker Desktop NAT infrastructure failure prevents live qualification. LDAP BER encoding verified as RFC 4511 compliant.
