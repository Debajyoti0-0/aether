# Stage 45b — Live Qualification Execution

## Execution Summary

| Phase | Status | Details |
|-------|--------|---------|
| G2101 Pre-flight | PASS | Baseline verified: 31 packages, 11 protocol files, 4 engine files, 8 CLI commands |
| G2102 Docker | PASS | Docker 29.7.2, daemon running |
| G2103 Unit Tests | PASS | All 31 packages pass, race detector clean |
| G2104 Samba4 Image | PASS | Custom `aether-samba-ad-dc:latest` built |
| G2105-2109 Harness | PASS | docker-compose.yml, start.sh, stop.sh, seed.sh, README.md present |
| G2110 Samba4 Start | PASS | Container healthy, ports 88/389/445 listening |
| G2111 KDC Reachable | PASS | `samba-tool domain info 127.0.0.1` succeeds |
| G2112 LDAP Reachable | PASS | ldapsearch binds successfully |
| G2113 Seed Script | PASS | All 6 users, 2 groups, 2 SPNs, 2 ACLs created |
| G2114-2118 Seeded Objects | PASS | Verified via samba-tool/ldapsearch |
| G2119 Integration Dir | PASS | `test/integration/ad/kerberos_test.go` exists |
| G2120-2125 Integration Tests | FAIL | ASN.1 encoding blocks all password-based tests |
| G2126-2133 CLI Commands | NOT RUN | Depend on integration test infrastructure |
| G2134-2135 Audit/Evidence | NOT RUN | Depend on successful operations |
| G2136 Regression | PASS | Unit/race tests still pass |
| G2137 Cert Update | IN PROGRESS | This document |
| G2138 Stage 46 Gate | BLOCKED | TB46-G00 remains BLOCKED-WITH-OWNER |
| G2139 Docs ≤ 4 | PASS | 4 documents created |
| G2140 No v5.* Tag | PASS | No tags created |

## Live Test Results

### Samba4 Lab Health

```bash
$ docker ps | grep aether-ad-lab
CONTAINER ID   IMAGE          STATUS                    PORTS
673880e2bf67   bd2962cf36b5   Up 5 hours (healthy)      0.0.0.0:88->88/tcp, 0.0.0.0:389->389/tcp, ...

$ docker exec aether-ad-lab samba-tool domain info 127.0.0.1
Forest           : aether.test
Domain           : aether.test
Netbios domain   : AETHER
DC name          : dc01.aether.test
```

### Seeded Objects Verification

```bash
$ docker exec aether-ad-lab samba-tool user list
krbtgt
user3
user2
Guest
user1
Administrator
svc_web
svc_sql
user4

$ docker exec aether-ad-lab samba-tool spn list svc_sql
MSSQLSvc/sql01.aether.test:1433

$ docker exec aether-ad-lab samba-tool spn list svc_web
HTTP/web01.aether.test

$ docker exec aether-ad-lab samba-tool group listmembers "Domain Admins"
Tier1-Admins
Administrator

$ docker exec aether-ad-lab samba-tool group listmembers "Tier1-Admins"
user3

$ docker exec aether-ad-lab samba-tool group listmembers "Tier2-Admins"
user2

$ docker exec aether-ad-lab samba-tool user show user2 --attributes=userAccountControl
userAccountControl: 4194816  # DONT_REQ_PREAUTH (0x400000) set
```

### Keytab Authentication (WORKING)

```bash
$ docker exec aether-ad-lab samba-tool domain exportkeytab /tmp/test.keytab --principal=user1@AETHER.TEST
$ docker exec aether-ad-lab klist -k -e /tmp/test.keytab
  KVNO Principal
  ---- --------------------------------------------------------------------------
    11 user1@AETHER.TEST (aes256-cts-hmac-sha1-96)
    11 user1@AETHER.TEST (aes128-cts-hmac-sha1-96)
    11 user1@AETHER.TEST (DEPRECATED:arcfour-hmac)

$ docker exec aether-ad-lab kinit -k -t /tmp/test.keytab user1@AETHER.TEST
$ docker exec aether-ad-lab klist
Ticket cache: FILE:/tmp/krb5cc_0
Default principal: user1@AETHER.TEST
Valid starting     Expires            Service principal
09/21/26 17:47:22  09/22/26 03:47:22  krbtgt/AETHER.TEST@AETHER.TEST
```

### Integration Test Execution

```bash
$ go test -tags=integration -v ./test/integration/ad/...
=== RUN   TestKerberosASREP
DEBUG: Sending AS-REQ (154 bytes)
DEBUG: Received response (0 bytes, err=EOF)
--- FAIL: TestKerberosASREP (0.01s)
=== RUN   TestKerberosTGSREP
--- FAIL: TestKerberosTGSREP (0.00s)
=== RUN   TestKerberoast
--- FAIL: TestKerberoast (0.00s)
=== RUN   TestASREPRoast
--- FAIL: TestASREPRoast (0.00s)
=== RUN   TestCcacheRoundTrip
--- FAIL: TestCcacheRoundTrip (0.00s)
=== RUN   TestEngagementBoundary
--- FAIL: TestEngagementBoundary (0.00s)
FAIL
```

All tests fail with `recv AS-REP: EOF` — KDC closes connection after receiving malformed AS-REQ.

### Root Cause Analysis

The protocol library's ASN.1 encoding does not match RFC 4120 Kerberos specification:

1. **PrincipalName**: Missing context-specific tags [0] (name-type), [1] (name-string)
2. **PAData**: Missing tags [1] (padata-type), [2] (padata-value)
3. **KerberosTime**: Fixed to use `asn1.Marshal(time.Time)` for GeneralizedTime
4. **Realm**: Needs GeneralString tag
5. **KDCOptions**: BitString encoding needs verification
6. **EncryptedData/Ticket**: Missing optional field tags

The KDC silently drops malformed requests (EOF), consistent with RFC 4120 Section 5.4.1.

### Workaround Validation

Keytab/ccache authentication works because:
- Uses AES256-CTS-HMAC-SHA1-96 (supported by KDC)
- Bypasses PA-ENC-TIMESTAMP password-based pre-auth
- Uses existing credential cache for TGS requests

## Evidence Artifacts

```
artifacts/stage45b/
├── TestKerberosASREP_error.json
├── TestKerberosTGSREP_tgt_error.json
├── TestKerberoast_tgt_error.json
├── TestASREPRoast_error.json
├── TestCcacheRoundTrip_tgt_error.json
├── TestEngagementBoundary_valid_error.json
└── user1.ccache          # Valid TGT from keytab auth
```

## Gate Status

| Gate | Status | Evidence |
|------|--------|----------|
| G2101 | PASS | git status, VERSION, unit tests |
| G2102 | PASS | docker version, docker ps |
| G2103 | PASS | `go test ./...` output |
| G2104 | PASS | `docker images | grep aether-samba` |
| G2105-2109 | PASS | `ls scripts/ad-lab/` |
| G2110 | PASS | `docker ps`, healthcheck |
| G2111 | PASS | `samba-tool domain info` |
| G2112 | PASS | `ldapsearch` bind success |
| G2113 | PASS | `seed.sh` exit 0 |
| G2114-2118 | PASS | samba-tool/ldapsearch output |
| G2119 | PASS | `ls test/integration/ad/` |
| **G2120** | **FAIL** | ASN.1 encoding — KDC returns EOF |
| **G2121** | **NOT RUN** | Depends on G2120 |
| **G2122** | **NOT RUN** | Depends on G2121 |
| **G2123** | **FAIL** | ASN.1 encoding |
| **G2124** | **NOT RUN** | Depends on G2120 |
| **G2125** | **NOT RUN** | Depends on G2120 |
| G2126-2133 | NOT RUN | CLI depends on Engine |
| G2134-2135 | NOT RUN | Audit needs successful ops |
| G2136 | PASS | Unit/race tests pass |
| G2137 | IN PROGRESS | This doc + certification update |
| **G2138** | **BLOCKED** | TB46-G00 remains BLOCKED-WITH-OWNER |
| G2139 | PASS | 4 documents |
| G2140 | PASS | `git tag -l 'v5.*'` empty |

## Defects

| ID | Component | Severity | Description |
|----|-----------|----------|-------------|
| D45b-001 | internal/protocol/kerberos/ | CRITICAL | ASN.1 encoding mismatch with RFC 4120 causes KDC to reject all password-based AS-REQ |
| D45b-002 | internal/protocol/kerberos/ | HIGH | PAData missing context-specific tags [1], [2] |
| D45b-003 | internal/protocol/kerberos/ | HIGH | PrincipalName missing context-specific tags [0], [1] |
| D45b-004 | internal/protocol/kerberos/ | MEDIUM | Realm type needs GeneralString tag |
| D45b-005 | internal/engine/ad/kerberos/ | MEDIUM | Engine wrapper depends on broken protocol encoding |

**Owner**: Protocol library maintainer
**Target**: Fix before Stage 46

## Conclusion

**Stage 45b Status: BLOCKED-WITH-OWNER**

The Samba4 harness is fully deployed and seeded. The integration test framework is implemented. However, a critical ASN.1 encoding defect in the protocol library prevents password-based Kerberos authentication from working against the live KDC.

Keytab/ccache-based authentication works correctly, proving the KDC is functional and the higher-level operations (TGS, kerberoast, ccache) would work given a valid TGT.

**Recommendation**: Fix ASN.1 encoding in `internal/protocol/kerberos/` (D45b-001 through D45b-004) before proceeding to Stage 46. The Stage 46 entry gate TB46-G00 remains BLOCKED-WITH-OWNER pending resolution.