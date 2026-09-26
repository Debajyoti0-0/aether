# Stage 45c Golden Vector Tests

## Overview
This document describes the deterministic protocol vectors used to verify ASN.1 encoding correctness.

---

## Test Vectors

### 1. AS-REQ for user1@AETHER.TEST (AES256)

**Input**:
```go
clientPrincipal := MakeUserPrincipal("user1", "AETHER.TEST")
clientRealm := Realm("AETHER.TEST")
serverPrincipal := MakeUserPrincipal("krbtgt", "AETHER.TEST")
etypes := []int32{18, 17, 23}  // AES256, AES128, RC4
nonce := int32(12345)
till := time.Now().Add(5 * time.Minute)
```

**Expected DER Characteristics**:
- Starts with `0x30` (SEQUENCE)
- PVNO at context-specific tag [1]: `0xA1 0x03 0x02 0x01 0x05`
- MsgType at tag [2]: `0xA2 0x03 0x02 0x01 0x0A` (KRB5_AS_REQ = 10)
- PAData at tag [3] (optional, omitted when nil)
- ReqBody at tag [4]: SEQUENCE with:
  - KDCOptions at tag [0]: `0xA0 0x0A 0x30 0x08 0x04 0x03 0x40 0x80 0x00 0x02 0x01 0x18`
  - CName at tag [1]: PrincipalName with name-type [0]=1, name-string [1]=["AETHER.TEST", "user1"]
  - Realm at tag [2]: IA5String "AETHER.TEST"
  - SName at tag [3]: PrincipalName for krbtgt
  - Till at tag [5]: KerberosTime
  - Nonce at tag [7]: 12345
  - EType at tag [8]: SEQUENCE OF [18, 17, 23]

### 2. TGS-REQ for MSSQLSvc/sql01.aether.test:1433

**Input**: Using TGT from AS-REQ above, requesting service ticket for SPN.

**Expected DER Characteristics**:
- MsgType = 12 (KRB5_TGS_REQ)
- PAData includes PA-PAC-REQUEST at tag [3]
- ReqBody with SName = MSSQLSvc/sql01.aether.test:1433

---

## Structural Verification Tests

The following tests verify tag structure without requiring exact byte matching:

```go
func TestASREQHasCorrectContextTags(t *testing.T) {
    req := buildASREQForTest("user1", "AETHER.TEST")
    encoded, _ := asn1.Marshal(req)
    
    // Verify SEQUENCE
    assert.Equal(t, byte(0x30), encoded[0])
    
    // Find PVNO tag [1] = 0xA1
    assert.Contains(t, encoded, []byte{0xA1, 0x03, 0x02, 0x01, 0x05})
    
    // Find MsgType tag [2] = 0xA2
    assert.Contains(t, encoded, []byte{0xA2, 0x03, 0x02, 0x01, 0x0A})
    
    // Find ReqBody tag [4] = 0xA4
    assert.Contains(t, encoded, []byte{0xA4})
}

func TestPADataHasCorrectTags(t *testing.T) {
    pa := PAData{
        PADataType:  PA_DATA_TYPE_ENC_TIMESTAMP,
        PADataValue: []byte{0x01, 0x02, 0x03},
    }
    encoded, _ := asn1.Marshal(pa)
    
    // SEQUENCE with explicit tags [1] and [2]
    assert.Equal(t, byte(0x30), encoded[0])
    // Tag [1] for padata-type
    assert.Contains(t, encoded, []byte{0xA1})
    // Tag [2] for padata-value
    assert.Contains(t, encoded, []byte{0xA2})
}

func TestPrincipalNameHasCorrectTags(t *testing.T) {
    pn := MakeUserPrincipal("user1", "AETHER.TEST")
    encoded, _ := asn1.Marshal(pn)
    
    assert.Equal(t, byte(0x30), encoded[0])
    // Tag [0] for name-type
    assert.Contains(t, encoded, []byte{0xA0})
    // Tag [1] for name-string
    assert.Contains(t, encoded, []byte{0xA1})
}

func TestRealmUsesIA5StringInStruct(t *testing.T) {
    reqBody := KDCREQBody{
        Realm: Realm("AETHER.TEST"),
        // ... other fields
    }
    encoded, _ := asn1.Marshal(reqBody)
    
    // Realm at tag [2] should contain IA5String (0x16)
    // Pattern: A2 [len] 16 [len] "AETHER.TEST"
    realmPattern := []byte{0xA2, 0x0D, 0x16, 0x0B}
    assert.Contains(t, encoded, realmPattern)
}
```

---

## Round-Trip Tests

All encoding tests verify round-trip:
1. Encode struct to DER
2. Decode DER back to struct
3. Verify semantic equality

```go
func TestRoundTrip(t *testing.T) {
    original := buildTestKDCREQ()
    encoded, _ := asn1.Marshal(original)
    
    var decoded KDCREQ
    _, err := asn1.Unmarshal(encoded, &decoded)
    if err != nil { t.Fatal(err) }
    
    // Verify semantic equality
    if decoded.PVNO != original.PVNO { t.Error("PVNO mismatch") }
    if decoded.MsgType != original.MsgType { t.Error("MsgType mismatch") }
    if decoded.ReqBody.Nonce != original.ReqBody.Nonce { t.Error("Nonce mismatch") }
    // ... etc
}
```

---

## Malformed Input Defense Tests

```go
func TestMalformedASREQ(t *testing.T) {
    testCases := []struct{
        name string
        data []byte
    }{
        {"truncated", []byte{0x30}},
        {"wrong_tag_pvno", []byte{0x30, 0x10, 0x81, 0x03, 0x02, 0x01, 0x05}},  // implicit tag 1
        {"missing_reqbody", []byte{0x30, 0x0A, 0xA1, 0x03, 0x02, 0x01, 0x05, 0xA2, 0x03, 0x02, 0x01, 0x0A}},
        {"invalid_length", []byte{0x30, 0xFF, 0xFF}},
    }
    
    for _, tc := range testCases {
        var req KDCREQ
        _, err := asn1.Unmarshal(tc.data, &req)
        if err == nil {
            t.Errorf("%s: expected error, got nil", tc.name)
        }
    }
}
```

---

## Integration Test Expectations

When run against live Samba4:

| Test | Expected Result |
|------|-----------------|
| TestKerberosASREP | PASS - AS-REQ accepted, AS-REP returned |
| TestKerberosTGSREP | PASS - TGS-REQ accepted, TGS-REP returned |
| TestKerberoast | PASS - Crackable blob extracted |
| TestASREPRoast | PASS - user2 blob extracted |
| TestCcacheRoundTrip | PASS - MIT/Heimdal round-trip |
| TestEngagementBoundary | PASS - Governance enforced |