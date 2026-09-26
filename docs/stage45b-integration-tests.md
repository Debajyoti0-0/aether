# Stage 45b — Integration Tests

## Overview

Six integration tests exercising the Stage 45 Kerberos implementation against a live Samba4 AD DC.

## Test Suite

Location: `test/integration/ad/kerberos_test.go`

Build tag: `integration`

```bash
go test -tags=integration -v ./test/integration/ad/...
```

## Tests

### 1. TestKerberosASREP
**Purpose**: Verify AS-REQ → AS-REP flow for normal user (user1)

**Flow**:
1. Engine.RequestTGT(ctx, "user1", "Passw0rd123!")
2. Validate TGT received with session key and ticket flags

**Expected**: TGT with valid session key, auth time, end time

**Status**: BLOCKED — ASN.1 encoding issue causes KDC to return EOF

---

### 2. TestKerberosTGSREP
**Purpose**: Verify TGS-REQ → TGS-REP flow for service ticket (svc_sql)

**Flow**:
1. RequestTGT for user1
2. RequestTGS for SPN `MSSQLSvc/sql01.aether.test:1433`
3. Validate TGS received with correct service name

**Expected**: TGS ticket for MSSQL service

**Status**: BLOCKED — Depends on TestKerberosASREP

---

### 3. TestKerberoast
**Purpose**: Extract Kerberoast hash (mode 13100) from service ticket

**Flow**:
1. RequestTGT for user1
2. RequestTGS for svc_sql SPN
3. ExtractKerberoastBlob from TGS
4. Validate blob format (krb5tgs) and hash

**Expected**: Kerberoast blob with SHA-256 hash of ticket

**Status**: BLOCKED — Depends on TestKerberosTGSREP

---

### 4. TestASREPRoast
**Purpose**: Extract AS-REP roast hash (mode 18200) from user without pre-auth

**Flow**:
1. RequestASREPNoPreauth for user2 (DONT_REQ_PREAUTH)
2. ExtractASREPRoastBlob from response
3. Validate blob format (krb5asrep) and hash

**Expected**: AS-REP roast blob for user2

**Status**: BLOCKED — ASN.1 encoding issue

---

### 5. TestCcacheRoundTrip
**Purpose**: Verify MIT ↔ Heimdal ccache serialization round-trip

**Flow**:
1. RequestTGT for user1
2. ExportCcache to MIT format
3. ExportCcache to Heimdal format
4. ParseCcache both formats
5. CompareTickets for semantic equivalence

**Expected**: Both round-trips preserve ticket semantics

**Status**: BLOCKED — Depends on TestKerberosASREP

---

### 6. TestEngagementBoundary
**Purpose**: Verify governance spine enforces engagement boundaries

**Test Cases**:
| Scenario | Expected |
|----------|----------|
| Valid engagement (user1, AETHER.TEST, localhost) | ALLOW |
| Wrong user (user2) | DENY |
| Expired engagement | DENY |
| Wrong realm (WRONG.REALM) | DENY |
| Missing capability (kerberoast not granted) | DENY |

**Expected**: All boundary checks enforced

**Status**: BLOCKED — Depends on TestKerberosASREP

## Artifacts

Each test writes JSON artifacts to `artifacts/stage45b/`:

- `TestKerberosASREP.json` / `_error.json`
- `TestKerberosTGSREP.json` / `_error.json`
- `TestKerberoast.json` / `_error.json`
- `TestASREPRoast.json` / `_error.json`
- `TestCcacheRoundTrip.json` / `_error.json`
- `TestEngagementBoundary.json` / `_error.json`

## Known Issues

### ASN.1 Encoding Mismatch

The protocol library (`internal/protocol/kerberos/`) uses Go's default ASN.1 marshaling which does not match RFC 4120 Kerberos encoding:

| Type | Issue | Fix Required |
|------|-------|--------------|
| `PrincipalName` | Missing context-specific tags [0], [1] | Custom MarshalASN1/UnmarshalASN1 |
| `PAData` | Missing tags [1] (padata-type), [2] (padata-value) | Custom marshaling |
| `KerberosTime` | Returns raw string, not ASN.1 GeneralizedTime | Fixed: uses asn1.Marshal(time.Time) |
| `Realm` | No GeneralString tag | Custom marshaling |
| `KDCOptions` | BitString encoding | Verify RFC 4120 compliance |
| `EncryptedData` | Missing optional KVNO tag | Custom marshaling |
| `Ticket` | Field tags missing | Custom marshaling |

### Impact

- KDC receives malformed AS-REQ
- KDC closes TCP connection without response (EOF)
- All password-based authentication fails
- Keytab/ccache-based authentication works (uses different code path)

### Workaround

Use keytab/ccache for initial authentication:
```bash
# Obtain TGT via keytab (works)
kinit -k -t /tmp/test.keytab user1@AETHER.TEST

# Use ccache for subsequent operations
```

## Gate Status

| Gate | Test | Status |
|------|------|--------|
| G2120 | TestKerberosASREP | FAIL (ASN.1 encoding) |
| G2121 | TestKerberosTGSREP | NOT RUN (depends on G2120) |
| G2122 | TestKerberoast | NOT RUN (depends on G2121) |
| G2123 | TestASREPRoast | FAIL (ASN.1 encoding) |
| G2124 | TestCcacheRoundTrip | NOT RUN (depends on G2120) |
| G2125 | TestEngagementBoundary | NOT RUN (depends on G2120) |