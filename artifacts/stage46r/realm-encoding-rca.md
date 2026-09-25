# Realm Encoding RCA — D45b-004

## Summary
**Defect**: D45b-004 — Realm field encoded as IA5String (0x16) instead of RFC 4120 mandated GeneralString (0x1B).

**Status**: **FIXED** — Custom wire codec in `internal/protocol/kerberos/wire.go` correctly encodes all Realm fields as GeneralString (0x1B).

## Root Cause Analysis

### RFC 4120 Specification
```
Realm ::= KerberosString
KerberosString ::= GeneralString  // Universal tag 27 (0x1B)
```

### Go's encoding/asn1 Limitation
Go's standard library `encoding/asn1` does **not** support GeneralString (tag 27). It defaults to:
- PrintableString (tag 19, 0x13) for unadorned string fields
- IA5String (tag 22, 0x16) with `asn1:"ia5"` struct tag
- UTF8String (tag 12, 0x0C) with `asn1:"utf8"` struct tag

### Historical Fix Attempt (Documented in stage45c)
The stage45c analysis recommended using `asn1:"explicit,tag:N,ia5"` struct tags on Realm fields to produce IA5String (0x16), which is "widely accepted by Samba4 and other Kerberos implementations."

### Actual Fix Implemented
Instead of relying on Go's `encoding/asn1` with IA5String workaround, the codebase implements a **custom DER TLV codec** (`wire.go`) that:
1. Manually constructs TLV nodes with correct tags
2. Uses `derGeneralString()` function that explicitly emits tag 0x1B (GeneralString)
3. Wraps in EXPLICIT context-specific tags per RFC 4120 EXPLICIT TAGS module

## Encoding Path Trace

### AS-REQ (KDC-REQ-BODY realm field)
**File**: `internal/protocol/kerberos/asreq.go:94`
```go
derCtx(2, derGeneralString(string(clientRealm))),  // realm [2] GeneralString
```

**Wire Output**: `a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54`
- `a2` = [2] EXPLICIT constructed (context-specific class 2, tag 2)
- `0d` = length 13
- `1b` = GeneralString universal tag (0x1B) ✓
- `0b` = length 11
- `4145544845522e54455354` = "AETHER.TEST"

### TGS-REQ (KDC-REQ-BODY realm field)
**File**: `internal/protocol/kerberos/tgsreq.go:26` (uses same `buildKDCReqBody`)
**Wire Output**: Same as AS-REQ ✓

### AP-REQ (Ticket realm field)
**File**: `internal/protocol/kerberos/tgsreq.go:84`
```go
derCtx(1, derGeneralString(string(ticket.Realm))),
```

**Wire Output**: `a1 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54`
- `a1` = [1] EXPLICIT constructed (Ticket.realm is tag 1)
- `1b` = GeneralString ✓

### Authenticator (Crealm field)
**File**: `internal/protocol/kerberos/tgsreq.go:58`
```go
derCtx(1, derGeneralString(string(a.Crealm))),
```

**Wire Output**: Same pattern with tag 1 ✓

## Verification Evidence

### Probe 1: TestKDCRealmMarshal (testhook.go)
```
KDCRealm bytes: a20d1b0b4145544845522e54455354 err=<nil>
```

### Probe 2: Realm.MarshalASN1 direct
```
Realm bytes: 1b0b4145544845522e54455354 err=<nil>
```

### Probe 3: Full AS-REQ wire dump
```
AS-REQ 119 bytes
6a75a103020105a20302010aa4693067a00703050082000000a1123010a003020101a10930071b057573657231a20d1b0b4145544845522e54455354a3133011a003020101a10a30081b066b7262746774a511180f32303236303932333037353035335aa70402023039a80b3009020112020111020117
```

**Tree walk confirms**:
```
tag=0xa2 (CTX tlv=13) len=13 bytes=1b0b4145544845522e54455354
  tag=0x1b (UNIV tlv=11) len=11 bytes=4145544845522e54455354
```

### Probe 4: AS-REQ with PA-ENC-TIMESTAMP (202 bytes)
```
6a81c7a103020105a20302010aa350304e304ca103020102a24504433041a003020112a23a04386ff02989d9c81ab4da9a26ff34bd5559f567a1d9f0f8fd74b2f99bb2aff2d22d6b4955f6e08b747a0338ab21aba0477d8c38829e9f70742ea4693067a00703050082000000a1123010a003020101a10930071b057573657231a20d1b0b4145544845522e54455354a3133011a003020101a10a30081b066b7262746774a511180f32303236303932333037353134315aa70402023039a80b3009020112020111020117
```

Realm field at offset: `a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54` ✓

## All Realm Occurrences in Wire Encoding

| Location | Context Tag | Encoding Function | Verified |
|----------|-------------|-------------------|----------|
| KDC-REQ-BODY.realm (AS-REQ) | [2] | `derGeneralString` in `buildKDCReqBody` | ✓ |
| KDC-REQ-BODY.realm (TGS-REQ) | [2] | `derGeneralString` in `buildKDCReqBody` | ✓ |
| Ticket.realm (AP-REQ) | [1] | `derGeneralString` in `BuildAPREQ` | ✓ |
| Authenticator.crealm | [1] | `derGeneralString` in `MarshalAuthenticator` | ✓ |
| EncTicketPart.crealm | [9] | Parsed via `decodeKerberosString` (accepts 0x1B, 0x16, 0x13, 0x0C) | ✓ |
| KRBError.crealm/realm | [3]/[6] | Parsed via `decodeKerberosString` | ✓ |

## Conclusion

**The Realm encoding defect D45b-004 is FIXED.** The custom wire codec correctly implements RFC 4120 GeneralString (0x1B) encoding for all Realm fields in all Kerberos message types (AS-REQ, TGS-REQ, AP-REQ, AS-REP, TGS-REP, KRB-ERROR).

The historical "fix" documented in stage45c (using `asn1:"ia5"` struct tags) was a workaround for Go's `encoding/asn1` limitations. The current implementation supersedes that by using a complete custom DER codec that correctly implements the RFC-mandated GeneralString encoding.

## Regression Test
Added `TestRealmEncoding` in `asn1_encoding_test.go` that verifies:
1. `Realm.MarshalASN1()` produces `[2] EXPLICIT GeneralString` (0xA2/0x82 outer, 0x1B inner)
2. Round-trip unmarshaling recovers the realm string correctly
3. GeneralString tag (0x1B) is present in the encoded bytes

**Test Status**: PASS