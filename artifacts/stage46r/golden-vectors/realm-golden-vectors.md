# Golden Vectors — Realm Encoding (D45b-004)

## Vector 1: Realm in KDC-REQ-BODY (AS-REQ/TGS-REQ)

### Input
```go
clientRealm := kerberos.Realm("AETHER.TEST")
```

### ASN.1 Structure (RFC 4120 §5.4.1)
```
KDC-REQ-BODY ::= SEQUENCE {
    kdc-options       [0] KDCOptions,
    cname             [1] PrincipalName,
    realm             [2] Realm,          -- Realm ::= KerberosString ::= GeneralString
    sname             [3] PrincipalName,
    ...
}
```

### Expected Wire Bytes (DER)
```
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```

### Breakdown
| Offset | Bytes | Meaning |
|--------|-------|---------|
| 0 | `a2` | Context-specific constructed, tag 2 ([2] EXPLICIT) |
| 1 | `0d` | Length: 13 bytes |
| 2 | `1b` | Universal tag 27: GeneralString |
| 3 | `0b` | Length: 11 bytes |
| 4-14 | `41 45 54 48 45 52 2e 54 45 53 54` | "AETHER.TEST" (ASCII) |

### Actual Output (from probe)
```
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```
✓ **MATCH**

---

## Vector 2: Realm in Ticket (AP-REQ/TGS-REQ)

### Input
```go
ticket := kerberos.Ticket{
    Realm: kerberos.Realm("AETHER.TEST"),
    ...
}
```

### ASN.1 Structure (RFC 4120 §5.3.1)
```
Ticket ::= SEQUENCE {
    tkt-vno      [0] INTEGER,
    realm        [1] Realm,
    sname        [2] PrincipalName,
    enc-part     [3] EncryptedData
}
```

### Expected Wire Bytes (DER)
```
a1 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```

### Breakdown
| Offset | Bytes | Meaning |
|--------|-------|---------|
| 0 | `a1` | Context-specific constructed, tag 1 ([1] EXPLICIT) |
| 1 | `0d` | Length: 13 bytes |
| 2 | `1b` | GeneralString |
| 3 | `0b` | Length: 11 |
| 4-14 | `41 45 54 48 45 52 2e 54 45 53 54` | "AETHER.TEST" |

### Actual Output (from AP-REQ probe)
```
a1 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```
✓ **MATCH**

---

## Vector 3: Realm in Authenticator (AP-REQ/TGS-REQ)

### Input
```go
auth := kerberos.Authenticator{
    Crealm: kerberos.Realm("AETHER.TEST"),
    ...
}
```

### ASN.1 Structure (RFC 4120 §5.5.1)
```
Authenticator ::= SEQUENCE {
    authenticator-vno    [0] INTEGER,
    crealm               [1] Realm,
    cname                [2] PrincipalName,
    ctime                [4] KerberosTime,
    ...
}
```

### Expected Wire Bytes (DER)
```
a1 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```

### Actual Output (from MarshalAuthenticator probe)
Same as Vector 2 ✓ **MATCH**

---

## Vector 4: Bare Realm.MarshalASN1() Output

### Input
```go
realm := kerberos.Realm("AETHER.TEST")
encoded, _ := realm.MarshalASN1()
```

### Expected Wire Bytes (DER)
```
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```

### Actual Output (Probe 2)
```
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```
✓ **MATCH**

---

## Vector 5: TestKDCRealmMarshal (testhook.go)

### Input
```go
enc, _ := kerberos.TestKDCRealmMarshal("AETHER.TEST")
```

### Expected Wire Bytes (DER)
```
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```

### Actual Output (Probe 1)
```
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
```
✓ **MATCH**

---

## Vector 6: Full AS-REQ (No Pre-Auth)

### Input Parameters
- Client: user1@AETHER.TEST
- Server: krbtgt@AETHER.TEST
- Etypes: [18, 17, 23] (AES256, AES128, RC4-HMAC)
- Nonce: 12345
- Till: now + 5 min
- PA-DATA: none

### Expected Total Length
119 bytes

### Actual Output
```
6a 75 a1 03 02 01 05 a2 03 02 01 0a a4 69 30 67
a0 07 03 05 00 82 00 00 00
a1 12 30 10 a0 03 02 01 01 a1 09 30 07 1b 05 75 73 65 72 31
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
a3 13 30 11 a0 03 02 01 01 a1 0a 30 08 1b 06 6b 72 62 74 67 74
a5 11 18 0f 32 30 32 36 30 39 32 33 30 37 35 30 35 33 5a
a7 04 02 02 30 39
a8 0b 30 09 02 01 12 02 01 11 02 01 17
```

### Realm Field Location
Offset 55-69: `a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54` ✓

---

## Vector 7: Full AS-REQ with PA-ENC-TIMESTAMP (AES256 Pre-Auth)

### Input Parameters
- Same as Vector 6
- Password: "password123"
- Salt: "AETHER.TESTuser1"
- Etype: AES256-CTS-HMAC-SHA1-96 (18)

### Expected Total Length
~202 bytes

### Actual Output
```
6a 81 c7 a1 03 02 01 05 a2 03 02 01 0a a3 50 30 4e
30 4c a1 03 02 01 02 a2 45 04 43 30 41 a0 03 02 01 12
a2 3a 04 38 6f f0 29 89 d9 c8 1a b4 da 9a 26 ff 34 bd
55 59 f5 67 a1 d9 f0 f8 fd 74 b2 f9 9b b2 af f2 d2 2d
6b 49 55 f6 e0 8b 74 7a 03 38 ab 21 ab a0 47 7d 8c 38
82 9e 9f 70 74 2e
a4 69 30 67 a0 07 03 05 00 82 00 00 00
a1 12 30 10 a0 03 02 01 01 a1 09 30 07 1b 05 75 73 65 72 31
a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54
a3 13 30 11 a0 03 02 01 01 a1 0a 30 08 1b 06 6b 72 62 74 67 74
a5 11 18 0f 32 30 32 36 30 39 32 33 30 37 35 31 34 31 5a
a7 04 02 02 30 39
a8 0b 30 09 02 01 12 02 01 11 02 01 17
```

### Realm Field Location
Offset 127-141: `a2 0d 1b 0b 41 45 54 48 45 52 2e 54 45 53 54` ✓

---

## Regression Test Criteria

A test **FAILS** if any Realm field encodes as:
- `0x16` (IA5String) — incorrect per RFC 4120
- `0x13` (PrintableString) — incorrect per RFC 4120
- `0x0c` (UTF8String) — incorrect per RFC 4120
- Missing `0x1b` (GeneralString) tag

A test **PASSES** only when:
- Outer tag is correct context-specific EXPLICIT (`0xa1`, `0xa2`, etc.)
- Inner tag is `0x1b` (GeneralString)
- Length encoding is minimal DER
- Value bytes match UTF-8/ASCII realm string exactly