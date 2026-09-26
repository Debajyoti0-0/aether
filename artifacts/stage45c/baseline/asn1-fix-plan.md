# Stage 45c ASN.1 Defect Analysis & Fix Plan

## Defect Mapping to Struct Tags

### D45b-001: KDCREQ (types.go:218) - Missing tags [1], [2], [3], [4]

**RFC 4120 §5.4.1:**
```
KDC-REQ ::= SEQUENCE {
    pvno        [1] INTEGER (5),
    msg-type    [2] INTEGER (10 | 30),
    padata      [3] SEQUENCE OF PA-DATA OPTIONAL,
    req-body    [4] KDC-REQ-BODY
}
```

**Current (BROKEN):**
```go
type KDCREQ struct {
    PVNO     int32
    MsgType  int32
    PAData   []PAData `asn1:"optional"`
    ReqBody  KDCREQBody
}
```

**FIX:**
```go
type KDCREQ struct {
    PVNO     int32       `asn1:"explicit,tag:1"`
    MsgType  int32       `asn1:"explicit,tag:2"`
    PAData   []PAData    `asn1:"explicit,optional,tag:3"`
    ReqBody  KDCREQBody  `asn1:"explicit,tag:4"`
}
```

---

### D45b-001 (cont): KDCREQBody (types.go:203) - Missing tags [0]-[8]

**RFC 4120 §5.4.2:**
```
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
    addresses         [9] HostAddresses OPTIONAL,
    enc-authorization-data [10] EncryptedData OPTIONAL,
    additional-tickets [11] SEQUENCE OF Ticket OPTIONAL
}
```

**Current (BROKEN):**
```go
type KDCREQBody struct {
    KDCOptions    KDCOptions
    CName         PrincipalName `asn1:"optional"`
    Realm         Realm         `asn1:"optional"`
    SName         PrincipalName `asn1:"optional"`
    From          KerberosTime `asn1:"optional"`
    Till          KerberosTime
    RTime         KerberosTime `asn1:"optional"`
    Nonce         int32
    EType         []int32
    Addresses     []HostAddress `asn1:"optional"`
    EncryptedData *EncryptedData `asn1:"optional,tag:0"`
    AdditionalTickets []Ticket `asn1:"optional"`
}
```

**FIX:**
```go
type KDCREQBody struct {
    KDCOptions         KDCOptions        `asn1:"explicit,tag:0"`
    CName              PrincipalName     `asn1:"explicit,optional,tag:1"`
    Realm              Realm             `asn1:"explicit,tag:2"`
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

---

### D45b-002: PAData (types.go:149) - Missing tags [1], [2]

**RFC 4120 §5.2.7:**
```
PA-DATA ::= SEQUENCE {
    padata-type  [1] Int32,
    padata-value [2] OCTET STRING
}
```

**Current (BROKEN):**
```go
type PAData struct {
    PADataType int32
    PADataValue []byte
}
```

**FIX:**
```go
type PAData struct {
    PADataType  int32   `asn1:"explicit,tag:1"`
    PADataValue []byte  `asn1:"explicit,tag:2"`
}
```

---

### D45b-003: PrincipalName (types.go:87) - Missing tags [0], [1] when embedded

**RFC 4120 §6.2:**
```
PrincipalName ::= SEQUENCE {
    name-type   [0] Int32,
    name-string [1] SEQUENCE OF KerberosString
}
```

**Current (BROKEN):**
```go
type PrincipalName struct {
    NameType int32
    NameString []string
}
```

**Note:** principal.go has custom marshaler using `principalNameASN1` with correct tags, but it's NOT used when embedded in KDCREQBody because Go's encoding/asn1 doesn't call custom marshalers for non-pointer embedded structs.

**FIX - Option A: Add tags to PrincipalName struct (used when embedded):**
```go
type PrincipalName struct {
    NameType   int32     `asn1:"explicit,tag:0"`
    NameString []string  `asn1:"explicit,tag:1"`
}
```
And update custom marshaler in principal.go to match.

---

### D45b-004: Realm (types.go:99) - Requires GeneralString (tag 27)

**RFC 4120 §6.1:**
```
Realm ::= KerberosString
KerberosString ::= GeneralString
```

**Current (BROKEN):**
```go
type Realm string  // Defaults to IA5String (tag 22) or PrintableString (tag 19)
```

**FIX:** Custom MarshalASN1/UnmarshalASN1 for Realm using asn1.RawValue with tag 27 (GeneralString).

```go
type Realm string

func (r Realm) MarshalASN1() ([]byte, error) {
    return asn1.Marshal(asn1.RawValue{Tag: 27, Class: asn1.ClassUniversal, Bytes: []byte(r), IsCompound: false})
}

func (r *Realm) UnmarshalASN1(data []byte) error {
    var rv asn1.RawValue
    _, err := asn1.Unmarshal(data, &rv)
    if err != nil {
        return err
    }
    if rv.Tag != 27 && rv.Tag != 22 && rv.Tag != 19 { // Accept GeneralString, IA5String, PrintableString for compatibility
        return fmt.Errorf("realm: unexpected tag %d", rv.Tag)
    }
    *r = Realm(rv.Bytes)
    return nil
}
```

---

## Additional Structures Needing Tags

### KDCREP (types.go:233) - AS-REP/TGS-REP
```
KDC-REP ::= SEQUENCE {
    pvno        [1] INTEGER (5),
    msg-type    [2] INTEGER (11 | 13),
    padata      [3] SEQUENCE OF PA-DATA OPTIONAL,
    crealm      [4] Realm,
    cname       [5] PrincipalName,
    ticket      [6] Ticket,
    enc-part    [7] EncryptedData
}
```

### Ticket (types.go:154)
```
Ticket ::= SEQUENCE {
    tkt-vno      [0] INTEGER (5),
    realm        [1] Realm,
    sname        [2] PrincipalName,
    enc-part     [3] EncryptedData
}
```

### EncryptedData (types.go:133)
```
EncryptedData ::= SEQUENCE {
    etype   [0] Int32,
    kvno    [1] UInt32 OPTIONAL,
    cipher  [2] OCTET STRING
}
```

### EncTicketPart (types.go:177)
```
EncTicketPart ::= SEQUENCE {
    flags            [0] TicketFlags,
    key              [1] EncryptionKey,
    crealm           [2] Realm,
    cname            [3] PrincipalName,
    transited        [4] TransitedEncoding,
    authtime         [5] KerberosTime,
    starttime        [6] KerberosTime OPTIONAL,
    endtime          [7] KerberosTime,
    renew-till       [8] KerberosTime OPTIONAL,
    caddr            [9] HostAddresses OPTIONAL,
    authorization-data [10] AuthorizationData OPTIONAL
}
```

---

## Implementation Order

1. Fix `PAData` (D45b-002) - simplest, independent
2. Fix `PrincipalName` (D45b-003) - needed by KDCREQBody
3. Fix `Realm` (D45b-004) - custom marshaler
4. Fix `KDCREQBody` (D45b-001 part 2)
5. Fix `KDCREQ` (D45b-001 part 1)
6. Fix `KDCREP`, `Ticket`, `EncryptedData`, `EncTicketPart` for completeness
7. Add golden-file test
8. Run integration tests