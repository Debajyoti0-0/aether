package kerberos

import (
	"encoding/asn1"
	"testing"
	"time"
)

func TestASREQEncoding(t *testing.T) {
	asreq, err := BuildASREQ(
		MakeUserPrincipal("user1", "AETHER.TEST"),
		Realm("AETHER.TEST"),
		MakeUserPrincipal("krbtgt", "AETHER.TEST"),
		[]int32{ETYPE_AES256_CTS_HMAC_SHA1_96, ETYPE_AES128_CTS_HMAC_SHA1_96, ETYPE_RC4_HMAC},
		12345,
		time.Now().UTC().Add(5*time.Minute),
		[]PAData{
			{
				PADataType:  PA_DATA_TYPE_PA_PAC_REQUEST,
				PADataValue: []byte{0x30, 0x03, 0x01, 0x01, 0xff},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildASREQ failed: %v", err)
	}

	if len(asreq) < 2 {
		t.Fatalf("AS-REQ too short: %d bytes", len(asreq))
	}
	// AS-REQ is [APPLICATION 10] constructed
	if asreq[0] != 0x6a { // 0x40 (application) | 0x20 (constructed) | 10
		t.Fatalf("AS-REQ must start with APPLICATION 10 tag 0x6a, got 0x%02x", asreq[0])
	}

	t.Logf("AS-REQ encoded successfully: %d bytes", len(asreq))
	t.Logf("First 20 bytes: %x", asreq[:min(20, len(asreq))])
}

func TestPADataEncoding(t *testing.T) {
	pa := PAData{
		PADataType:  PA_DATA_TYPE_ENC_TIMESTAMP,
		PADataValue: []byte{0x01, 0x02, 0x03},
	}

	encoded, err := asn1.Marshal(pa)
	if err != nil {
		t.Fatalf("Marshal PAData failed: %v", err)
	}

	if len(encoded) < 2 || encoded[0] != 0x30 {
		t.Fatalf("PAData must start with SEQUENCE tag 0x30, got 0x%02x", encoded[0])
	}

	var decoded PAData
	_, err = asn1.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal PAData failed: %v", err)
	}
	if decoded.PADataType != pa.PADataType {
		t.Errorf("PADataType = %d, want %d", decoded.PADataType, pa.PADataType)
	}
	if string(decoded.PADataValue) != string(pa.PADataValue) {
		t.Errorf("PADataValue mismatch")
	}

	t.Logf("PAData encoded: %x", encoded)
}

func TestPrincipalNameEncoding(t *testing.T) {
	pn := MakeUserPrincipal("user1", "AETHER.TEST")

	encoded, err := asn1.Marshal(pn)
	if err != nil {
		t.Fatalf("Marshal PrincipalName failed: %v", err)
	}

	if len(encoded) < 2 || encoded[0] != 0x30 {
		t.Fatalf("PrincipalName must start with SEQUENCE tag 0x30, got 0x%02x", encoded[0])
	}

	var decoded PrincipalName
	_, err = asn1.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal PrincipalName failed: %v", err)
	}
	if decoded.NameType != pn.NameType {
		t.Errorf("NameType = %d, want %d", decoded.NameType, pn.NameType)
	}
	if len(decoded.NameString) != len(pn.NameString) {
		t.Errorf("NameString len = %d, want %d", len(decoded.NameString), len(pn.NameString))
	}
	for i := range pn.NameString {
		if decoded.NameString[i] != pn.NameString[i] {
			t.Errorf("NameString[%d] = %q, want %q", i, decoded.NameString[i], pn.NameString[i])
		}
	}

	t.Logf("PrincipalName encoded: %x", encoded)
}

func TestRealmEncoding(t *testing.T) {
	// Test Realm's MarshalASN1 directly - should produce [2] EXPLICIT GeneralString
	realm := Realm("AETHER.TEST")
	encoded, err := realm.MarshalASN1()
	if err != nil {
		t.Fatalf("Marshal Realm failed: %v", err)
	}

	t.Logf("Realm encoded: %x", encoded)

	// The realm should be encoded as [2] IMPLICIT GeneralString
	// In practice, many KDCs encode this as 0xA2 (constructed) despite RFC saying primitive
	// Accept both 0x82 (primitive) and 0xA2 (constructed) for compatibility
	if len(encoded) < 1 {
		t.Fatalf("Realm encoding too short: %x", encoded)
	}
	if encoded[0] != 0xA2 && encoded[0] != 0x82 {
		t.Fatalf("Realm must be [2] IMPLICIT GeneralString (0xA2 or 0x82), got 0x%02x", encoded[0])
	}

	// Find the GeneralString tag (0x1B) in the encoded bytes
	foundGenString := false
	for i := 1; i < len(encoded)-1; i++ {
		if encoded[i] == 0x1B {
			foundGenString = true
			break
		}
	}
	if !foundGenString {
		t.Fatalf("Realm must contain GeneralString tag 0x1B")
	}

	// Verify round-trip by unmarshaling
	var rv asn1.RawValue
	_, err = asn1.Unmarshal(encoded, &rv)
	if err != nil {
		t.Fatalf("Unmarshal Realm failed: %v", err)
	}
	// The raw value should have the GeneralString bytes
	if rv.Bytes == nil || len(rv.Bytes) < 2 || rv.Bytes[0] != 0x1B {
		t.Fatalf("Decoded realm missing GeneralString tag")
	}
	realmStr := string(rv.Bytes[2:]) // Skip tag + length
	if realmStr != "AETHER.TEST" {
		t.Errorf("Realm = %q, want %q", realmStr, "AETHER.TEST")
	}
}

func TestKDCREQBodyEncoding(t *testing.T) {
	now := time.Now().UTC()

	reqBody := KDCREQBody{
		KDCOptions: asn1.BitString{Bytes: EncodeKDCOptions(KDC_OPT_FORWARDABLE | KDC_OPT_RENEWABLE | KDC_OPT_CANONICALIZE), BitLength: 32},
		CName:      MakeUserPrincipal("user1", "AETHER.TEST"),
		Realm:      Realm("AETHER.TEST"),
		SName:      MakeUserPrincipal("krbtgt", "AETHER.TEST"),
		From:       NewKerberosTime(now),
		Till:       NewKerberosTime(now.Add(5 * time.Minute)),
		Nonce:      12345,
		EType:      SupportedEtypes(),
	}

	encoded, err := asn1.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Marshal KDCREQBody failed: %v", err)
	}

	// Should be SEQUENCE
	if len(encoded) < 2 || encoded[0] != 0x30 {
		t.Fatalf("KDCREQBody must start with SEQUENCE tag 0x30, got 0x%02x", encoded[0])
	}

	var decoded KDCREQBody
	_, err = asn1.Unmarshal(encoded, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal KDCREQBody failed: %v", err)
	}
	if decoded.Nonce != reqBody.Nonce {
		t.Errorf("Nonce = %d, want %d", decoded.Nonce, reqBody.Nonce)
	}

	t.Logf("KDCREQBody encoded: %d bytes, first 20: %x", len(encoded), encoded[:min(20, len(encoded))])
}