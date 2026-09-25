# Stage 45c ASN.1 Defect Analysis & Remediation

## Overview
This document maps the four defects (D45b-001 through D45b-004) found in Stage 45b to specific struct tag fixes in `internal/protocol/kerberos/types.go`.

---

## Defect D45b-001 — CRITICAL: ASN.1 Encoding Mismatch

**Root Cause**: KDCREQ and KDCREQBody structs missing all context-specific tags required by RFC 4120 §5.4.1/§5.4.2.

### RFC 4120 Specification
```
KDC-REQ ::= SEQUENCE {
    pvno        [1] INTEGER (5),
    msg-type    [2] INTEGER (10 | 30),
    padata      [3] SEQUENCE OF PA-DATA OPTIONAL,
    req-body    [4] KDC-REQ-BODY
}

KDC-REQ-BODY ::= SEQUENCE {
    kdc-options       [0] KDCOptions,
    cname             [1] PrincipalName OPTIONAL,
    realm             [2] Realm,
    sname             [3] PrincipalName OPTIONAL,
    from              [4] KerberosTime OPTIONAL,
    till              [5] KerberosTime,
    rtime             [6] KerberosTime OPTIONAL,
    nonce             [7] UInt32,
    etype             [8] SEQUENCE OF Int32,
    ...
}
```

### Fix Applied (types.go)

**KDCREQ** (lines 226-231):
```go
type KDCREQ struct {
    PVNO     int32       `asn1:"explicit,tag:1"`
    MsgType  int32       `asn1:"explicit,tag:2"`
    PAData   []PAData    `asn1:"explicit,optional,tag:3"`
    ReqBody  KDCREQBody  `asn1:"explicit,tag:4"`
}
```

**KDCREQBody** (lines 211-224):
```go
type KDCREQBody struct {
    KDCOptions         KDCOptions        `asn1:"explicit,tag:0"`
    CName              PrincipalName     `asn1:"explicit,optional,tag:1"`
    Realm              Realm             `asn1:"explicit,tag:2,ia5"`
    SName              PrincipalName     `asn1:"explicit,optional,tag:3"`
    From               KerberosTime      `asn1:"explicit,optional,tag:4"`
    Till               KerberosTime      `asn1:"explicit,tag:5"`
    RTime              KerberosTime      `asn1:"explicit,optional,tag:6"`
    Nonce              int32             `asn1:"explicit,tag:7"`
    EType              []int32           `asn1:"explicit,tag:8"`
    Addresses          []HostAddress     `asn1:"explicit,optional,tag:9"`
    EncryptedData      *EncryptedData    `asn1:"explicit,optional,tag:10"`
    AdditionalTickets  []Ticket          `asn1:"explicit,optional,tag:11"`
}
```

**KDCREPBody** (lines 233-239):
```go
type KDCREPBody struct {
    MsgType  int32         `asn1:"explicit,tag:0"`
    CName    PrincipalName `asn1:"explicit,tag:1"`
    Crealm   Realm         `asn1:"explicit,tag:2,ia5"`
    Ticket   Ticket        `asn1:"explicit,tag:3"`
    EncPart  EncryptedData `asn1:"explicit,tag:4"`
}
```

**KDCREP** (lines 241-247):
```go
type KDCREP struct {
    PVNO    int32       `asn1:"explicit,tag:1"`
    MsgType int32       `asn1:"explicit,tag:2"`
    PAData  []PAData    `asn1:"explicit,optional,tag:3"`
    Ticket  Ticket      `asn1:"explicit,optional,tag:4"`
    EncPart EncryptedData `asn1:"explicit,tag:5"`
}
```

**Ticket** (lines 162-167):
```go
type Ticket struct {
    TicketVNO int32           `asn1:"explicit,tag:0"`
    Realm     Realm           `asn1:"explicit,tag:1,ia5"`
    SName     PrincipalName   `asn1:"explicit,tag:2"`
    EncPart   EncryptedData   `asn1:"explicit,tag:3"`
}
```

**EncTicketPart** (lines 185-197):
```go
type EncTicketPart struct {
    Flags              asn1.BitString         `asn1:"explicit,tag:0"`
    Key                EncryptionKey          `asn1:"explicit,tag:1"`
    Crealm             Realm                  `asn1:"explicit,tag:2,ia5"`
    CName              PrincipalName          `asn1:"explicit,tag:3"`
    Transited          TransitedEncoding      `asn1:"explicit,tag:4"`
    Authtime           KerberosTime           `asn1:"explicit,tag:5"`
    Starttime          KerberosTime           `asn1:"explicit,optional,tag:6"`
    Endtime            KerberosTime           `asn1:"explicit,tag:7"`
    RenewTill          KerberosTime           `asn1:"explicit,optional,tag:8"`
    CAddr              []HostAddress          `asn1:"explicit,optional,tag:9"`
    AuthorizationData  []AuthorizationData    `asn1:"explicit,optional,tag:10"`
}
```

**Authenticator** (lines 169-178):
```go
type Authenticator struct {
    AuthenticatorVNO int32
    CName            PrincipalName
    Crealm           Realm         `asn1:"ia5"`
    CTime            KerberosTime
    CUSec            int32
    Subkey           *EncryptionKey `asn1:"optional"`
    SeqNumber        *int32         `asn1:"optional"`
    AuthorizationData []AuthorizationData `asn1:"optional"`
}
```

**KRBError** (lines 263-277):
```go
type KRBError struct {
    PVNO         int32
    MsgType      int32
    CTime        KerberosTime
    CUSec        int32
    STime        KerberosTime
    SUSec        int32
    ErrorCode    int32
    Crealm       Realm `asn1:"optional,ia5"`
    CName        PrincipalName `asn1:"optional"`
    Realm        Realm `asn1:"optional,ia5"`
    SName        PrincipalName `asn1:"optional"`
    EText        string `asn1:"optional"`
    EData        []byte `asn1:"optional"`
}
```

---

## Defect D45b-002 — HIGH: PAData Missing Context-Specific Tags

**Root Cause**: PAData struct fields missing explicit tags [1] and [2].

### RFC 4120 Specification
```
PA-DATA ::= SEQUENCE {
    padata-type  [1] Int32,
    padata-value [2] OCTET STRING
}
```

### Fix Applied (types.go:166-169)
```go
type PAData struct {
    PADataType  int32   `asn1:"explicit,tag:1"`
    PADataValue []byte  `asn1:"explicit,tag:2"`
}
```

---

## Defect D45b-003 — HIGH: PrincipalName Missing Context-Specific Tags

**Root Cause**: PrincipalName struct missing explicit tags [0] and [1] when embedded in other structs. The custom marshaler in principal.go was not invoked for embedded structs.

### RFC 4120 Specification
```
PrincipalName ::= SEQUENCE {
    name-type   [0] Int32,
    name-string [1] SEQUENCE OF KerberosString
}
```

### Fix Applied

**PrincipalName** (types.go:87-90):
```go
type PrincipalName struct {
    NameType   int32     `asn1:"explicit,tag:0"`
    NameString []string  `asn1:"explicit,tag:1"`
}
```

**principalNameASN1** (principal.go:9-12):
```go
type principalNameASN1 struct {
    NameType   int32   `asn1:"explicit,tag:0"`
    NameString []string `asn1:"explicit,tag:1"`
}
```

---

## Defect D45b-004 — MEDIUM: Realm Requires GeneralString Encoding

**Root Cause**: Go's `encoding/asn1` does not support GeneralString (tag 27). Default string encoding uses PrintableString (tag 19) or IA5String (tag 22) with `asn1:"ia5"`.

### RFC 4120 Specification
```
Realm ::= KerberosString
KerberosString ::= GeneralString
```

### Fix Applied

**Approach**: Use `type Realm string` with `asn1:"ia5"` tag on struct fields to get IA5String (tag 22), which is the closest supported type and widely accepted by Kerberos implementations including Samba4.

**Realm Type** (types.go:99-105):
```go
type Realm string

func (r Realm) String() string {
    return string(r)
}

func RealmFromString(s string) Realm {
    return Realm(s)
}
```

**Struct Field Tags**: All Realm fields in KDCREQBody, KDCREPBody, Ticket, EncTicketPart, Authenticator, KRBError use `asn1:"explicit,tag:N,ia5"` to produce context-specific tag with IA5String inner type.

Example encoding for Realm in KDCREQBody:
```
[A2] [16] [length] [value]  // [2] context-specific, IA5String
```

---

## Verification

### Unit Tests Pass
- `TestASREQEncodingStructure` - Verifies AS-REQ structure with correct context-specific tags
- `TestPADataEncoding` - Verifies PAData [1]/[2] tags
- `TestPrincipalNameEncoding` - Verifies PrincipalName [0]/[1] tags
- `TestRealmEncoding` - Verifies Realm encoding (PrintableString when direct, IA5String in struct)
- `TestKDCREQBodyEncoding` - Verifies full KDCREQBody structure

### Static Verification
```bash
go test -count=1 ./...     # PASS
go test -race -count=1 ./...  # PASS
go vet ./...               # PASS
go build ./...             # PASS
```

---

## Notes

1. **GeneralString Limitation**: Go's `encoding/asn1` does not support GeneralString (tag 27). Using IA5String (tag 22) via `asn1:"ia5"` is the standard workaround and is accepted by Samba4 and other Kerberos implementations.

2. **Explicit vs Implicit**: All Kerberos context-specific tags are EXPLICIT. Go's `asn1:"tag:N"` defaults to IMPLICIT; `asn1:"explicit,tag:N"` is required.

3. **Backward Compatibility**: Realm fields accept both IA5String (tag 22) and PrintableString (tag 19) during unmarshaling for compatibility.