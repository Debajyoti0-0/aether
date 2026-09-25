# RFC 4120 Wire Format Verification — Stage 46R

## Field-Level Encoding Table

| Field | RFC 4120 ASN.1 | Expected Tag | Actual Tag | Correct? |
|-------|----------------|--------------|------------|----------|
| **KDC-REQ** | | | | |
| pvno | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| msg-type | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| padata | SEQUENCE OF PA-DATA | 0x30 (UNIV) | 0x30 | ✓ |
| &nbsp;&nbsp;padata-type | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| &nbsp;&nbsp;padata-value | OCTET STRING | 0x04 (UNIV) | 0x04 | ✓ |
| req-body | KDC-REQ-BODY | 0x30 (UNIV) | 0x30 | ✓ |
| &nbsp;&nbsp;kdc-options | BIT STRING | 0x03 (UNIV) | 0x03 | ✓ |
| &nbsp;&nbsp;cname | PrincipalName | 0x30 (UNIV) | 0x30 | ✓ |
| &nbsp;&nbsp;&nbsp;&nbsp;name-type | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| &nbsp;&nbsp;&nbsp;&nbsp;name-string | SEQUENCE OF KerberosString | 0x30 (UNIV) | 0x30 | ✓ |
| &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;KerberosString | GeneralString | **0x1B (UNIV)** | **0x1B** | **✓** |
| &nbsp;&nbsp;realm | Realm | **0x1B (UNIV)** in [2] EXPLICIT | **0x1B** | **✓** |
| &nbsp;&nbsp;sname | PrincipalName | 0x30 (UNIV) | 0x30 | ✓ |
| &nbsp;&nbsp;&nbsp;&nbsp;name-string | GeneralString | **0x1B (UNIV)** | **0x1B** | **✓** |
| &nbsp;&nbsp;till | KerberosTime | 0x18 (UNIV) GeneralizedTime | 0x18 | ✓ |
| &nbsp;&nbsp;nonce | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| &nbsp;&nbsp;etype | SEQUENCE OF INTEGER | 0x30 (UNIV) | 0x30 | ✓ |
| **PA-ENC-TIMESTAMP** | | | | |
| patimestamp | KerberosTime | 0x18 (UNIV) | 0x18 | ✓ |
| pausec | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| **EncryptedData** | | | | |
| etype | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| kvno | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| cipher | OCTET STRING | 0x04 (UNIV) | 0x04 | ✓ |
| **Ticket** | | | | |
| tkt-vno | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| realm | Realm | **0x1B (UNIV)** in [1] EXPLICIT | **0x1B** | **✓** |
| sname | PrincipalName | 0x30 (UNIV) | 0x30 | ✓ |
| enc-part | EncryptedData | 0x30 (UNIV) | 0x30 | ✓ |
| **Authenticator** | | | | |
| authenticator-vno | INTEGER | 0x02 (UNIV) | 0x02 | ✓ |
| crealm | Realm | **0x1B (UNIV)** in [1] EXPLICIT | **0x1B** | **✓** |
| cname | PrincipalName | 0x30 (UNIV) | 0x30 | ✓ |
| ctime | KerberosTime | 0x18 (UNIV) | 0x18 | ✓ |

## Context-Specific Tag Mapping (EXPLICIT TAGS)

| Structure | Field | Context Tag | Wire Tag (Constructed) |
|-----------|-------|-------------|------------------------|
| KDC-REQ | pvno | [1] | 0xA1 |
| KDC-REQ | msg-type | [2] | 0xA2 |
| KDC-REQ | padata | [3] | 0xA3 |
| KDC-REQ | req-body | [4] | 0xA4 |
| KDC-REQ-BODY | kdc-options | [0] | 0xA0 |
| KDC-REQ-BODY | cname | [1] | 0xA1 |
| KDC-REQ-BODY | realm | [2] | 0xA2 |
| KDC-REQ-BODY | sname | [3] | 0xA3 |
| KDC-REQ-BODY | till | [5] | 0xA5 |
| KDC-REQ-BODY | nonce | [7] | 0xA7 |
| KDC-REQ-BODY | etype | [8] | 0xA8 |
| Ticket | tkt-vno | [0] | 0xA0 |
| Ticket | realm | [1] | 0xA1 |
| Ticket | sname | [2] | 0xA2 |
| Ticket | enc-part | [3] | 0xA3 |
| Authenticator | authenticator-vno | [0] | 0xA0 |
| Authenticator | crealm | [1] | 0xA1 |
| Authenticator | cname | [2] | 0xA2 |
| Authenticator | ctime | [4] | 0xA4 |

## Application Tags

| Message | Application Tag | Wire Tag (Constructed) |
|---------|-----------------|------------------------|
| AS-REQ | [APPLICATION 10] | 0x6A |
| AS-REP | [APPLICATION 11] | 0x6B |
| TGS-REQ | [APPLICATION 12] | 0x6C |
| TGS-REP | [APPLICATION 13] | 0x6D |
| AP-REQ | [APPLICATION 14] | 0x6E |
| AP-REP | [APPLICATION 15] | 0x6F |
| KRB-ERROR | [APPLICATION 30] | 0x7E |

## Verification Method

All tags verified via:
1. Custom DER TLV codec (`wire.go`) — manual tag assignment
2. `cmd/asn1probe` — runtime hex dump and tree walk
3. Unit tests (`asn1_encoding_test.go`) — structural assertions

## Discrepancies Found

**None.** All fields encode with RFC-correct tags.

## Note on GeneralString vs IA5String

RFC 4120 mandates `KerberosString ::= GeneralString` (tag 27). Go's `encoding/asn1` lacks GeneralString support. Previous workaround (stage45c) used `asn1:"ia5"` to emit IA5String (tag 22), which Samba4 accepts.

**Current implementation** uses custom codec emitting true GeneralString (tag 0x1B), which is RFC-correct and also accepted by Samba4.

Both approaches interoperate; GeneralString is strictly correct per RFC.